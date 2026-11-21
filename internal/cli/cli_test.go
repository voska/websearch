package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/voska/websearch/internal/search"
)

func TestHelpVersionAndUsage(t *testing.T) {
	for _, tt := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"--help"}, 0, "--provider"},
		{[]string{"--version"}, 0, "test-version"},
		{[]string{"-n", "21", "query"}, 2, "between 1 and 20"},
		{[]string{"--mode", "answer", "query"}, 2, "answer"},
		{[]string{"--domain", "https://go.dev", "query"}, 2, "expects a hostname"},
		{[]string{"--provider", "tavily", "--country", "US", "query"}, 2, "unavailable"},
	} {
		var out, stderr bytes.Buffer
		got := Run(context.Background(), tt.args, &out, &stderr, "test-version")
		if got != tt.code || !strings.Contains(out.String()+stderr.String(), tt.want) {
			t.Errorf("%v returned %d, stdout %q, stderr %q", tt.args, got, &out, &stderr)
		}
	}
}

func TestErrorExitCodes(t *testing.T) {
	for _, tt := range []struct {
		err  error
		code int
	}{
		{search.ErrEmpty, 3},
		{search.HTTPError(401), 4},
		{search.HTTPError(404), 5},
		{search.HTTPError(403), 6},
		{search.HTTPError(429), 7},
		{search.HTTPError(503), 8},
		{search.ErrTimeout, 8},
	} {
		var stderr bytes.Buffer
		if got := reportSearchError(&stderr, tt.err); got != tt.code || stderr.Len() == 0 {
			t.Errorf("%v returned %d with stderr %q", tt.err, got, &stderr)
		}
	}
}

func TestCompactOutput(t *testing.T) {
	results := []search.Result{{Title: "Go\nDocs", URL: "https://go.dev/",
		Snippet: "read\tthis\x00 " + strings.Repeat("é", 400)}}
	var out bytes.Buffer
	if err := writeResults(&out, false, results, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "Go Docs\nhttps://go.dev/\nread this ") ||
		!strings.HasSuffix(out.String(), "…\n") || strings.Contains(out.String(), "\x00") {
		t.Fatalf("unexpected compact output: %q", &out)
	}
}

type fixtureTransport struct{}

func (fixtureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(
		`{"results":[{"title":"Go","url":"https://go.dev/","snippet":"Documentation"}],"extra":"<b>"}`,
	))}, nil
}

func TestRunSearchOutputs(t *testing.T) {
	t.Setenv("VOSKA_AI_KEY", "fixture")
	original := http.DefaultTransport
	http.DefaultTransport = fixtureTransport{}
	t.Cleanup(func() { http.DefaultTransport = original })

	for _, tt := range []struct {
		args []string
		want string
	}{
		{[]string{"Go documentation"}, "Go\nhttps://go.dev/\nDocumentation\n"},
		{[]string{"Go documentation", "--json"}, `"extra":"<b>"`},
	} {
		var out, stderr bytes.Buffer
		if code := Run(context.Background(), tt.args, &out, &stderr, "test"); code != 0 ||
			stderr.Len() != 0 || !strings.Contains(out.String(), tt.want) {
			t.Errorf("%v returned %d, stdout %q, stderr %q", tt.args, code, &out, &stderr)
		}
	}
}
