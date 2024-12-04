# RSS to Nostr Account Converter

This program converts any RSS feed into a nostr account, allowing you to follow and interact with RSS content through the nostr protocol.

## Features

Convert RSS feeds to nostr accounts  
Automatically publish new RSS entries as Nostr events  
Simple Setup  
Configurable update intervals

## Running the bot

Go the the [releases](https://github.com/0ceanSlim/go-nostrss/releases) and download the latest release  
Run the executable  
If a configuration does not exist the wazard 🧙 will primpt you for input to create it for you  
You will need:  

- Your RSS Feed URL
- The Private Key of the nostr account to post to
- The Public Key of the nostr account to post to
- A nostr Relay URL to send the events to  

You will also be prompted for how often in minutes to fetch the rss feed and check for new stories  

## How it works

The program fetches RSS feeds at configured intervals.  
New entries are converted to Nostr events.  
Events are published to the configured Nostr relay.  
Articles that have been posted are cached in a json to prevent reposting articles.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

### License

This project is Open Source and licensed under the MIT License. See the [LICENSE](license) file for details.
