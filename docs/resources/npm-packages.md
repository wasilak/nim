# NpmPackages

Manage global npm packages.

```yaml
apiVersion: github.com/wasilak/nim/v1
kind: NpmPackages
metadata:
  name: js-tools
spec:
  packages:
    - name: typescript
    - name: prettier
      version: "3.1.0"
    - name: "@angular/cli"
```

## Spec fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `packages` | []Package | yes | List of npm packages to manage globally |

### Package

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Package name (e.g. `typescript`, `@angular/cli`) |
| `version` | string | no | Pin to a specific version. Omit for latest. |

## Provider details

- **Provider name**: `npm`
- **CLI dependency**: `npm` (auto-detected)
- **Install**: `npm install -g <pkg>`
- **Uninstall**: `npm uninstall -g <pkg>`
- **Upgrade**: `npm install -g <pkg>@<version>`

## Notes

- Scoped packages (e.g. `@angular/cli`) are supported.
- Installed packages are discovered via `npm list -g --depth=0 --json`.