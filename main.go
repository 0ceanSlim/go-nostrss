package main

import (
	"log"
	"time"

	"go-nostrss/nostr"
	"go-nostrss/utils"

	"github.com/mmcdole/gofeed"
)

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
	cache, err := utils.LoadCache(config.CacheFile)
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
			cache.Mu.Lock()
			alreadyPosted := cache.PostedLinks[item.Link]
			cache.Mu.Unlock()

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

			cache.Mu.Lock()
			cache.PostedLinks[item.Link] = true
			cache.Mu.Unlock()

			log.Printf("Posted event: %s", event.ID)
		}

		err = utils.SaveCache(config.CacheFile, cache)
		if err != nil {
			log.Printf("Error saving cache: %v", err)
		}

		<-ticker.C
	}
}
