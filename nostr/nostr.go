package nostr

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"go-nostrss/types"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/gorilla/websocket"
)

// CreateNostrEvent creates a Nostr event with the given content and public key
func CreateNostrEvent(content, pubkey string, createdAt int64) (*types.NostrEvent, error) {
	event := &types.NostrEvent{
		Pubkey:    pubkey,
		CreatedAt: createdAt, // Use the provided timestamp
		Kind:      1,
		Content:   content,
		Tags:      [][]string{},
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

// SignAndSendEvent signs the event and sends it to the Nostr relay
func SignAndSendEvent(event *types.NostrEvent, privKeyHex, relayURL string) error {
	privKeyBytes, err := hex.DecodeString(privKeyHex)
	if err != nil {
		return fmt.Errorf("failed to decode private key: %w", err)
	}

	privKey, _ := btcec.PrivKeyFromBytes(privKeyBytes)
	sig, err := SignEventSchnorr(event.ID, privKey)
	if err != nil {
		return fmt.Errorf("failed to sign event: %w", err)
	}
	event.Sig = sig

	return SendEvent(relayURL, *event)
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

// SendEvent sends the event to the Nostr relay via WebSocket
func SendEvent(relayURL string, event types.NostrEvent) error {
	ws, _, err := websocket.DefaultDialer.Dial(relayURL, nil)
	if err != nil {
		return fmt.Errorf("error connecting to Nostr relay: %w", err)
	}
	defer ws.Close()

	msg := []interface{}{"EVENT", event}
	eventJSON, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to serialize event: %w", err)
	}

	err = ws.WriteMessage(websocket.TextMessage, eventJSON)
	if err != nil {
		return fmt.Errorf("failed to send event: %w", err)
	}

	_, message, err := ws.ReadMessage()
	if err != nil {
		return fmt.Errorf("failed to read response from relay: %w", err)
	}

	log.Printf("Relay response: %s", string(message))
	return nil
}