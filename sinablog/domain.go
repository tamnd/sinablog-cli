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
	"errors"
	"fmt"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

func init() { kit.Register(Domain{}) }

// Domain is the Sina Blog driver for the kit framework.
type Domain struct{}

// Info describes the scheme and identity used by both the standalone binary
// and multi-domain hosts.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme:  "sinablog",
		Aliases: []string{"sb"},
		Hosts:   []string{Host, "search.sina.com.cn"},
		Identity: kit.Identity{
			Binary: "sinablog",
			Short:  "Fetch public Sina Blog posts from the command line",
			Long: `sinablog turns blog.sina.com.cn into a fast, scriptable command line.

Browse top-ranked blog posts and search across millions of Sina Blog entries.
No account, cookie, or API key required.

Quick start:
  sinablog hot                     top 30 blog posts today
  sinablog hot --period week       top 30 this week
  sinablog hot --category 113      Internet & Tech blogs, daily
  sinablog search "人工智能"         search for AI blogs
  sinablog search "python" -o json results as JSON`,
			Site: Host,
			Repo: "https://github.com/tamnd/sinablog-cli",
		},
	}
}

// Register installs the client factory and operations onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{
		Name:    "hot",
		Group:   "posts",
		Summary: "List top-ranked Sina Blog posts",
	}, hotPosts)

	kit.Handle(app, kit.OpMeta{
		Name:    "search",
		Group:   "posts",
		Summary: "Search Sina Blog content",
		Args:    []kit.Arg{{Name: "query", Help: "search query (Chinese or English)"}},
	}, searchPosts)
}

func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	return NewClient(c), nil
}

type hotInput struct {
	Period   string  `kit:"flag" help:"ranking period: day|week|month" default:"day"`
	Category string  `kit:"flag" help:"category ID (999=all, 113=tech)" default:"999"`
	Limit    int     `kit:"flag" help:"max results (1-100)" default:"30"`
	Client   *Client `kit:"inject"`
}

type searchInput struct {
	Query  string  `kit:"arg"  help:"search query"`
	Limit  int     `kit:"flag" help:"max results" default:"20"`
	Page   int     `kit:"flag" help:"starting page number" default:"1"`
	Client *Client `kit:"inject"`
}

func hotPosts(ctx context.Context, in hotInput, emit func(HotPost) error) error {
	posts, err := in.Client.Hot(ctx, in.Category, in.Period, in.Limit)
	if err != nil {
		return mapErr(err)
	}
	for _, p := range posts {
		if err := emit(p); err != nil {
			return err
		}
	}
	return nil
}

func searchPosts(ctx context.Context, in searchInput, emit func(SearchPost) error) error {
	if strings.TrimSpace(in.Query) == "" {
		return errs.Usage("query is required")
	}
	posts, err := in.Client.Search(ctx, in.Query, in.Limit, in.Page)
	if err != nil {
		return mapErr(err)
	}
	for _, p := range posts {
		if err := emit(p); err != nil {
			return err
		}
	}
	return nil
}

// Classify turns a Sina Blog URL or post ID into (type, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", errs.Usage("empty input")
	}
	// http://blog.sina.com.cn/s/blog_<id>.html
	if strings.Contains(input, "blog.sina.com.cn/s/blog_") {
		parts := strings.Split(input, "/s/blog_")
		if len(parts) > 1 {
			pid := strings.TrimSuffix(parts[1], ".html")
			pid = strings.Split(pid, "?")[0]
			if pid != "" {
				return "post", pid, nil
			}
		}
	}
	// Sina user page: http://blog.sina.com.cn/u/<uid>
	if strings.Contains(input, "blog.sina.com.cn/u/") {
		parts := strings.Split(input, "/u/")
		if len(parts) > 1 {
			uid := strings.Split(parts[1], "?")[0]
			uid = strings.TrimSuffix(uid, "/")
			if uid != "" {
				return "user", uid, nil
			}
		}
	}
	return "", "", errs.Usage("sinablog: unrecognized reference: %q", input)
}

// Locate returns the canonical URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "post":
		return fmt.Sprintf("http://blog.sina.com.cn/s/blog_%s.html", id), nil
	case "user":
		return fmt.Sprintf("http://blog.sina.com.cn/u/%s", id), nil
	default:
		return "", errs.Usage("sinablog has no resource type %q", uriType)
	}
}

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotFound) {
		return errs.NotFound("%s", err.Error())
	}
	return err
}
