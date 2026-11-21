package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestModesAndOverride(t *testing.T) {
	tests := []struct {
		mode, provider, route string
		options               map[string]any
	}{
		{"fast", "", "parallel", map[string]any{"mode": "fast"}},
		{"deep", "", "parallel", map[string]any{"mode": "advanced"}},
		{"quick", "", "perplexity", nil},
		{"semantic", "", "exa", map[string]any{"type": "neural", "contents": map[string]any{"text": map[string]any{"maxCharacters": 1200.0}}}},
		{"fresh", "", "brave", map[string]any{"freshness": "pm"}},
		{"page", "", "firecrawl", map[string]any{"scrapeOptions": map[string]any{"formats": []any{"markdown"}, "onlyMainContent": true}}},
		{"semantic", "tavily", "tavily", nil},
	}
	for _, tt := range tests {
		t.Run(tt.mode+"/"+tt.provider, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/search/"+tt.route+"-search" || r.Method != http.MethodPost {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer fixture" {
					t.Error("Bearer key missing")
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				want := map[string]any{"max_results": 2.0, "country": "US",
					"query": "Go manual", "search_domain_filter": []any{"go.dev", "pkg.go.dev"}}
				if tt.route == "tavily" {
					delete(want, "country")
				}
				if tt.route == "firecrawl" {
					delete(want, "search_domain_filter")
					want["query"] = "(site:go.dev OR site:pkg.go.dev) Go manual"
				}
				for k, v := range tt.options {
					want[k] = v
				}
				if !reflect.DeepEqual(body, want) {
					t.Errorf("payload = %#v, want %#v", body, want)
				}
				fmt.Fprint(w, `{"extra":123,"results":[{"title":"Go","url":"https://go.dev/","snippet":"Text"}]}`)
			}))
			defer srv.Close()
			c := NewClient("test")
			c.baseURL = srv.URL
			q := Query{Text: "Go manual", Mode: tt.mode, Provider: tt.provider,
				Limit: 2, Domains: []string{"go.dev", "pkg.go.dev"}, Country: "US"}
			if tt.route == "tavily" {
				q.Country = ""
			}
			results, full, err := c.Search(context.Background(), q, "fixture")
			if err != nil || len(results) != 1 {
				t.Fatalf("search returned %v, %v", results, err)
			}
			var extra struct{ Extra int }
			if err := json.Unmarshal(full, &extra); err != nil || extra.Extra != 123 {
				t.Errorf("response extension missing from %s", full)
			}
		})
	}
}

func TestEmptyVersusMalformed(t *testing.T) {
	for _, tt := range []struct {
		body  string
		empty bool
	}{{`{"results":[]}`, true}, {`{"results":null}`, false}, {`{"object":"search"}`, false}, {`{"results":[]}{"other":"value"}`, false}} {
		t.Run(tt.body, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, tt.body)
			}))
			defer srv.Close()
			c := NewClient("test")
			c.baseURL = srv.URL
			_, _, err := c.Search(context.Background(), Query{Mode: "fast", Text: "x", Limit: 1}, "fixture")
			if err == nil || errors.Is(err, ErrEmpty) != tt.empty {
				t.Fatalf("err = %v, empty = %v", err, tt.empty)
			}
		})
	}
}

func TestTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()
	c := NewClient("test")
	c.baseURL = srv.URL
	c.http.Timeout = 30 * time.Millisecond
	_, _, err := c.Search(context.Background(), Query{Mode: "fast", Text: "x", Limit: 1}, "fixture")
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want ErrTimeout", err)
	}
}

func TestRedirectIsNotFollowed(t *testing.T) {
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("search followed a redirect")
	}))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL, http.StatusTemporaryRedirect)
	}))
	defer srv.Close()
	c := NewClient("test")
	c.baseURL = srv.URL
	_, _, err := c.Search(context.Background(), Query{Mode: "fast", Text: "x", Limit: 1}, "fixture")
	if !errors.Is(err, HTTPError(http.StatusTemporaryRedirect)) {
		t.Fatalf("err = %v, want HTTP 307", err)
	}
}

func TestTavilyRejectsCountry(t *testing.T) {
	c := NewClient("test")
	_, _, err := c.Search(context.Background(), Query{Mode: "fast", Provider: "tavily", Country: "US"}, "fixture")
	if err == nil || err.Error() != "--country is unavailable with the Tavily route" {
		t.Fatalf("err = %v, want validation error", err)
	}
}
