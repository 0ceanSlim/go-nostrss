package types

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

// LegacyTimestamp marks entries read from the pre-v1.2 cache format, which
// recorded a bare `true` and carried no time of its own. LoadCache restamps
// them on read so they get a full retention window instead of being pruned on
// the first pass.
const LegacyTimestamp int64 = 0

// PostedLinks maps a feed item link to the unix time its note was published.
type PostedLinks map[string]int64

// UnmarshalJSON accepts both the current {"link": <unix ts>} form and the
// legacy {"link": true} form, so an existing cache file survives the upgrade
// without the bot forgetting what it has already posted.
func (p *PostedLinks) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	out := make(PostedLinks, len(raw))
	for link, value := range raw {
		var ts int64
		if err := json.Unmarshal(value, &ts); err == nil {
			out[link] = ts
			continue
		}

		var posted bool
		if err := json.Unmarshal(value, &posted); err != nil {
			return fmt.Errorf("cache entry %q: unexpected value %s", link, value)
		}
		if posted {
			out[link] = LegacyTimestamp
		}
	}

	*p = out
	return nil
}

// Cache tracks which feed items have already been published.
type Cache struct {
	PostedLinks PostedLinks `json:"posted_links"`
	Mu          sync.Mutex  `json:"-"` // Ensure this isn't serialized
}

// NewCache returns an empty, ready-to-use cache.
func NewCache() *Cache {
	return &Cache{PostedLinks: make(PostedLinks)}
}

// Has reports whether the link has already been posted.
func (c *Cache) Has(link string) bool {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	_, ok := c.PostedLinks[link]
	return ok
}

// Add records the link as posted at the given time.
func (c *Cache) Add(link string, at time.Time) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	if c.PostedLinks == nil {
		c.PostedLinks = make(PostedLinks)
	}
	c.PostedLinks[link] = at.Unix()
}

// Len returns the number of cached links.
func (c *Cache) Len() int {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	return len(c.PostedLinks)
}

// RestampLegacy gives entries migrated from the old bool format a timestamp, so
// they age out one retention period from now rather than immediately. Returns
// how many were restamped.
func (c *Cache) RestampLegacy(at time.Time) int {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	n := 0
	for link, ts := range c.PostedLinks {
		if ts == LegacyTimestamp {
			c.PostedLinks[link] = at.Unix()
			n++
		}
	}
	return n
}

// Prune drops entries older than maxAge, then removes the oldest remaining
// entries until at most maxEntries are left. A non-positive value disables that
// half of the rule. Returns the number of entries removed.
//
// Retention only needs to outlast the window a feed itself covers: once an item
// falls off the feed it can never be seen again, so forgetting it cannot cause a
// repost. Pruning too aggressively is what causes duplicates.
func (c *Cache) Prune(now time.Time, maxAge time.Duration, maxEntries int) int {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	before := len(c.PostedLinks)

	if maxAge > 0 {
		cutoff := now.Add(-maxAge).Unix()
		for link, ts := range c.PostedLinks {
			if ts < cutoff {
				delete(c.PostedLinks, link)
			}
		}
	}

	if excess := len(c.PostedLinks) - maxEntries; maxEntries > 0 && excess > 0 {
		links := make([]string, 0, len(c.PostedLinks))
		for link := range c.PostedLinks {
			links = append(links, link)
		}

		// Oldest first, ties broken by link so pruning is deterministic.
		sort.Slice(links, func(i, j int) bool {
			if c.PostedLinks[links[i]] != c.PostedLinks[links[j]] {
				return c.PostedLinks[links[i]] < c.PostedLinks[links[j]]
			}
			return links[i] < links[j]
		})

		for _, link := range links[:excess] {
			delete(c.PostedLinks, link)
		}
	}

	return before - len(c.PostedLinks)
}
