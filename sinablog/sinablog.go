// Package sinablog is the library behind the sinablog command line:
// the HTTP client, request shaping, and typed data models for Sina Blog.
package sinablog

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// DefaultUserAgent identifies the client to Sina Blog.
const DefaultUserAgent = "Mozilla/5.0 (compatible; sinablog-cli/0.1)"

// Host is the site this client talks to.
const Host = "blog.sina.com.cn"

// BaseURL is the root every request is built from.
const BaseURL = "https://" + Host

// hotURL is the hot articles listing page.
const hotURL = "https://blog.sina.com.cn/lm/plist/allblog_hot.d.html"

// Client talks to Sina Blog over HTTP.
type Client struct {
	HTTP      *http.Client
	UserAgent string
	// Rate is the minimum gap between requests. Zero means no pacing.
	Rate    time.Duration
	Retries int

	last time.Time
}

// NewClient returns a Client with sensible defaults.
func NewClient() *Client {
	return &Client{
		HTTP:      &http.Client{Timeout: 30 * time.Second},
		UserAgent: DefaultUserAgent,
		Rate:      200 * time.Millisecond,
		Retries:   5,
	}
}

// Get fetches url and returns the response body.
func (c *Client) Get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", url, lastErr)
}

func (c *Client) do(ctx context.Context, url string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

// pace blocks until at least Rate has passed since the previous request.
func (c *Client) pace() {
	if c.Rate <= 0 {
		return
	}
	if wait := c.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// Article is a single blog article from Sina Blog.
type Article struct {
	ID    string `json:"id"    kit:"id" table:"id"`
	Title string `json:"title"          table:"title"`
	URL   string `json:"url"            table:"url,url"`
}

// articleRE matches blog article links and their anchor text.
var articleRE = regexp.MustCompile(`<a[^>]+href="(https://blog\.sina\.com\.cn/s/blog_[^"]+\.html)"[^>]*>([^<]{10,100})</a>`)

// blogIDRE extracts the blog ID from a URL like .../s/blog_XXXXX.html.
var blogIDRE = regexp.MustCompile(`/s/(blog_[^.]+)\.html`)

// parseArticles extracts Article records from the hot page HTML.
func parseArticles(body []byte, limit int) []*Article {
	var out []*Article
	seen := map[string]bool{}
	for _, m := range articleRE.FindAllSubmatch(body, -1) {
		rawURL := string(m[1])
		title := strings.TrimSpace(string(m[2]))
		if title == "" {
			continue
		}
		idm := blogIDRE.FindStringSubmatch(rawURL)
		if idm == nil {
			continue
		}
		id := idm[1]
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, &Article{ID: id, Title: title, URL: rawURL})
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// Hot fetches the hot articles listing from Sina Blog.
func (c *Client) Hot(ctx context.Context, limit int) ([]*Article, error) {
	body, err := c.Get(ctx, hotURL)
	if err != nil {
		return nil, err
	}
	articles := parseArticles(body, limit)
	return articles, nil
}
