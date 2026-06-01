# ManagedFilePartial

Manage a subset of keys in JSON or YAML files — inject or update specific keys without overwriting the entire file.

```yaml
apiVersion: github.com/wasilak/nim/v1
kind: ManagedFilePartial
metadata:
  name: vscode-settings
spec:
  path: "{{ .Env.HOME }}/.config/Code/User/settings.json"
  keys:
    - key: "editor.fontSize"
      value: "14"
    - key: "editor.formatOnSave"
      value: "true"
    - key: "telemetry.enabled"
      value: "false"
```

## Spec fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `path` | string | yes | Absolute path to the file (supports template expressions) |
| `keys` | []PartialKey | yes | List of key-value pairs to manage |

### PartialKey

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `key` | string | yes | JSON/YAML key path (dot notation for nested keys) |
| `value` | string | no | Value to set (supports templates). Empty string is valid. |

## Provider details

- **Provider name**: `file`
- **No external CLI dependency**

## Notes

- Only the declared keys are managed. All other content in the file is left untouched.
- Key paths use dot notation for nested access (e.g. `editor.fontSize`).
- `path` supports template expressions (e.g. `{{ .Env.HOME }}`).
- Works with both JSON and YAML files (auto-detected by extension).