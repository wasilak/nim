package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/wasilak/nim/pkg/cmdutil"
	"github.com/wasilak/nim/pkg/provider"
	"github.com/wasilak/nim/pkg/resource"
)

// NpmProvider implements the Provider interface for npm packages
type NpmProvider struct{}

// NewNpmProvider creates a new NpmProvider.
func NewNpmProvider() *NpmProvider {
	return &NpmProvider{}
}

// Name returns the provider name.
func (p *NpmProvider) Name() string {
	return "npm"
}

// Available checks if npm is available on this system.
func (p *NpmProvider) Available() (bool, string) {
	if path := cmdutil.CheckExecutable("npm"); path == "" {
		return false, "npm not found in PATH; install Node.js from https://nodejs.org/"
	}
	return true, "npm found"
}

// Reconcile compares the desired resource groups with the current system state.
func (p *NpmProvider) Reconcile(ctx context.Context,
	desired []resource.ResourceGroup[any],
	state []provider.ResourceState,
) provider.GroupPlan {
	return provider.BaseReconcile(resource.KindNpmPackages, desired, state, p.getInstalledPackages(ctx), nil)
}

func (p *NpmProvider) getInstalledPackages(ctx context.Context) map[string]string {
	if ctx == nil {
		slog.Warn("npm getInstalledPackages called with nil context; returning empty set")
		return make(map[string]string)
	}
	stdout, _, err := cmdutil.RunSimpleFn(ctx, "npm", "list", "-g", "--depth=0", "--json")
	if err != nil {
		slog.Warn("npm getInstalledPackages failed", "err", err)
		return make(map[string]string)
	}

	// Parse via JSON — handles scoped packages like @babel/core correctly.
	var parsed struct {
		Dependencies map[string]struct {
			Version string `json:"version"`
		} `json:"dependencies"`
	}
	installed := make(map[string]string)
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		slog.Warn("npm getInstalledPackages: failed to parse json", "err", err)
		return installed
	}
	for name, dep := range parsed.Dependencies {
		installed[name] = dep.Version
	}
	return installed
}

// InstalledForKind implements provider.CoverageProvider.
func (p *NpmProvider) InstalledForKind(ctx context.Context, kind string) (map[string]string, error) {
	if ctx == nil || kind != resource.KindNpmPackages {
		return nil, nil
	}
	return p.getInstalledPackages(ctx), nil
}

// Apply executes the given GroupPlan
func (p *NpmProvider) Apply(ctx context.Context, plan provider.GroupPlan) ([]provider.ApplyItemResult, error) {
	var results []provider.ApplyItemResult
	for _, addition := range plan.Additions {
		results = append(results, p.applyGroupAddition(ctx, addition)...)
	}
	for _, removal := range plan.Removals {
		results = append(results, p.applyGroupRemoval(ctx, removal)...)
	}
	for _, modification := range plan.Modifications {
		results = append(results, p.applyGroupModification(ctx, modification)...)
	}
	return results, nil
}

// isExecutable reports whether an item should be run via npx instead of
// installed globally with `npm install -g`.
func isExecutable(item resource.ResourceItem) bool {
	return item.Metadata != nil && item.Metadata[resource.MetaExecutable] == "true"
}

// execArgs decodes the extra npx arguments carried in item metadata.
func execArgs(item resource.ResourceItem) []string {
	if item.Metadata == nil {
		return nil
	}
	raw := item.Metadata[resource.MetaArgs]
	if raw == "" {
		return nil
	}
	var args []string
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		slog.Warn("npm: failed to decode executable args", "item", item.Name, "err", err)
		return nil
	}
	return args
}

// runNpx runs an executable package ephemerally via npx.
func (p *NpmProvider) runNpx(ctx context.Context, item resource.ResourceItem) error {
	spec := item.Name
	if item.Version != "" {
		spec = fmt.Sprintf("%s@%s", item.Name, item.Version)
	}
	args := append([]string{"--yes", spec}, execArgs(item)...)
	slog.Info("running npx package", "package", spec)
	_, stderr, err := cmdutil.RunSimpleFn(ctx, "npx", args...)
	if err != nil {
		return fmt.Errorf("failed to run npx %s: %s: %w", spec, stderr, err)
	}
	return nil
}

func (p *NpmProvider) applyGroupAddition(ctx context.Context, addition provider.GroupAddition) []provider.ApplyItemResult {
	var results []provider.ApplyItemResult

	// Executable packages run one-by-one via npx (each with its own args).
	// The rest are installed globally in a single batched npm call.
	normal := make([]resource.ResourceItem, 0, len(addition.Items))
	for _, item := range addition.Items {
		if isExecutable(item) {
			r := provider.ApplyItemResult{Kind: addition.Kind, Group: addition.Group, Item: item.Name, Op: "add"}
			if err := p.runNpx(ctx, item); err != nil {
				r.Err = err
			}
			results = append(results, r)
			continue
		}
		normal = append(normal, item)
	}
	if len(normal) == 0 {
		return results
	}

	pkgs := make([]string, 0, len(normal))
	for _, item := range normal {
		pkg := item.Name
		if item.Version != "" {
			pkg = fmt.Sprintf("%s@%s", item.Name, item.Version)
		}
		pkgs = append(pkgs, pkg)
	}
	failed := batchWithFallback(pkgs, func(ns []string) error {
		args := append([]string{"install", "-g"}, ns...)
		_, stderr, err := cmdutil.RunSimpleFn(ctx, "npm", args...)
		if err != nil {
			if len(ns) == 1 {
				return fmt.Errorf("failed to install %s: %s: %w", ns[0], stderr, err)
			}
			return err
		}
		return nil
	})
	for i, item := range normal {
		r := provider.ApplyItemResult{Kind: addition.Kind, Group: addition.Group, Item: item.Name, Op: "add"}
		if err, bad := failed[pkgs[i]]; bad {
			r.Err = err
		}
		results = append(results, r)
	}
	return results
}

func (p *NpmProvider) applyGroupRemoval(ctx context.Context, removal provider.GroupRemoval) []provider.ApplyItemResult {
	names := make([]string, 0, len(removal.Items))
	for _, item := range removal.Items {
		names = append(names, item.Name)
	}
	failed := batchWithFallback(names, func(ns []string) error {
		args := append([]string{"uninstall", "-g"}, ns...)
		_, stderr, err := cmdutil.RunSimpleFn(ctx, "npm", args...)
		if err != nil {
			if len(ns) == 1 {
				return fmt.Errorf("failed to uninstall %s: %s: %w", ns[0], stderr, err)
			}
			return err
		}
		return nil
	})
	var results []provider.ApplyItemResult
	for _, item := range removal.Items {
		r := provider.ApplyItemResult{Kind: removal.Kind, Group: removal.Group, Item: item.Name, Op: "remove"}
		if err, bad := failed[item.Name]; bad {
			r.Err = err
		}
		results = append(results, r)
	}
	return results
}

func (p *NpmProvider) applyGroupModification(ctx context.Context, modification provider.GroupModification) []provider.ApplyItemResult {
	items := make([]resource.ResourceItem, 0, len(modification.Changes))
	for _, change := range modification.Changes {
		items = append(items, resource.ResourceItem{
			Name:    change.ItemName,
			Version: change.NewState.Version,
		})
	}
	results := p.applyGroupAddition(ctx, provider.GroupAddition{
		Kind:  modification.Kind,
		Group: modification.Group,
		Items: items,
	})
	// Correct the Op field
	for i := range results {
		results[i].Op = "modify"
	}
	return results
}

// Import not supported for npm provider
func (p *NpmProvider) Import(ctx context.Context, group string) (provider.ResourceState, error) {
	return provider.ResourceState{}, fmt.Errorf("import not supported for provider npm")
}
