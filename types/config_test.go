package types

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRelaysMergesLegacyField(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   []string
	}{
		{
			name:   "legacy relay_url only",
			config: Config{RelayURL: "wss://relay.example.com"},
			want:   []string{"wss://relay.example.com"},
		},
		{
			name:   "relay_urls only",
			config: Config{RelayURLs: []string{"wss://a.com", "wss://b.com"}},
			want:   []string{"wss://a.com", "wss://b.com"},
		},
		{
			name: "legacy field comes first and duplicates collapse",
			config: Config{
				RelayURL:  "wss://a.com",
				RelayURLs: []string{"wss://b.com", "wss://a.com"},
			},
			want: []string{"wss://a.com", "wss://b.com"},
		},
		{
			name:   "blanks are dropped",
			config: Config{RelayURL: "  ", RelayURLs: []string{"", "  wss://a.com  "}},
			want:   []string{"wss://a.com"},
		},
		{
			name:   "nothing configured",
			config: Config{},
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.Relays(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Relays() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPruningDefaults(t *testing.T) {
	var unset Config
	if got := unset.RetentionDays(); got != DefaultCacheRetentionDays {
		t.Errorf("unset RetentionDays() = %d, want %d", got, DefaultCacheRetentionDays)
	}
	if got := unset.MaxCacheEntries(); got != DefaultCacheMaxEntries {
		t.Errorf("unset MaxCacheEntries() = %d, want %d", got, DefaultCacheMaxEntries)
	}

	// An explicit 0 disables the rule and must not fall back to the default.
	zero := 0
	off := Config{CacheRetentionDays: &zero, CacheMaxEntries: &zero}
	if got := off.RetentionDays(); got != 0 {
		t.Errorf("explicit 0 RetentionDays() = %d, want 0", got)
	}
	if got := off.MaxCacheEntries(); got != 0 {
		t.Errorf("explicit 0 MaxCacheEntries() = %d, want 0", got)
	}
}

func TestConfigYAMLBackwardCompatible(t *testing.T) {
	// A v1.1-era config file must still load and produce a usable relay list.
	const old = `
rss_feed: "https://www.reddit.com/r/greentext/.rss"
nostr_private_key: "abc"
nostr_public_key: "def"
relay_url: "wss://relay.example.com"
fetch_interval_minutes: 5
cache_file: "posted_articles.json"
`
	var config Config
	if err := yaml.Unmarshal([]byte(old), &config); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got := config.Relays(); !reflect.DeepEqual(got, []string{"wss://relay.example.com"}) {
		t.Errorf("Relays() = %v", got)
	}
	if config.RetentionDays() != DefaultCacheRetentionDays {
		t.Error("an old config should pick up the default retention")
	}
}

func TestConfigYAMLParsesPruningFields(t *testing.T) {
	const cfg = `
relay_urls:
  - "wss://a.com"
  - "wss://b.com"
cache_retention_days: 0
cache_max_entries: 100
`
	var config Config
	if err := yaml.Unmarshal([]byte(cfg), &config); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if config.CacheRetentionDays == nil || *config.CacheRetentionDays != 0 {
		t.Error("explicit cache_retention_days: 0 should parse as a set zero")
	}
	if config.MaxCacheEntries() != 100 {
		t.Errorf("MaxCacheEntries() = %d, want 100", config.MaxCacheEntries())
	}
	if len(config.Relays()) != 2 {
		t.Errorf("Relays() = %v", config.Relays())
	}
}
