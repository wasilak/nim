# nim — Task Completion Checklist

Run these before considering a coding task done:

```bash
# 1. Vet (mandatory gate — same as CI)
go vet ./...

# 2. Unit tests
go test ./...

# 3. Integration tests (if touching providers, state, or engine)
go test -tags integration ./...

# 4. Context usage check (auto-runs as part of go test ./tools/...)
go test ./tools/...
```

## Notes

- No linter beyond `go vet` — do not add golangci-lint calls
- `tools/check_context_usage_test.go` enforces no `context.Background()` in `pkg/` — runs as a test
- Dagger (`make test`) is the CI-accurate path but slow; `go test ./...` locally is sufficient for day-to-day
- No formatter step needed beyond `gofmt` (handled by editor/goimports)
