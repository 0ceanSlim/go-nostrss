package main

import (
	"log"
	"os"
	"strings"
	"time"

	"go-nostrss/nostr"
	"go-nostrss/types"
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
	const configFileName = "config.yml"

	var config *types.Config
	if _, err := os.Stat(configFileName); os.IsNotExist(err) {
		log.Println("Configuration file not found. Starting setup wizard...")
		var setupErr error
		config, setupErr = utils.SetupConfig(configFileName)
		if setupErr != nil {
			log.Fatalf("Error setting up configuration: %v", setupErr)
		}
	} else {
		var loadErr error
		config, loadErr = utils.LoadConfig(configFileName)
		if loadErr != nil {
			log.Fatalf("Error loading configuration: %v", loadErr)
		}
	}

	cache, err := utils.LoadCache(config.CacheFile)
	if err != nil {
		log.Fatalf("Error loading cache: %v", err)
	}

	ticker := time.NewTicker(time.Duration(config.FetchIntervalMins) * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		items, err := FetchRSSFeed(config.RSSFeed)
		if err != nil {
			log.Printf("Error fetching RSS feed: %v", err)
			continue
		}

		for _, item := range items {
			cache.Mu.Lock()
			if cache.PostedLinks[item.Link] {
				cache.Mu.Unlock()
				continue
			}
			cache.Mu.Unlock()

			content := strings.TrimSpace(item.Title) + "\n" + item.Link

			var createdAt int64
			if item.PublishedParsed != nil {
				createdAt = item.PublishedParsed.Unix()
			} else {
				createdAt = time.Now().Unix()
			}

			event, err := nostr.CreateNostrEvent(content, config.NostrPublicKey, createdAt)
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
	}

}
