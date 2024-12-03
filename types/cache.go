package types

import (
	"sync"
)

// Cache structure to hold posted article links
type Cache struct {
	PostedLinks map[string]bool `json:"posted_links"`
	Mu          sync.Mutex      `json:"-"` // Ensure this isn't serialized
}