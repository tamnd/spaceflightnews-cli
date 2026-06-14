// Package spaceflightnews is the library behind the spaceflightnews command line:
// the HTTP client, request shaping, and the typed data models for the
// Spaceflight News API (https://api.spaceflightnewsapi.net/v4/).
//
// The Client here is the spine every command shares. It sets a real
// User-Agent, paces requests so a busy session stays polite, and retries the
// transient failures (429 and 5xx) that any public API throws under load.
package spaceflightnews

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Host is the API hostname this client talks to, and the host the URI driver
// in domain.go claims.
const Host = "api.spaceflightnewsapi.net"

// BaseURL is the root every request is built from.
const BaseURL = "https://" + Host

// DefaultUserAgent identifies the client honestly to the API.
const DefaultUserAgent = "spaceflightnews-cli/0.1 (tamnd87@gmail.com)"

// Config holds the tunable parameters for the client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Timeout   time.Duration
	Retries   int
}

// DefaultConfig returns production-safe defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   BaseURL,
		UserAgent: DefaultUserAgent,
		Rate:      300 * time.Millisecond,
		Timeout:   15 * time.Second,
		Retries:   3,
	}
}

// Client talks to the Spaceflight News API over HTTPS.
type Client struct {
	HTTP      *http.Client
	UserAgent string
	BaseURL   string
	Rate      time.Duration
	Retries   int

	last time.Time
}

// NewClient returns a Client with sensible defaults.
func NewClient() *Client {
	cfg := DefaultConfig()
	return &Client{
		HTTP:      &http.Client{Timeout: cfg.Timeout},
		UserAgent: cfg.UserAgent,
		BaseURL:   cfg.BaseURL,
		Rate:      cfg.Rate,
		Retries:   cfg.Retries,
	}
}

// Get fetches url and returns the response body. It paces and retries
// according to the client settings. The body is read fully and closed here.
func (c *Client) Get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")

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

// --- wire types (unexported) ---

type wireArticle struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	ImageURL    string `json:"image_url"`
	NewsSite    string `json:"news_site"`
	Summary     string `json:"summary"`
	PublishedAt string `json:"published_at"`
	Featured    bool   `json:"featured"`
}

type wirePage struct {
	Count   int           `json:"count"`
	Results []wireArticle `json:"results"`
}

// --- output types ---

// Article is a space news article, blog post, or report from the API.
type Article struct {
	ID          int    `kit:"id" json:"id"`
	Title       string `json:"title"`
	NewsSite    string `json:"news_site"`
	PublishedAt string `json:"published_at"`
	Summary     string `json:"summary"`
	URL         string `json:"url"`
}

func toArticle(w wireArticle) *Article {
	return &Article{
		ID:          w.ID,
		Title:       w.Title,
		NewsSite:    w.NewsSite,
		PublishedAt: w.PublishedAt,
		Summary:     w.Summary,
		URL:         w.URL,
	}
}

// --- API methods ---

// Articles lists space news articles. search and site may be empty.
// featured filters to featured articles only when true.
func (c *Client) Articles(ctx context.Context, search, site string, limit int, featured bool) ([]*Article, error) {
	return c.fetch(ctx, "/v4/articles/", search, site, limit, featured)
}

// Blogs lists space blog posts. search may be empty.
func (c *Client) Blogs(ctx context.Context, search string, limit int) ([]*Article, error) {
	return c.fetch(ctx, "/v4/blogs/", search, "", limit, false)
}

// Reports lists spaceflight reports. search may be empty.
func (c *Client) Reports(ctx context.Context, search string, limit int) ([]*Article, error) {
	return c.fetch(ctx, "/v4/reports/", search, "", limit, false)
}

func (c *Client) fetch(ctx context.Context, path, search, site string, limit int, featured bool) ([]*Article, error) {
	if limit <= 0 {
		limit = 10
	}
	q := url.Values{}
	q.Set("limit", strconv.Itoa(limit))
	if search != "" {
		q.Set("search", search)
	}
	if site != "" {
		q.Set("news_site", site)
	}
	if featured {
		q.Set("is_featured", "true")
	}
	rawURL := c.BaseURL + path + "?" + q.Encode()

	body, err := c.Get(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	var pg wirePage
	if err := json.Unmarshal(body, &pg); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	out := make([]*Article, 0, len(pg.Results))
	for _, w := range pg.Results {
		out = append(out, toArticle(w))
	}
	return out, nil
}
