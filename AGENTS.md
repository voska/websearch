# websearch

- Keep provider selection and verified options in `internal/search`.
- Keep stdout for results and stderr for diagnostics. Preserve CLI exit codes
  and pass through successful JSON responses, including unfamiliar fields.
- Use mock HTTP servers for tests. Live checks use public queries and never
  print or record credentials.
- Run `make ci` before proposing a change.
