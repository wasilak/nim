# AppStoreApps

Manage Mac App Store applications via the `mas` CLI.

```yaml
apiVersion: github.com/wasilak/nim/v1
kind: AppStoreApps
metadata:
  name: mac-apps
spec:
  apps:
    - id: 497799835
      name: Xcode
    - id: 1091189122
      name: Bear
    - id: 128919828
      name: Things
```

## Spec fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `apps` | []AppStoreApp | yes | List of Mac App Store apps to manage |

### AppStoreApp

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | int | yes | Mac App Store ADAM ID (numeric identifier, e.g. `497799835`) |
| `name` | string | no | Human-readable label for documentation (not used for matching) |

## Provider details

- **Provider name**: `appstore`
- **CLI dependency**: `mas` (install via `brew install mas`; macOS only)
- **Install**: `mas install <id>`
- **Uninstall**: `mas uninstall <id>`
- **Upgrade**: `mas upgrade <id>`

## Notes

- `mas` requires root privileges for install and uninstall operations. It prompts for sudo automatically.
- An Apple Account must be signed in to the App Store for install and upgrade operations.
- The `id` field is the ADAM ID — the numeric identifier from the Mac App Store. Find IDs with `mas search <name>`.
- The `name` field is optional and serves as a human-readable comment. It is stored in state metadata but not used for reconciliation.
- Version pinning is not supported. `mas` always installs the latest version.
- Installed versions are tracked in state for drift detection (`mas outdated`).

## Finding app IDs

```bash
# Search by name
$ mas search xcode
   497799835  Xcode

# List all installed apps
$ mas list
497799835  Xcode        (16.2)
1091189122  Bear         (2.0)
```

Use the numeric ID (first column) as the `id` field in your resource.