package feeds

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/mmcdole/gofeed"
	"github.com/pelagornis/communityos/internal/security"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Item struct {
	GUID, URL, Title string
	Published        time.Time
}
type Result struct {
	Items              []Item
	ETag, LastModified string
	NotModified        bool
}
type Client struct {
	http   *http.Client
	parser *gofeed.Parser
}

func New() *Client {
	c := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return fmt.Errorf("too many redirects")
		}
		_, e := security.SafeURL(r.Context(), r.URL.String())
		return e
	}}
	p := gofeed.NewParser()
	p.Client = c
	return &Client{http: c, parser: p}
}
func (c *Client) Fetch(ctx context.Context, raw, etag, modified string) (Result, error) {
	u, e := security.SafeURL(ctx, raw)
	if e != nil {
		return Result{}, e
	}
	r, e := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if e != nil {
		return Result{}, e
	}
	r.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml;q=0.9")
	if etag != "" {
		r.Header.Set("If-None-Match", etag)
	}
	if modified != "" {
		r.Header.Set("If-Modified-Since", modified)
	}
	resp, e := c.http.Do(r)
	if e != nil {
		return Result{}, e
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		return Result{NotModified: true, ETag: etag, LastModified: modified}, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Result{}, fmt.Errorf("feed HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > 2<<20 {
		return Result{}, fmt.Errorf("feed exceeds 2MiB")
	}
	body := io.LimitReader(resp.Body, 2<<20)
	f, e := c.parser.Parse(body)
	if e != nil {
		return Result{}, e
	}
	out := Result{ETag: resp.Header.Get("ETag"), LastModified: resp.Header.Get("Last-Modified")}
	for _, v := range f.Items {
		guid := Key(v.GUID, v.Link, v.Title)
		if guid == "" {
			continue
		}
		p := time.Time{}
		if v.PublishedParsed != nil {
			p = *v.PublishedParsed
		}
		out.Items = append(out.Items, Item{GUID: guid, URL: canonical(v.Link), Title: strings.TrimSpace(v.Title), Published: p})
	}
	return out, nil
}
func Key(guid, link, title string) string {
	if guid != "" {
		return strings.TrimSpace(guid)
	}
	if link != "" {
		return canonical(link)
	}
	if title == "" {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(title)))
}
func canonical(raw string) string {
	u, e := url.Parse(raw)
	if e != nil {
		return raw
	}
	u.Fragment = ""
	u.RawQuery = ""
	return u.String()
}

var _ = io.EOF
