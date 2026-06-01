package engine

import (
	"context"
	"sync"

	"github.com/wasilak/nim/pkg/provider"
	"github.com/wasilak/nim/pkg/resource"
)

// KindStat holds aggregate counts for a single resource kind from nim state.
type KindStat struct {
	Kind   string
	Groups int
	Items  int
}

// CoverageStat holds installed vs tracked counts for a single resource kind.
type CoverageStat struct {
	Kind      string
	Installed int
	Tracked   int
}

// StatsResult is returned by Engine.Stats.
type StatsResult struct {
	KindStats   []KindStat
	TotalGroups int
	TotalItems  int
	// Coverage is populated only when withCoverage is true.
	Coverage []CoverageStat
}

// allKinds is the display order for resource kinds in the stats table.
var allKinds = []string{
	resource.KindHomeBrewPackages,
	resource.KindHomeBrewCasks,
	resource.KindHomeBrewTaps,
	resource.KindNpmPackages,
	resource.KindGoPackages,
	resource.KindCargoPackages,
	resource.KindAppStoreApps,
	resource.KindManagedFile,
	resource.KindAISkillPackages,
}

// coverageKinds are the kinds for which coverage is reported with --all.
// ManagedFile and AISkillPackages are excluded — "installed" isn't meaningful.
var coverageKinds = []string{
	resource.KindHomeBrewPackages,
	resource.KindHomeBrewCasks,
	resource.KindHomeBrewTaps,
	resource.KindNpmPackages,
	resource.KindGoPackages,
	resource.KindCargoPackages,
	resource.KindAppStoreApps,
}

// Stats loads state and computes per-kind statistics. When withCoverage is
// true it also queries each provider for all installed items in parallel.
func (e *Engine) Stats(ctx context.Context, withCoverage bool) (*StatsResult, error) {
	currentState, err := e.StateBackend.Load(ctx)
	if err != nil {
		return nil, err
	}

	// Aggregate state counts per kind.
	kindGroups := make(map[string]int)
	kindItems := make(map[string]int)
	for _, res := range currentState.Resources {
		kindGroups[res.Kind]++
		kindItems[res.Kind] += len(res.Items)
	}

	result := &StatsResult{}
	for _, kind := range allKinds {
		g := kindGroups[kind]
		it := kindItems[kind]
		if g == 0 && it == 0 {
			continue
		}
		result.KindStats = append(result.KindStats, KindStat{Kind: kind, Groups: g, Items: it})
		result.TotalGroups += g
		result.TotalItems += it
	}

	if !withCoverage {
		return result, nil
	}

	// For coverage, collect tracked counts per kind.
	trackedPerKind := make(map[string]int)
	for _, res := range currentState.Resources {
		trackedPerKind[res.Kind] += len(res.Items)
	}

	// Query all providers in parallel.
	type coverageEntry struct {
		kind      string
		installed int
		err       error
	}
	ch := make(chan coverageEntry, len(coverageKinds))
	var wg sync.WaitGroup

	for _, kind := range coverageKinds {
		provName, ok := provider.ProviderNameForKind(kind)
		if !ok {
			continue
		}
		prov, ok := e.Providers[provName]
		if !ok {
			continue
		}
		cp, ok := prov.(provider.CoverageProvider)
		if !ok {
			continue
		}
		wg.Go(func() {
			installed, err := cp.InstalledForKind(ctx, kind)
			ch <- coverageEntry{kind: kind, installed: len(installed), err: err}
		})
	}

	wg.Wait()
	close(ch)

	installedPerKind := make(map[string]int)
	for entry := range ch {
		if entry.err == nil {
			installedPerKind[entry.kind] = entry.installed
		}
	}

	for _, kind := range coverageKinds {
		installed := installedPerKind[kind]
		tracked := trackedPerKind[kind]
		if installed == 0 && tracked == 0 {
			continue
		}
		result.Coverage = append(result.Coverage, CoverageStat{
			Kind:      kind,
			Installed: installed,
			Tracked:   tracked,
		})
	}

	return result, nil
}
