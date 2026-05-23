# nim — Suggested Commands

## Development

```bash
# Fast local test (no Dagger)
go test ./...
go test -tags integration ./...
go vet ./...

# Via Dagger (CI-accurate, slower)
make test    # runs go test inside Dagger container
make vet     # runs go vet inside Dagger container
make build   # cross-compiles all targets → dist/
make ci      # full pipeline: vet + test + build
make clean   # rm -rf dist/
```

## Running nim locally

```bash
go run . plan     # plan without building binary
go run . apply
go run . state list
go run . diff
go run . doctor
go run . pull
```

## Tool checks (Darwin-specific)

```bash
# Use rg (ripgrep) instead of grep — enforced by permissions
rg 'pattern' ./pkg/

# Use fd instead of find
fd --type f --extension go ./pkg/
```

## Useful one-liners

```bash
# Check for context.Background() violations in pkg/
rg 'context\.Background\(\)' pkg/

# List all provider kinds
fd --type f --extension go pkg/providers/ --max-depth 1
```
