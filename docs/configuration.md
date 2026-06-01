# Configuration

## Config file: `~/.nim/config.yaml`

```yaml
# Dotfiles location (default: ~/.config/nim)
dotfiles_root: ~/projects/dotfiles

# State backend (local or S3)
state:
  backend: s3
  s3:
    endpoint: s3.amazonaws.com
    bucket: my-nim-state
    key: state.json
    region: us-east-1
    access_key_id: ${AWS_ACCESS_KEY_ID}
    secret_access_key: ${AWS_SECRET_ACCESS_KEY}

# Log level
log_level: info
```

## Values: `~/.config/nim/values.yaml`

User-defined template variables:

```yaml
user: "{{ .Env.USER }}"
home: "{{ .Env.HOME }}"
hostname: "{{ .OS.Hostname }}"
arch: "{{ .OS.Arch }}"
full_name: Piotr
email: piotr@example.com
editor: nvim
```

## Template context

Available variables in resource `source` fields (when `template: true`):

| Variable | Description |
|----------|-------------|
| `{{ .Values }}` | All keys from `values.yaml` |
| `{{ .Env.VAR }}` | Environment variables |
| `{{ .OS.Hostname }}` | System hostname |
| `{{ .OS.Arch }}` | Architecture (amd64, arm64) |
| `{{ .OS.GOOS }}` | Operating system (darwin, linux) |
| `{{ .Namespace }}` | Active namespace |

[Sprig template functions](https://masterminds.github.io/sprig/) are available (e.g. `default`, `trim`, `regexMatch`).

## State backends

| Backend | Setting | Description |
|---------|---------|-------------|
| Local | `state.backend: local` | Stores state in `state.json` next to config (default) |
| S3 | `state.backend: s3` | Stores state in an S3-compatible bucket |

Manage state with:

```bash
nim state list                   # Show all tracked resources
nim state pull                   # Download from S3
nim state push                   # Upload to S3
nim state import <Kind>/<group>[<item>]  # Import existing item
nim state move <src> <dest>      # Move item between groups
nim state remove <Kind>/<group>[<item>]  # Remove from state
```