package utils

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"go-nostrss/types"

	"gopkg.in/yaml.v3"
)

// PromptForInput prompts the user for input and returns the response
func PromptForInput(prompt string) string {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		log.Fatalf("Error reading input: %v", err)
	}
	return strings.TrimSpace(response)
}

// PromptForInt prompts the user for an integer input with validation
func PromptForInt(prompt string) int {
	for {
		input := PromptForInput(prompt)
		value, err := strconv.Atoi(input)
		if err != nil || value <= 0 {
			fmt.Println("Invalid input. Please enter a positive integer.")
			continue
		}
		return value
	}
}

// PromptForList prompts for a comma-separated list and returns the non-empty,
// deduplicated entries.
func PromptForList(prompt string) []string {
	seen := make(map[string]bool)
	var values []string

	for _, value := range strings.Split(PromptForInput(prompt), ",") {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		values = append(values, value)
	}

	return values
}

// SetupConfig initializes the configuration through user input
func SetupConfig(filename string) (*types.Config, error) {
	var config types.Config

	config.RSSFeed = PromptForInput("Enter the URL of the RSS Feed: ")
	config.NostrPrivateKey = PromptForInput("Enter your Private Key in Hex Format: ")
	config.NostrPublicKey = PromptForInput("Enter your Public Key in Hex Format: ")

	for len(config.RelayURLs) == 0 {
		config.RelayURLs = PromptForList(
			"Enter Relay URLs, comma separated (e.g., wss://relay.damus.io, wss://nos.lol): ")
		if len(config.RelayURLs) == 0 {
			fmt.Println("At least one relay is required.")
		}
	}

	config.FetchIntervalMins = PromptForInt("Enter the Fetch Interval in minutes: ")

	// Defaults, written out explicitly so the file documents what is in effect.
	config.CacheFile = "posted_articles.json"
	config.MessageFormat = "{{ .Title }}\n{{ .Thumb }}\n{{ .Link }}"
	retention := types.DefaultCacheRetentionDays
	maxEntries := types.DefaultCacheMaxEntries
	config.CacheRetentionDays = &retention
	config.CacheMaxEntries = &maxEntries

	data, err := yaml.Marshal(&config)
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(filename, data, 0644)
	return &config, err
}
