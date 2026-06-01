# CargoPackages

Manage Rust CLI tools installed via `cargo install`.

```yaml
apiVersion: github.com/wasilak/nim/v1
kind: CargoPackages
metadata:
  name: rust-tools
spec:
  packages:
    - name: ripgrep
    - name: fd-find
      version: "0.1.0"
```

## Spec fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `packages` | []Package | yes | List of Cargo packages to install |

### Package

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Crate name (e.g. `ripgrep`, `fd-find`) |
| `version` | string | no | Pin to a specific version. Omit for latest. |

## Provider details

- **Provider name**: `cargo`
- **CLI dependency**: `cargo` (auto-detected; requires Rust from https://rustup.rs/)
- **Install**: `cargo install <name>`
- **Uninstall**: `cargo uninstall <name>`

## Notes

- Cargo binaries may have different names than their crate names (e.g. crate `fd-find` installs binary `fd`). Nim tracks by crate name.