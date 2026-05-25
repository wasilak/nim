// Package engine: Plan logic extracted from engine.go
package engine

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/wasilak/nim/pkg/graph"
	"github.com/wasilak/nim/pkg/planctx"
	"github.com/wasilak/nim/pkg/provider"
	"github.com/wasilak/nim/pkg/resource"
	"github.com/wasilak/nim/pkg/state"
)

// PlanResult contains the result of a plan operation.
type PlanResult struct {
	CurrentState       *state.State
	ProviderPlans      map[string]provider.GroupPlan
	TotalAdditions     int
	TotalModifications int
	TotalRemovals      int
	TotalCleanup       int
	TotalInSync        int
	TotalDrifted       int
	HasChanges         bool
	// UnmatchedTargets are targets provided by the user that didn't match any resource
	UnmatchedTargets []string
	// DependencyOrder is the topological order of resources by NodeID.
	// Resources earlier in the slice must be applied before those later.
	DependencyOrder []graph.NodeID

	// DAG is the validated dependency graph built during planning.
	// Used by Apply to check per-node dependencies for skip propagation.
	DAG *graph.DAG
}

// Plan loads state, parses resources, and generates plans from all providers.
// It accepts PlanOptions which can be used to target specific resources.
// Note: Plan show-diff context key lives in pkg/planctx to avoid
// import cycles between engine and providers.
func (e *Engine) Plan(ctx context.Context, opts PlanOptions) (*PlanResult, error) {
	// Load current state
	currentState, err := e.StateBackend.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	// Parse all resources from dotfiles
	resources, err := e.loadResources()
	if err != nil {
		return nil, fmt.Errorf("failed to load resources: %w", err)
	}

	// Filter resources by namespace BEFORE building DAG.
	// activeNS may itself be a /pattern/ regex when the user passes e.g.
	// NIM_NAMESPACE="/(default|work)/"; MatchesNamespaceFilter handles both cases.
	activeNS := opts.Namespace
	var filtered []resource.Resource
	for _, res := range resources {
		if resource.MatchesNamespaceFilter(res, activeNS) {
			filtered = append(filtered, res)
		}
	}
	if len(filtered) != len(resources) {
		slog.Debug("filtered resources by namespace",
			"activeNamespace", activeNS,
			"total", len(resources),
			"included", len(filtered),
			"excluded", len(resources)-len(filtered))
	}
	resources = filtered

	// Convert resources to groups
	resourceGroups := e.resourcesToGroups(resources)

	// Build dependency graph and validate it before proceeding.
	dag, err := graph.Build(resources)
	if err != nil {
		return nil, fmt.Errorf("failed to build dependency graph: %w", err)
	}
	dependencyOrder, err := dag.TopologicalOrder()
	if err != nil {
		return nil, fmt.Errorf("dependency error: %w", err)
	}

	// If targets provided, parse them. We'll not filter out resourceGroups here
	// (desired) because targets may refer to state-only groups. Instead we will
	// augment resourceGroups with synthetic groups based on state when needed
	// and detect unmatched targets against both desired resources and state.
	var targetMatches []TargetMatch
	var unmatched []string
	if len(opts.Targets) > 0 {
		var parseErr error
		targetMatches, parseErr = ParseTargets(opts.Targets)
		if parseErr != nil {
			return nil, parseErr
		}

		// For each parsed target, check if it matches any desired group/item.
		// If it doesn't but exists in state, add a synthetic empty desired group
		// so providers will produce plans for removals, and mark as matched.
		for i, raw := range opts.Targets {
			t := targetMatches[i]

			// Regex targets: skip unmatched detection (we can't know statically
			// which resources will match; filterPlanByTargets handles it).
			if t.IsRegex() {
				continue
			}

			found := false

			// Check desired groups
			for _, g := range resourceGroups {
				if t.Matches(g.Kind, g.Name, "") {
					if t.Item == "" {
						found = true
						break
					}
					for _, it := range g.Items {
						if t.Matches(g.Kind, g.Name, it.Name) {
							found = true
							break
						}
					}
					if found {
						break
					}
				}
			}

			if found {
				continue
			}

			// Check state resources
			for _, s := range currentState.Resources {
				if t.Matches(s.Kind, s.Group, "") {
					if t.Item == "" {
						found = true
					} else {
						for _, it := range s.Items {
							if t.Matches(s.Kind, s.Group, it.Name) {
								found = true
								break
							}
						}
					}
				}

				if found {
					// If desired didn't contain this group, add a synthetic empty
					// group so provider.Reconcile will compute removals for items
					// present in state but not desired.
					existsInDesired := false
					for _, g := range resourceGroups {
						if strings.EqualFold(g.Kind, s.Kind) && strings.EqualFold(g.Name, s.Group) {
							existsInDesired = true
							break
						}
					}
					if !existsInDesired {
						resourceGroups = append(resourceGroups, resource.ResourceGroup[any]{
							Kind:  s.Kind,
							Name:  s.Group,
							Items: []resource.ResourceItem{},
						})
					}
					break
				}
			}

			if !found {
				unmatched = append(unmatched, raw)
			}
		}
	}

	// Filter resource groups to only targeted ones before reconciliation so
	// providers don't run expensive operations (e.g. brew info) for every group.
	if len(targetMatches) > 0 {
		resourceGroups = filterResourceGroupsByTargets(resourceGroups, targetMatches)
	}

	// Group resources by provider
	groupsByProvider := e.groupResourcesByProvider(resourceGroups)

	// Generate plans for each provider
	providerPlans := make(map[string]provider.GroupPlan)
	result := &PlanResult{
		CurrentState:    currentState,
		ProviderPlans:   providerPlans,
		DependencyOrder: dependencyOrder,
		DAG:             dag,
	}

	if len(unmatched) > 0 {
		result.UnmatchedTargets = unmatched
	}

	// When a specific namespace is active, restrict state to only groups present
	// in the desired resource set. This prevents cross-namespace removals: resources
	// from other namespaces are in state but not in desire, so without this scoping
	// the provider would plan to remove them.
	scopedState := currentState.Resources
	if activeNS != "default" {
		scopedState = scopeStateToDesiredGroups(currentState.Resources, resourceGroups)
	}

	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)
	ctxWithDiff := context.WithValue(ctx, planctx.PlanShowDiffKey, opts.ShowDiff)
	for providerName, prov := range e.Providers {
		providerGroups := groupsByProvider[providerName]
		if len(providerGroups) == 0 {
			continue
		}
		providerState := e.filterStateForProvider(scopedState, providerName)

		wg.Go(func() {
			plan := prov.Reconcile(ctxWithDiff, providerGroups, providerState)
			if len(targetMatches) > 0 {
				plan = filterPlanByTargets(plan, targetMatches)
			}
			mu.Lock()
			providerPlans[providerName] = plan
			for _, add := range plan.Additions {
				result.TotalAdditions += len(add.Items)
			}
			for _, mod := range plan.Modifications {
				result.TotalModifications += len(mod.Changes)
			}
			for _, rem := range plan.Removals {
				result.TotalRemovals += len(rem.Items)
			}
			for _, cleanup := range plan.Cleanup {
				result.TotalCleanup += len(cleanup.Items)
			}
			for _, s := range plan.InSync {
				result.TotalInSync += len(s.Items)
			}
			result.TotalDrifted += len(plan.Drifted)
			mu.Unlock()
		})
	}
	wg.Wait()

	result.HasChanges = result.TotalAdditions > 0 || result.TotalModifications > 0 ||
		result.TotalRemovals > 0 || result.TotalCleanup > 0 || result.TotalDrifted > 0

	return result, nil
}

// loadResources parses all resource files from the dotfiles directory.
func (e *Engine) loadResources() ([]resource.Resource, error) {
	loader := resource.NewLoader(e.Config.DotfilesRoot, e.TemplateContext)
	return loader.LoadResources()
}

// resourcesToGroups converts Resources to ResourceGroups
func (e *Engine) resourcesToGroups(resources []resource.Resource) []resource.ResourceGroup[any] {
	var groups []resource.ResourceGroup[any]
	for _, res := range resources {
		groups = append(groups, res.ToGroup())
	}
	return groups
}

// groupResourcesByProvider groups resource groups by their provider type.
func (e *Engine) groupResourcesByProvider(groups []resource.ResourceGroup[any]) map[string][]resource.ResourceGroup[any] {
	grouped := make(map[string][]resource.ResourceGroup[any])

	for _, group := range groups {
		// Look up provider by kind from the registry. If not registered, skip.
		if provName, ok := provider.ProviderNameForKind(group.Kind); ok {
			grouped[provName] = append(grouped[provName], group)
		}
	}

	return grouped
}

// scopeStateToDesiredGroups returns only the state entries whose {Kind, Group}
// pair appears in the desired resource groups. Used during namespace-scoped runs
// to prevent cross-namespace removals: state entries for resources outside the
// active namespace are ignored rather than treated as "desired removed".
func scopeStateToDesiredGroups(state []provider.ResourceState, desired []resource.ResourceGroup[any]) []provider.ResourceState {
	type key struct{ kind, group string }
	desiredKeys := make(map[key]bool, len(desired))
	for _, g := range desired {
		desiredKeys[key{g.Kind, g.Name}] = true
	}
	var out []provider.ResourceState
	for _, s := range state {
		if desiredKeys[key{s.Kind, s.Group}] {
			out = append(out, s)
		}
	}
	return out
}

// filterStateForProvider filters state entries for a specific provider.
func (e *Engine) filterStateForProvider(stateResources []provider.ResourceState, providerName string) []provider.ResourceState {
	var filtered []provider.ResourceState

	// Build a reverse mapping: kind -> providerName is in registry. Filter
	// stateResources by asking the registry which provider handles each kind.
	for _, s := range stateResources {
		if provName, ok := provider.ProviderNameForKind(s.Kind); ok && provName == providerName {
			filtered = append(filtered, s)
		}
	}

	return filtered
}
