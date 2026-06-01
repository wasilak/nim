# Resources

Nim manages system resources through a declarative YAML interface. Each resource kind has its own provider that handles discovery, reconciliation, and application.

## Supported resource kinds

| Kind | Description | Provider | CLI Dependency |
|------|-------------|----------|---------------|
| `AppStoreApps` | Mac App Store applications | `appstore` | `mas` |
| `AISkillPackages` | AI agent skill packages | `aiskill` | `npx` |
| `CargoPackages` | Rust CLI tools | `cargo` | `cargo` |
| `GoPackages` | Go CLI tools | `go` | `go` |
| `HomeBrewCasks` | Homebrew casks (GUI apps) | `homebrew` | `brew` |
| `HomeBrewPackages` | Homebrew formulae (CLI tools) | `homebrew` | `brew` |
| `HomeBrewTaps` | Homebrew taps (repos) | `homebrew` | `brew` |
| `ManagedFile` | Individual files with templating | `file` | — |
| `ManagedFilePartial` | Key-level file management | `file` | — |
| `NpmPackages` | Global npm packages | `npm` | `npm` |

## Common structure

Every resource follows the same skeleton:

```yaml
apiVersion: github.com/wasilak/nim/v1
kind: <Kind>
metadata:
  name: <group-name>
  namespace: <optional-namespace>   # exact match or /regex/
  labels: <optional-key-value-pairs>
  dependsOn:                        # optional dependency list
    - <Kind>/<group-name>
spec:
  <kind-specific-fields>
```

 ## Namespaces

Set `metadata.namespace` to control which resources apply on which machines:

```bash
# Only resources matching this namespace are applied
nim apply --namespace work
NIM_NAMESPACE=personal nim plan
```

See [../namespaces.md](namespaces.md) for full details.

## Detailed resource docs

- [AppStoreApps](resources/appstore-apps.md)
- [AISkillPackages](resources/aiskill-packages.md)
- [CargoPackages](resources/cargo-packages.md)
- [GoPackages](resources/go-packages.md)
- [HomeBrewCasks](resources/homebrew-casks.md)
- [HomeBrewPackages](resources/homebrew-packages.md)
- [HomeBrewTaps](resources/homebrew-taps.md)
- [ManagedFile](resources/managedfile.md)
- [ManagedFilePartial](resources/managedfile-partial.md)
- [NpmPackages](resources/npm-packages.md)