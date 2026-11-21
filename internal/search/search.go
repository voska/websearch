package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	ErrEmpty   = errors.New("no results. Broaden the query or remove filters")
	ErrNetwork = errors.New("search connection failed. Check connectivity and retry")
	ErrTimeout = errors.New("search timed out. Retry or lower the result count")
)

type preset struct {
	provider string
	options  map[string]any
}

var modes = map[string]preset{
	"fast":     {"parallel", map[string]any{"mode": "fast"}},
	"quick":    {"perplexity", nil},
	"semantic": {"exa", map[string]any{"type": "neural", "contents": map[string]any{"text": map[string]int{"maxCharacters": 1200}}}},
	"fresh":    {"brave", map[string]any{"freshness": "pm"}},
	"deep":     {"parallel", map[string]any{"mode": "advanced"}},
	"page":     {"firecrawl", map[string]any{"scrapeOptions": map[string]any{"formats": []string{"markdown"}, "onlyMainContent": true}}},
}

type Result struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

type HTTPError int

func (e HTTPError) Error() string {
	return fmt.Sprintf("search proxy returned HTTP %d", e)
}

type Client struct {
	baseURL string
	http    *http.Client
	version string
}

func NewClient(version string) Client {
	return Client{
		baseURL: "https://api.voska.org",
		version: version,
		http: &http.Client{
			Timeout: 25 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

type Query struct {
	Text     string
	Mode     string
	Provider string
	Limit    int
	Domains  []string
	Country  string
}

func (q Query) Validate() error {
	if q.Country != "" && q.Provider == "tavily" {
		return errors.New("--country is unavailable with the Tavily route")
	}
	return nil
}

func (q Query) request() (string, map[string]any) {
	selected := modes[q.Mode]
	if q.Provider != "" {
		selected = preset{provider: q.Provider}
	}
	payload := map[string]any{
		"query":       q.Text,
		"max_results": q.Limit,
	}
	if q.Country != "" {
		payload["country"] = q.Country
	}
	if len(q.Domains) > 0 {
		if selected.provider == "firecrawl" {
			filters := make([]string, 0, len(q.Domains))
			for _, domain := range q.Domains {
				filters = append(filters, "site:"+domain)
			}
			payload["query"] = "(" + strings.Join(filters, " OR ") + ") " + q.Text
		} else {
			payload["search_domain_filter"] = q.Domains
		}
	}
	for key, value := range selected.options {
		payload[key] = value
	}
	return selected.provider + "-search", payload
}

func (c Client) Search(ctx context.Context, q Query, key string) ([]Result, json.RawMessage, error) {
	if err := q.Validate(); err != nil {
		return nil, nil, err
	}
	route, payload := q.request()
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, errors.New("cannot encode search parameters")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/search/"+route, bytes.NewReader(body))
	if err != nil {
		return nil, nil, errors.New("cannot construct search request")
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "websearch/"+c.version)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, transportError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, HTTPError(resp.StatusCode)
	}

	const responseLimit = 32 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, responseLimit+1))
	if err != nil {
		return nil, nil, transportError(err)
	}
	if len(data) > responseLimit {
		return nil, nil, errors.New("search response exceeds 32 MiB. Lower the result count")
	}
	var envelope struct {
		Results []Result `json:"results"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil || envelope.Results == nil {
		return nil, nil, errors.New("search proxy returned an invalid results response")
	}
	if len(envelope.Results) == 0 {
		return nil, nil, ErrEmpty
	}
	return envelope.Results, data, nil
}

func transportError(err error) error {
	switch {
	case os.IsTimeout(err):
		return ErrTimeout
	case errors.Is(err, context.Canceled):
		return errors.New("search canceled")
	default:
		return ErrNetwork
	}
}
