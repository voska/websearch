# Live provider checks

On November 21, 2026, LiteLLM 1.102.2 returned successful results for the six
presets below. Each command requested two results, supplied `--country US`,
and returned URLs from the requested domain. Country influences ranking and
localization. These checks observed request acceptance and domain filtering.
They did not measure a ranking difference by country.

| Mode | Domain and result | Provider evidence |
|---|---|---|
| `fast` | [pkg.go.dev/context](https://pkg.go.dev/context) | Parallel reported a $0.001 search |
| `quick` | [pkg.go.dev/net/url](https://pkg.go.dev/net/url) | Perplexity reported $0.005 and both results honored the domain |
| `semantic` | [pkg.go.dev/net/http](https://pkg.go.dev/net/http) | An Exa control request with a 400-character cap returned 400-character snippets. The shipped cap returned 1,200 |
| `fresh` | [Chrome Releases](https://chromereleases.googleblog.com/) | Brave's one-month filter returned November 9–17 results. A one-day control returned November 20 results |
| `deep` | [Go blog](https://go.dev/blog/context) | Parallel advanced reported $0.005 for the request |
| `page` | [Go 1.27 release notes](https://go.dev/doc/go1.27) | Firecrawl returned about 34,000 characters with Markdown enabled and 5,200 in its control response |

The deployed Exa adapter omits the resolved search type. Its response confirms
the text cap. The deployed Tavily adapter returned search results when asked
for an answer and omitted the answer field. Tavily is available by explicit
provider override for standard searches.

The proxy's provider settings follow the [LiteLLM search docs](https://docs.litellm.ai/docs/search/).
