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

package sinablog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const hotJSFixture = `var json999_day =[{"blog_title":"Test Title","uname":"TestUser","asc_nums":"1000","article_hits":"5000","blog_hits":"99999","blog_pubdate":"2026-06-16 08:00:00","uid":"1234567","blog_url":"http://blog.sina.com.cn/s/blog_abc.html"}]`

const searchJSONFixture = `{
    "code": 0,
    "message": "success",
    "data": {
        "list": [
            {
                "title": "Test <font color='red'>Blog</font> Post",
                "intro": "A blog about testing",
                "url": "https://k.sina.com.cn/article_test.html",
                "time": "2026-06-16 10:00:00",
                "ctime": 1781913600,
                "author": "TestAuthor",
                "media_show": "TestMedia",
                "thumb": "https://n.sinaimg.cn/test.jpg"
            }
        ]
    }
}`

func TestHot_ParsesJS(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(hotJSFixture))
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0

	c := NewClient(cfg)
	posts, err := c.Hot(context.Background(), "999", "day", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("want 1 post, got %d", len(posts))
	}

	p := posts[0]
	if p.Rank != 1 {
		t.Errorf("Rank: want 1, got %d", p.Rank)
	}
	if p.Title != "Test Title" {
		t.Errorf("Title: want %q, got %q", "Test Title", p.Title)
	}
	if p.Author != "TestUser" {
		t.Errorf("Author: want %q, got %q", "TestUser", p.Author)
	}
	if p.ViewInc != 1000 {
		t.Errorf("ViewInc: want 1000, got %d", p.ViewInc)
	}
	if p.TotalViews != 5000 {
		t.Errorf("TotalViews: want 5000, got %d", p.TotalViews)
	}
	if p.BlogViews != 99999 {
		t.Errorf("BlogViews: want 99999, got %d", p.BlogViews)
	}
	if p.PublishedAt != "2026-06-16 08:00:00" {
		t.Errorf("PublishedAt: want %q, got %q", "2026-06-16 08:00:00", p.PublishedAt)
	}
	if p.AuthorID != "1234567" {
		t.Errorf("AuthorID: want %q, got %q", "1234567", p.AuthorID)
	}
	if p.URL != "http://blog.sina.com.cn/s/blog_abc.html" {
		t.Errorf("URL: want %q, got %q", "http://blog.sina.com.cn/s/blog_abc.html", p.URL)
	}
}

func TestHot_RankIsSequential(t *testing.T) {
	fixture := `var json999_day =[{"blog_title":"Post1","uname":"User1","asc_nums":"100","article_hits":"200","blog_hits":"300","blog_pubdate":"2026-01-01 00:00:00","uid":"1","blog_url":"http://example.com/1"},{"blog_title":"Post2","uname":"User2","asc_nums":"50","article_hits":"100","blog_hits":"200","blog_pubdate":"2026-01-01 00:00:01","uid":"2","blog_url":"http://example.com/2"}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(fixture))
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0

	c := NewClient(cfg)
	posts, err := c.Hot(context.Background(), "999", "day", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("want 2 posts, got %d", len(posts))
	}
	if posts[0].Rank != 1 {
		t.Errorf("first Rank: want 1, got %d", posts[0].Rank)
	}
	if posts[1].Rank != 2 {
		t.Errorf("second Rank: want 2, got %d", posts[1].Rank)
	}
}

func TestHot_LimitApplied(t *testing.T) {
	fixture := `var json999_day =[{"blog_title":"P1","uname":"U1","asc_nums":"1","article_hits":"1","blog_hits":"1","blog_pubdate":"2026-01-01","uid":"1","blog_url":"http://a.com"},{"blog_title":"P2","uname":"U2","asc_nums":"1","article_hits":"1","blog_hits":"1","blog_pubdate":"2026-01-01","uid":"2","blog_url":"http://b.com"},{"blog_title":"P3","uname":"U3","asc_nums":"1","article_hits":"1","blog_hits":"1","blog_pubdate":"2026-01-01","uid":"3","blog_url":"http://c.com"}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(fixture))
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0

	c := NewClient(cfg)
	posts, err := c.Hot(context.Background(), "999", "day", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(posts) != 2 {
		t.Errorf("want 2 posts (limit applied), got %d", len(posts))
	}
}

func TestHot_CategoryURLBuilt(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		_, _ = w.Write([]byte(hotJSFixture))
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0

	c := NewClient(cfg)
	_, err := c.Hot(context.Background(), "113", "week", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(capturedPath, "113_week.js") {
		t.Errorf("expected path ending with 113_week.js, got %q", capturedPath)
	}
}

func TestHot_MonthPeriodMapped(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		_, _ = w.Write([]byte(hotJSFixture))
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0

	c := NewClient(cfg)
	_, err := c.Hot(context.Background(), "999", "month", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(capturedPath, "999_Mon.js") {
		t.Errorf("expected path ending with 999_Mon.js, got %q", capturedPath)
	}
}

func TestHot_InvalidPeriod(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Rate = 0
	c := NewClient(cfg)
	_, err := c.Hot(context.Background(), "999", "quarter", 10)
	if err == nil {
		t.Error("expected error for unknown period, got nil")
	}
}

func TestSearch_ParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(searchJSONFixture))
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.SearchURL = srv.URL
	cfg.Rate = 0

	c := NewClient(cfg)
	posts, err := c.Search(context.Background(), "python", 10, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("want 1 post, got %d", len(posts))
	}

	p := posts[0]
	if p.Title != "Test Blog Post" {
		t.Errorf("Title: want %q, got %q", "Test Blog Post", p.Title)
	}
	if strings.Contains(p.Title, "<font") {
		t.Error("font tags not stripped from title")
	}
	if p.URL != "https://k.sina.com.cn/article_test.html" {
		t.Errorf("URL: want %q, got %q", "https://k.sina.com.cn/article_test.html", p.URL)
	}
	if p.Time != "2026-06-16 10:00:00" {
		t.Errorf("Time: want %q, got %q", "2026-06-16 10:00:00", p.Time)
	}
	if p.Author != "TestAuthor" {
		t.Errorf("Author: want %q, got %q", "TestAuthor", p.Author)
	}
	if p.Source != "TestMedia" {
		t.Errorf("Source: want %q, got %q", "TestMedia", p.Source)
	}
	if p.Thumbnail != "https://n.sinaimg.cn/test.jpg" {
		t.Errorf("Thumbnail: want %q, got %q", "https://n.sinaimg.cn/test.jpg", p.Thumbnail)
	}
}

func TestSearch_StripsFontTags(t *testing.T) {
	title := cleanTitle("Test <font color='red'>Blog</font> <font>Post</font>")
	if title != "Test Blog Post" {
		t.Errorf("cleanTitle = %q, want %q", title, "Test Blog Post")
	}
}

// buildFullPage returns a search API response with n items.
func buildFullPage(n int) string {
	items := make([]string, n)
	for i := 0; i < n; i++ {
		items[i] = `{"title":"Post","intro":"Body","url":"https://k.sina.com.cn/a.html","time":"2026-01-01","author":"","media_show":"","thumb":""}`
	}
	return `{"code":0,"message":"success","data":{"list":[` + strings.Join(items, ",") + `]}}`
}

func TestSearch_MultiPage(t *testing.T) {
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		if page > 2 {
			// Return empty list to stop pagination
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"list":[]}}`))
			return
		}
		// Return a full page (10 items) to trigger pagination
		_, _ = w.Write([]byte(buildFullPage(10)))
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.SearchURL = srv.URL
	cfg.Rate = 0

	c := NewClient(cfg)
	posts, err := c.Search(context.Background(), "python", 30, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(posts) != 20 { // 10 items per page, 2 full pages
		t.Errorf("want 20 posts from 2 pages, got %d", len(posts))
	}
}

func TestSearch_EmptyResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"list":[]}}`))
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.SearchURL = srv.URL
	cfg.Rate = 0

	c := NewClient(cfg)
	posts, err := c.Search(context.Background(), "zzznoresults", 10, 1)
	if err != nil {
		t.Fatalf("unexpected error for empty results: %v", err)
	}
	if len(posts) != 0 {
		t.Errorf("want 0 posts, got %d", len(posts))
	}
}

func TestParseHotJS_StripVariable(t *testing.T) {
	// Test that the variable prefix is correctly stripped
	js := `var json113_week = [{"blog_title":"Tech Post","uname":"Dev","asc_nums":"500","article_hits":"2000","blog_hits":"50000","blog_pubdate":"2026-06-01","uid":"999","blog_url":"http://blog.sina.com.cn/s/tech.html"}];`
	posts, err := parseHotJS([]byte(js))
	if err != nil {
		t.Fatalf("parseHotJS error: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("want 1 post, got %d", len(posts))
	}
	if posts[0].Title != "Tech Post" {
		t.Errorf("Title: want %q, got %q", "Tech Post", posts[0].Title)
	}
}
