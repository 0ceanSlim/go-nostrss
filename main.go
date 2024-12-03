package main

import (
	"encoding/json"
	"fmt"

	"log"
	"os"
	"sync"
	"time"

	"go-nostrss/nostr"
	"go-nostrss/utils"

	"github.com/mmcdole/gofeed"
)

// Cache structure to hold posted article links
type Cache struct {
	PostedLinks map[string]bool `json:"posted_links"`
	mu          sync.Mutex
}

func LoadCache(filename string) (*Cache, error) {
	log.Printf("Loading cache from file: %s", filename) // Debug log

	cache := &Cache{PostedLinks: make(map[string]bool)}

	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Cache file not found, initializing empty cache") // Debug log
			return cache, nil
		}
		log.Printf("Error reading cache file: %v", err) // Debug log
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	err = json.Unmarshal(data, cache)
	if err != nil {
		log.Printf("Cache file is invalid or corrupted, reinitializing empty cache") // Debug log
		return &Cache{PostedLinks: make(map[string]bool)}, nil
	}

	log.Printf("Cache loaded successfully: %d items", len(cache.PostedLinks)) // Debug log
	return cache, nil
}


func SaveCache(filename string, cache *Cache) error {
	log.Printf("Saving cache to file: %s", filename) // Debug log

	cache.mu.Lock()
	log.Println("Cache lock acquired for saving") // Debug log
	defer func() {
		cache.mu.Unlock()
		log.Println("Cache lock released after saving") // Debug log
	}()

	data, err := json.Marshal(cache)
	if err != nil {
		log.Printf("Error serializing cache: %v", err) // Debug log
		return fmt.Errorf("failed to serialize cache: %w", err)
	}

	_ = os.Rename(filename, filename+".bak") // Optional backup
	log.Println("Backup created for cache file") // Debug log

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		log.Printf("Error writing cache to file: %v", err) // Debug log
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	log.Println("Cache saved successfully") // Debug log
	return nil
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

func main() {
	// Load configuration
	config, err := utils.LoadConfig("config.yml")
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
			event, err := nostr.CreateNostrEvent(item.Link, config.NostrPublicKey)
			if err != nil {
				log.Printf("Error creating Nostr event: %v", err)
				continue
}

			err = nostr.SignAndSendEvent(event, config.NostrPrivateKey, config.RelayURL)
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
