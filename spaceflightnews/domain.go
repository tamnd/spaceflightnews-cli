package spaceflightnews

import (
	"context"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes the Spaceflight News API as a kit Domain: a driver that a
// multi-domain host (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/spaceflightnews-cli/spaceflightnews"
//
// exactly as a database/sql program enables a driver with `import _
// "github.com/lib/pq"`. The init below registers it; the host then routes
// spaceflightnews:// URIs to the operations Register installs. The same Domain
// also builds the standalone spaceflightnews binary (see cli.NewApp), so the
// binary and a host share one source of truth.
func init() { kit.Register(Domain{}) }

// Domain is the Spaceflight News driver. It carries no state; the per-run
// client is built by the factory Register hands kit.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "spaceflightnews",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "spaceflightnews",
			Short:  "Read space news articles, blogs, and reports",
			Long: `spaceflightnews reads public data from the Spaceflight News API
(https://api.spaceflightnewsapi.net/v4/), shapes it into clean records, and
prints output that pipes into the rest of your tools. No API key required.`,
			Site: Host,
			Repo: "https://github.com/tamnd/spaceflightnews-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	// articles: list / search space news articles.
	kit.Handle(app, kit.OpMeta{
		Name:    "articles",
		Group:   "read",
		List:    true,
		Summary: "List or search space news articles",
	}, listArticles)

	// blogs: list / search space blog posts.
	kit.Handle(app, kit.OpMeta{
		Name:    "blogs",
		Group:   "read",
		List:    true,
		Summary: "List or search space blog posts",
	}, listBlogs)

	// reports: list / search spaceflight reports.
	kit.Handle(app, kit.OpMeta{
		Name:    "reports",
		Group:   "read",
		List:    true,
		Summary: "List or search spaceflight reports",
	}, listReports)
}

// newClient builds the client from the host-resolved config.
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

type articlesInput struct {
	Search   string  `kit:"flag" help:"filter by keyword"`
	Site     string  `kit:"flag" help:"filter by news site (e.g. NASA)"`
	Limit    int     `kit:"flag,inherit" help:"max results" default:"10"`
	Featured bool    `kit:"flag" help:"show featured articles only"`
	Client   *Client `kit:"inject"`
}

type listInput struct {
	Search string  `kit:"flag" help:"filter by keyword"`
	Limit  int     `kit:"flag,inherit" help:"max results" default:"10"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func listArticles(ctx context.Context, in articlesInput, emit func(*Article) error) error {
	items, err := in.Client.Articles(ctx, in.Search, in.Site, in.Limit, in.Featured)
	if err != nil {
		return mapErr(err)
	}
	for _, a := range items {
		if err := emit(a); err != nil {
			return err
		}
	}
	return nil
}

func listBlogs(ctx context.Context, in listInput, emit func(*Article) error) error {
	items, err := in.Client.Blogs(ctx, in.Search, in.Limit)
	if err != nil {
		return mapErr(err)
	}
	for _, a := range items {
		if err := emit(a); err != nil {
			return err
		}
	}
	return nil
}

func listReports(ctx context.Context, in listInput, emit func(*Article) error) error {
	items, err := in.Client.Reports(ctx, in.Search, in.Limit)
	if err != nil {
		return mapErr(err)
	}
	for _, a := range items {
		if err := emit(a); err != nil {
			return err
		}
	}
	return nil
}

// --- Resolver: pure string functions ---

// Classify is a no-op resolver for this API domain; resources are not
// addressable by a single id the way HTML-scraped sites are.
func (Domain) Classify(input string) (uriType, id string, err error) {
	return "", "", errs.Usage("spaceflightnews URIs are not supported for direct resolution")
}

// Locate is the inverse of Classify.
func (Domain) Locate(uriType, id string) (string, error) {
	return "", errs.Usage("spaceflightnews has no resource type %q", uriType)
}

// mapErr converts a library error into the kit error kind that carries the
// right exit code.
func mapErr(err error) error {
	return err
}
