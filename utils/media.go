package utils

import (
	"html"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/mmcdole/gofeed"
)

var (
	imgTagRe  = regexp.MustCompile(`(?i)<img[^>]+src\s*=\s*["']([^"']+)["']`)
	hrefTagRe = regexp.MustCompile(`(?i)<a[^>]+href\s*=\s*["']([^"']+)["']`)
)

// imageExts maps the file extensions nostr clients recognize to their MIME type.
// Clients decide whether to embed a URL by looking at the extension on the end
// of it, so anything not in this table gets rendered as a plain link.
var imageExts = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".avif": "image/avif",
	".bmp":  "image/bmp",
}

// placeholderThumbs are the sentinel values Reddit puts in media:thumbnail when
// a post has no real image of its own.
var placeholderThumbs = map[string]bool{
	"self": true, "default": true, "nsfw": true, "spoiler": true, "image": true,
}

// lowResHosts serve feed-generated thumbnails rather than the original upload.
// Worth posting only when nothing better turned up.
var lowResHosts = map[string]bool{
	"a.thumbs.redditmedia.com": true,
	"b.thumbs.redditmedia.com": true,
	"external-preview.redd.it": true,
}

// ImageMIME returns the MIME type for an image URL, or "" if the URL does not
// end in an extension nostr clients will embed.
func ImageMIME(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return imageExts[strings.ToLower(path.Ext(u.Path))]
}

// IsImageURL reports whether a nostr client will render the URL inline.
func IsImageURL(rawURL string) bool { return ImageMIME(rawURL) != "" }

// NormalizeImageURL rewrites host-specific preview URLs to the direct
// full-resolution image.
//
// Reddit serves post previews from preview.redd.it with a signed query string
// (?width=640&auto=webp&s=...). Because clients match on the extension at the
// end of the URL, a preview link is posted as text instead of an image. The
// same upload is available unsigned at i.redd.it under the same media ID, so
// swapping the host and dropping the query gives a URL that both embeds and
// resolves at full size.
//
// external-preview.redd.it is deliberately left alone: those IDs are internal
// hashes for off-site link posts and have no i.redd.it equivalent.
func NormalizeImageURL(rawURL string) string {
	rawURL = html.UnescapeString(strings.TrimSpace(rawURL))
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	if strings.EqualFold(u.Host, "preview.redd.it") && imageExts[strings.ToLower(path.Ext(u.Path))] != "" {
		u.Host = "i.redd.it"
		u.RawQuery = ""
		u.Fragment = ""
	}

	return u.String()
}

// ExtractImageURL finds the best embeddable image for a feed item, preferring
// the original upload over a feed-generated thumbnail. It returns "" when the
// item has no image a client could render.
func ExtractImageURL(item *gofeed.Item) string {
	if item == nil {
		return ""
	}

	var candidates []string

	// A link post pointing straight at an image (i.redd.it, i.imgur.com, ...).
	candidates = append(candidates, item.Link)

	// The rendered HTML body. Reddit's Atom <content> holds the post preview
	// here, which is the only place the full-size image appears.
	for _, body := range []string{item.Content, item.Description} {
		for _, m := range imgTagRe.FindAllStringSubmatch(body, -1) {
			candidates = append(candidates, m[1])
		}
		for _, m := range hrefTagRe.FindAllStringSubmatch(body, -1) {
			candidates = append(candidates, m[1])
		}
	}

	// MRSS media:content / media:thumbnail.
	if media, ok := item.Extensions["media"]; ok {
		for _, key := range []string{"content", "thumbnail"} {
			for _, ext := range media[key] {
				candidates = append(candidates, ext.Attrs["url"])
			}
		}
	}

	for _, enc := range item.Enclosures {
		if enc != nil && strings.HasPrefix(strings.ToLower(enc.Type), "image/") {
			candidates = append(candidates, enc.URL)
		}
	}

	if item.Image != nil {
		candidates = append(candidates, item.Image.URL)
	}

	// First pass takes full-size sources only; second pass accepts thumbnails
	// so a post with nothing but a 140px preview still gets an image.
	for _, allowLowRes := range []bool{false, true} {
		for _, c := range candidates {
			c = strings.TrimSpace(c)
			if c == "" || placeholderThumbs[strings.ToLower(c)] {
				continue
			}

			normalized := NormalizeImageURL(c)
			if !IsImageURL(normalized) {
				continue
			}

			if !allowLowRes {
				if u, err := url.Parse(normalized); err == nil && lowResHosts[strings.ToLower(u.Host)] {
					continue
				}
			}
			return normalized
		}
	}

	return ""
}
