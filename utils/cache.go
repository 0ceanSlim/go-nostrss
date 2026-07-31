package utils

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"go-nostrss/types"
)

// LoadCache loads the cache from the cache file
func LoadCache(filename string) (*types.Cache, error) {
	log.Printf("Loading cache from file: %s", filename)

	cache := types.NewCache()

	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Cache file not found, initializing empty cache")
			return cache, nil
		}
		log.Printf("Error reading cache file: %v", err)
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	if err := json.Unmarshal(data, cache); err != nil {
		log.Printf("Cache file is invalid or corrupted, reinitializing empty cache")
		return types.NewCache(), nil
	}

	if cache.PostedLinks == nil {
		cache.PostedLinks = make(types.PostedLinks)
	}

	// Entries carried over from the old bool-valued format have no timestamp.
	// Stamp them now so they age out normally instead of being pruned at once.
	if n := cache.RestampLegacy(time.Now()); n > 0 {
		log.Printf("Migrated %d cache entries from the legacy format", n)
	}

	log.Printf("Cache loaded successfully: %d items", cache.Len())
	return cache, nil
}

// SaveCache writes the cache to disk atomically: it renders to a temp file in
// the same directory and renames it over the target. A crash partway through a
// plain write would otherwise leave truncated JSON, which LoadCache treats as
// corrupt and replaces with an empty cache, causing the whole feed to be
// reposted.
func SaveCache(filename string, cache *types.Cache) error {
	cache.Mu.Lock()
	data, err := json.Marshal(cache)
	count := len(cache.PostedLinks)
	cache.Mu.Unlock()

	if err != nil {
		log.Printf("Error serializing cache: %v", err)
		return fmt.Errorf("failed to serialize cache: %w", err)
	}

	dir := filepath.Dir(filename)
	tmp, err := os.CreateTemp(dir, filepath.Base(filename)+".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp cache file: %w", err)
	}
	tmpName := tmp.Name()

	// Best-effort cleanup; a no-op once the rename below has succeeded.
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to write temp cache file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to flush temp cache file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temp cache file: %w", err)
	}
	if err := os.Chmod(tmpName, 0644); err != nil {
		return fmt.Errorf("failed to set cache file permissions: %w", err)
	}
	if err := os.Rename(tmpName, filename); err != nil {
		return fmt.Errorf("failed to replace cache file: %w", err)
	}

	log.Printf("Cache saved successfully: %d items", count)
	return nil
}

// PruneCache trims the cache according to the configured retention rules and
// logs what it removed.
func PruneCache(cache *types.Cache, config *types.Config) int {
	maxAge := time.Duration(config.RetentionDays()) * 24 * time.Hour
	maxEntries := config.MaxCacheEntries()

	removed := cache.Prune(time.Now(), maxAge, maxEntries)
	if removed > 0 {
		log.Printf("Pruned %d cache entries (%d remaining)", removed, cache.Len())
	}
	return removed
}
