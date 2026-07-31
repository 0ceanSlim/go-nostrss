package types

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPostedLinksUnmarshalLegacyFormat(t *testing.T) {
	// The pre-v1.2 file recorded a bare bool with no timestamp.
	var links PostedLinks
	if err := json.Unmarshal([]byte(`{"a":true,"b":false,"c":1700000000}`), &links); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if _, ok := links["a"]; !ok {
		t.Error("legacy true entry should be retained")
	}
	if links["a"] != LegacyTimestamp {
		t.Errorf("legacy entry ts = %d, want %d", links["a"], LegacyTimestamp)
	}
	if _, ok := links["b"]; ok {
		t.Error("legacy false entry should be dropped")
	}
	if links["c"] != 1700000000 {
		t.Errorf("timestamped entry = %d, want 1700000000", links["c"])
	}
}

func TestPostedLinksUnmarshalRejectsGarbage(t *testing.T) {
	var links PostedLinks
	if err := json.Unmarshal([]byte(`{"a":{"nested":1}}`), &links); err == nil {
		t.Error("expected an error for an unexpected entry value")
	}
}

func TestPostedLinksRoundTrip(t *testing.T) {
	cache := NewCache()
	now := time.Unix(1700000000, 0)
	cache.Add("https://example.com/1", now)

	data, err := json.Marshal(cache)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	restored := NewCache()
	if err := json.Unmarshal(data, restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if restored.PostedLinks["https://example.com/1"] != now.Unix() {
		t.Errorf("round trip lost the timestamp: %v", restored.PostedLinks)
	}
}

func TestRestampLegacy(t *testing.T) {
	now := time.Unix(1700000000, 0)
	cache := &Cache{PostedLinks: PostedLinks{"old": LegacyTimestamp, "new": 123}}

	if n := cache.RestampLegacy(now); n != 1 {
		t.Errorf("restamped %d entries, want 1", n)
	}
	if cache.PostedLinks["old"] != now.Unix() {
		t.Errorf("legacy entry not restamped: %d", cache.PostedLinks["old"])
	}
	if cache.PostedLinks["new"] != 123 {
		t.Error("already-timestamped entry should be untouched")
	}
}

func TestPruneByAge(t *testing.T) {
	now := time.Unix(1700000000, 0)
	cache := &Cache{PostedLinks: PostedLinks{
		"fresh":      now.Add(-1 * time.Hour).Unix(),
		"borderline": now.Add(-29 * 24 * time.Hour).Unix(),
		"stale":      now.Add(-31 * 24 * time.Hour).Unix(),
	}}

	removed := cache.Prune(now, 30*24*time.Hour, 0)
	if removed != 1 {
		t.Errorf("removed %d, want 1", removed)
	}
	if _, ok := cache.PostedLinks["stale"]; ok {
		t.Error("stale entry should have been pruned")
	}
	if len(cache.PostedLinks) != 2 {
		t.Errorf("remaining = %d, want 2", len(cache.PostedLinks))
	}
}

func TestPruneByMaxEntriesEvictsOldest(t *testing.T) {
	now := time.Unix(1700000000, 0)
	cache := &Cache{PostedLinks: PostedLinks{
		"oldest": 100,
		"middle": 200,
		"newest": 300,
	}}

	removed := cache.Prune(now, 0, 2)
	if removed != 1 {
		t.Errorf("removed %d, want 1", removed)
	}
	if _, ok := cache.PostedLinks["oldest"]; ok {
		t.Error("oldest entry should have been evicted first")
	}
	if _, ok := cache.PostedLinks["newest"]; !ok {
		t.Error("newest entry should have been kept")
	}
}

func TestPruneDisabled(t *testing.T) {
	now := time.Unix(1700000000, 0)
	cache := &Cache{PostedLinks: PostedLinks{"ancient": 1, "old": 2}}

	if removed := cache.Prune(now, 0, 0); removed != 0 {
		t.Errorf("removed %d with pruning disabled, want 0", removed)
	}
	if len(cache.PostedLinks) != 2 {
		t.Errorf("remaining = %d, want 2", len(cache.PostedLinks))
	}
}

func TestPruneAgeThenCap(t *testing.T) {
	now := time.Unix(1700000000, 0)
	cache := &Cache{PostedLinks: PostedLinks{
		"stale": now.Add(-40 * 24 * time.Hour).Unix(),
		"a":     now.Add(-3 * time.Hour).Unix(),
		"b":     now.Add(-2 * time.Hour).Unix(),
		"c":     now.Add(-1 * time.Hour).Unix(),
	}}

	// Age drops "stale", then the cap of 2 evicts the oldest survivor "a".
	if removed := cache.Prune(now, 30*24*time.Hour, 2); removed != 2 {
		t.Errorf("removed %d, want 2", removed)
	}
	for _, want := range []string{"b", "c"} {
		if _, ok := cache.PostedLinks[want]; !ok {
			t.Errorf("expected %q to survive, have %v", want, cache.PostedLinks)
		}
	}
}

func TestHasAndAdd(t *testing.T) {
	cache := NewCache()
	if cache.Has("https://example.com") {
		t.Error("empty cache should not report a hit")
	}

	cache.Add("https://example.com", time.Unix(1700000000, 0))
	if !cache.Has("https://example.com") {
		t.Error("added link should report a hit")
	}
	if cache.Len() != 1 {
		t.Errorf("Len = %d, want 1", cache.Len())
	}
}
