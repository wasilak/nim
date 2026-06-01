# HomeBrewTaps

Manage Homebrew taps (third-party repositories).

```yaml
apiVersion: github.com/wasilak/nim/v1
kind: HomeBrewTaps
metadata:
  name: taps
spec:
  taps:
    - name: homebrew/cask-fonts
    - name: dagger/tap
```

## Spec fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `taps` | []Tap | yes | List of Homebrew taps to manage |

### Tap

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Tap name (e.g. `homebrew/cask-fonts`, `dagger/tap`) |

## Provider details

- **Provider name**: `homebrew`
- **CLI dependency**: `brew` (auto-detected)
- **Install**: `brew tap <name>`
- **Uninstall**: `brew untap <name>`

## Notes

- Taps must be installed before any formulae that reference them. Nim automatically orders taps before formulae and casks during apply.
- Tap names with slashes (e.g. `stigoleg/tap`) are normalized — Homebrew strips the `homebrew-` prefix internally. Both forms are tracked.