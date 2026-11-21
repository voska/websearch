# websearch

Search the web through a LiteLLM proxy with one command. Pick a mode by what
you need and use `--json` when you need the full provider response.

```sh
websearch -n 3 'Go HTTP client timeout'
websearch --mode fresh --domain go.dev 'Go security release'
websearch --mode page --json 'Go release notes'
```

## Install

Install a published release with `brew install voska/tap/websearch`. You can also build from source
with Go 1.26 or later and `make build`. The binary is `bin/websearch`.

The CLI connects to `https://api.voska.org` and needs a LiteLLM virtual key.
It checks `VOSKA_AI_KEY`, then `~/.config/voska/ai-key`, then the macOS
login Keychain item `org.voska.litellm.client`. For SSH and containers, use an
owner-only key file (0400 or 0600). The file may link to a read-only secret
mount owned by the process user.

To install the agent skill, link the repository's `skills/websearch` directory
into `~/.agents/skills/websearch`.

## Modes

| Mode | Provider | Settings |
|---|---|---|
| `fast` | Parallel | Fast tier |
| `quick` | Perplexity | Standard search |
| `semantic` | Exa | Neural search with up to 1,200 characters of text per result |
| `fresh` | Brave | Past month |
| `deep` | Parallel | Advanced tier |
| `page` | Firecrawl | Main-content Markdown in each result's `snippet` |

`--provider` can select `parallel`, `perplexity`, `exa`, `brave`, `firecrawl`,
or `tavily` with its defaults. `--domain` is repeatable, `--country` is a
two-letter country code (unavailable with the Tavily route), and `-n` sets the maximum result
count from 1 to 20. The default is 5. The CLI gives the request 25 seconds.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Search returned results |
| 1 | Unexpected proxy or local error |
| 2 | Invalid arguments |
| 3 | No results |
| 4 | Key missing or rejected |
| 5 | Route missing |
| 6 | Access denied |
| 7 | Rate limited |
| 8 | Network error, temporary gateway failure, or timeout |
| 10 | Invalid local key configuration |

Run `make ci` to verify formatting, vet, race tests, and build.
