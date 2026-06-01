# GoPackages

Manage Go CLI tools installed via `go install`.

```yaml
apiVersion: github.com/wasilak/nim/v1
kind: GoPackages
metadata:
  name: go-tools
spec:
  packages:
    - module: golang.org/x/tools/gopls
      version: v0.15.0
    - module: github.com/golangci/golangci-lint/cmd/golangci-lint
```

## Spec fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `packages` | []GoPackage | yes | List of Go modules to install |

### GoPackage

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `module` | string | yes | Full module path (e.g. `golang.org/x/tools/gopls`) |
| `version` | string | no | Version tag (e.g. `v0.15.0`, `latest`). Default: latest. |
| `dependsOn` | []string | no | Resource dependencies |

## Provider details

- **Provider name**: `go`
- **CLI dependency**: `go` (auto-detected)
- **Install**: `go install <module>@<version>`
- **Uninstall**: removes binary from `$(go env GOPATH)/bin`

## Notes

- Go packages are identified by their full module path, not a short name.
- Uninstall removes the binary but does not clean module caches.