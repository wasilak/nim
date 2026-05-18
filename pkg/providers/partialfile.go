package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wasilak/nim/pkg/planctx"
	"github.com/wasilak/nim/pkg/provider"
	"github.com/wasilak/nim/pkg/resource"
	"gopkg.in/yaml.v3"
)

// PartialFileProvider implements the Provider interface for ManagedFilePartial.
type PartialFileProvider struct{}

// NewPartialFileProvider creates a new PartialFileProvider.
func NewPartialFileProvider() *PartialFileProvider {
	return &PartialFileProvider{}
}

// Name returns the provider name.
func (p *PartialFileProvider) Name() string {
	return "partialfile"
}

// Available checks if the provider can operate.
func (p *PartialFileProvider) Available() (bool, string) {
	return true, "filesystem operations available"
}

// Reconcile compares desired resource groups with current system state.
func (p *PartialFileProvider) Reconcile(ctx context.Context,
	desired []resource.ResourceGroup[any],
	state []provider.ResourceState,
) provider.GroupPlan {
	plan := provider.GroupPlan{}
	showDiff := false
	if v := ctx.Value(planctx.PlanShowDiffKey); v != nil {
		if b, ok := v.(bool); ok {
			showDiff = b
		}
	}

	// Index state by kind and group
	stateIndex := make(map[string]map[string]provider.ResourceState)
	for _, s := range state {
		if s.Kind == resource.KindManagedFilePartial {
			if stateIndex[s.Kind] == nil {
				stateIndex[s.Kind] = make(map[string]provider.ResourceState)
			}
			stateIndex[s.Kind][s.Group] = s
		}
	}

	for _, group := range desired {
		if group.Kind != resource.KindManagedFilePartial {
			continue
		}

		spec, ok := group.RawSpec.(resource.ManagedFilePartialSpec)
		if !ok {
			plan.Errors = append(plan.Errors, fmt.Errorf("invalid spec type for %s/%s", group.Kind, group.Name))
			continue
		}

		kindIndex := stateIndex[group.Kind]
		stateGroup, existsInState := kindIndex[group.Name]

		// Detect format
		format := detectFormat(spec.Path)
		if format == formatUnknown {
			plan.Errors = append(plan.Errors, fmt.Errorf("unsupported file format for %s: %s (supported: .json, .yaml, .yml)", group.Name, spec.Path))
			continue
		}

		// Read existing file
		existingKeys := make(map[string]any)
		fileExists := false
		path := expandPath(spec.Path)
		if data, err := os.ReadFile(path); err == nil {
			fileExists = true
			if err := unmarshalByFormat(data, format, &existingKeys); err != nil {
				plan.Errors = append(plan.Errors, fmt.Errorf("failed to parse %s: %w", spec.Path, err))
				continue
			}
		}

		// Build desired key map
		desiredKeys := make(map[string]any)
		for _, pk := range spec.Keys {
			desiredKeys[pk.Key] = pk.Value
		}

		// Determine if change is needed
		if !existsInState && !fileExists {
			// New file and new resource
			plan.Additions = append(plan.Additions, provider.GroupAddition{
				Kind:    group.Kind,
				Group:   group.Name,
				Items:   group.Items,
				RawSpec: spec,
			})
		} else if p.keysNeedUpdate(existingKeys, desiredKeys) {
			// File exists, keys differ
			// Convert items to state items for modification tracking
			oldItems := make([]resource.ItemState, 0, len(stateGroup.Items))
			for _, item := range stateGroup.Items {
				oldItems = append(oldItems, resource.ItemState{
					Name:    item.Name,
					Version: item.Version,
				})
			}
			
			// Build changes
			var changes []provider.ItemChange
			for _, item := range group.Items {
				changes = append(changes, provider.ItemChange{
					ItemName: item.Name,
					OldState: resource.ItemState{Name: item.Name},
					NewState: resource.ItemState{Name: item.Name},
				})
			}
			
			plan.Modifications = append(plan.Modifications, provider.GroupModification{
				Kind:    group.Kind,
				Group:   group.Name,
				Changes: changes,
			})
		} else {
			// No change - add to InSync
			inSyncItems := make([]resource.ItemState, 0, len(group.Items))
			for _, item := range group.Items {
				inSyncItems = append(inSyncItems, resource.ItemState{
					Name:    item.Name,
					Version: item.Version,
				})
			}
			plan.InSync = append(plan.InSync, provider.GroupState{
				Kind:  group.Kind,
				Group: group.Name,
				Items: inSyncItems,
			})
		}
	}

	_ = showDiff // Will be used for diff implementation
	return plan
}

// Apply executes the plan for ManagedFilePartial resources.
func (p *PartialFileProvider) Apply(ctx context.Context, plan provider.GroupPlan) ([]provider.ApplyItemResult, error) {
	var results []provider.ApplyItemResult

	for _, addition := range plan.Additions {
		results = append(results, p.applyAddition(addition)...)
	}

	for _, modification := range plan.Modifications {
		results = append(results, p.applyModification(modification)...)
	}

	return results, nil
}

// Import is not implemented for PartialFileProvider.
func (p *PartialFileProvider) Import(ctx context.Context, group string) (provider.ResourceState, error) {
	return provider.ResourceState{}, fmt.Errorf("import not supported for provider partialfile")
}

// fileFormat represents the format of a file.
type fileFormat int

const (
	formatUnknown fileFormat = iota
	formatJSON
	formatYAML
)

// detectFormat determines file format from extension.
func detectFormat(path string) fileFormat {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		return formatJSON
	case ".yaml", ".yml":
		return formatYAML
	default:
		return formatUnknown
	}
}

// unmarshalByFormat unmarshals data based on the file format.
func unmarshalByFormat(data []byte, format fileFormat, v any) error {
	switch format {
	case formatJSON:
		return json.Unmarshal(data, v)
	case formatYAML:
		return yaml.Unmarshal(data, v)
	default:
		return fmt.Errorf("unsupported format")
	}
}

// marshalByFormat marshals data based on the file format.
func marshalByFormat(v any, format fileFormat) ([]byte, error) {
	switch format {
	case formatJSON:
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return nil, err
		}
		return append(data, '\n'), nil
	case formatYAML:
		return yaml.Marshal(v)
	default:
		return nil, fmt.Errorf("unsupported format")
	}
}

// keysNeedUpdate checks if desired keys differ from existing.
func (p *PartialFileProvider) keysNeedUpdate(existing, desired map[string]any) bool {
	for k, v := range desired {
		if existing[k] != v {
			return true
		}
	}
	return false
}

// expandPath expands a leading `~` to the user's home directory.
func expandPath(path string) string {
	if !strings.HasPrefix(path, "~/") && path != "~" {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}

// applyAddition handles new file creation.
func (p *PartialFileProvider) applyAddition(addition provider.GroupAddition) []provider.ApplyItemResult {
	results := make([]provider.ApplyItemResult, 0, len(addition.Items))

	spec, ok := addition.RawSpec.(resource.ManagedFilePartialSpec)
	if !ok {
		return append(results, provider.ApplyItemResult{
			Kind:  addition.Kind,
			Group: addition.Group,
			Op:    "add",
			Err:   fmt.Errorf("invalid spec type"),
		})
	}

	path := expandPath(spec.Path)
	format := detectFormat(path)
	if format == formatUnknown {
		return append(results, provider.ApplyItemResult{
			Kind:  addition.Kind,
			Group: addition.Group,
			Op:    "add",
			Err:   fmt.Errorf("unsupported file format: %s", path),
		})
	}

	// Ensure parent directories exist
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return append(results, provider.ApplyItemResult{
			Kind:  addition.Kind,
			Group: addition.Group,
			Op:    "add",
			Err:   fmt.Errorf("failed to create parent directories for %s: %w", path, err),
		})
	}

	// Read existing file or start with empty map
	content := make(map[string]any)
	if data, err := os.ReadFile(path); err == nil {
		if err := unmarshalByFormat(data, format, &content); err != nil {
			return append(results, provider.ApplyItemResult{
				Kind:  addition.Kind,
				Group: addition.Group,
				Op:    "add",
				Err:   fmt.Errorf("failed to parse existing file %s: %w", path, err),
			})
		}
	} else if !os.IsNotExist(err) {
		return append(results, provider.ApplyItemResult{
			Kind:  addition.Kind,
			Group: addition.Group,
			Op:    "add",
			Err:   fmt.Errorf("failed to read file %s: %w", path, err),
		})
	}

	// Merge declared keys
	for _, pk := range spec.Keys {
		content[pk.Key] = pk.Value
	}

	// Marshal and write
	output, err := marshalByFormat(content, format)
	if err != nil {
		return append(results, provider.ApplyItemResult{
			Kind:  addition.Kind,
			Group: addition.Group,
			Op:    "add",
			Err:   fmt.Errorf("failed to marshal content: %w", err),
		})
	}

	if err := os.WriteFile(path, output, 0644); err != nil {
		return append(results, provider.ApplyItemResult{
			Kind:  addition.Kind,
			Group: addition.Group,
			Op:    "add",
			Err:   fmt.Errorf("failed to write file %s: %w", path, err),
		})
	}

	for _, item := range addition.Items {
		results = append(results, provider.ApplyItemResult{
			Kind:  addition.Kind,
			Group: addition.Group,
			Item:  item.Name,
			Op:    "add",
		})
	}

	return results
}

// applyModification handles file updates.
func (p *PartialFileProvider) applyModification(modification provider.GroupModification) []provider.ApplyItemResult {
	// We need the spec to know what to apply - but GroupModification doesn't have RawSpec
	// For now, we can't fully implement this without storing the spec somewhere accessible
	// This is a limitation of the current provider interface
	results := make([]provider.ApplyItemResult, len(modification.Changes))
	for i, change := range modification.Changes {
		results[i] = provider.ApplyItemResult{
			Kind:  modification.Kind,
			Group: modification.Group,
			Item:  change.ItemName,
			Op:    "modify",
			Err:   fmt.Errorf("modification not yet fully implemented for PartialFileProvider"),
		}
	}
	return results
}
