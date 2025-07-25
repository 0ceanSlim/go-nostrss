package main

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"text/template"
	"time"

	"go-nostrss/nostr"
	"go-nostrss/types"
	"go-nostrss/utils"

	"github.com/mmcdole/gofeed"
)

// sanitizeXML removes illegal XML characters that can cause parsing errors
func sanitizeXML(data []byte) []byte {
	// Remove control characters except tab, newline, and carriage return
	// XML 1.0 spec allows: #x9 | #xA | #xD | [#x20-#xD7FF] | [#xE000-#xFFFD] | [#x10000-#x10FFFF]
	re := regexp.MustCompile(`[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]`)
	return re.ReplaceAll(data, []byte(""))
}

// FetchRSSFeed fetches and parses the RSS feed with XML sanitization
func FetchRSSFeed(url string) ([]*gofeed.Item, error) {
	// Fetch the RSS feed manually to sanitize it
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read the response body
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Sanitize the XML data
	sanitizedData := sanitizeXML(data)

	// Parse the sanitized XML
	parser := gofeed.NewParser()
	feed, err := parser.ParseString(string(sanitizedData))
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
	if config.MessageFormat == "" {
		config.MessageFormat = "{{ .Title }}\n{{ .Link }}"
	}

	message_template := template.Must(template.New("message_template").Parse(config.MessageFormat))

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

			var content_buf bytes.Buffer
			if err := message_template.Execute(&content_buf,
				struct {
					Title, Link string
				}{
					Title: strings.TrimSpace(item.Title),
					Link:  strings.TrimSpace(item.Link),
				}); err != nil {
				log.Printf("Error executing template: %v", err)
			}

			content := content_buf.String()

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
