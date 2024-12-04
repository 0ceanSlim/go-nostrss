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

// SetupConfig initializes the configuration through user input
func SetupConfig(filename string) (*types.Config, error) {
	var config types.Config

	config.RSSFeed = PromptForInput("Enter the URL of the RSS Feed: ")
	config.NostrPrivateKey = PromptForInput("Enter your Private Key in Hex Format: ")
	config.NostrPublicKey = PromptForInput("Enter your Public Key in Hex Format: ")
	config.RelayURL = PromptForInput("Enter the Relay URL (e.g., wss://relay.example.com): ")
	config.FetchIntervalMins = PromptForInt("Enter the Fetch Interval in minutes: ")
	config.CacheFile = "posted_articles.json" // Default value

	data, err := yaml.Marshal(&config)
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(filename, data, 0644)
	return &config, err
}
