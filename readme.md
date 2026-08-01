# RSS to Nostr Account Converter

This program converts any RSS feed into a nostr account, allowing you to follow and interact with RSS content through the nostr protocol.

## Features

Convert RSS feeds to nostr accounts  
Automatically publish new RSS entries as Nostr events  
Embeds post images inline instead of linking them  
Publishes to multiple relays  
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
- One or more nostr Relay URLs to send the events to  

You will also be prompted for how often in minutes to fetch the rss feed and check for new stories  

To run more than one feed, run the binary again from a separate working
directory with its own `config.yml` and `cache_file`.

## Relays

`relay_urls` takes a list. The event is signed once and sent to every relay in
parallel:

```yaml
relay_urls:
  - "wss://relay.damus.io"
  - "wss://nos.lol"
```

The older single `relay_url` field still works and is merged into the list, so
an existing config needs no changes.

An item is only recorded as posted once at least one relay returns an `OK`
accepting it. A relay that is unreachable, rate-limiting, or rejecting the
pubkey leaves the item uncached so the next cycle retries it.

## Post formatting

`message_format` in `config.yml` is a Go [text/template](https://pkg.go.dev/text/template)
controlling the note body. Available fields:

| Field | Description |
| --- | --- |
| `.Title` | The feed item title |
| `.Link` | The item's permalink |
| `.Image` | The post's full-size image URL, or `""` if it has none |
| `.Thumb` | Alias for `.Image`, kept so older configs keep working |

Default:

```yaml
message_format: "{{ .Title }}\n{{ with .Image }}{{ . }}\n{{ end }}{{ .Link }}"
```

The `with` block keeps a blank line out of the note when an item has no image.
Use double quotes in YAML, otherwise `\n` stays a literal backslash-n.

### Embedding images instead of linking them

Nostr clients decide whether to inline an image by matching a file extension on
the end of the URL. Reddit's feed points at `preview.redd.it/<id>.png?width=640&auto=webp&s=<sig>`
— the signed query string hides the extension, so clients render it as a plain
link and the image never shows up in the note.

`.Image` resolves this. It pulls the image out of the item's HTML content,
rewrites `preview.redd.it` to the equivalent unsigned `i.redd.it` URL, and drops
the query string, which both restores inline embedding and gets the original
full-size upload rather than the 640px crop. The note also carries a
[NIP-92](https://github.com/nostr-protocol/nips/blob/master/92.md) `imeta` tag so
clients can embed without sniffing the link.

Items with no usable image leave `.Image` empty and no `imeta` tag is added.

## The posted-articles cache

Posted links are recorded in `cache_file` so an item is never published twice.
Each entry stores the unix time it was posted, and the file is pruned on startup
and after every fetch:

```yaml
cache_retention_days: 30   # drop entries older than this
cache_max_entries: 5000    # then evict oldest until at most this many remain
```

Set either to `0` to disable that rule. Omit them entirely and the defaults above
apply.

Retention only has to outlast the window the feed itself covers. A feed shows a
limited number of recent items, so once an entry falls off it can never be seen
again and forgetting it cannot cause a repost — pruning too aggressively is what
causes duplicates, not pruning too little.

The file is written atomically (temp file plus rename), so a crash or container
restart mid-write cannot leave truncated JSON. Cache files from earlier versions
recorded a bare `true` per link; those load and are migrated automatically, so
upgrading does not make the bot forget what it has posted.

## How it works

The program fetches the RSS feed on startup and then at the configured interval.  
New entries are converted to Nostr events.  
Events are published to every configured Nostr relay.  
Articles that have been posted are cached in a json to prevent reposting articles.

## Building from source

```sh
go build -o go-nostrss .
go test ./...
```

Pushing a `v*` tag runs the tests and publishes cross-compiled binaries for
linux, macOS and windows (amd64/arm64) to a GitHub release automatically.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

### License

This project is Open Source and licensed under the MIT License. See the [LICENSE](license) file for details.
