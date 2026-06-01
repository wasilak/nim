# HomeBrewCasks

Manage Homebrew casks (GUI applications).

```yaml
apiVersion: github.com/wasilak/nim/v1
kind: HomeBrewCasks
metadata:
  name: apps
spec:
  casks:
    - name: raycast
    - name: warp
    - name: rectangle
```

## Spec fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `casks` | []Package | yes | List of Homebrew casks to manage |

### Package

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Cask name (e.g. `raycast`, `firefox`) |
| `version` | string | no | Pin to a specific version. Omit for latest. |

## Provider details

- **Provider name**: `homebrew`
- **CLI dependency**: `brew` (auto-detected)
- **Install**: `brew install --cask <cask>`
- **Uninstall**: `brew uninstall --cask <cask>`

## Notes

- Casks use `--cask` flag to disambiguate from formulae when a name exists as both.
- Version detection reads from the Caskroom directory to avoid Homebrew API metadata bugs.
- Already-installed casks are skipped during apply to avoid triggering API bugs.