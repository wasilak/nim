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

	if showDiff {
		for ai := range plan.Additions {
			spec, ok := plan.Additions[ai].RawSpec.(resource.ManagedFilePartialSpec)
			if !ok {
				continue
			}
			format := detectFormat(spec.Path)
			if format == formatUnknown {
				continue
			}
			path := expandPath(spec.Path)
			existing := make(map[string]any)
			if data, err := os.ReadFile(path); err == nil {
				_ = unmarshalByFormat(data, format, &existing)
			}
			newContent, err := computeMergedContent(existing, spec.Keys, format)
			if err != nil {
				continue
			}
			if plan.Additions[ai].Contents == nil {
				plan.Additions[ai].Contents = make(map[string]string)
			}
			for _, item := range plan.Additions[ai].Items {
				plan.Additions[ai].Contents[item.Name] = newContent
			}
		}

		for mi := range plan.Modifications {
			spec, ok := plan.Modifications[mi].RawSpec.(resource.ManagedFilePartialSpec)
			if !ok {
				continue
			}
			format := detectFormat(spec.Path)
			if format == formatUnknown {
				continue
			}
			path := expandPath(spec.Path)
			existing := make(map[string]any)
			oldContent := ""
			var oldData []byte
			if data, err := os.ReadFile(path); err == nil {
				oldData = data
				oldContent = string(data)
				_ = unmarshalByFormat(data, format, &existing)
			}
			var newContent string
			var err error
			if format == formatJSON && len(oldData) > 0 {
				newContent, err = computePatchedJSONContent(oldData, spec.Keys)
			} else {
				newContent, err = computeMergedContent(existing, spec.Keys, format)
			}
			if err != nil {
				continue
			}
			for ci := range plan.Modifications[mi].Changes {
				plan.Modifications[mi].Changes[ci].OldContent = oldContent
				plan.Modifications[mi].Changes[ci].NewContent = newContent
			}
		}
	}
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

// PatchTopLevelJSONKeyPreserveFormatting replaces or appends a top-level JSON
// key without reserializing the rest of the document.
func PatchTopLevelJSONKeyPreserveFormatting(content []byte, key string, newValue []byte) ([]byte, error) {
	if !json.Valid(newValue) {
		return nil, fmt.Errorf("invalid JSON value for key %q", key)
	}

	pos := skipJSONWhitespace(content, 0)
	if pos >= len(content) || content[pos] != '{' {
		return nil, fmt.Errorf("JSON document must be an object")
	}
	pos++

	for {
		pos = skipJSONWhitespace(content, pos)
		if pos >= len(content) {
			return nil, fmt.Errorf("unterminated JSON object")
		}
		if content[pos] == '}' {
			return appendTopLevelJSONKey(content, pos, key, newValue)
		}
		if content[pos] != '"' {
			return nil, fmt.Errorf("expected object key at byte %d", pos)
		}

		keyStart := pos
		keyEnd, err := scanJSONString(content, keyStart)
		if err != nil {
			return nil, err
		}
		var currentKey string
		if err := json.Unmarshal(content[keyStart:keyEnd], &currentKey); err != nil {
			return nil, fmt.Errorf("decode JSON object key at byte %d: %w", keyStart, err)
		}

		pos = skipJSONWhitespace(content, keyEnd)
		if pos >= len(content) || content[pos] != ':' {
			return nil, fmt.Errorf("expected colon after key %q", currentKey)
		}
		valueStart := skipJSONWhitespace(content, pos+1)
		valueEnd, err := scanJSONValue(content, valueStart)
		if err != nil {
			return nil, fmt.Errorf("scan value for key %q: %w", currentKey, err)
		}

		if currentKey == key {
			patched := make([]byte, 0, len(content)-(valueEnd-valueStart)+len(newValue))
			patched = append(patched, content[:valueStart]...)
			patched = append(patched, newValue...)
			patched = append(patched, content[valueEnd:]...)
			return patched, nil
		}

		pos = skipJSONWhitespace(content, valueEnd)
		if pos >= len(content) {
			return nil, fmt.Errorf("unterminated JSON object after key %q", currentKey)
		}
		switch content[pos] {
		case ',':
			pos++
		case '}':
			return appendTopLevelJSONKey(content, pos, key, newValue)
		default:
			return nil, fmt.Errorf("expected comma or object end after key %q", currentKey)
		}
	}
}

func skipJSONWhitespace(content []byte, pos int) int {
	for pos < len(content) {
		switch content[pos] {
		case ' ', '\n', '\r', '\t':
			pos++
		default:
			return pos
		}
	}
	return pos
}

func scanJSONString(content []byte, pos int) (int, error) {
	if pos >= len(content) || content[pos] != '"' {
		return 0, fmt.Errorf("expected string at byte %d", pos)
	}
	for i := pos + 1; i < len(content); i++ {
		switch content[i] {
		case '\\':
			i++
			if i >= len(content) {
				return 0, fmt.Errorf("unterminated escape in string at byte %d", pos)
			}
		case '"':
			return i + 1, nil
		}
	}
	return 0, fmt.Errorf("unterminated string at byte %d", pos)
}

func scanJSONValue(content []byte, pos int) (int, error) {
	pos = skipJSONWhitespace(content, pos)
	if pos >= len(content) {
		return 0, fmt.Errorf("expected value")
	}

	switch content[pos] {
	case '"':
		return scanJSONString(content, pos)
	case '{', '[':
		return scanJSONCompositeValue(content, pos)
	case 't', 'f', 'n':
		return scanJSONLiteral(content, pos)
	default:
		return scanJSONNumber(content, pos)
	}
}

func scanJSONCompositeValue(content []byte, pos int) (int, error) {
	stack := []byte{content[pos]}
	for i := pos + 1; i < len(content); i++ {
		switch content[i] {
		case '"':
			end, err := scanJSONString(content, i)
			if err != nil {
				return 0, err
			}
			i = end - 1
		case '{', '[':
			stack = append(stack, content[i])
		case '}', ']':
			if len(stack) == 0 {
				return 0, fmt.Errorf("unexpected closing delimiter at byte %d", i)
			}
			open := stack[len(stack)-1]
			if open == '{' && content[i] != '}' || open == '[' && content[i] != ']' {
				return 0, fmt.Errorf("mismatched closing delimiter at byte %d", i)
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return i + 1, nil
			}
		}
	}
	return 0, fmt.Errorf("unterminated JSON value at byte %d", pos)
}

func scanJSONLiteral(content []byte, pos int) (int, error) {
	for _, literal := range []string{"true", "false", "null"} {
		end := pos + len(literal)
		if end <= len(content) && string(content[pos:end]) == literal {
			return end, nil
		}
	}
	return 0, fmt.Errorf("invalid literal at byte %d", pos)
}

func scanJSONNumber(content []byte, pos int) (int, error) {
	end := pos
	for end < len(content) {
		switch content[end] {
		case ' ', '\n', '\r', '\t', ',', '}', ']':
			if end == pos || !json.Valid(content[pos:end]) {
				return 0, fmt.Errorf("invalid number at byte %d", pos)
			}
			return end, nil
		default:
			end++
		}
	}
	if end == pos || !json.Valid(content[pos:end]) {
		return 0, fmt.Errorf("invalid number at byte %d", pos)
	}
	return end, nil
}

func appendTopLevelJSONKey(content []byte, closePos int, key string, newValue []byte) ([]byte, error) {
	encodedKey, err := json.Marshal(key)
	if err != nil {
		return nil, fmt.Errorf("encode JSON key %q: %w", key, err)
	}

	prev := closePos - 1
	for prev >= 0 {
		switch content[prev] {
		case ' ', '\n', '\r', '\t':
			prev--
		default:
			goto foundPrevious
		}
	}

foundPrevious:
	if prev >= 0 && content[prev] == '{' {
		insert := append([]byte("\n  "), encodedKey...)
		insert = append(insert, []byte(": ")...)
		insert = append(insert, newValue...)
		insert = append(insert, '\n')
		patched := make([]byte, 0, len(content)+len(insert))
		patched = append(patched, content[:closePos]...)
		patched = append(patched, insert...)
		patched = append(patched, content[closePos:]...)
		return patched, nil
	}

	insertPos := prev + 1
	insert := append([]byte(",\n  "), encodedKey...)
	insert = append(insert, []byte(": ")...)
	insert = append(insert, newValue...)
	patched := make([]byte, 0, len(content)+len(insert))
	patched = append(patched, content[:insertPos]...)
	patched = append(patched, insert...)
	patched = append(patched, content[insertPos:]...)
	return patched, nil
}

// normalizePartialValue tries to JSON-parse a string value so that JSON objects
// and arrays stored as strings round-trip correctly when written to JSON/YAML files.
// Primitive JSON types (numbers, booleans, null) are left as-is to preserve the
// original string representation.
func normalizePartialValue(v any) any {
	if s, ok := v.(string); ok {
		var parsed any
		if err := json.Unmarshal([]byte(s), &parsed); err == nil {
			switch parsed.(type) {
			case map[string]any, []any:
				return parsed
			}
		}
	}
	return v
}

// computeMergedContent merges desired keys into existing and returns the
// serialized result. Used for diff generation without touching the filesystem.
func computeMergedContent(existing map[string]any, keys []resource.PartialKey, format fileFormat) (string, error) {
	merged := make(map[string]any, len(existing))
	for k, v := range existing {
		merged[k] = v
	}
	for _, pk := range keys {
		merged[pk.Key] = normalizePartialValue(pk.Value)
	}
	data, err := marshalByFormat(merged, format)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func computePatchedJSONContent(existing []byte, keys []resource.PartialKey) (string, error) {
	content := existing
	for _, pk := range keys {
		value, err := partialValueJSONBytes(pk.Value)
		if err != nil {
			return "", fmt.Errorf("prepare JSON value for key %q: %w", pk.Key, err)
		}
		content, err = PatchTopLevelJSONKeyPreserveFormatting(content, pk.Key, value)
		if err != nil {
			return "", fmt.Errorf("patch key %q: %w", pk.Key, err)
		}
	}
	return string(content), nil
}

func partialValueJSONBytes(value string) ([]byte, error) {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') && json.Valid([]byte(trimmed)) {
		return []byte(trimmed), nil
	}
	data, err := json.Marshal(normalizePartialValue(value))
	if err != nil {
		return nil, err
	}
	return data, nil
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
	var existingData []byte
	fileExists := false
	if data, err := os.ReadFile(path); err == nil {
		existingData = data
		fileExists = true
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

	var output []byte
	var err error
	if format == formatJSON && fileExists {
		patched, patchErr := computePatchedJSONContent(existingData, spec.Keys)
		if patchErr != nil {
			err = patchErr
		} else {
			output = []byte(patched)
		}
	} else {
		// Merge declared keys — try JSON-parsing string values so that JSON objects
		// are stored as structured data rather than raw strings.
		for _, pk := range spec.Keys {
			content[pk.Key] = normalizePartialValue(pk.Value)
		}

		output, err = marshalByFormat(content, format)
	}
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
