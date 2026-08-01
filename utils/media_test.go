package utils

import (
	"testing"

	"github.com/mmcdole/gofeed"
	"github.com/mmcdole/gofeed/extensions"
)

func mediaExt(tag, url string) ext.Extensions {
	return ext.Extensions{
		"media": {tag: []ext.Extension{{Name: tag, Attrs: map[string]string{"url": url}}}},
	}
}

func TestNormalizeImageURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "reddit preview is rewritten to the direct upload",
			in:   "https://preview.redd.it/abc123.jpg?width=640&crop=smart&auto=webp&s=deadbeef",
			want: "https://i.redd.it/abc123.jpg",
		},
		{
			name: "direct upload is left alone",
			in:   "https://i.redd.it/abc123.png",
			want: "https://i.redd.it/abc123.png",
		},
		{
			name: "external preview has no i.redd.it equivalent",
			in:   "https://external-preview.redd.it/hash.jpg?auto=webp&s=sig",
			want: "https://external-preview.redd.it/hash.jpg?auto=webp&s=sig",
		},
		{
			name: "html entities are decoded",
			in:   "https://preview.redd.it/abc123.jpg?width=640&amp;auto=webp",
			want: "https://i.redd.it/abc123.jpg",
		},
		{
			name: "non-image reddit preview keeps its query",
			in:   "https://preview.redd.it/abc123?width=640",
			want: "https://preview.redd.it/abc123?width=640",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeImageURL(tt.in); got != tt.want {
				t.Errorf("NormalizeImageURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestImageMIME(t *testing.T) {
	tests := []struct{ in, want string }{
		{"https://i.redd.it/a.jpg", "image/jpeg"},
		{"https://i.redd.it/a.JPEG", "image/jpeg"},
		{"https://i.redd.it/a.png", "image/png"},
		{"https://i.redd.it/a.gif", "image/gif"},
		{"https://i.redd.it/a.webp", "image/webp"},
		{"https://www.reddit.com/r/greentext/comments/x/y/", ""},
		{"https://preview.redd.it/a.jpg?s=sig", "image/jpeg"},
	}

	for _, tt := range tests {
		if got := ImageMIME(tt.in); got != tt.want {
			t.Errorf("ImageMIME(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestExtractImageURL(t *testing.T) {
	tests := []struct {
		name string
		item *gofeed.Item
		want string
	}{
		{
			name: "reddit image post prefers the content preview over media:thumbnail",
			item: &gofeed.Item{
				Link: "https://www.reddit.com/r/greentext/comments/abc/anon_does_a_thing/",
				Content: `<table><tr><td><a href="https://www.reddit.com/r/greentext/comments/abc/anon_does_a_thing/">` +
					`<img src="https://preview.redd.it/xyz789.png?width=640&amp;crop=smart&amp;auto=webp&amp;s=sig" alt="post image"/>` +
					`</a></td><td>submitted by <a href="https://www.reddit.com/user/anon">/u/anon</a> ` +
					`<a href="https://www.reddit.com/r/greentext/comments/abc/">[comments]</a></td></tr></table>`,
				Extensions: mediaExt("thumbnail", "https://b.thumbs.redditmedia.com/tiny.jpg"),
			},
			want: "https://i.redd.it/xyz789.png",
		},
		{
			name: "falls back to the low-res thumbnail when nothing better exists",
			item: &gofeed.Item{
				Link:       "https://www.reddit.com/r/greentext/comments/abc/text_post/",
				Content:    `<div>just some text</div>`,
				Extensions: mediaExt("thumbnail", "https://b.thumbs.redditmedia.com/tiny.jpg"),
			},
			want: "https://b.thumbs.redditmedia.com/tiny.jpg",
		},
		{
			name: "reddit placeholder thumbnails are ignored",
			item: &gofeed.Item{
				Link:       "https://www.reddit.com/r/greentext/comments/abc/self_post/",
				Extensions: mediaExt("thumbnail", "self"),
			},
			want: "",
		},
		{
			name: "direct image link post",
			item: &gofeed.Item{Link: "https://i.redd.it/direct.jpg"},
			want: "https://i.redd.it/direct.jpg",
		},
		{
			name: "anchor href to an image is picked up",
			item: &gofeed.Item{
				Link:    "https://www.reddit.com/r/greentext/comments/abc/link/",
				Content: `<a href="https://i.imgur.com/pic.png">[link]</a>`,
			},
			want: "https://i.imgur.com/pic.png",
		},
		{
			name: "image enclosure",
			item: &gofeed.Item{
				Link:       "https://example.com/post",
				Enclosures: []*gofeed.Enclosure{{URL: "https://cdn.example.com/pic.gif", Type: "image/gif"}},
			},
			want: "https://cdn.example.com/pic.gif",
		},
		{
			name: "text-only item yields no image",
			item: &gofeed.Item{
				Link:    "https://example.com/post",
				Content: `<p>no pictures here</p>`,
			},
			want: "",
		},
		{
			name: "nil item",
			item: nil,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractImageURL(tt.item); got != tt.want {
				t.Errorf("ExtractImageURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
