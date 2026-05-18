# Nim

## What This Is

Nim is a declarative dotfiles and machine configuration manager for personal use. It reads YAML resource manifests, computes a diff against persisted state, and applies changes through typed providers — bringing Terraform's plan/apply workflow to dotfiles. The current milestone (v0.16.0) adds four quality-of-life improvements: automatic directory creation, post-apply caveats, partial YAML/JSON file management, and process locking.

## Core Value

A user can manage their dotfiles declaratively — with safe plan/apply previews, namespace-scoped machine profiles, and robust file handling — without duplicating manifests or risking concurrent mutations.

## Requirements

### Validated

- ✓ Declarative plan/apply model with diff preview — Phase 1
- ✓ Resource manifests as Kubernetes-style YAML (kind, metadata, spec) — Phase 1
- ✓ 8 built-in providers: ManagedFile, HomeBrewPackages, Casks, Taps, NpmPackages, GoPackages, CargoPackages, AISkill — Phase 1
- ✓ DAG dependency ordering via `metadata.dependsOn` — Phase 1
- ✓ Two-pass Go template rendering (values.yaml + env vars, Sprig functions) — Phase 1
- ✓ Local JSON and S3-compatible state backends — Phase 1
- ✓ `--target` flag with `/pattern/` regex support — Phase 1
- ✓ `metadata.namespace` on resource manifests with regex matching — v0.15.0
- ✓ Active namespace resolved from `NIM_NAMESPACE` env var or `--namespace` flag — v0.15.0
- ✓ `{{ .Namespace }}` in template context for conditional rendering — v0.15.0

### Active

- [ ] ManagedFile provider creates all parent directories automatically before writing a file
- [ ] All resource kinds support a `spec.notes` field (caveats); notes are printed after a successful `nim apply`
- [ ] New resource kind `ManagedFilePartial` manages a subset of keys in an existing JSON or YAML file without touching other keys
- [ ] `nim plan` and `nim apply` acquire a process-level lock on startup; a second invocation prints a friendly error and exits immediately

### Out of Scope

- Multiple simultaneous active namespaces — one namespace active per invocation
- Namespace-scoped state backends — state is shared; namespace is a filter
- Hostname-based auto-detection — explicit `NIM_NAMESPACE` is more predictable
- Full JSON/YAML file ownership via `ManagedFilePartial` — only declared keys are managed; all other keys are preserved
- Recursive partial management (nested key paths beyond top-level keys) — flat key list only for v0.16.0

## Context

**Existing codebase state (post v0.15.0):**
- `pkg/providers/file.go` — ManagedFile provider; does not currently create parent dirs
- `pkg/engine/apply.go` — apply loop; good place to collect and print post-apply notes
- `pkg/resource/resource.go` — Resource struct; notes field needs to be added to the base or per-kind spec
- No existing locking mechanism; `cmd/root.go` or `cmd/plan_apply.go` is the right acquisition point
- `pkg/providers/` — provider registry; new `ManagedFilePartial` provider goes here

**Known concerns carried forward (not in scope):**
- Non-atomic state writes in `pkg/state/local.go`
- AISkillProvider idempotency bug

## Constraints

- **Compatibility**: Existing manifests with no `spec.notes` or no parent dir must continue to work — zero breaking changes
- **Tech stack**: Go 1.26, Cobra CLI, Sprig templates — no new dependencies preferred
- **Testing**: stdlib `testing` only; table-driven tests following existing patterns
- **Architecture**: No `context.Background()` in `pkg/`; wrap all errors with `fmt.Errorf("...: %w", err)`
- **Partial locking**: Process-based lock preferred over file lock; friendly, colorful error message on contention

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| `spec.notes` on all resource kinds | Consistent UX — any resource can have a caveat, not just packages | — Pending |
| Notes printed after successful apply only | Avoids noise on dry-run plan output | — Pending |
| `ManagedFilePartial` as new kind (not extending ManagedFile) | Clean separation — full ownership vs. partial ownership have different semantics | — Pending |
| Process lock at CLI entry point (cmd/) not in pkg/ | Lock is a CLI concern; library code stays lock-agnostic | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd:complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-05-18 — new milestone v0.16.0*
