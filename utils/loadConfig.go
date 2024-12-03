package utils

import (
	"go-nostrss/types"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadConfig loads the configuration from config.yml
func LoadConfig(filename string) (*types.Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config types.Config
	err = yaml.Unmarshal(data, &config)
	return &config, err
}
