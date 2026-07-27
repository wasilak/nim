package providers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wasilak/nim/pkg/planctx"
	"github.com/wasilak/nim/pkg/provider"
	"github.com/wasilak/nim/pkg/resource"
)

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected fileFormat
	}{
		{"json file", "config.json", formatJSON},
		{"yaml file", "config.yaml", formatYAML},
		{"yml file", "config.yml", formatYAML},
		{"unknown extension", "config.txt", formatUnknown},
		{"uppercase JSON", "CONFIG.JSON", formatJSON},
		{"uppercase YAML", "CONFIG.YAML", formatYAML},
		{"no extension", "config", formatUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectFormat(tt.path)
			if got != tt.expected {
				t.Errorf("detectFormat(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestKeysNeedUpdate(t *testing.T) {
	p := &PartialFileProvider{}

	tests := []struct {
		name     string
		existing map[string]any
		desired  map[string]any
		want     bool
	}{
		{
			name:     "empty both",
			existing: map[string]any{},
			desired:  map[string]any{},
			want:     false,
		},
		{
			name:     "new key",
			existing: map[string]any{},
			desired:  map[string]any{"key": "value"},
			want:     true,
		},
		{
			name:     "same value",
			existing: map[string]any{"key": "value"},
			desired:  map[string]any{"key": "value"},
			want:     false,
		},
		{
			name:     "different value",
			existing: map[string]any{"key": "old"},
			desired:  map[string]any{"key": "new"},
			want:     true,
		},
		{
			name:     "multiple keys no change",
			existing: map[string]any{"a": "1", "b": "2"},
			desired:  map[string]any{"a": "1", "b": "2"},
			want:     false,
		},
		{
			name:     "multiple keys one change",
			existing: map[string]any{"a": "1", "b": "2"},
			desired:  map[string]any{"a": "1", "b": "3"},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.keysNeedUpdate(tt.existing, tt.desired)
			if got != tt.want {
				t.Errorf("keysNeedUpdate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPartialFileProvider_Reconcile_NewFile(t *testing.T) {
	p := NewPartialFileProvider()
	ctx := context.Background()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test-new.json")

	desired := []resource.ResourceGroup[any]{
		{
			Kind:  resource.KindManagedFilePartial,
			Name:  "test-config",
			Items: []resource.ResourceItem{{Name: "key1"}},
			RawSpec: resource.ManagedFilePartialSpec{
				Path: path,
				Keys: []resource.PartialKey{
					{Key: "name", Value: "test"},
				},
			},
		},
	}

	plan := p.Reconcile(ctx, desired, nil)

	if len(plan.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", plan.Errors)
	}

	if len(plan.Additions) != 1 {
		t.Errorf("expected 1 addition, got %d", len(plan.Additions))
	}
	if len(plan.Modifications) != 0 {
		t.Errorf("expected 0 modifications, got %d", len(plan.Modifications))
	}
	if len(plan.InSync) != 0 {
		t.Errorf("expected 0 in-sync, got %d", len(plan.InSync))
	}
}

func TestPartialFileProvider_Reconcile_ExistingFileNoChange(t *testing.T) {
	p := NewPartialFileProvider()
	ctx := context.Background()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test-existing.json")

	// Create existing file
	existingContent := `{"name": "test"}`
	if err := os.WriteFile(path, []byte(existingContent), 0644); err != nil {
		t.Fatal(err)
	}

	desired := []resource.ResourceGroup[any]{
		{
			Kind:  resource.KindManagedFilePartial,
			Name:  "test-config",
			Items: []resource.ResourceItem{{Name: "name"}},
			RawSpec: resource.ManagedFilePartialSpec{
				Path: path,
				Keys: []resource.PartialKey{
					{Key: "name", Value: "test"},
				},
			},
		},
	}

	plan := p.Reconcile(ctx, desired, nil)

	if len(plan.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", plan.Errors)
	}

	if len(plan.Additions) != 0 {
		t.Errorf("expected 0 additions, got %d", len(plan.Additions))
	}
	if len(plan.Modifications) != 0 {
		t.Errorf("expected 0 modifications, got %d", len(plan.Modifications))
	}
	if len(plan.InSync) != 1 {
		t.Errorf("expected 1 in-sync, got %d", len(plan.InSync))
	}
}

func TestPartialFileProvider_Reconcile_ExistingFileWithChange(t *testing.T) {
	p := NewPartialFileProvider()
	ctx := context.Background()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test-change.json")

	// Create existing file
	existingContent := `{"name": "old"}`
	if err := os.WriteFile(path, []byte(existingContent), 0644); err != nil {
		t.Fatal(err)
	}

	desired := []resource.ResourceGroup[any]{
		{
			Kind:  resource.KindManagedFilePartial,
			Name:  "test-config",
			Items: []resource.ResourceItem{{Name: "name"}},
			RawSpec: resource.ManagedFilePartialSpec{
				Path: path,
				Keys: []resource.PartialKey{
					{Key: "name", Value: "new"},
				},
			},
		},
	}

	plan := p.Reconcile(ctx, desired, nil)

	if len(plan.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", plan.Errors)
	}

	if len(plan.Additions) != 0 {
		t.Errorf("expected 0 additions, got %d", len(plan.Additions))
	}
	if len(plan.Modifications) != 1 {
		t.Errorf("expected 1 modification, got %d", len(plan.Modifications))
	}
	if len(plan.InSync) != 0 {
		t.Errorf("expected 0 in-sync, got %d", len(plan.InSync))
	}
}

func TestPartialFileProvider_Reconcile_UnsupportedFormat(t *testing.T) {
	p := NewPartialFileProvider()
	ctx := context.Background()

	desired := []resource.ResourceGroup[any]{
		{
			Kind:  resource.KindManagedFilePartial,
			Name:  "test-config",
			Items: []resource.ResourceItem{{Name: "key1"}},
			RawSpec: resource.ManagedFilePartialSpec{
				Path: "/tmp/test.txt",
				Keys: []resource.PartialKey{
					{Key: "name", Value: "test"},
				},
			},
		},
	}

	plan := p.Reconcile(ctx, desired, nil)

	if len(plan.Errors) != 1 {
		t.Errorf("expected 1 error for unsupported format, got %d", len(plan.Errors))
	}
}

func TestPartialFileProvider_Apply_JSON(t *testing.T) {
	p := NewPartialFileProvider()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.json")

	addition := provider.GroupAddition{
		Kind:  resource.KindManagedFilePartial,
		Group: "test",
		Items: []resource.ResourceItem{
			{Name: "version"},
			{Name: "enabled"},
		},
		RawSpec: resource.ManagedFilePartialSpec{
			Path: path,
			Keys: []resource.PartialKey{
				{Key: "version", Value: "1.0.0"},
				{Key: "enabled", Value: "true"},
			},
		},
	}

	results := p.applyAddition(addition)

	for _, r := range results {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
		}
	}

	// Verify file was created
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	// Verify content contains our keys
	s := string(content)
	if !containsSubstring(s, `"version"`) || !containsSubstring(s, `"1.0.0"`) {
		t.Error("JSON missing expected version key")
	}
	if !containsSubstring(s, `"enabled"`) || !containsSubstring(s, `"true"`) {
		t.Error("JSON missing expected enabled key")
	}
}

func TestPartialFileProvider_Apply_YAML(t *testing.T) {
	p := NewPartialFileProvider()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.yaml")

	addition := provider.GroupAddition{
		Kind:  resource.KindManagedFilePartial,
		Group: "test",
		Items: []resource.ResourceItem{
			{Name: "theme"},
		},
		RawSpec: resource.ManagedFilePartialSpec{
			Path: path,
			Keys: []resource.PartialKey{
				{Key: "theme", Value: "dark"},
			},
		},
	}

	results := p.applyAddition(addition)

	for _, r := range results {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
		}
	}

	// Verify file was created
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	// Verify content contains our keys
	s := string(content)
	if !containsSubstring(s, `theme`) || !containsSubstring(s, `dark`) {
		t.Error("YAML missing expected theme key")
	}
}

func TestPartialFileProvider_Apply_MergeExisting(t *testing.T) {
	p := NewPartialFileProvider()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.json")

	// Create existing file with other keys
	existing := `{"existing": "value", "shared": "old"}`
	if err := os.WriteFile(path, []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	addition := provider.GroupAddition{
		Kind:  resource.KindManagedFilePartial,
		Group: "test",
		Items: []resource.ResourceItem{
			{Name: "shared"},
			{Name: "managed"},
		},
		RawSpec: resource.ManagedFilePartialSpec{
			Path: path,
			Keys: []resource.PartialKey{
				{Key: "shared", Value: "new"},
				{Key: "managed", Value: "yes"},
			},
		},
	}

	results := p.applyAddition(addition)

	for _, r := range results {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
		}
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	s := string(content)
	// Managed keys should be present
	if !containsSubstring(s, `"shared"`) || !containsSubstring(s, `"new"`) {
		t.Error("Missing managed key update")
	}
	if !containsSubstring(s, `"managed"`) {
		t.Error("Missing new managed key")
	}
	// Existing unmanaged key should be preserved
	if !containsSubstring(s, `"existing"`) {
		t.Error("Unmanaged key was removed")
	}
}

func TestPatchTopLevelJSONKeyPreserveFormatting_ExistingKeyPreservesOtherBytes(t *testing.T) {
	existing := []byte("{\n  \"projects\": {\"/tmp\": {\"seen\": true}},\n  \"mcpServers\": {\"taskmaster\": {\"command\": \"npx\"}},\n  \"tengu_cache\": [1, 2, 3]\n}\n")
	newValue := []byte("{\"serena\":{\"command\":\"serena\"}}")

	got, err := PatchTopLevelJSONKeyPreserveFormatting(existing, "mcpServers", newValue)
	if err != nil {
		t.Fatalf("PatchTopLevelJSONKeyPreserveFormatting() error = %v", err)
	}

	want := "{\n  \"projects\": {\"/tmp\": {\"seen\": true}},\n  \"mcpServers\": {\"serena\":{\"command\":\"serena\"}},\n  \"tengu_cache\": [1, 2, 3]\n}\n"
	if string(got) != want {
		t.Fatalf("patched content mismatch\ngot:  %q\nwant: %q", string(got), want)
	}
}

func TestPatchTopLevelJSONKeyPreserveFormatting_AddsMissingKeyAtEnd(t *testing.T) {
	existing := []byte("{\n  \"alpha\": 1,\n  \"beta\": { \"nested\": true }\n}\n")
	newValue := []byte("[1,2,3]")

	got, err := PatchTopLevelJSONKeyPreserveFormatting(existing, "gamma", newValue)
	if err != nil {
		t.Fatalf("PatchTopLevelJSONKeyPreserveFormatting() error = %v", err)
	}

	want := "{\n  \"alpha\": 1,\n  \"beta\": { \"nested\": true },\n  \"gamma\": [1,2,3]\n}\n"
	if string(got) != want {
		t.Fatalf("patched content mismatch\ngot:  %q\nwant: %q", string(got), want)
	}
}

func TestPatchTopLevelJSONKeyPreserveFormatting_NestedValuesAndEscapedStrings(t *testing.T) {
	existing := []byte("{\n  \"mcpServers\": {\"keep\": [\"{not brace}\", \"quote \\\" ok\", {\"escaped\": \"\\\\\"}]},\n  \"after\": \"unchanged { }\"\n}")
	newValue := []byte(`{"replacement":[{"text":"braces { } and quote \" and slash \\"}]}`)

	got, err := PatchTopLevelJSONKeyPreserveFormatting(existing, "mcpServers", newValue)
	if err != nil {
		t.Fatalf("PatchTopLevelJSONKeyPreserveFormatting() error = %v", err)
	}

	want := "{\n  \"mcpServers\": {\"replacement\":[{\"text\":\"braces { } and quote \\\" and slash \\\\\"}]},\n  \"after\": \"unchanged { }\"\n}"
	if string(got) != want {
		t.Fatalf("patched content mismatch\ngot:  %q\nwant: %q", string(got), want)
	}
}

func TestComputePatchedJSONContent_UsesRawFormattedObjectValue(t *testing.T) {
	existing := []byte("{\n  \"mcpServers\": {\n    \"serena\": {\"command\": \"serena\"},\n    \"taskmaster\": {\"command\": \"npx\"},\n    \"obsidian\": {\"command\": \"obsidian-mcp\"}\n  },\n  \"pluginUsage\": {\"x\": 1}\n}\n")
	keys := []resource.PartialKey{
		{
			Key: "mcpServers",
			Value: `{ 
    "serena": {"command": "serena"},
    "obsidian": {"command": "obsidian-mcp"}
  }`,
		},
	}

	got, err := computePatchedJSONContent(existing, keys)
	if err != nil {
		t.Fatalf("computePatchedJSONContent() error = %v", err)
	}

	if strings.Contains(got, "taskmaster") {
		t.Fatalf("taskmaster entry was not removed: %s", got)
	}
	if !strings.Contains(got, "{ \n    \"serena\":") {
		t.Fatalf("formatted object value was compacted: %s", got)
	}
	if !strings.Contains(got, `  "pluginUsage": {"x": 1}`) {
		t.Fatalf("unmanaged key changed or disappeared: %s", got)
	}
}

func TestPartialFileProvider_Apply_JSONPreservesClaudeRuntimeFields(t *testing.T) {
	p := NewPartialFileProvider()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "claude.json")
	existing := "{\n  \"projects\": {\"/repo\": {\"history\": [\"keep\"]}},\n  \"cachedExperimentData\": {\"exp\": true},\n  \"mcpServers\": {\n    \"serena\": {\"command\": \"serena\"},\n    \"taskmaster\": {\"command\": \"npx\", \"args\": [\"-y\", \"task-master-ai@latest\"], \"env\": {\"TASK_MASTER_TOOLS\": \"standard\"}},\n    \"obsidian\": {\"command\": \"obsidian-mcp\"}\n  },\n  \"pluginUsage\": {\"x\": 1},\n  \"tengu_runtime\": [\"do-not-touch\"]\n}\n"
	if err := os.WriteFile(path, []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	addition := provider.GroupAddition{
		Kind:  resource.KindManagedFilePartial,
		Group: "claude-code-mcp",
		Items: []resource.ResourceItem{{Name: "mcpServers"}},
		RawSpec: resource.ManagedFilePartialSpec{
			Path: path,
			Keys: []resource.PartialKey{
				{Key: "mcpServers", Value: `{"serena":{"command":"serena"},"obsidian":{"command":"obsidian-mcp"}}`},
			},
		},
	}

	for _, result := range p.applyAddition(addition) {
		if result.Err != nil {
			t.Fatalf("applyAddition() unexpected error = %v", result.Err)
		}
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read patched file: %v", err)
	}
	s := string(content)
	if strings.Contains(s, "task-master-ai") {
		t.Fatalf("taskmaster entry was not removed: %s", s)
	}
	for _, unchanged := range []string{
		`  "projects": {"/repo": {"history": ["keep"]}},`,
		`  "cachedExperimentData": {"exp": true},`,
		`  "pluginUsage": {"x": 1},`,
		`  "tengu_runtime": ["do-not-touch"]`,
	} {
		if !strings.Contains(s, unchanged) {
			t.Fatalf("runtime/cache bytes changed or disappeared; missing %q in %s", unchanged, s)
		}
	}
}

func TestPartialFileProvider_Reconcile_DiffOnlyChangesManagedJSONKey(t *testing.T) {
	p := NewPartialFileProvider()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "claude.json")
	existing := "{\n  \"projects\": {\"/repo\": {\"history\": [\"keep\"]}},\n  \"mcpServers\": {\"taskmaster\": {\"command\": \"npx\"}},\n  \"pluginUsage\": {\"x\": 1}\n}\n"
	if err := os.WriteFile(path, []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.WithValue(context.Background(), planctx.PlanShowDiffKey, true)
	desired := []resource.ResourceGroup[any]{
		{
			Kind:  resource.KindManagedFilePartial,
			Name:  "claude-code-mcp",
			Items: []resource.ResourceItem{{Name: "mcpServers"}},
			RawSpec: resource.ManagedFilePartialSpec{
				Path: path,
				Keys: []resource.PartialKey{{Key: "mcpServers", Value: `{"serena":{"command":"serena"}}`}},
			},
		},
	}

	plan := p.Reconcile(ctx, desired, []provider.ResourceState{{Kind: resource.KindManagedFilePartial, Group: "claude-code-mcp"}})
	if len(plan.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", plan.Errors)
	}
	if len(plan.Modifications) != 1 || len(plan.Modifications[0].Changes) != 1 {
		t.Fatalf("expected one modification with one change, got %#v", plan.Modifications)
	}

	change := plan.Modifications[0].Changes[0]
	if change.OldContent != existing {
		t.Fatalf("OldContent mismatch")
	}
	if !strings.Contains(change.NewContent, `  "projects": {"/repo": {"history": ["keep"]}},`) {
		t.Fatalf("projects bytes changed or disappeared: %s", change.NewContent)
	}
	if !strings.Contains(change.NewContent, `  "pluginUsage": {"x": 1}`) {
		t.Fatalf("pluginUsage bytes changed or disappeared: %s", change.NewContent)
	}
	if strings.Contains(change.NewContent, "taskmaster") {
		t.Fatalf("taskmaster entry was not removed: %s", change.NewContent)
	}
}

func TestPartialFileProvider_Apply_CreatesParentDirs(t *testing.T) {
	p := NewPartialFileProvider()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nested", "deep", "test.json")

	addition := provider.GroupAddition{
		Kind:  resource.KindManagedFilePartial,
		Group: "test",
		Items: []resource.ResourceItem{{Name: "key"}},
		RawSpec: resource.ManagedFilePartialSpec{
			Path: path,
			Keys: []resource.PartialKey{
				{Key: "key", Value: "value"},
			},
		},
	}

	results := p.applyAddition(addition)

	for _, r := range results {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
		}
	}

	// Verify file was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("File was not created in nested directory")
	}
}

func TestPartialFileProvider_Available(t *testing.T) {
	p := NewPartialFileProvider()
	available, msg := p.Available()
	if !available {
		t.Error("Expected provider to be available")
	}
	if msg == "" {
		t.Error("Expected non-empty availability message")
	}
}

func TestPartialFileProvider_Name(t *testing.T) {
	p := NewPartialFileProvider()
	if name := p.Name(); name != "partialfile" {
		t.Errorf("expected name 'partialfile', got %q", name)
	}
}

// containsSubstring checks if s contains substr
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstringHelper(s, substr))
}

func containsSubstringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
