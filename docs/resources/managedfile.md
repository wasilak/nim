# ManagedFile

Manage individual files with templating support — the core resource for dotfiles.

```yaml
apiVersion: github.com/wasilak/nim/v1
kind: ManagedFile
metadata:
  name: zshrc
spec:
  source: |
    # {{ .Values.shell }} config
    export EDITOR={{ .Values.editor }}

  destination: ~/.zshrc
  mode: "0644"
  template: true
```

## Spec fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `source` | string | yes* | Inline file content (use `\|` for multi-line) |
| `sourceFile` | string | yes* | Path to external file (relative to dotfiles root) |
| `destination` | string | yes | Absolute path where the file will be rendered |
| `mode` | string | no | Unix file permissions (e.g. `"0644"`, `"0755"`). Default: `"0644"` |
| `template` | bool | no | Enable Go templating with Sprig functions. Default: `false` |

> *One of `source` or `sourceFile` is required.

## Provider details

- **Provider name**: `file`
- **No external CLI dependency**

## Notes

- Templates have access to `{{ .Values }}` (from `values.yaml`), `{{ .Env }}`, `{{ .OS }}`, and `{{ .Namespace }}`.
- `sourceFile` is resolved relative to the dotfiles root (`~/.config/nim` by default).
- When `template: false`, the source is written verbatim without any processing.
- File diffs are shown in plan output when using `--diff`.
- For partial file management (injecting blocks into existing files), see [ManagedFilePartial](managedfile-partial.md).