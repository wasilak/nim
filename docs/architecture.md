# Architecture

Nim brings Terraform-inspired plan/apply workflows to dotfiles and package management.

## High-level flow

```mermaid
flowchart LR
    A[Config Files<br>YAML + Templates] --> B[Template Engine<br>Sprig]
    B --> C[Desired State]
    C --> D[Reconciler]
    E[Current State<br>state.json] --> D
    F[System State<br>brew/npm/mas/...<br>discovery] --> D
    D --> G[GroupPlan<br>add/modify/remove/cleanup]
    G --> H{plan or apply?}
    H -->|plan| I[Diff Display]
    H -->|apply| J[Execute Changes<br>+ Update State]
```

## Resource lifecycle

```mermaid
stateDiagram-v2
    [*] --> Discovered: nim plan/apply reads config
    Discovered --> Planned: Reconciler computes diff
    Planned --> Applied: nim apply --confirm
    Applied --> InSync: next plan shows no changes
    InSync --> Drifted: manual change on system
    Drifted --> Planned: nim plan detects drift
```

## Component architecture

```mermaid
flowchart TB
    subgraph CLI
        cmd[cmd/root.go<br>cmd/plan.go<br>cmd/apply.go<br>cmd/state.go]
    end

    subgraph Engine["pkg/engine"]
        engine[Engine]
        graph[Dependency Graph<br>pkg/graph]
        stats[Stats]
    end

    subgraph Providers["pkg/providers"]
        brew[BrewProvider]
        npm[NpmProvider]
        go[GoProvider]
        cargo[CargoProvider]
        file[FileProvider]
        aiskill[AISkillProvider]
        appstore[AppStoreProvider]
    end

    subgraph Resources["pkg/resource"]
        res[Resource Types<br>YAML → structs]
        loader[Loader<br>filesystem → resources]
    end

    subgraph State["pkg/state"]
        local[LocalBackend<br>state.json]
        s3[S3Backend]
    end

    cmd --> engine
    engine --> Providers
    engine --> Resources
    engine --> State
    Providers --> State
```

## State backends

| Backend | Implementation | Config |
|---------|---------------|--------|
| Local JSON | `pkg/state/local.go` | `state.backend: local` |
| S3-compatible | `pkg/state/s3.go` | `state.backend: s3` with endpoint, bucket, key, region |

## Provider interface

Every provider implements:

```go
type Provider interface {
    Name() string
    Available() (bool, string)
    Reconcile(ctx, desired, state) GroupPlan
    Apply(ctx, GroupPlan) ([]ApplyItemResult, error)
}
```

Optional extensions:
- `CoverageProvider` — reports installed items (`InstalledForKind`)
- `Importer` — discovers existing state (`Import`)

## Dependency graph

Resources can declare `dependsOn` in metadata. Nim builds a DAG and applies in topological order:

```yaml
metadata:
  name: my-app
  dependsOn:
    - HomeBrewPackages/core-tools
```

```mermaid
flowchart TD
    Taps --> Formulae
    Taps --> Casks
    Formulae --> ManagedFiles
    Casks --> ManagedFiles
```