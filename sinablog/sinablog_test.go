package sinablog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0

	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "ok" {
		t.Errorf("body = %q, want %q", body, "ok")
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("recovered"))
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0
	c.Retries = 5

	start := time.Now()
	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "recovered" {
		t.Errorf("body = %q after retries", body)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

const sampleHotHTML = `<!DOCTYPE html>
<html>
<body>
<ul class="list">
<li><a href="https://blog.sina.com.cn/s/blog_abc123def.html" title="Go tips">Go language tips in 2024</a></li>
<li><a href="https://blog.sina.com.cn/s/blog_xyz789ghi.html">Python automation scripts guide</a></li>
<li><a href="https://blog.sina.com.cn/s/blog_mno456pqr.html">Docker and Kubernetes best practices</a></li>
</ul>
</body>
</html>`

func TestParseArticles(t *testing.T) {
	articles := parseArticles([]byte(sampleHotHTML), 0)
	if len(articles) == 0 {
		t.Fatal("expected at least one article, got none")
	}
	found := false
	for _, a := range articles {
		if a.ID == "blog_abc123def" {
			found = true
			if a.URL != "https://blog.sina.com.cn/s/blog_abc123def.html" {
				t.Errorf("URL = %q, want full URL", a.URL)
			}
			if a.Title == "" {
				t.Error("Title should not be empty")
			}
		}
	}
	if !found {
		t.Errorf("did not find blog_abc123def in %d articles", len(articles))
	}
}

func TestParseArticlesLimit(t *testing.T) {
	articles := parseArticles([]byte(sampleHotHTML), 1)
	if len(articles) != 1 {
		t.Errorf("got %d articles with limit=1, want 1", len(articles))
	}
}

func TestHotViaHTTPTest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(sampleHotHTML))
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0

	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	articles := parseArticles(body, 0)
	if len(articles) == 0 {
		t.Fatal("expected articles from sample HTML")
	}
}
