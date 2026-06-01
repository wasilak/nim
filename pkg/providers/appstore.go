package providers

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/wasilak/nim/pkg/cmdutil"
	"github.com/wasilak/nim/pkg/provider"
	"github.com/wasilak/nim/pkg/resource"
)

// AppStoreProvider manages Mac App Store applications via the mas CLI.
type AppStoreProvider struct{}

// NewAppStoreProvider creates a new AppStoreProvider.
func NewAppStoreProvider() *AppStoreProvider {
	return &AppStoreProvider{}
}

// Name returns the provider name.
func (p *AppStoreProvider) Name() string {
	return "appstore"
}

// Available checks if the mas CLI is available on this system.
func (p *AppStoreProvider) Available() (bool, string) {
	if path := cmdutil.CheckExecutable("mas"); path == "" {
		return false, "mas not found in PATH; install from https://github.com/mas-cli/mas or brew install mas"
	}
	return true, "mas found"
}

// Reconcile compares the desired resource groups with the current system state.
func (p *AppStoreProvider) Reconcile(ctx context.Context,
	desired []resource.ResourceGroup[any],
	state []provider.ResourceState,
) provider.GroupPlan {
	return provider.BaseReconcile(resource.KindAppStoreApps, desired, state, p.getInstalledApps(ctx), nil)
}

// getInstalledApps returns a map of installed App Store apps by their ADAM ID.
// The key is the string representation of the numeric ID; the value is the version.
func (p *AppStoreProvider) getInstalledApps(ctx context.Context) map[string]string {
	if ctx == nil {
		slog.Warn("appstore getInstalledApps called with nil context; returning empty set")
		return make(map[string]string)
	}

	installed := make(map[string]string)

	stdout, _, err := cmdutil.RunSimpleFn(ctx, "mas", "list")
	if err != nil {
		slog.Warn("mas list failed", "err", err)
		return installed
	}

	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		entry := parseMasListLine(line)
		if entry.id != "" {
			installed[entry.id] = entry.version
		}
	}

	return installed
}

// masListEntry holds parsed fields from a mas list line.
type masListEntry struct {
	id      string
	name    string
	version string
}

// parseMasListLine parses a single line from `mas list` output.
// Expected format: "497799835 Xcode (16.2)"
func parseMasListLine(line string) masListEntry {
	entry := masListEntry{}

	spaceIdx := strings.IndexByte(line, ' ')
	if spaceIdx == -1 {
		return entry
	}
	entry.id = line[:spaceIdx]
	rest := line[spaceIdx+1:]

	openIdx := strings.LastIndex(rest, "(")
	closeIdx := strings.LastIndex(rest, ")")
	if openIdx != -1 && closeIdx > openIdx {
		entry.name = strings.TrimSpace(rest[:openIdx])
		entry.version = rest[openIdx+1 : closeIdx]
	} else {
		entry.name = strings.TrimSpace(rest)
	}

	return entry
}

// InstalledForKind implements provider.CoverageProvider.
func (p *AppStoreProvider) InstalledForKind(ctx context.Context, kind string) (map[string]string, error) {
	if ctx == nil || kind != resource.KindAppStoreApps {
		return nil, nil
	}
	return p.getInstalledApps(ctx), nil
}

// Apply executes the given GroupPlan.
func (p *AppStoreProvider) Apply(ctx context.Context, plan provider.GroupPlan) ([]provider.ApplyItemResult, error) {
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

func (p *AppStoreProvider) applyGroupAddition(ctx context.Context, addition provider.GroupAddition) []provider.ApplyItemResult {
	var results []provider.ApplyItemResult
	for _, item := range addition.Items {
		r := provider.ApplyItemResult{Kind: addition.Kind, Group: addition.Group, Item: item.Name, Op: "add"}
		slog.Info("installing App Store app", "id", item.Name)
		_, stderr, err := cmdutil.RunSimpleFn(ctx, "mas", "install", item.Name)
		if err != nil {
			r.Err = fmt.Errorf("failed to install app %s: %s: %w", item.Name, stderr, err)
		}
		results = append(results, r)
	}
	return results
}

func (p *AppStoreProvider) applyGroupRemoval(ctx context.Context, removal provider.GroupRemoval) []provider.ApplyItemResult {
	var results []provider.ApplyItemResult
	for _, item := range removal.Items {
		r := provider.ApplyItemResult{Kind: removal.Kind, Group: removal.Group, Item: item.Name, Op: "remove"}
		slog.Info("uninstalling App Store app", "id", item.Name)
		_, stderr, err := cmdutil.RunSimpleFn(ctx, "mas", "uninstall", item.Name)
		if err != nil {
			r.Err = fmt.Errorf("failed to uninstall app %s: %s: %w", item.Name, stderr, err)
		}
		results = append(results, r)
	}
	return results
}

func (p *AppStoreProvider) applyGroupModification(ctx context.Context, modification provider.GroupModification) []provider.ApplyItemResult {
	var results []provider.ApplyItemResult
	for _, change := range modification.Changes {
		r := provider.ApplyItemResult{Kind: modification.Kind, Group: modification.Group, Item: change.ItemName, Op: "modify"}
		slog.Info("upgrading App Store app", "id", change.ItemName)
		_, stderr, err := cmdutil.RunSimpleFn(ctx, "mas", "upgrade", change.ItemName)
		if err != nil {
			r.Err = fmt.Errorf("failed to upgrade app %s: %s: %w", change.ItemName, stderr, err)
		}
		results = append(results, r)
	}
	return results
}

// Import discovers all installed App Store apps and returns them as state.
func (p *AppStoreProvider) Import(ctx context.Context, group string) (provider.ResourceState, error) {
	if ctx == nil {
		return provider.ResourceState{}, fmt.Errorf("nil context")
	}

	installed := p.getInstalledApps(ctx)
	if len(installed) == 0 {
		return provider.ResourceState{}, fmt.Errorf("no installed App Store apps found; is mas installed and the App Store accessible?")
	}

	items := make([]resource.ItemState, 0, len(installed))
	for id, version := range installed {
		items = append(items, resource.ItemState{Name: id, Version: version, Status: "present"})
	}

	return provider.ResourceState{Kind: resource.KindAppStoreApps, Group: group, Items: items}, nil
}

// validateAppID checks that a string is a valid numeric App Store ID.
func validateAppID(id string) bool {
	_, err := strconv.Atoi(id)
	return err == nil
}