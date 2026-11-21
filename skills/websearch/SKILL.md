---
name: websearch
description: Use when finding current web sources, documentation, related pages, or provider-returned page text. Choose a search mode and run the websearch CLI.
---

# Web search

Run `websearch -n 5 'search terms'`. Use `--json` for the full response,
including longer snippets and any provider metadata. Use `--domain example.com`
to narrow sources and repeat it to include more domains. Use `--country BR`
for localization where the provider supports it.

| Mode | Use when |
|---|---|
| `fast` (default) | You need a first pass or a set of sources for a general question. |
| `quick` | You need a quick lookup for a specific fact or API. |
| `semantic` | You know the concept and need related pages that may use different terms. |
| `fresh` | You need news or other pages from the past month. |
| `deep` | You need stronger sources for a difficult or multi-part question. |
| `page` | You want the provider's page Markdown in `.results[].snippet`. Use `--json`. |

```sh
websearch --mode semantic -n 4 'Go request cancellation during response body reads'
websearch --mode fresh --domain go.dev 'Go security release'
websearch --mode page --json -n 2 'Go release notes'
```

Each search sends its query to a web provider. Use public lookup terms. Treat
snippets as source material and verify claims against their URLs. The CLI
cannot fetch a URL directly, search private content, or synthesize a cited
answer. Exit 3 means there were no matches. Exit 7 means wait before retrying.
