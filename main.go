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

// buildNote renders the note body for an item and returns it alongside any tags
// the item warrants.
func buildNote(tmpl *template.Template, item *gofeed.Item) (string, [][]string, error) {
	thumbURL := ""
	if item.Image != nil && item.Image.URL != "" {
		thumbURL = strings.TrimSpace(item.Image.URL)
	} else if mediaExt, ok := item.Extensions["media"]; ok {
		if thumbnails, ok := mediaExt["thumbnail"]; ok && len(thumbnails) > 0 {
			if urlAttr, ok := thumbnails[0].Attrs["url"]; ok {
				thumbURL = strings.TrimSpace(urlAttr)
			}
		}
	}

	var buf bytes.Buffer
	err := tmpl.Execute(&buf, struct {
		Title, Link, Thumb string
	}{
		Title: strings.TrimSpace(item.Title),
		Link:  strings.TrimSpace(item.Link),
		Thumb: thumbURL,
	})
	if err != nil {
		return "", nil, err
	}

	return strings.TrimSpace(buf.String()), nil, nil
}

// publish sends the event to every relay and returns how many accepted it.
func publish(event *types.NostrEvent, privKey string, relays []string) int {
	results, err := nostr.SignAndPublish(event, privKey, relays)
	if err != nil {
		log.Printf("Error publishing event: %v", err)
		return 0
	}

	accepted := 0
	for _, r := range results {
		switch {
		case r.Err != nil:
			log.Printf("  %s: failed: %v", r.Relay, r.Err)
		case !r.Accepted:
			log.Printf("  %s: rejected: %s", r.Relay, r.Message)
		default:
			accepted++
		}
	}

	return accepted
}

// processFeed fetches the feed once and publishes every item not already cached.
func processFeed(config *types.Config, tmpl *template.Template, cache *types.Cache, relays []string) {
	items, err := FetchRSSFeed(config.RSSFeed)
	if err != nil {
		log.Printf("Error fetching RSS feed: %v", err)
		return
	}

	for _, item := range items {
		if cache.Has(item.Link) {
			continue
		}

		content, tags, err := buildNote(tmpl, item)
		if err != nil {
			log.Printf("Error executing template: %v", err)
			continue
		}

		createdAt := time.Now().Unix()
		if item.PublishedParsed != nil {
			createdAt = item.PublishedParsed.Unix()
		}

		event, err := nostr.CreateNostrEvent(content, config.NostrPublicKey, createdAt, tags)
		if err != nil {
			log.Printf("Error creating Nostr event: %v", err)
			continue
		}

		// Only cache the item once a relay has actually taken it, so a rejected
		// or unreachable relay leaves the item to be retried next cycle.
		accepted := publish(event, config.NostrPrivateKey, relays)
		if accepted == 0 {
			log.Printf("No relay accepted event for %s, will retry", item.Link)
			continue
		}

		cache.Add(item.Link, time.Now())
		log.Printf("Posted event %s (%d/%d relays)", event.ID, accepted, len(relays))
	}

	utils.PruneCache(cache, config)

	if err := utils.SaveCache(config.CacheFile, cache); err != nil {
		log.Printf("Error saving cache: %v", err)
	}
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
		config.MessageFormat = "{{ .Title }}\n{{ .Thumb }}\n{{ .Link }}"
	}

	tmpl, err := template.New("message_template").Parse(config.MessageFormat)
	if err != nil {
		log.Fatalf("Invalid message_format: %v", err)
	}

	relays := config.Relays()
	if len(relays) == 0 {
		log.Fatalf("No relays configured: set relay_urls (or relay_url) in %s", configFileName)
	}
	log.Printf("Publishing to %d relay(s): %s", len(relays), strings.Join(relays, ", "))

	if config.FetchIntervalMins <= 0 {
		log.Fatalf("fetch_interval_minutes must be greater than 0")
	}

	cache, err := utils.LoadCache(config.CacheFile)
	if err != nil {
		log.Fatalf("Error loading cache: %v", err)
	}
	utils.PruneCache(cache, config)

	ticker := time.NewTicker(time.Duration(config.FetchIntervalMins) * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		processFeed(config, tmpl, cache, relays)
	}
}
