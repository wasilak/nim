# Namespaces

Namespaces let you apply different resources on different machines — e.g. work vs. personal laptops.

## Declaring namespaces

Add `metadata.namespace` to any resource:

```yaml
# Only applied when active namespace is "work"
apiVersion: github.com/wasilak/nim/v1
kind: HomeBrewPackages
metadata:
  name: work-tools
  namespace: work
spec:
  formulae:
    - name: docker
    - name: kubectl
---
# Regex match — applied for "personal", "personal-laptop", etc.
apiVersion: github.com/wasilak/nim/v1
kind: HomeBrewCasks
metadata:
  name: personal-apps
  namespace: /personal.*/
spec:
  casks:
    - name: spotify
```

Resources **without** a namespace belong to the `"default"` namespace.

## Setting the active namespace

```bash
# Via environment variable
export NIM_NAMESPACE=work
nim plan

# Via CLI flag (takes precedence)
nim plan --namespace work
nim apply --namespace work --confirm

# Regex active namespace
NIM_NAMESPACE="/(work|personal)/" nim plan
```

Only resources whose namespace matches the active namespace are included in plan/apply.

## Namespace templating

Access the active namespace in templates:

```yaml
apiVersion: github.com/wasilak/nim/v1
kind: ManagedFile
metadata:
  name: gitconfig-{{ .Namespace }}
spec:
  destination: ~/.gitconfig.{{ .Namespace }}
  source: |
    [user]
        name = {{ .Values.full_name }}
        {{ if eq .Namespace "work" }}email = work@company.com{{ end }}
        {{ if eq .Namespace "personal" }}email = personal@example.com{{ end }}
  template: true
```

## Matching rules

| Resource namespace | Active namespace | Match? |
|---|---|---|
| (empty/absent) | any | only if active NS is `"default"` |
| `"work"` | `"work"` | exact match |
| `"/personal.*/"` | `"personal-laptop"` | regex match |
| `"/(work|personal)/"` | `"work"` | alternation match |
| `"/(work|personal)/"` | `"home"` | no match |