package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

func New() *Client { return &Client{HTTP: &http.Client{Timeout: 10 * time.Second}} }
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
