# AppStoreApps — Design Spec

## Overview

Add a new resource kind `AppStoreApps` to Nim for managing Mac App Store applications via the `mas` CLI tool. This follows the existing provider pattern (npm, cargo, go) with `BaseReconcile` for reconciliation.

## Resource Kind

- **Constant**: `KindAppStoreApps = "AppStoreApps"`
- **Provider**: `AppStoreProvider`, registered as `"appstore"`
- **CLI dependency**: `mas` (installable via `brew install mas`)

## YAML Format

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
    - id: 128919828  # name is optional, used as a comment/label
      name: Things
```

### Field semantics

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | int | yes | Mac App Store numeric identifier. The primary key for reconciliation. |
| `name` | string | no | Human-readable label. Stored in state metadata but not used for matching. |

**Why `id` as the key?** The `mas` CLI operates exclusively on numeric IDs. App names can change or be ambiguous (`mas search` returns multiple results). IDs are deterministic and stable.

## Reconciliation

### Installed discovery

Use `mas list --json` for structured output (available in mas 7.0+). Each line is a JSON object:

```json
{"adamId":497799835,"bundleId":"com.apple.dt.Xcode","name":"Xcode","version":"16.2"}
{"adamId":1091189122,"bundleId":"net.shinyfrog.bear-mac","name":"Bear","version":"2.0"}
```

Parsed into `map[string]string` where key = `fmt.Sprintf("%d", adamId)`, value = version. This feeds directly into `BaseReconcile`.

Fallback: if `--json` fails (older mas versions), parse tabular `mas list` output:
```
497799835 Xcode (16.2)
```
Extract ID from first field, version from parenthesized suffix.

### Install

```bash
mas install 497799835
```

No version pinning — `mas` always installs the latest version. The `Version` field in spec is not supported (would be misleading).

Note: `mas install` requires root privileges and an Apple Account signed in to the App Store.

### Uninstall

```bash
mas uninstall 497799835
```

Note: `mas uninstall` requires root privileges.

### Upgrade (modifications)

For version drift detected in state, run:
```bash
mas upgrade 497799835
```

### Availability check

`Available()` checks for `mas` binary in PATH. If missing, suggests `brew install mas`.

## Data Model

### Resource struct (`pkg/resource/app_store.go`)

```go
type AppStoreApps struct {
    BaseResource `yaml:",inline"`
    Spec         AppStoreAppsSpec `yaml:"spec" validate:"required"`
}

type AppStoreAppsSpec struct {
    Apps []AppStoreApp `yaml:"apps" validate:"required,dive"`
}

type AppStoreApp struct {
    ID   int    `yaml:"id" validate:"required"`
    Name string `yaml:"name,omitempty"`
}
```

### ToGroup mapping

In `ToGroup()`, `ResourceItem.Name` holds `fmt.Sprintf("%d", app.ID)` and `ResourceItem.Metadata["display_name"]` holds `app.Name`. This ensures reconciliation matches on stable IDs while preserving human-readable names for display.

## Files to Create

| File | Purpose |
|------|---------|
| `pkg/resource/app_store.go` | Resource type, spec, ToGroup, Validate |
| `pkg/providers/appstore.go` | AppStoreProvider: Available, Reconcile, Apply, InstalledForKind, Import |
| `pkg/providers/appstore_test.go` | Unit tests following existing test patterns |

## Files to Modify

| File | Change |
|------|--------|
| `pkg/resource/constants.go` | Add `KindAppStoreApps` constant |
| `pkg/resource/unmarshal.go` | Add case in switch; add to `ValidResourceKinds()` |
| `cmd/state.go` | Register `AppStoreProvider` in `ensureProvidersRegistered()` |
| `pkg/engine/stats.go` | Add to `allKinds` and `coverageKinds` |
| `README.md` | Add AppStoreApps resource documentation section |

## Provider Implementation Detail

### `AppStoreProvider` struct

```go
type AppStoreProvider struct{}
```

No HTTP client needed (unlike BrewProvider). All operations go through `cmdutil.RunSimpleFn` calling `mas`.

### Key methods

- **`Name()`** → `"appstore"`
- **`Available()`** → checks `mas` in PATH
- **`Reconcile()`** → delegates to `BaseReconcile(KindAppStoreApps, ...)` using `getInstalledApps(ctx)`
- **`Apply()`** → iterates plan additions/removals/modifications
- **`InstalledForKind()`** → returns `getInstalledApps(ctx)` for coverage stats
- **`Import()`** → calls `mas list` and returns all installed apps as state

### `getInstalledApps`

Parses `mas list` output:
```
497799835 Xcode (16.2)
```
→ `map[string]string{"497799835": "16.2"}`

Falls back to empty map on error (logged as warning).

### Apply operations

| Operation | Command |
|-----------|---------|
| Install | `mas install <id>` |
| Uninstall | `mas uninstall <id>` |
| Upgrade | `mas upgrade <id>` |

No batching — `mas` doesn't support multi-app install/uninstall. Each item processed sequentially, like taps.

## Constraints

- **macOS only**: `mas` only works on macOS. `Available()` returns false on Linux with an appropriate message.
- **No version pinning**: `mas install` always installs latest. Spec does not include a version field.
- **ID is integer**: The `id` field is `int` in the struct, serialized as YAML integer. This prevents accidental string/number confusion.
- **State tracking**: Version is stored in state (from `mas list`) but cannot be specified in desired config.

## Testing

Follow existing patterns from `pkg/providers/brew_test.go` and `pkg/providers/npm.go` (no unit tests in that file, but brew_test.go has tests).

- Mock `mas list` output for reconciliation tests
- Mock `mas install`/`mas uninstall` for apply tests
- Test ToGroup conversion (ID → string key, name → metadata)
- Test Import with `mas list` output
- Test Available() with/without mas binary