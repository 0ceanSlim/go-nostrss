package nostr

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"go-nostrss/types"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/gorilla/websocket"
)

// CreateNostrEvent creates a Nostr event with the given content, public key and tags
func CreateNostrEvent(content, pubkey string, createdAt int64, tags [][]string) (*types.NostrEvent, error) {
	if tags == nil {
		tags = [][]string{}
	}

	event := &types.NostrEvent{
		Pubkey:    pubkey,
		CreatedAt: createdAt, // Use the provided timestamp
		Kind:      1,
		Content:   content,
		Tags:      tags,
	}

	eventStr, err := SerializeEventForID(*event)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize event for ID: %w", err)
	}

	event.ID = ComputeEventID(eventStr)
	return event, nil
}

// escapeJSONString escapes a string according to NIP-01 requirements
func escapeJSONString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\") // backslash first
	s = strings.ReplaceAll(s, "\"", "\\\"") // double quote
	s = strings.ReplaceAll(s, "\n", "\\n")  // line break
	s = strings.ReplaceAll(s, "\r", "\\r")  // carriage return
	s = strings.ReplaceAll(s, "\t", "\\t")  // tab
	s = strings.ReplaceAll(s, "\b", "\\b")  // backspace
	s = strings.ReplaceAll(s, "\f", "\\f")  // form feed
	return s
}

// SerializeEventForID serializes the event into the format required by NIP-01 for ID computation
func SerializeEventForID(event types.NostrEvent) (string, error) {
	// Manually construct JSON to ensure NIP-01 compliance
	var tagsJSON strings.Builder
	tagsJSON.WriteString("[")
	for i, tag := range event.Tags {
		if i > 0 {
			tagsJSON.WriteString(",")
		}
		tagsJSON.WriteString("[")
		for j, tagValue := range tag {
			if j > 0 {
				tagsJSON.WriteString(",")
			}
			tagsJSON.WriteString(`"` + escapeJSONString(tagValue) + `"`)
		}
		tagsJSON.WriteString("]")
	}
	tagsJSON.WriteString("]")

	// Build the complete serialized event array
	serializedEvent := fmt.Sprintf(`[0,"%s",%d,%d,%s,"%s"]`,
		event.Pubkey,
		event.CreatedAt,
		event.Kind,
		tagsJSON.String(),
		escapeJSONString(event.Content),
	)

	return serializedEvent, nil
}

// ComputeEventID computes the ID for a given event
func ComputeEventID(serializedEvent string) string {
	hash := sha256.Sum256([]byte(serializedEvent))
	return hex.EncodeToString(hash[:])
}

// PublishResult records the outcome of sending an event to a single relay.
type PublishResult struct {
	Relay    string
	Accepted bool
	Message  string
	Err      error
}

// SignAndPublish signs the event once and sends the identical signed event to
// every relay concurrently. It returns one result per relay in the order given;
// the caller decides how many acceptances count as success.
func SignAndPublish(event *types.NostrEvent, privKeyHex string, relays []string) ([]PublishResult, error) {
	if len(relays) == 0 {
		return nil, fmt.Errorf("no relays configured")
	}

	privKeyBytes, err := hex.DecodeString(privKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode private key: %w", err)
	}

	privKey, _ := btcec.PrivKeyFromBytes(privKeyBytes)
	sig, err := SignEventSchnorr(event.ID, privKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign event: %w", err)
	}
	event.Sig = sig

	results := make([]PublishResult, len(relays))
	var wg sync.WaitGroup

	for i, relay := range relays {
		wg.Add(1)
		go func(i int, relay string) {
			defer wg.Done()

			accepted, message, err := SendEvent(relay, *event)
			results[i] = PublishResult{Relay: relay, Accepted: accepted, Message: message, Err: err}
		}(i, relay)
	}

	wg.Wait()
	return results, nil
}

// SignEventSchnorr signs the event ID using Schnorr signatures
func SignEventSchnorr(eventID string, privKey *btcec.PrivateKey) (string, error) {
	idBytes, err := hex.DecodeString(eventID)
	if err != nil {
		return "", fmt.Errorf("failed to decode event ID: %w", err)
	}

	sig, err := schnorr.Sign(privKey, idBytes)
	if err != nil {
		return "", fmt.Errorf("failed to sign event with Schnorr: %w", err)
	}

	return hex.EncodeToString(sig.Serialize()), nil
}

// RelayTimeout bounds the whole publish exchange with one relay. Without it a
// relay that accepts the connection and then goes quiet blocks the fetch loop
// indefinitely.
const RelayTimeout = 15 * time.Second

// SendEvent sends the event to a Nostr relay via WebSocket and waits for the
// NIP-01 OK response. It reports whether the relay accepted the event, along
// with any message the relay attached.
//
// A relay can accept the connection and still reject the event ("rate-limited",
// "blocked: pubkey not allowed"). Reporting that rejection lets the caller keep
// the item out of the cache so the next cycle retries it.
func SendEvent(relayURL string, event types.NostrEvent) (bool, string, error) {
	dialer := websocket.Dialer{HandshakeTimeout: RelayTimeout}

	ws, _, err := dialer.Dial(relayURL, nil)
	if err != nil {
		return false, "", fmt.Errorf("error connecting to Nostr relay: %w", err)
	}
	defer ws.Close()

	deadline := time.Now().Add(RelayTimeout)
	_ = ws.SetWriteDeadline(deadline)
	_ = ws.SetReadDeadline(deadline)

	eventJSON, err := json.Marshal([]interface{}{"EVENT", event})
	if err != nil {
		return false, "", fmt.Errorf("failed to serialize event: %w", err)
	}

	if err := ws.WriteMessage(websocket.TextMessage, eventJSON); err != nil {
		return false, "", fmt.Errorf("failed to send event: %w", err)
	}

	// Relays may emit NOTICE or other frames first, so read until the OK that
	// names this event arrives or the deadline expires.
	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			return false, "", fmt.Errorf("failed to read response from relay: %w", err)
		}

		var frame []json.RawMessage
		if err := json.Unmarshal(message, &frame); err != nil || len(frame) < 3 {
			log.Printf("Relay %s sent: %s", relayURL, string(message))
			continue
		}

		var kind string
		if err := json.Unmarshal(frame[0], &kind); err != nil || kind != "OK" {
			log.Printf("Relay %s sent: %s", relayURL, string(message))
			continue
		}

		var id string
		if err := json.Unmarshal(frame[1], &id); err != nil || id != event.ID {
			continue
		}

		var accepted bool
		if err := json.Unmarshal(frame[2], &accepted); err != nil {
			return false, "", fmt.Errorf("malformed OK from relay: %s", string(message))
		}

		var reason string
		if len(frame) > 3 {
			_ = json.Unmarshal(frame[3], &reason)
		}

		return accepted, reason, nil
	}
}
