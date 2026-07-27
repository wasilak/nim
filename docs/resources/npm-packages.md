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
    # Run a package ephemerally via npx instead of installing it globally.
    - name: claude-code-templates
      version: latest
      executable: true
      args: ["--setting", "statusline/context-monitor", "--yes"]
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
| `executable` | bool | no | Run the package ephemerally via `npx` instead of `npm install -g`. |
| `args` | []string | no | Extra arguments passed to `npx` after the package (only with `executable: true`). |

## Provider details

- **Provider name**: `npm`
- **CLI dependency**: `npm` (auto-detected); `npx` for executable packages
- **Install**: `npm install -g <pkg>`
- **Uninstall**: `npm uninstall -g <pkg>`
- **Upgrade**: `npm install -g <pkg>@<version>`
- **Executable run**: `npx --yes <pkg>[@<version>] <args...>`

## Executable packages

Set `executable: true` for CLI installers that write their own files/config
(e.g. `claude-code-templates`) rather than living as a global binary. Instead of
installing globally, nim runs the package via `npx` on apply and tracks it in
state.

- Use `version: latest` (or omit) so re-running always pulls the newest release.
  Pinned-version *changes* are not re-routed through `npx`.
- `npx` side effects are **not reversible**: removing an executable package from
  config drops it from nim state but does not undo files it wrote — clean those
  up manually.
- `executable` / `args` are only valid on `NpmPackages`; other package kinds
  reject them.

## Notes

- Scoped packages (e.g. `@angular/cli`) are supported.
- Installed packages are discovered via `npm list -g --depth=0 --json`
  (executable packages are not globally installed, so they are tracked purely
  via nim state).