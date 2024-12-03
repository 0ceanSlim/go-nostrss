package utils

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"go-nostrss/types"
)

// LoadCache loads the cache from the cache file
func LoadCache(filename string) (*types.Cache, error) {
	log.Printf("Loading cache from file: %s", filename)

	cache := &types.Cache{PostedLinks: make(map[string]bool)}

	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Cache file not found, initializing empty cache")
			return cache, nil
		}
		log.Printf("Error reading cache file: %v", err)
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	err = json.Unmarshal(data, cache)
	if err != nil {
		log.Printf("Cache file is invalid or corrupted, reinitializing empty cache")
		return &types.Cache{PostedLinks: make(map[string]bool)}, nil
	}

	log.Printf("Cache loaded successfully: %d items", len(cache.PostedLinks))
	return cache, nil
}

// SaveCache saves the cache to the cache file
func SaveCache(filename string, cache *types.Cache) error {
	log.Printf("Saving cache to file: %s", filename)

	cache.Mu.Lock()
	log.Println("Cache lock acquired for saving")
	defer func() {
		cache.Mu.Unlock()
		log.Println("Cache lock released after saving")
	}()

	data, err := json.Marshal(cache)
	if err != nil {
		log.Printf("Error serializing cache: %v", err)
		return fmt.Errorf("failed to serialize cache: %w", err)
	}

	_ = os.Rename(filename, filename+".bak") // Optional backup
	log.Println("Backup created for cache file")

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		log.Printf("Error writing cache to file: %v", err)
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	log.Println("Cache saved successfully")
	return nil
}