package providers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/wasilak/nim/pkg/planctx"
	"github.com/wasilak/nim/pkg/provider"
	"github.com/wasilak/nim/pkg/resource"
)

// FileProvider implements the Provider interface for ManagedFile
type FileProvider struct {
	dotfilesRoot string
}

// NewFileProvider creates a new FileProvider.
func NewFileProvider(dotfilesRoot string) *FileProvider {
	return &FileProvider{
		dotfilesRoot: dotfilesRoot,
	}
}

// Name returns the provider name.
func (p *FileProvider) Name() string {
	return "file"
}

// Available checks if the provider can operate on this system.
func (p *FileProvider) Available() (bool, string) {
	if p.dotfilesRoot != "" {
		if _, err := os.Stat(p.dotfilesRoot); err != nil {
			return false, fmt.Sprintf("dotfiles root not accessible: %s", err)
		}
	}
	return true, "filesystem operations available"
}

// Reconcile compares the desired resource groups with the current system state.
func (p *FileProvider) Reconcile(ctx context.Context,
	desired []resource.ResourceGroup[any],
	state []provider.ResourceState,
) provider.GroupPlan {
	plan := provider.GroupPlan{}
	// Diff flag via context
	showDiff := false
	if v := ctx.Value(planctx.PlanShowDiffKey); v != nil {
		if b, ok := v.(bool); ok {
			showDiff = b
		}
	}
	// Index state by kind and group
	stateIndex := make(map[string]map[string]provider.ResourceState)
	for _, s := range state {
		if s.Kind == "ManagedFile" {
			if stateIndex[s.Kind] == nil {
				stateIndex[s.Kind] = make(map[string]provider.ResourceState)
			}
			stateIndex[s.Kind][s.Group] = s
		}
	}

	// Process each desired group
	for _, group := range desired {
		if group.Kind != "ManagedFile" {
			continue
		}

		kindIndex := stateIndex[group.Kind]
		stateGroup, exists := kindIndex[group.Name]

		if !exists {
			// New group - all items are additions (bring them under management even if dest exists)
			plan.Additions = append(plan.Additions, provider.GroupAddition{
				Kind:  group.Kind,
				Group: group.Name,
				Items: group.Items,
			})

			// For any addition where a destination already exists on disk, emit a
			// PlanWarning so the user is informed and can choose to import the
			// existing resource into state instead of blindly overwriting.
			// Also check if parent directories will need to be created.
			for _, it := range group.Items {
				dest := ""
				if it.FileExtra != nil {
					dest = it.FileExtra.Destination
				}
				if dest == "" {
					dest = it.Name
				}
				dest = p.resolveDest(dest)
				if _, err := os.Stat(dest); err == nil {
					plan.Warnings = append(plan.Warnings, provider.PlanWarning{
						GroupID:    group.Kind + "/" + group.Name,
						ItemID:     it.Name,
						Severity:   "warning",
						Message:    fmt.Sprintf("Destination file already exists at %s", dest),
						Suggestion: fmt.Sprintf("nim state import %s/%s[%s] %s", group.Kind, group.Name, it.Name, dest),
						ImportItems: []provider.ImportCandidate{{
							ID:          fmt.Sprintf("%s/%s[%s]", group.Kind, group.Name, it.Name),
							ActualValue: dest,
						}},
					})
				}
				// Check if parent directory will be created
				parentDir := filepath.Dir(dest)
				if parentDir != "" && parentDir != "." {
					if _, err := os.Stat(parentDir); os.IsNotExist(err) {
						plan.Warnings = append(plan.Warnings, provider.PlanWarning{
							GroupID:  group.Kind + "/" + group.Name,
							ItemID:   it.Name,
							Severity: "info",
							Message:  fmt.Sprintf("Will create parent directory %s", parentDir),
						})
					}
				}
			}
		} else {
			// Existing group - compare items
			additions, removals, modifications, inSync := p.compareGroupItems(group, stateGroup)

		if len(additions) > 0 {
			plan.Additions = append(plan.Additions, provider.GroupAddition{
				Kind:  group.Kind,
				Group: group.Name,
				Items: additions,
			})

			// Check for directory creation needed for additions to existing groups
			for _, it := range additions {
				dest := ""
				if it.FileExtra != nil {
					dest = it.FileExtra.Destination
				}
				if dest == "" {
					dest = it.Name
				}
				dest = p.resolveDest(dest)
				parentDir := filepath.Dir(dest)
				if parentDir != "" && parentDir != "." {
					if _, err := os.Stat(parentDir); os.IsNotExist(err) {
						plan.Warnings = append(plan.Warnings, provider.PlanWarning{
							GroupID:  group.Kind + "/" + group.Name,
							ItemID:   it.Name,
							Severity: "info",
							Message:  fmt.Sprintf("Will create parent directory %s", parentDir),
						})
					}
				}
			}
		}

		if len(removals) > 0 {
				plan.Removals = append(plan.Removals, provider.GroupRemoval{
					Kind:  group.Kind,
					Group: group.Name,
					Items: removals,
				})
			}

			if len(modifications) > 0 {
				plan.Modifications = append(plan.Modifications, provider.GroupModification{
					Kind:    group.Kind,
					Group:   group.Name,
					Changes: modifications,
				})
			}

			if len(inSync) > 0 && len(additions) == 0 && len(removals) == 0 && len(modifications) == 0 {
				plan.InSync = append(plan.InSync, provider.GroupState{
					Kind:  group.Kind,
					Group: group.Name,
					Items: inSync,
				})
			}
		}
	}

	// Check for removals
	desiredGroups := make(map[string]bool)
	for _, group := range desired {
		if group.Kind == "ManagedFile" {
			desiredGroups[group.Kind+"/"+group.Name] = true
		}
	}

	for kind, groups := range stateIndex {
		for groupName := range groups {
			if !desiredGroups[kind+"/"+groupName] {
				stateGroup := stateIndex[kind][groupName]
				items := make([]resource.ResourceItem, 0, len(stateGroup.Items))
				for _, item := range stateGroup.Items {
					items = append(items, resource.ResourceItem{
						Name:    item.Name,
						Version: item.Version,
					})
				}
				plan.Removals = append(plan.Removals, provider.GroupRemoval{
					Kind:  kind,
					Group: groupName,
					Items: items,
				})
			}
		}
	}

	if showDiff {
		for ai := range plan.Additions {
			for _, item := range plan.Additions[ai].Items {
				if item.FileExtra == nil {
					continue
				}
				extra := item.FileExtra
				var content []byte
				if extra.Source != "" && extra.Source != "(inline)" {
					content, _ = os.ReadFile(p.resolveSource(extra.Source))
				} else if extra.Inline != "" {
					content = []byte(extra.Inline)
				}
				if len(content) > 0 {
					if plan.Additions[ai].Contents == nil {
						plan.Additions[ai].Contents = make(map[string]string)
					}
					plan.Additions[ai].Contents[item.Name] = string(content)
				}
			}
		}
		for ri := range plan.Removals {
			for _, item := range plan.Removals[ri].Items {
				dest := item.Name
				if item.FileExtra != nil && item.FileExtra.Destination != "" {
					dest = p.resolveDest(item.FileExtra.Destination)
				}
				content, _ := os.ReadFile(dest)
				if len(content) > 0 {
					if plan.Removals[ri].Contents == nil {
						plan.Removals[ri].Contents = make(map[string]string)
					}
					plan.Removals[ri].Contents[item.Name] = string(content)
				}
			}
		}
		for mi := range plan.Modifications {
			for ci, change := range plan.Modifications[mi].Changes {
				oldContent, newContent := []byte{}, []byte{}
				if fe := change.NewState.FileExtra; fe != nil {
					if fe.Destination != "" {
						oldContent, _ = os.ReadFile(p.resolveDest(fe.Destination))
					}
					if fe.Source != "" && fe.Source != "(inline)" {
						newContent, _ = os.ReadFile(p.resolveSource(fe.Source))
					} else if fe.Inline != "" {
						newContent = []byte(fe.Inline)
					}
				}
				plan.Modifications[mi].Changes[ci].OldContent = string(oldContent)
				plan.Modifications[mi].Changes[ci].NewContent = string(newContent)
			}
		}
	}
	return plan
}

// expandPath expands a leading `~` to the user's home directory.
func expandPath(p string) string {
	if !strings.HasPrefix(p, "~/") && p != "~" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return filepath.Join(home, p[1:])
}

// resolveSource returns the full path for a source value. Absolute paths are
// used as-is; relative paths are joined with the dotfiles root.
func (p *FileProvider) resolveSource(source string) string {
	source = expandPath(source)
	if filepath.IsAbs(source) {
		return source
	}
	// Relative source paths are resolved against the dotfiles root
	return filepath.Join(p.dotfilesRoot, source)
}

// resolveDest expands `~` in a destination path.
func (p *FileProvider) resolveDest(dest string) string {
	return expandPath(dest)
}

// compareGroupItems compares desired group items with state
func (p *FileProvider) compareGroupItems(
	group resource.ResourceGroup[any],
	stateGroup provider.ResourceState,
) (additions, removals []resource.ResourceItem, modifications []provider.ItemChange, inSync []resource.ItemState) {
	stateItems := make(map[string]resource.ItemState)
	for _, item := range stateGroup.Items {
		stateItems[item.Name] = item
	}

	for _, desiredItem := range group.Items {
		name := desiredItem.Name
		fe := desiredItem.FileExtra // always set for ManagedFile items

		stateItem, inState := stateItems[name]

		destExists := false
		if fe != nil && fe.Destination != "" {
			_, err := os.Stat(p.resolveDest(fe.Destination))
			destExists = !os.IsNotExist(err)
		}

		// Compute desired content checksum (from inline or source)
		desiredHash := ""
		if fe != nil {
			if fe.Inline != "" {
				h := sha256.Sum256([]byte(fe.Inline))
				desiredHash = hex.EncodeToString(h[:])
			} else if fe.Source != "" && fe.Source != "(inline)" {
				desiredHash = p.hashFile(p.resolveSource(fe.Source))
			}
		}

		if !destExists {
			additions = append(additions, desiredItem)
			continue
		}

		// Destination exists; compute current hash
		currentHash := ""
		if fe != nil {
			currentHash = p.hashFile(p.resolveDest(fe.Destination))
		}

		if inState {
			// actualMode returns the on-disk mode string and whether it differs from desired.
			actualMode := func() (string, bool) {
				if fe == nil || fe.Mode == "" {
					return "", false
				}
				info, err := os.Stat(p.resolveDest(fe.Destination))
				if err != nil {
					return "", false
				}
				perm := info.Mode().Perm()
				changed := perm != parseMode(fe.Mode)
				return fmt.Sprintf("%04o", perm), changed
			}

			// stateWithActualMode returns a copy of stateItem with FileExtra.Mode set to the
			// real on-disk mode, so the display layer can show the full old→new transition.
			// Creates a FileExtra if none exists (e.g. state predates FileExtra tracking).
			stateWithActualMode := func(onDiskMode string) resource.ItemState {
				s := stateItem
				if onDiskMode != "" {
					var extra resource.FileItemExtra
					if s.FileExtra != nil {
						extra = *s.FileExtra
					}
					extra.Mode = onDiskMode
					s.FileExtra = &extra
				}
				return s
			}

			if desiredHash != "" {
				if desiredHash != currentHash {
					modifications = append(modifications, provider.ItemChange{
						ItemName: name,
						OldState: stateItem,
						NewState: resource.ItemState{
							Name:      name,
							Checksum:  desiredHash,
							Status:    "present",
							FileExtra: desiredItem.FileExtra,
						},
						Diff: "content changed",
					})
				} else if onDisk, changed := actualMode(); changed {
					modifications = append(modifications, provider.ItemChange{
						ItemName: name,
						OldState: stateWithActualMode(onDisk),
						NewState: resource.ItemState{
							Name:      name,
							Checksum:  desiredHash,
							Status:    "present",
							FileExtra: desiredItem.FileExtra,
						},
						Diff: "mode changed",
					})
				} else {
					inSync = append(inSync, stateItem)
				}
			} else {
				// Fallback: compare current vs saved state
				if currentHash != stateItem.Checksum {
					modifications = append(modifications, provider.ItemChange{
						ItemName: name,
						OldState: stateItem,
						NewState: resource.ItemState{
							Name:      name,
							Checksum:  currentHash,
							Status:    "present",
							FileExtra: desiredItem.FileExtra,
						},
						Diff: "content changed",
					})
				} else if onDisk, changed := actualMode(); changed {
					modifications = append(modifications, provider.ItemChange{
						ItemName: name,
						OldState: stateWithActualMode(onDisk),
						NewState: resource.ItemState{
							Name:      name,
							Checksum:  currentHash,
							Status:    "present",
							FileExtra: desiredItem.FileExtra,
						},
						Diff: "mode changed",
					})
				} else {
					inSync = append(inSync, stateItem)
				}
			}
		} else {
			// File exists but not tracked
			additions = append(additions, desiredItem)
		}
	}

	desiredItems := make(map[string]bool)
	for _, item := range group.Items {
		desiredItems[item.Name] = true
	}

	for name, stateItem := range stateItems {
		if !desiredItems[name] {
			removals = append(removals, resource.ResourceItem{
				Name:    name,
				Version: stateItem.Version,
			})
		}
	}

	return
}

// hashFile computes SHA256 hash of file content
func (p *FileProvider) hashFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// Apply executes the given GroupPlan
func (p *FileProvider) Apply(ctx context.Context, plan provider.GroupPlan) ([]provider.ApplyItemResult, error) {
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

// applyGroupAddition handles file/directory creation.
// Items are applied concurrently (bounded to 8) — each writes to a distinct
// destination path so there are no ordering constraints or conflicts.
func (p *FileProvider) applyGroupAddition(_ context.Context, addition provider.GroupAddition) []provider.ApplyItemResult {
	results := make([]provider.ApplyItemResult, len(addition.Items))
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for i, item := range addition.Items {
		fe := item.FileExtra
		results[i] = provider.ApplyItemResult{Kind: addition.Kind, Group: addition.Group, Item: item.Name, Op: "add"}
		if fe == nil || fe.Destination == "" {
			continue
		}
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			dest := p.resolveDest(fe.Destination)
			var err error
			if mkErr := os.MkdirAll(filepath.Dir(dest), 0755); mkErr != nil {
				err = fmt.Errorf("failed to create parent directory for %s: %w", dest, mkErr)
			} else {
				fileMode := parseMode(fe.Mode)
				if fe.Source != "" && fe.Source != "(inline)" {
					if cpErr := p.copyFile(p.resolveSource(fe.Source), dest, fileMode); cpErr != nil {
						err = fmt.Errorf("failed to copy %s to %s: %w", fe.Source, dest, cpErr)
					}
				} else {
					if wErr := os.WriteFile(dest, []byte(fe.Inline), fileMode); wErr != nil {
						err = fmt.Errorf("failed to create %s: %w", dest, wErr)
					}
				}
				if err == nil {
					// WriteFile only sets the mode at creation; chmod ensures the mode is
					// applied even when overwriting an existing file with different permissions.
					if chErr := os.Chmod(dest, fileMode); chErr != nil {
						err = fmt.Errorf("failed to set mode on %s: %w", dest, chErr)
					}
				}
			}
			results[i].Err = err
		})
	}
	wg.Wait()
	return results
}

// applyGroupRemoval handles file/directory removal in parallel.
func (p *FileProvider) applyGroupRemoval(_ context.Context, removal provider.GroupRemoval) []provider.ApplyItemResult {
	results := make([]provider.ApplyItemResult, len(removal.Items))
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for i, item := range removal.Items {
		dest := item.Name
		if item.FileExtra != nil && item.FileExtra.Destination != "" {
			dest = p.resolveDest(item.FileExtra.Destination)
		}
		results[i] = provider.ApplyItemResult{Kind: removal.Kind, Group: removal.Group, Item: item.Name, Op: "remove"}
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := os.RemoveAll(dest); err != nil {
				results[i].Err = fmt.Errorf("failed to remove %s: %w", dest, err)
			}
		})
	}
	wg.Wait()
	return results
}

// applyGroupModification handles file/directory updates.
func (p *FileProvider) applyGroupModification(ctx context.Context, modification provider.GroupModification) []provider.ApplyItemResult {
	items := make([]resource.ResourceItem, 0, len(modification.Changes))
	for _, change := range modification.Changes {
		items = append(items, resource.ResourceItem{
			Name:      change.ItemName,
			FileExtra: change.NewState.FileExtra,
		})
	}
	results := p.applyGroupAddition(ctx, provider.GroupAddition{
		Kind:  modification.Kind,
		Group: modification.Group,
		Items: items,
	})
	for i := range results {
		results[i].Op = "modify"
	}
	return results
}

// parseMode converts a "0644"-style octal string to os.FileMode, defaulting to 0644.
func parseMode(mode string) os.FileMode {
	if mode == "" {
		return 0644
	}
	v, err := strconv.ParseUint(mode, 8, 32)
	if err != nil {
		return 0644
	}
	return os.FileMode(v)
}

// copyFile copies a file from src to dst with the given permissions.
func (p *FileProvider) copyFile(src, dst string, mode os.FileMode) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, mode)
}

// Import is intentionally not implemented for FileProvider. ImportItem support
// was removed: provider-level import/export functionality is deprecated and
// handled outside providers.
func (p *FileProvider) Import(ctx context.Context, group string) (provider.ResourceState, error) {
	return provider.ResourceState{}, fmt.Errorf("import not supported for provider file")
}
