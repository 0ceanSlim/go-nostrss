package nostr

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"

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

// SerializeEventForID serializes the event into the format required by NIP-01 for ID computation
func SerializeEventForID(event types.NostrEvent) (string, error) {
	serializedEvent := []interface{}{
		0,
		event.Pubkey,
		event.CreatedAt,
		event.Kind,
		event.Tags,
		event.Content,
	}

	eventBytes, err := json.Marshal(serializedEvent)
	if err != nil {
		return "", err
	}

	return string(eventBytes), nil
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