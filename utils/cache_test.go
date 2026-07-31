package utils

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go-nostrss/types"
)

func TestLoadCacheMigratesLegacyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "posted_articles.json")

	// A cache file written by v1.1.
	legacy := `{"posted_links":{"https://example.com/a":true,"https://example.com/b":true}}`
	if err := os.WriteFile(path, []byte(legacy), 0644); err != nil {
		t.Fatal(err)
	}

	cache, err := LoadCache(path)
	if err != nil {
		t.Fatalf("LoadCache: %v", err)
	}

	if cache.Len() != 2 {
		t.Fatalf("Len = %d, want 2 (upgrade must not forget posted items)", cache.Len())
	}
	if !cache.Has("https://example.com/a") {
		t.Error("legacy entry missing after migration")
	}

	// Restamped entries must survive an immediate prune rather than being
	// treated as infinitely old.
	if removed := cache.Prune(time.Now(), 30*24*time.Hour, 0); removed != 0 {
		t.Errorf("migrated entries pruned immediately (%d removed)", removed)
	}
}

func TestLoadCacheMissingFile(t *testing.T) {
	cache, err := LoadCache(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("LoadCache: %v", err)
	}
	if cache.Len() != 0 {
		t.Errorf("Len = %d, want 0", cache.Len())
	}

	// Must be writable, not a nil map.
	cache.Add("https://example.com", time.Now())
	if !cache.Has("https://example.com") {
		t.Error("fresh cache should be writable")
	}
}

func TestLoadCacheCorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")
	if err := os.WriteFile(path, []byte(`{"posted_links":{"a":tr`), 0644); err != nil {
		t.Fatal(err)
	}

	cache, err := LoadCache(path)
	if err != nil {
		t.Fatalf("LoadCache: %v", err)
	}
	if cache.Len() != 0 {
		t.Errorf("Len = %d, want 0", cache.Len())
	}
	cache.Add("x", time.Now()) // must not panic on a nil map
}

func TestSaveCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")

	cache := types.NewCache()
	cache.Add("https://example.com/a", time.Unix(1700000000, 0))

	if err := SaveCache(path, cache); err != nil {
		t.Fatalf("SaveCache: %v", err)
	}

	loaded, err := LoadCache(path)
	if err != nil {
		t.Fatalf("LoadCache: %v", err)
	}
	if !loaded.Has("https://example.com/a") {
		t.Error("saved link missing after reload")
	}

	// The atomic write must not leave temp files lying around.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "cache.json" {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("expected only cache.json, found %v", names)
	}
}

func TestSaveCacheOverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")

	first := types.NewCache()
	first.Add("a", time.Unix(1700000000, 0))
	if err := SaveCache(path, first); err != nil {
		t.Fatalf("first SaveCache: %v", err)
	}

	second := types.NewCache()
	second.Add("b", time.Unix(1700000001, 0))
	if err := SaveCache(path, second); err != nil {
		t.Fatalf("second SaveCache: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out types.Cache
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := out.PostedLinks["a"]; ok {
		t.Error("rename should have fully replaced the previous file")
	}
	if _, ok := out.PostedLinks["b"]; !ok {
		t.Error("second save missing")
	}
}

func TestPruneCacheUsesConfigDefaults(t *testing.T) {
	cache := types.NewCache()
	cache.Add("stale", time.Now().Add(-90*24*time.Hour))
	cache.Add("fresh", time.Now())

	if removed := PruneCache(cache, &types.Config{}); removed != 1 {
		t.Errorf("removed %d, want 1", removed)
	}
	if !cache.Has("fresh") {
		t.Error("fresh entry should survive the default 30 day retention")
	}
}
