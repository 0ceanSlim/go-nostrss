// File: main.go
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/gorilla/websocket"
	"github.com/mmcdole/gofeed"
	"gopkg.in/yaml.v3"
)

// Config represents the YAML configuration structure
type Config struct {
	RSSFeed            string `yaml:"rss_feed"`
	NostrPrivateKey    string `yaml:"nostr_private_key"`
	NostrPublicKey     string `yaml:"nostr_public_key"` // Added public key
	RelayURL           string `yaml:"relay_url"`
	FetchIntervalMins  int    `yaml:"fetch_interval_minutes"`
	CacheFile          string `yaml:"cache_file"`
}


// NostrEvent represents a Nostr event
type NostrEvent struct {
	ID        string     `json:"id"`
	Pubkey    string     `json:"pubkey"`
	CreatedAt int64      `json:"created_at"`
	Kind      int        `json:"kind"`
	Tags      [][]string `json:"tags"`
	Content   string     `json:"content"`
	Sig       string     `json:"sig"`
}

// Cache structure to hold posted article links
type Cache struct {
	PostedLinks map[string]bool `json:"posted_links"`
	mu          sync.Mutex
}

// LoadConfig loads the configuration from config.yml
func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	return &config, err
}

// LoadCache loads the cache from the cache file
func LoadCache(filename string) (*Cache, error) {
	cache := &Cache{PostedLinks: make(map[string]bool)}

	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			// Return an empty cache if the file doesn't exist
			return cache, nil
		}
		return nil, err
	}

	err = json.Unmarshal(data, cache)
	return cache, err
}

// SaveCache saves the cache to the cache file
func SaveCache(filename string, cache *Cache) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

// FetchRSSFeed fetches and parses the RSS feed
func FetchRSSFeed(url string) ([]*gofeed.Item, error) {
	parser := gofeed.NewParser()
	feed, err := parser.ParseURL(url)
	if err != nil {
		return nil, err
	}
	return feed.Items, nil
}

// CreateNostrEvent creates a Nostr event with the given content and public key
func CreateNostrEvent(content, pubkey string) (*NostrEvent, error) {
	event := &NostrEvent{
		Pubkey:    pubkey,
		CreatedAt: time.Now().Unix(),
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
func SerializeEventForID(event NostrEvent) (string, error) {
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
func SignAndSendEvent(event *NostrEvent, privKeyHex, relayURL string) error {
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
func SendEvent(relayURL string, event NostrEvent) error {
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

func main() {
	// Load configuration
	config, err := LoadConfig("config.yml")
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	// Load cache
	cache, err := LoadCache(config.CacheFile)
	if err != nil {
		log.Fatalf("Error loading cache: %v", err)
	}

	// Main loop
	ticker := time.NewTicker(time.Duration(config.FetchIntervalMins) * time.Minute)
	defer ticker.Stop()

	for {
		items, err := FetchRSSFeed(config.RSSFeed)
		if err != nil {
			log.Printf("Error fetching RSS feed: %v", err)
			continue
		}

		for _, item := range items {
			cache.mu.Lock()
			alreadyPosted := cache.PostedLinks[item.Link]
			cache.mu.Unlock()

			if alreadyPosted {
				continue
			}

			// In the main function, ensure the public key is passed into CreateNostrEvent:
			event, err := CreateNostrEvent(item.Link, config.NostrPublicKey)
			if err != nil {
				log.Printf("Error creating Nostr event: %v", err)
				continue
}

			err = SignAndSendEvent(event, config.NostrPrivateKey, config.RelayURL)
			if err != nil {
				log.Printf("Error sending Nostr event: %v", err)
				continue
			}

			cache.mu.Lock()
			cache.PostedLinks[item.Link] = true
			cache.mu.Unlock()

			log.Printf("Posted event: %s", event.ID)
		}

		err = SaveCache(config.CacheFile, cache)
		if err != nil {
			log.Printf("Error saving cache: %v", err)
		}

		<-ticker.C
	}
}
