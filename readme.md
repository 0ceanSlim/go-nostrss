# RSS to Nostr Account Converter

This program converts any RSS feed into a Nostr account, allowing you to follow and interact with RSS content through the Nostr protocol.

## Features

Convert RSS feeds to Nostr accounts  
Automatically publish new RSS entries as Nostr events  
Simple Setup  
Configurable update intervals

## Running the bot

Go the the releases and download the latest release (Work in Progress)  
The program will automatically create the configuration with the built in 🧙 wizard if one does not exist

## How it works

The program fetches RSS feeds at configured intervals.  
New entries are converted to Nostr events.  
Events are published to the configured Nostr relay.  
Articles that have been posted are cached in a json to prevent reposting articles.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

### License

This project is licensed under the MIT License.
