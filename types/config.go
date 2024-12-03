package types

// Config represents the YAML configuration structure
type Config struct {
	RSSFeed            string `yaml:"rss_feed"`
	NostrPrivateKey    string `yaml:"nostr_private_key"`
	NostrPublicKey     string `yaml:"nostr_public_key"` // Added public key
	RelayURL           string `yaml:"relay_url"`
	FetchIntervalMins  int    `yaml:"fetch_interval_minutes"`
	CacheFile          string `yaml:"cache_file"`
}