# HomeBrewPackages

Manage Homebrew formulae (command-line tools).

```yaml
apiVersion: github.com/wasilak/nim/v1
kind: HomeBrewPackages
metadata:
  name: core-tools
spec:
  formulae:
    - name: ripgrep
    - name: fzf
    - name: neovim
      version: "0.9.0"   # optional: pin to a specific version
```

## Spec fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `formulae` | []Package | yes | List of Homebrew formulae to manage |

### Package

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Formula name (e.g. `ripgrep`, `git`) |
| `version` | string | no | Pin to a specific version. Omit for latest. |

## Provider details

- **Provider name**: `homebrew`
- **CLI dependency**: `brew` (auto-detected)
- **Install**: `brew install <formula>`
- **Uninstall**: `brew uninstall <formula>`
- **Upgrade**: `brew reinstall <formula>`

## Notes

- Formulae are installed and uninstalled individually; batching is attempted but falls back to single installs on failure.
- Version pinning compares against installed version. Upgrades are performed via `brew reinstall`.
- Tap-qualified formula names (e.g. `dagger/tap/dagger`) are supported.