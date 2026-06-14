package spaceflightnews

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// fakeArticle returns a minimal wireArticle payload wrapped in wirePage JSON.
func fakePageJSON(t *testing.T, articles ...wireArticle) []byte {
	t.Helper()
	pg := wirePage{Count: len(articles), Results: articles}
	b, err := json.Marshal(pg)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return b
}

func fakeArticle(id int, title, site string) wireArticle {
	return wireArticle{
		ID:          id,
		Title:       title,
		URL:         "https://example.com/" + title,
		NewsSite:    site,
		Summary:     "summary of " + title,
		PublishedAt: "2026-06-14T12:00:00Z",
	}
}

// newTestClient returns a Client pointed at srv with no pacing.
func newTestClient(srv *httptest.Server) *Client {
	c := NewClient()
	c.BaseURL = srv.URL
	c.Rate = 0
	return c
}

func TestGet_UserAgent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		_, _ = w.Write([]byte(`{"count":0,"results":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
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

	c := newTestClient(srv)
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

func TestArticles(t *testing.T) {
	articles := []wireArticle{
		fakeArticle(1, "Mars mission confirmed", "NASA"),
		fakeArticle(2, "SpaceX launch upcoming", "SpaceX"),
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v4/articles/" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fakePageJSON(t, articles...))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	got, err := c.Articles(context.Background(), "", "", 2, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("Articles returned %d items, want 2", len(got))
	}
	if got[0].ID != 1 || got[0].Title != "Mars mission confirmed" {
		t.Errorf("got[0] = %+v", got[0])
	}
	if got[1].NewsSite != "SpaceX" {
		t.Errorf("got[1].NewsSite = %q, want SpaceX", got[1].NewsSite)
	}
}

func TestArticles_SearchParam(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("search"); got != "mars" {
			t.Errorf("search param = %q, want mars", got)
		}
		if got := r.URL.Query().Get("news_site"); got != "NASA" {
			t.Errorf("news_site param = %q, want NASA", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fakePageJSON(t, fakeArticle(10, "Mars water found", "NASA")))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	got, err := c.Articles(context.Background(), "mars", "NASA", 5, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("Articles returned %d items, want 1", len(got))
	}
}

func TestBlogs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v4/blogs/" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fakePageJSON(t, fakeArticle(20, "Blog post one", "SpacePolicyOnline")))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	got, err := c.Blogs(context.Background(), "", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 20 {
		t.Errorf("Blogs = %+v", got)
	}
}

func TestReports(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v4/reports/" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fakePageJSON(t, fakeArticle(30, "Annual report 2025", "ESA")))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	got, err := c.Reports(context.Background(), "", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 30 {
		t.Errorf("Reports = %+v", got)
	}
}

func TestFeaturedParam(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("is_featured"); got != "true" {
			t.Errorf("is_featured param = %q, want true", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fakePageJSON(t, fakeArticle(99, "Featured item", "NASA")))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	got, err := c.Articles(context.Background(), "", "", 5, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("Articles(featured) returned %d items, want 1", len(got))
	}
}
