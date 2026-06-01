# AISkillPackages

Install AI agent skill packages from GitHub repositories via the `skills` CLI.

```yaml
apiVersion: github.com/wasilak/nim/v1
kind: AISkillPackages
metadata:
  name: my-skills
spec:
  packages:
    - source: "Ar9av/obsidian-wiki"
      targets:
        - claude
        - opencode
    - source: "some-org/another-skill"
```

## Spec fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `packages` | []AISkillPackage | yes | List of skill packages to install |

### AISkillPackage

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `source` | string | yes | GitHub repo slug or full URL (e.g. `Ar9av/obsidian-wiki`) |
| `targets` | []string | no | Agent targets (e.g. `["claude", "opencode"]`). Default: all detected agents. |

## Provider details

- **Provider name**: `aiskill`
- **CLI dependency**: `npx` (Node.js; auto-detected)
- **Install**: `npx --yes skills add <source> --global --all`
- **Uninstall**: `npx --yes skills remove --global --all --skill <names>`

## Notes

- The `skills` CLI is invoked via `npx`, so Node.js must be installed.
- `source` is the GitHub repository slug — the unique key for reconciliation.
- Import is not supported (the skills CLI does not expose source repos for installed skills).