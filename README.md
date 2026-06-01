# Nim 🏠⚡

> **Declarative dotfiles management for developers who treat their environment like infrastructure.**

[![Go Version](https://img.shields.io/badge/go-1.26+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Nim brings Terraform-inspired workflows to your dotfiles. Declare your desired state in YAML, compute diffs, and apply changes — including **clean removals** when you stop using a tool.

```bash
$ nim plan          # See what would change
$ nim apply --confirm  # Apply with confidence
```

## Why Nim?

| Feature | Chezmoi | Nim |
|---------|---------|-----|
| Forward sync | ✅ | ✅ |
| **Clean removals** | ❌ | ✅ |
| **Package management** | ❌ | ✅ |
| **Namespace support** | ❌ | ✅ |
| Drift detection | ❌ | ✅ |
| State tracking | ❌ | ✅ |

Traditional dotfile managers (chezmoi, stow) apply changes forward but never clean up. Nim tracks every managed resource — when you remove it from config, Nim removes it from your system.

## Quick start

```bash
# Install
go install github.com/wasilak/nim@latest

# Initialize
nim init

# Define resources in ~/.config/nim/resources/
# ...

# Preview & apply
nim plan
nim apply --confirm
```

## Supported resources

| Kind | Description | Docs |
|------|-------------|------|
| `AppStoreApps` | Mac App Store apps (via `mas`) | [→](docs/resources/appstore-apps.md) |
| `AISkillPackages` | AI skill packages | [→](docs/resources/aiskill-packages.md) |
| `CargoPackages` | Rust CLI tools | [→](docs/resources/cargo-packages.md) |
| `GoPackages` | Go CLI tools | [→](docs/resources/go-packages.md) |
| `HomeBrewCasks` | Homebrew casks | [→](docs/resources/homebrew-casks.md) |
| `HomeBrewPackages` | Homebrew formulae | [→](docs/resources/homebrew-packages.md) |
| `HomeBrewTaps` | Homebrew taps | [→](docs/resources/homebrew-taps.md) |
| `ManagedFile` | Files with templating | [→](docs/resources/managedfile.md) |
| `ManagedFilePartial` | Key-level file management | [→](docs/resources/managedfile-partial.md) |
| `NpmPackages` | Global npm packages | [→](docs/resources/npm-packages.md) |

## Example

```yaml
# ~/.config/nim/resources/dev-tools.yaml
apiVersion: github.com/wasilak/nim/v1
kind: HomeBrewPackages
metadata:
  name: core-tools
spec:
  formulae:
    - name: git
    - name: ripgrep
    - name: neovim
---
apiVersion: github.com/wasilak/nim/v1
kind: HomeBrewCasks
metadata:
  name: apps
spec:
  casks:
    - name: raycast
    - name: rectangle
---
apiVersion: github.com/wasilak/nim/v1
kind: ManagedFile
metadata:
  name: gitconfig
spec:
  source: |
    [user]
        name = {{ .Values.full_name }}
        email = {{ .Values.email }}
  destination: ~/.gitconfig
  mode: "0644"
  template: true
```

## Documentation

- **[Resources](docs/resources.md)** — all resource kinds with YAML examples
- **[Architecture](docs/architecture.md)** — how Nim works (Mermaid diagrams)
- **[Configuration](docs/configuration.md)** — config.yaml, values.yaml, state backends
- **[Namespaces](docs/namespaces.md)** — per-machine resource sets
- **[CLI Reference](docs/cli-reference.md)** — all commands

## Development

```bash
git clone https://github.com/wasilak/nim.git
cd nim
go build -o nim .
go test ./...
```

## License

[MIT](LICENSE)