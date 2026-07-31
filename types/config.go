package types

// Cache pruning defaults, applied when the config leaves the field unset. A
// month comfortably outlasts the window any RSS feed covers, so an item pruned
// at this age can no longer reappear and be reposted.
const (
	DefaultCacheRetentionDays = 30
	DefaultCacheMaxEntries    = 5000
)

// Config represents the YAML configuration structure
type Config struct {
	RSSFeed           string `yaml:"rss_feed"`
	NostrPrivateKey   string `yaml:"nostr_private_key"`
	NostrPublicKey    string `yaml:"nostr_public_key"` // Added public key
	RelayURL          string `yaml:"relay_url"`
	FetchIntervalMins int    `yaml:"fetch_interval_minutes"`
	CacheFile         string `yaml:"cache_file"`
	MessageFormat     string `yaml:"message_format"`

	// Pointers so an unset field is distinguishable from an explicit 0, which
	// disables that pruning rule.
	CacheRetentionDays *int `yaml:"cache_retention_days,omitempty"`
	CacheMaxEntries    *int `yaml:"cache_max_entries,omitempty"`
}

// RetentionDays returns the configured cache retention in days, or the default
// when unset. Zero means "never prune by age".
func (c *Config) RetentionDays() int {
	if c.CacheRetentionDays == nil {
		return DefaultCacheRetentionDays
	}
	return *c.CacheRetentionDays
}

// MaxCacheEntries returns the configured cache size cap, or the default when
// unset. Zero means "no cap".
func (c *Config) MaxCacheEntries() int {
	if c.CacheMaxEntries == nil {
		return DefaultCacheMaxEntries
	}
	return *c.CacheMaxEntries
}
