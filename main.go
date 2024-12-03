package main

import (
	"encoding/json"

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
