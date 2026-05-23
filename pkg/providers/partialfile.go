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

		// Build desired key map (render templates in values)
		desiredKeys := make(map[string]any)
		for _, pk := range spec.Keys {
			desiredKeys[pk.Key] = pk.Value
		}

		// Determine if change is needed
		needsUpdate := p.keysNeedUpdate(existingKeys, desiredKeys)
		
		if !existsInState && !fileExists {
			// New file and new resource
			plan.Additions = append(plan.Additions, provider.GroupAddition{
				Kind:    group.Kind,
				Group:   group.Name,
				Items:   group.Items,
				RawSpec: spec,
			})
		} else if needsUpdate {
			// File exists, keys differ - need modification
			var changes []provider.ItemChange
			for _, item := range group.Items {
				oldState := resource.ItemState{Name: item.Name}
				// Try to find old state if exists
				for _, si := range stateGroup.Items {
					if si.Name == item.Name {
						oldState = si
						break
					}
				}
				changes = append(changes, provider.ItemChange{
					ItemName: item.Name,
					OldState: oldState,
					NewState: resource.ItemState{Name: item.Name},
				})
			}
			
			plan.Modifications = append(plan.Modifications, provider.GroupModification{
				Kind:    group.Kind,
				Group:   group.Name,
				Changes: changes,
				RawSpec: spec,
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

// normalizePartialValue tries to JSON-parse a string value so that JSON objects
// stored as strings round-trip correctly when written to JSON/YAML files.
func normalizePartialValue(v any) any {
	if s, ok := v.(string); ok {
		var parsed any
		if err := json.Unmarshal([]byte(s), &parsed); err == nil {
			return parsed
		}
	}
	return v
}

// keysNeedUpdate checks if desired keys differ from existing.
func (p *PartialFileProvider) keysNeedUpdate(existing, desired map[string]any) bool {
	for k, v := range desired {
		d1, _ := json.Marshal(normalizePartialValue(v))
		d2, _ := json.Marshal(existing[k])
		if string(d1) != string(d2) {
			return true
		}
	}
	return false
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

	return p.applySpec(spec, addition.Kind, addition.Group, addition.Items, "add")
}

// applyModification handles file updates.
func (p *PartialFileProvider) applyModification(modification provider.GroupModification) []provider.ApplyItemResult {
	spec, ok := modification.RawSpec.(resource.ManagedFilePartialSpec)
	if !ok {
		results := make([]provider.ApplyItemResult, len(modification.Changes))
		for i := range modification.Changes {
			results[i] = provider.ApplyItemResult{
				Kind:  modification.Kind,
				Group: modification.Group,
				Op:    "modify",
				Err:   fmt.Errorf("invalid spec type"),
			}
		}
		return results
	}

	// Convert changes back to items
	items := make([]resource.ResourceItem, len(modification.Changes))
	for i, change := range modification.Changes {
		items[i] = resource.ResourceItem{Name: change.ItemName}
	}

	return p.applySpec(spec, modification.Kind, modification.Group, items, "modify")
}

// applySpec applies the spec to the file (used by both add and modify).
func (p *PartialFileProvider) applySpec(spec resource.ManagedFilePartialSpec, kind, group string, items []resource.ResourceItem, op string) []provider.ApplyItemResult {
	results := make([]provider.ApplyItemResult, 0, len(items))

	path := expandPath(spec.Path)
	format := detectFormat(path)
	if format == formatUnknown {
		for _, item := range items {
			results = append(results, provider.ApplyItemResult{
				Kind:  kind,
				Group: group,
				Item:  item.Name,
				Op:    op,
				Err:   fmt.Errorf("unsupported file format: %s", path),
			})
		}
		return results
	}

	// Ensure parent directories exist
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		for _, item := range items {
			results = append(results, provider.ApplyItemResult{
				Kind:  kind,
				Group: group,
				Item:  item.Name,
				Op:    op,
				Err:   fmt.Errorf("failed to create parent directories for %s: %w", path, err),
			})
		}
		return results
	}

	// Read existing file or start with empty map
	content := make(map[string]any)
	if data, err := os.ReadFile(path); err == nil {
		if err := unmarshalByFormat(data, format, &content); err != nil {
			for _, item := range items {
				results = append(results, provider.ApplyItemResult{
					Kind:  kind,
					Group: group,
					Item:  item.Name,
					Op:    op,
					Err:   fmt.Errorf("failed to parse existing file %s: %w", path, err),
				})
			}
			return results
		}
	} else if !os.IsNotExist(err) {
		for _, item := range items {
			results = append(results, provider.ApplyItemResult{
				Kind:  kind,
				Group: group,
				Item:  item.Name,
				Op:    op,
				Err:   fmt.Errorf("failed to read file %s: %w", path, err),
			})
		}
		return results
	}

	// Merge declared keys — try JSON-parsing string values so that JSON objects
	// are stored as structured data rather than raw strings.
	for _, pk := range spec.Keys {
		content[pk.Key] = normalizePartialValue(pk.Value)
	}

	// Marshal and write
	output, err := marshalByFormat(content, format)
	if err != nil {
		for _, item := range items {
			results = append(results, provider.ApplyItemResult{
				Kind:  kind,
				Group: group,
				Item:  item.Name,
				Op:    op,
				Err:   fmt.Errorf("failed to marshal content: %w", err),
			})
		}
		return results
	}

	if err := os.WriteFile(path, output, 0644); err != nil {
		for _, item := range items {
			results = append(results, provider.ApplyItemResult{
				Kind:  kind,
				Group: group,
				Item:  item.Name,
				Op:    op,
				Err:   fmt.Errorf("failed to write file %s: %w", path, err),
			})
		}
		return results
	}

	for _, item := range items {
		results = append(results, provider.ApplyItemResult{
			Kind:  kind,
			Group: group,
			Item:  item.Name,
			Op:    op,
		})
	}

	return results
}
