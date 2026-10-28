package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type Release struct {
	ID         int64     `json:"id"`
	Tag        string    `json:"tag_name"`
	Name       string    `json:"name"`
	Body       string    `json:"body"`
	URL        string    `json:"html_url"`
	Draft      bool      `json:"draft"`
	Prerelease bool      `json:"prerelease"`
	Published  time.Time `json:"published_at"`
}
type Client struct{ HTTP *http.Client }

var openGraphImage = regexp.MustCompile(`(?is)<meta[^>]+(?:property="og:image"|name="twitter:image")[^>]+content="([^"]+)"`)

func New() *Client { return &Client{HTTP: &http.Client{Timeout: 10 * time.Second}} }

// ReleaseImage reads GitHub's canonical, cache-keyed Open Graph image URL.
// Constructing this URL locally is unreliable because GitHub owns the hash.
func (c *Client) ReleaseImage(ctx context.Context, releaseURL string) (string, error) {
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.HTTP.Do(r)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github release page HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	m := openGraphImage.FindSubmatch(body)
	if len(m) != 2 {
		return "", fmt.Errorf("github release page has no og:image")
	}
	return string(m[1]), nil
}

// Releases returns every published and unpublished Release exposed by GitHub,
// newest first. The first page's ETag avoids pagination when nothing changed.
func (c *Client) Releases(ctx context.Context, repo, etag string) ([]Release, string, bool, error) {
	if !validRepo(repo) {
		return nil, "", false, fmt.Errorf("invalid repository")
	}
	var releases []Release
	var nextETag string
	for page := 1; ; page++ {
		u := fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=100&page=%d", repo, page)
		r, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, "", false, err
		}
		r.Header.Set("Accept", "application/vnd.github+json")
		if page == 1 && etag != "" {
			r.Header.Set("If-None-Match", etag)
		}
		resp, err := c.HTTP.Do(r)
		if err != nil {
			return nil, "", false, err
		}
		if resp.StatusCode == http.StatusNotModified {
			resp.Body.Close()
			return nil, etag, true, nil
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, "", false, fmt.Errorf("github HTTP %d", resp.StatusCode)
		}
		var batch []Release
		err = json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&batch)
		if page == 1 {
			nextETag = resp.Header.Get("ETag")
		}
		resp.Body.Close()
		if err != nil {
			return nil, "", false, err
		}
		releases = append(releases, batch...)
		if len(batch) < 100 {
			return releases, nextETag, false, nil
		}
	}
}

func (c *Client) Latest(ctx context.Context, repo, etag string) (Release, string, bool, error) {
	if !validRepo(repo) {
		return Release{}, "", false, fmt.Errorf("invalid repository")
	}
	r, e := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+repo+"/releases/latest", nil)
	if e != nil {
		return Release{}, "", false, e
	}
	r.Header.Set("Accept", "application/vnd.github+json")
	if etag != "" {
		r.Header.Set("If-None-Match", etag)
	}
	resp, e := c.HTTP.Do(r)
	if e != nil {
		return Release{}, "", false, e
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		return Release{}, etag, true, nil
	}
	if resp.StatusCode != http.StatusOK {
		return Release{}, "", false, fmt.Errorf("github HTTP %d", resp.StatusCode)
	}
	var out Release
	if e = json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&out); e != nil {
		return Release{}, "", false, e
	}
	return out, resp.Header.Get("ETag"), false, nil
}
func validRepo(s string) bool {
	x := strings.Split(s, "/")
	return len(x) == 2 && x[0] != "" && x[1] != "" && !strings.Contains(s, "..")
}
func ValidRepo(s string) bool { return validRepo(s) }
