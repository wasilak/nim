// Package provider defines the Provider interface and registry for nim.
//
// Providers are the core abstraction that enables nim to manage different
// types of resources (files, packages, etc.). Each provider implements the
// Provider interface and registers itself with the global registry.
//
// Example provider implementations:
//   - FileProvider: Manages files and directories
//   - BrewProvider: Manages Homebrew packages
//   - NpmProvider: Manages npm packages
//   - GoProvider: Manages Go modules
//   - CargoProvider: Manages Rust crates
package provider

import (
	"context"

	"github.com/wasilak/nim/pkg/resource"
)

// ItemStatus represents the apply-time outcome for a single item.
type ItemStatus string

const (
	// ItemSkipped means the item was not applied because a dependency failed.
	ItemSkipped ItemStatus = "skipped"
	// ItemApplied means the item was applied successfully.
	ItemApplied ItemStatus = "applied"
	// ItemFailed means the item failed to apply.
	ItemFailed ItemStatus = "failed"
)

// GroupPlan represents the changes needed to reconcile desired state with actual state.
// Organized by resource groups (3-level hierarchy: Kind -> Group -> Items)
type GroupPlan struct {
	// Additions are groups/items that need to be created
	Additions []GroupAddition

	// Modifications are groups that need item-level updates
	Modifications []GroupModification

	// Removals are groups/items that need to be deleted
	Removals []GroupRemoval

	// Cleanup are items that exist in state but not in config or system.
	// These will be removed from state only (no system changes).
	Cleanup []GroupCleanup

	// InSync are groups that match desired state
	InSync []GroupState

	// Drifted are items that have changed outside of nim's management
	Drifted []ItemDrift

	// Warnings are provider-generated advisory messages that do not block apply
	Warnings []PlanWarning

	// Skipped are groups that were not applied because a dependency failed.
	Skipped []GroupSkip

	// Errors are non-fatal errors encountered during reconcile
	Errors []error
}

// GroupAddition represents items to add within a resource group
type GroupAddition struct {
	Kind     string
	Group    string
	Items    []resource.ResourceItem
	Contents map[string]string // item name → content (for additions, when diff enabled)
	RawSpec  any               // provider-specific spec data (e.g., ManagedFilePartialSpec)
}

// GroupModification represents changes within an existing group
type GroupModification struct {
	Kind    string
	Group   string
	Changes []ItemChange
	RawSpec any // provider-specific spec data (e.g., ManagedFilePartialSpec)
}

// ItemChange represents a change to a specific item
type ItemChange struct {
	ItemName   string
	OldState   resource.ItemState
	NewState   resource.ItemState
	Diff       string
	OldContent string // for files, pre-change content if diff enabled
	NewContent string // for files, post-change (desired) content if diff enabled
}

// GroupRemoval represents items to remove from a group
type GroupRemoval struct {
	Kind     string
	Group    string
	Items    []resource.ResourceItem
	Contents map[string]string // item name → removed content (for removals, when diff enabled)
}

// GroupCleanup represents items that exist in state but not in config or system.
// These will be removed from state only (no system changes).
type GroupCleanup struct {
	Kind   string
	Group  string
	Items  []resource.ResourceItem
	Reason string // e.g., "not_in_config_and_not_installed"
}

// GroupState represents a group that is in sync
type GroupState struct {
	Kind    string
	Group   string
	Items   []resource.ItemState
	Version string
}

// ItemDrift represents an item that has drifted from expected state
type ItemDrift struct {
	Kind          string
	Group         string
	Item          string
	ExpectedState resource.ItemState
	ActualState   resource.ItemState
	Description   string
	Diff          string
}

// GroupSkip records a resource group that was skipped during apply due to a dependency failure.
type GroupSkip struct {
	Kind   string
	Group  string
	Reason string
}

// ImportCandidate represents a single resource that can be auto-imported.
type ImportCandidate struct {
	// ID is the canonical resource identifier, e.g. "GoPackages/go-dev-tools[gopls]"
	ID string

	// ActualValue is the real system value when it differs from the item name,
	// e.g. the destination path for ManagedFile imports.
	ActualValue string
}

// PlanWarning represents a non-blocking advisory produced during reconcile.
// It can point to a resource and optionally include a suggestion (copy-pasteable).
type PlanWarning struct {
	// GroupID is an optional kind/group identifier (e.g., "BrewPackages/core-tools")
	GroupID string

	// ItemID is an optional item identifier (e.g., "ripgrep")
	ItemID string

	// Severity indicates importance: "warning" or "info"
	Severity string

	// Message is a human-friendly description of the issue
	Message string

	// Suggestion is an optional copy-pasteable command or snippet
	Suggestion string

	// ImportItems lists structured candidates for "nim state import all".
	// Populated when the warning describes resources that can be auto-imported.
	ImportItems []ImportCandidate
}

// ResourceState represents the state of a resource group as tracked by nim.
// Uses 3-level hierarchy: Kind -> Group -> Items
type ResourceState struct {
	// Kind is the resource type (e.g., "HomeBrewPackages")
	Kind string `json:"kind"`

	// Group is the resource group name (e.g., "core-tools")
	Group string `json:"group"`

	// Items are the individual items within this group
	Items []resource.ItemState `json:"items"`
}

// ApplyItemResult reports the outcome for a single item within an Apply call.
// Per-item failures are reported here rather than as a fatal error return.
type ApplyItemResult struct {
	Kind  string
	Group string
	Item  string
	Op    string // "add", "remove", "modify"
	Err   error  // nil = success
}

// Provider is the interface implemented by all resource providers.
// Each provider knows how to manage a specific type of resource.
type Provider interface {
	// Name returns the provider name (e.g., "brew", "file", "npm")
	Name() string

	// Available checks if the provider can operate on this system.
	// Returns true if available, false with a descriptive message if not.
	Available() (bool, string)

	// Reconcile compares the desired resource groups with the current system state
	// and returns a plan of changes needed to reach the desired state.
	// The state parameter contains the previously saved state for these resources.
	// Reconcile now accepts a context so providers can perform cancellable
	// operations (running external commands, I/O) and observe cancellation from
	// the caller.
	Reconcile(ctx context.Context, desired []resource.ResourceGroup[any], state []ResourceState) GroupPlan

	// Apply executes the given plan, making actual changes to the system.
	// Returns per-item results so callers can track individual successes and
	// failures. The error return is reserved for fatal infrastructure failures
	// (e.g. the tool binary is missing); per-item failures go in ApplyItemResult.Err.
	Apply(ctx context.Context, plan GroupPlan) ([]ApplyItemResult, error)

	// Import discovers an existing resource on the system and returns its state.
	// This is used by the `state import` command to bring unmanaged resources
	// under nim's control. NOTE: provider-level ImportItem was removed —
	// CLI import functionality (and provider-level import helpers) are no
	// longer supported; providers should expose only Reconcile/Apply.
	Import(ctx context.Context, group string) (ResourceState, error)
}

// CoverageProvider is an optional interface for providers that can enumerate
// all items currently installed on the system for a given resource kind.
// Used by 'nim stats --all' to compute nim coverage per kind.
type CoverageProvider interface {
	// InstalledForKind returns the full set of installed items for the given kind.
	// Returns nil, nil when the kind is not handled by this provider.
	InstalledForKind(ctx context.Context, kind string) (map[string]string, error)
}
