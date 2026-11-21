# websearch v1

`websearch [flags] QUERY` searches the configured LiteLLM search proxy. The
mode selects a named route and provider settings. An explicit provider replaces
the mode preset and uses that provider's defaults. There is one request per
invocation and a 25-second HTTP deadline. Queries and credentials never appear in
CLI error messages.

- `--mode` defaults to `fast`. Other modes cover quick lookups, semantic
  matches, fresh information, harder searches, and provider-returned page text.
- `--provider` accepts `parallel`, `perplexity`, `exa`, `brave`, `firecrawl`, or `tavily`.
- `-n` / `--max-results` accepts 1–20 and defaults to 5.
- `--domain` is repeatable, up to 20 domains. `--country` takes a two-letter
  country code where supported.
- Compact output contains a title, URL, and bounded snippet per result. `--json`
  returns the full successful proxy response, including extra provider fields.
- Credentials resolve from `VOSKA_AI_KEY`, `~/.config/voska/ai-key`, then the
  login Keychain on macOS. A configured file must be regular and readable only
  by its owner. Invalid configured credentials fail with actionable diagnostics.

Go and Kong provide the command line. The HTTP client, auth lookup, and command
wiring live in `internal/search`, `internal/auth`, and `internal/cli`.
Builds target macOS and Linux on arm64 and amd64.
