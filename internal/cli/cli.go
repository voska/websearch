package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"unicode"

	"github.com/alecthomas/kong"
	"github.com/voska/websearch/internal/auth"
	"github.com/voska/websearch/internal/search"
)

const (
	exitError     = 1
	exitUsage     = 2
	exitEmpty     = 3
	exitAuth      = 4
	exitNotFound  = 5
	exitForbidden = 6
	exitRateLimit = 7
	exitRetry     = 8
	exitConfig    = 10
)

type options struct {
	Version  kong.VersionFlag `short:"V" help:"Print the version and exit."`
	Mode     string           `default:"fast" enum:"fast,quick,semantic,fresh,deep,page" help:"Search mode (fast, quick, semantic, fresh, deep, page)."`
	Provider string           `default:"auto" enum:"auto,parallel,perplexity,exa,brave,firecrawl,tavily" help:"Use a provider's defaults (parallel, perplexity, exa, brave, firecrawl, tavily)."`
	Limit    int              `name:"max-results" short:"n" default:"5" help:"Maximum number of results (1–20)."`
	Domains  []string         `name:"domain" sep:"none" help:"Restrict results to a domain. Repeat to include more domains."`
	Country  string           `help:"Two-letter country code for localization, where supported."`
	JSON     bool             `help:"Print the full proxy response as JSON."`
	Query    []string         `arg:"" name:"query" help:"Search terms."`
}

var domainPattern = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
var countryPattern = regexp.MustCompile(`^[A-Z]{2}$`)

func (o options) searchQuery() (search.Query, error) {
	query := strings.TrimSpace(strings.Join(o.Query, " "))
	country := strings.ToUpper(o.Country)
	switch {
	case query == "":
		return search.Query{}, errors.New("enter a search query")
	case strings.ContainsFunc(query, unicode.IsControl):
		return search.Query{}, errors.New("query must be a single line")
	case o.Limit < 1 || o.Limit > 20:
		return search.Query{}, errors.New("--max-results must be between 1 and 20")
	case country != "" && !countryPattern.MatchString(country):
		return search.Query{}, errors.New("--country must be a two-letter country code")
	case len(o.Domains) > 20:
		return search.Query{}, errors.New("provide at most 20 --domain flags")
	}

	domains := make([]string, 0, len(o.Domains))
	for _, value := range o.Domains {
		domain := strings.ToLower(strings.TrimSuffix(value, "."))
		if len(domain) > 253 || !domainPattern.MatchString(domain) {
			return search.Query{}, errors.New("--domain expects a hostname such as example.com")
		}
		domains = append(domains, domain)
	}
	provider := o.Provider
	if provider == "auto" {
		provider = ""
	}
	result := search.Query{
		Text: query, Mode: o.Mode, Provider: provider, Limit: o.Limit,
		Domains: domains, Country: country,
	}
	return result, result.Validate()
}

func Run(ctx context.Context, args []string, out, stderr io.Writer, version string) int {
	var opts options
	var helpOrVersion bool
	parser, err := kong.New(&opts,
		kong.Name("websearch"),
		kong.Description("Search the web through a LiteLLM proxy."),
		kong.Writers(out, stderr),
		kong.Vars{"version": version},
		kong.Exit(func(code int) { helpOrVersion = code == 0 }),
	)
	if err != nil {
		return report(stderr, exitError, err)
	}
	_, err = parser.Parse(args)
	if helpOrVersion {
		return 0
	}
	if err != nil {
		return report(stderr, exitUsage, err)
	}
	query, err := opts.searchQuery()
	if err != nil {
		return report(stderr, exitUsage, err)
	}
	key, err := auth.Key(ctx)
	if errors.Is(err, auth.ErrMissing) {
		return report(stderr, exitAuth, err)
	}
	if err != nil {
		return report(stderr, exitConfig, err)
	}

	results, raw, err := search.NewClient(version).Search(ctx, query, key)
	if err != nil {
		return reportSearchError(stderr, err)
	}
	if err := writeResults(out, opts.JSON, results, raw); err != nil {
		return report(stderr, exitError, errors.New("could not write the search results"))
	}
	return 0
}

func writeResults(out io.Writer, full bool, results []search.Result, raw json.RawMessage) error {
	if full {
		_, err := fmt.Fprintf(out, "%s\n", raw)
		return err
	}
	for i, result := range results {
		if i > 0 {
			if _, err := fmt.Fprintln(out); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(out, "%s\n%s\n%s\n",
			trimText(result.Title, 200), trimText(result.URL, 0), trimText(result.Snippet, 360)); err != nil {
			return err
		}
	}
	return nil
}

func trimText(value string, limit int) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return -1
		}
		return r
	}, value)
	value = strings.Join(strings.Fields(value), " ")
	if limit > 0 {
		length := 0
		for offset := range value {
			if length == limit {
				return strings.TrimSpace(value[:offset]) + "…"
			}
			length++
		}
	}
	return value
}

func reportSearchError(stderr io.Writer, err error) int {
	var status search.HTTPError
	if errors.As(err, &status) {
		switch int(status) {
		case http.StatusUnauthorized:
			return report(stderr, exitAuth, errors.New("search proxy rejected the LiteLLM key (HTTP 401). Check or renew the key"))
		case http.StatusForbidden:
			return report(stderr, exitForbidden, errors.New("search proxy denied access (HTTP 403). Check the key's search permissions"))
		case http.StatusNotFound:
			return report(stderr, exitNotFound, errors.New("search route not found (HTTP 404). Check the proxy's named search tools"))
		case http.StatusTooManyRequests:
			return report(stderr, exitRateLimit, errors.New("search rate limit reached (HTTP 429). Back off before retrying"))
		case http.StatusBadGateway, http.StatusServiceUnavailable:
			return report(stderr, exitRetry, fmt.Errorf("%w. Retry in a moment", status))
		case http.StatusRequestTimeout, http.StatusGatewayTimeout:
			return report(stderr, exitRetry, search.ErrTimeout)
		}
	}
	switch {
	case errors.Is(err, search.ErrEmpty):
		return report(stderr, exitEmpty, err)
	case errors.Is(err, search.ErrTimeout), errors.Is(err, search.ErrNetwork):
		return report(stderr, exitRetry, err)
	default:
		return report(stderr, exitError, err)
	}
}

func report(stderr io.Writer, code int, err error) int {
	fmt.Fprintln(stderr, "websearch:", err)
	return code
}
