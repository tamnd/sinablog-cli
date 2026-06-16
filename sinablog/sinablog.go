// Copyright 2026 Duc-Tam Nguyen
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package sinablog is the library behind the sinablog command line:
// the HTTP client, parsers, and typed data models for Sina Blog
// (blog.sina.com.cn, 新浪博客).
//
// Two data sources are used:
//
//  1. Hot rankings: JavaScript variable files at blog.sina.com.cn that
//     contain JSON arrays of top-ranked blog posts by category and period.
//
//  2. Search: the Sina search JSON API at search.sina.com.cn, which indexes
//     Sina content including blog posts.
package sinablog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Host is the primary blog hostname.
const Host = "blog.sina.com.cn"

// DefaultUserAgent is the browser User-Agent used on every request.
const DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// ErrNotFound is returned when a resource does not exist.
var ErrNotFound = errors.New("sinablog: not found")

// Config holds constructor parameters for Client.
type Config struct {
	BaseURL   string // blog.sina.com.cn
	SearchURL string // search.sina.com.cn
	UserAgent string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
}

// DefaultConfig returns sensible defaults for Sina Blog.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://blog.sina.com.cn",
		SearchURL: "https://search.sina.com.cn",
		UserAgent: DefaultUserAgent,
		Rate:      500 * time.Millisecond,
		Retries:   3,
		Timeout:   30 * time.Second,
	}
}

// Client is a rate-limited HTTP client for Sina Blog.
type Client struct {
	cfg  Config
	http *http.Client
	mu   sync.Mutex
	last time.Time
}

// NewClient returns a Client configured with cfg.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// HotPost is one entry from the Sina Blog daily/weekly/monthly ranking.
type HotPost struct {
	Rank        int    `json:"rank"`
	Title       string `json:"title"`
	Author      string `json:"author"`
	ViewInc     int    `json:"view_inc"`     // incremental view count (asc_nums)
	TotalViews  int    `json:"total_views"`  // total article views (article_hits)
	BlogViews   int    `json:"blog_views"`   // total blog views (blog_hits)
	PublishedAt string `json:"published_at"` // publication time (blog_pubdate)
	AuthorID    string `json:"author_id"`    // Sina user ID (uid)
	URL         string `json:"url"`          // blog post URL (blog_url)
}

// hotPostRaw matches the JSON structure in the Sina ranking JS files.
type hotPostRaw struct {
	Title       string `json:"blog_title"`
	Author      string `json:"uname"`
	AscNums     string `json:"asc_nums"`
	ArticleHits string `json:"article_hits"`
	BlogHits    string `json:"blog_hits"`
	PubDate     string `json:"blog_pubdate"`
	UID         string `json:"uid"`
	URL         string `json:"blog_url"`
}

// SearchPost is one result from the Sina search API for blog content.
type SearchPost struct {
	Title     string `json:"title"`
	Summary   string `json:"summary"`
	URL       string `json:"url"`
	Time      string `json:"time"`
	Author    string `json:"author"`
	Source    string `json:"source"`
	Thumbnail string `json:"thumbnail"`
}

// PeriodMap maps user-friendly period names to the JS file period suffix.
var PeriodMap = map[string]string{
	"day":     "day",
	"daily":   "day",
	"week":    "week",
	"weekly":  "week",
	"month":   "Mon",
	"monthly": "Mon",
}

// CategoryNames maps category IDs to English names.
var CategoryNames = map[string]string{
	"999": "All Categories",
	"101": "Humanities & Ideas",
	"102": "Literature",
	"103": "Fun & Humor",
	"104": "Sports",
	"105": "Entertainment",
	"106": "Film & Music Reviews",
	"107": "Relationships",
	"108": "Food",
	"109": "Cars",
	"110": "Real Estate",
	"111": "Industry",
	"112": "Securities",
	"113": "Internet & Tech",
	"114": "Science",
	"115": "Gadgets",
	"116": "Military",
	"117": "Misc Talk",
	"118": "Games",
	"119": "Personal Journal",
	"120": "Life Reflections",
	"121": "Visual",
	"122": "Education",
	"123": "Campus Life",
	"124": "Other",
	"125": "Parenting",
	"126": "Pets",
	"127": "Travel",
	"128": "Art",
	"129": "Property",
	"130": "Automotive",
	"131": "Horoscope",
	"132": "Original Literature",
	"133": "Knowledge",
	"134": "Fashion",
	"135": "Social Commentary",
	"136": "Home & Decor",
	"137": "Society & Documentary",
	"138": "Career & Motivation",
	"139": "Strange News",
	"140": "Horror Stories",
}

// Hot fetches the blog post ranking for the given category and period.
// period must be one of: day, week, month (or their aliases).
// category is a Sina Blog category ID string (e.g. "999" for all, "113" for tech).
// limit caps the number of returned posts (max 100).
func (c *Client) Hot(ctx context.Context, category, period string, limit int) ([]HotPost, error) {
	jsPeriod, ok := PeriodMap[strings.ToLower(period)]
	if !ok {
		return nil, fmt.Errorf("sinablog: unknown period %q: use day, week, or month", period)
	}
	if category == "" {
		category = "999"
	}

	u := fmt.Sprintf("%s/main/top_new/article_sort/%s_%s.js", c.cfg.BaseURL, category, jsPeriod)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}

	posts, err := parseHotJS(body)
	if err != nil {
		return nil, err
	}

	if limit > 0 && limit < len(posts) {
		posts = posts[:limit]
	}
	return posts, nil
}

// parseHotJS extracts the JSON array from a Sina Blog ranking JS file.
// The file format is:  var json999_day = [{ ... }];
func parseHotJS(body []byte) ([]HotPost, error) {
	s := strings.TrimSpace(string(body))
	// Find the start of the JSON array
	idx := strings.Index(s, "[")
	if idx < 0 {
		return nil, fmt.Errorf("sinablog: no JSON array found in ranking JS")
	}
	s = s[idx:]
	// Strip trailing semicolon and whitespace
	s = strings.TrimRight(s, ";\r\n ")

	var raw []hotPostRaw
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return nil, fmt.Errorf("sinablog: parse ranking JS: %w", err)
	}

	posts := make([]HotPost, 0, len(raw))
	for i, r := range raw {
		posts = append(posts, HotPost{
			Rank:        i + 1,
			Title:       cleanTitle(r.Title),
			Author:      r.Author,
			ViewInc:     parseInt(r.AscNums),
			TotalViews:  parseInt(r.ArticleHits),
			BlogViews:   parseInt(r.BlogHits),
			PublishedAt: r.PubDate,
			AuthorID:    r.UID,
			URL:         r.URL,
		})
	}
	return posts, nil
}

// searchAPIResponse is the JSON envelope from search.sina.com.cn.
type searchAPIResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List []searchItem `json:"list"`
	} `json:"data"`
}

type searchItem struct {
	Title      string `json:"title"`
	Intro      string `json:"intro"`
	URL        string `json:"url"`
	Time       string `json:"time"`
	Author     string `json:"author"`
	MediaShow  string `json:"media_show"`
	Thumb      string `json:"thumb"`
}

// Search fetches blog-related content from the Sina search API.
func (c *Client) Search(ctx context.Context, query string, limit, startPage int) ([]SearchPost, error) {
	if limit <= 0 {
		limit = 20
	}
	if startPage <= 0 {
		startPage = 1
	}

	const pageSize = 10 // approximate items per Sina search page

	var all []SearchPost
	for page := startPage; len(all) < limit; page++ {
		items, err := c.searchPage(ctx, query, page)
		if err != nil {
			if len(all) > 0 {
				break
			}
			return nil, err
		}
		all = append(all, items...)
		if len(items) < pageSize { // end of results or last partial page
			break
		}
	}
	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

func (c *Client) searchPage(ctx context.Context, query string, page int) ([]SearchPost, error) {
	u := fmt.Sprintf("%s/api/search?q=%s&tp=blog&page=%d",
		c.cfg.SearchURL,
		url.QueryEscape(query),
		page,
	)
	body, err := c.getJSON(ctx, u)
	if err != nil {
		return nil, err
	}

	var resp searchAPIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("sinablog: parse search response: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("sinablog: search API error %d: %s", resp.Code, resp.Message)
	}

	posts := make([]SearchPost, 0, len(resp.Data.List))
	for _, item := range resp.Data.List {
		posts = append(posts, SearchPost{
			Title:     cleanTitle(item.Title),
			Summary:   strings.TrimSpace(item.Intro),
			URL:       item.URL,
			Time:      item.Time,
			Author:    item.Author,
			Source:    item.MediaShow,
			Thumbnail: item.Thumb,
		})
	}
	return posts, nil
}

var (
	fontTagRE = regexp.MustCompile(`(?i)</?font[^>]*>`)
	allTagRE  = regexp.MustCompile(`<[^>]+>`)
)

func cleanTitle(s string) string {
	s = fontTagRE.ReplaceAllString(s, "")
	s = allTagRE.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	return strings.TrimSpace(s)
}

func parseInt(s string) int {
	s = strings.ReplaceAll(s, ",", "")
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

// get fetches a URL and returns the body. Used for JS files (blog.sina.com.cn).
func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	return c.fetch(ctx, rawURL, "text/javascript,text/html,*/*;q=0.8")
}

// getJSON fetches a URL expecting a JSON response. Used for search API.
func (c *Client) getJSON(ctx context.Context, rawURL string) ([]byte, error) {
	return c.fetch(ctx, rawURL, "application/json,*/*;q=0.8")
}

func (c *Client) fetch(ctx context.Context, rawURL, accept string) ([]byte, error) {
	var lastErr error
	attempts := c.cfg.Retries
	if attempts < 1 {
		attempts = 1
	}
	for attempt := 1; attempt <= attempts; attempt++ {
		if attempt > 1 {
			wait := time.Duration(attempt-1) * 500 * time.Millisecond
			if wait > 8*time.Second {
				wait = 8 * time.Second
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
		}

		c.pace()
		body, code, err := c.do(rawURL, accept)
		if err != nil {
			lastErr = err
			continue
		}
		if code == http.StatusNotFound {
			return nil, ErrNotFound
		}
		if code == http.StatusTooManyRequests || code >= 500 {
			lastErr = fmt.Errorf("http %d", code)
			continue
		}
		if code != http.StatusOK {
			return nil, fmt.Errorf("http %d", code)
		}
		return body, nil
	}
	return nil, fmt.Errorf("fetch %s after %d attempts: %w", rawURL, attempts, lastErr)
}

func (c *Client) do(rawURL, accept string) ([]byte, int, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", accept)
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

func (c *Client) pace() {
	if c.cfg.Rate <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}
