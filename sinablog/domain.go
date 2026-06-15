package sinablog

import (
	"context"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

func init() { kit.Register(Domain{}) }

// Domain is the sinablog driver.
type Domain struct{}

// Info describes the scheme, hostnames, and binary identity.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "sinablog",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "sinablog",
			Short:  "A command line for Sina Blog.",
			Long: `A command line for Sina Blog.

sinablog reads public Sina Blog data over plain HTTPS and prints output that
pipes into the rest of your tools. No API key or account required.`,
			Site: Host,
			Repo: "https://github.com/tamnd/sinablog-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{Name: "hot", Group: "read", List: true,
		Summary: "List hot blog articles from Sina Blog",
	}, listHot)
}

// newClient builds the client from kit config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := NewClient()
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
		c.HTTP.Timeout = cfg.Timeout
	}
	return c, nil
}

// --- inputs ---

type hotInput struct {
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func listHot(ctx context.Context, in hotInput, emit func(*Article) error) error {
	articles, err := in.Client.Hot(ctx, in.Limit)
	if err != nil {
		return mapErr(err)
	}
	for _, a := range articles {
		if err := emit(a); err != nil {
			return err
		}
	}
	return nil
}

// Classify turns any accepted input into the canonical (type, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	input = strings.TrimSpace(input)
	if m := blogIDRE.FindStringSubmatch(input); m != nil {
		return "article", m[1], nil
	}
	return "", "", errs.Usage("unrecognized sinablog reference: %q", input)
}

// Locate is the inverse: the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	if uriType != "article" {
		return "", errs.Usage("sinablog has no resource type %q", uriType)
	}
	return "https://blog.sina.com.cn/s/" + id + ".html", nil
}

// mapErr converts a library error into the kit error kind.
func mapErr(err error) error {
	return err
}
