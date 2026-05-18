package providers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wasilak/nim/pkg/provider"
	"github.com/wasilak/nim/pkg/resource"
)

// minimalStateMock implements the subset of state interface used by FileProvider
type minimalStateMock struct{}

// Note: FileProvider.Reconcile expects []provider.ResourceState for state.
// We only need HasResource-like behaviour via the provided slice, so no methods required.

func TestFileProvider_Apply_UsesMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		wantPerm os.FileMode
	}{
		{"explicit 0755", "0755", 0755},
		{"explicit 0600", "0600", 0600},
		{"empty defaults to 0644", "", 0644},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			dest := filepath.Join(tmp, "testfile")
			p := NewFileProvider(tmp)

			addition := resource.ResourceItem{
				Name: dest,
				FileExtra: &resource.FileItemExtra{
					Source:      "(inline)",
					Inline:      "hello",
					Destination: dest,
					Mode:        tc.mode,
				},
			}
			results := p.applyGroupAddition(context.Background(), groupAdditionFrom(addition))
			for _, r := range results {
				if r.Err != nil {
					t.Fatalf("applyGroupAddition: %v", r.Err)
				}
			}

			info, err := os.Stat(dest)
			if err != nil {
				t.Fatalf("stat: %v", err)
			}
			if got := info.Mode().Perm(); got != tc.wantPerm {
				t.Errorf("mode = %04o, want %04o", got, tc.wantPerm)
			}
		})
	}
}

func TestFileProvider_Reconcile_DetectsModeDrift(t *testing.T) {
	tmp := t.TempDir()
	dest := filepath.Join(tmp, "managed")
	content := []byte("content")

	// Write with wrong mode (0600 but desired is 0755)
	if err := os.WriteFile(dest, content, 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	item := resource.ResourceItem{
		Name: dest,
		FileExtra: &resource.FileItemExtra{
			Source:      "(inline)",
			Inline:      string(content),
			Destination: dest,
			Mode:        "0755",
		},
	}
	group := resource.ResourceGroup[any]{
		Kind:  "ManagedFile",
		Name:  "myfiles",
		Items: []resource.ResourceItem{item},
	}

	p := NewFileProvider("")
	// Provide existing state so the item is "tracked"
	state := stateWithItem("ManagedFile", "myfiles", dest, p.hashFile(dest))
	plan := p.Reconcile(context.Background(), []resource.ResourceGroup[any]{group}, state)

	if len(plan.Modifications) == 0 {
		t.Fatalf("expected modification for mode drift, got none (additions=%d inSync=%d)", len(plan.Additions), len(plan.InSync))
	}
	if plan.Modifications[0].Changes[0].Diff != "mode changed" {
		t.Errorf("diff = %q, want %q", plan.Modifications[0].Changes[0].Diff, "mode changed")
	}
}

func TestFileProvider_Apply_FixesModeOnModification(t *testing.T) {
	tmp := t.TempDir()
	dest := filepath.Join(tmp, "managed")

	if err := os.WriteFile(dest, []byte("hello"), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	p := NewFileProvider(tmp)
	addition := resource.ResourceItem{
		Name: dest,
		FileExtra: &resource.FileItemExtra{
			Source:      "(inline)",
			Inline:      "hello",
			Destination: dest,
			Mode:        "0755",
		},
	}
	for _, r := range p.applyGroupAddition(context.Background(), groupAdditionFrom(addition)) {
		if r.Err != nil {
			t.Fatalf("apply: %v", r.Err)
		}
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0755 {
		t.Errorf("mode = %04o, want 0755", got)
	}
}

// helpers

func groupAdditionFrom(item resource.ResourceItem) provider.GroupAddition {
	return provider.GroupAddition{Kind: "ManagedFile", Group: "test", Items: []resource.ResourceItem{item}}
}

func stateWithItem(kind, group, name, checksum string) []provider.ResourceState {
	return []provider.ResourceState{{
		Kind:  kind,
		Group: group,
		Items: []resource.ItemState{{Name: name, Checksum: checksum, Status: "present"}},
	}}
}

func TestFileProvider_Apply_CreatesMissingDirectories(t *testing.T) {
	tmp := t.TempDir()
	// Nested path where parent directories don't exist
	dest := filepath.Join(tmp, "deeply", "nested", "path", "file.txt")
	p := NewFileProvider(tmp)

	addition := resource.ResourceItem{
		Name: dest,
		FileExtra: &resource.FileItemExtra{
			Source:      "(inline)",
			Inline:      "hello world",
			Destination: dest,
			Mode:        "0644",
		},
	}
	results := p.applyGroupAddition(context.Background(), groupAdditionFrom(addition))
	for _, r := range results {
		if r.Err != nil {
			t.Fatalf("applyGroupAddition: %v", r.Err)
		}
	}

	// Verify file was created
	content, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(content) != "hello world" {
		t.Errorf("content = %q, want %q", string(content), "hello world")
	}

	// Verify all parent directories were created
	dir := filepath.Dir(dest)
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("parent directory not created: %v", err)
	}
}

func TestFileProvider_Apply_IdempotentDirectoryCreation(t *testing.T) {
	tmp := t.TempDir()
	dest := filepath.Join(tmp, "existing", "path", "file.txt")
	p := NewFileProvider(tmp)

	// First apply - creates directories and file
	addition := resource.ResourceItem{
		Name: dest,
		FileExtra: &resource.FileItemExtra{
			Source:      "(inline)",
			Inline:      "first write",
			Destination: dest,
			Mode:        "0644",
		},
	}
	results1 := p.applyGroupAddition(context.Background(), groupAdditionFrom(addition))
	for _, r := range results1 {
		if r.Err != nil {
			t.Fatalf("first apply: %v", r.Err)
		}
	}

	// Second apply to same path - should not error
	addition.FileExtra.Inline = "second write"
	results2 := p.applyGroupAddition(context.Background(), groupAdditionFrom(addition))
	for _, r := range results2 {
		if r.Err != nil {
			t.Fatalf("second apply: %v", r.Err)
		}
	}

	// Verify content was updated
	content, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(content) != "second write" {
		t.Errorf("content = %q, want %q", string(content), "second write")
	}
}

func TestFileProvider_Apply_HomeDirectoryDestination(t *testing.T) {
	// This test verifies that destinations starting with ~ are handled correctly
	// The directory creation should happen after path expansion
	tmp := t.TempDir()
	p := NewFileProvider(tmp)

	// Use a path that looks like a home directory path but in temp
	homeDir := tmp
	dest := filepath.Join(homeDir, ".config", "test", "file.txt")
	addition := resource.ResourceItem{
		Name: dest,
		FileExtra: &resource.FileItemExtra{
			Source:      "(inline)",
			Inline:      "config content",
			Destination: dest,
			Mode:        "0644",
		},
	}
	results := p.applyGroupAddition(context.Background(), groupAdditionFrom(addition))
	for _, r := range results {
		if r.Err != nil {
			t.Fatalf("apply: %v", r.Err)
		}
	}

	// Verify file exists
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("file not created: %v", err)
	}
}

func TestFileProvider_Apply_DirectoryCreationError(t *testing.T) {
	// Test that directory creation errors are properly wrapped
	tmp := t.TempDir()
	p := NewFileProvider(tmp)

	// Create a file where we want a directory to be
	// This should cause mkdir to fail
	blockingPath := filepath.Join(tmp, "blocking")
	if err := os.WriteFile(blockingPath, []byte("x"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Try to create a file under the blocking file (impossible)
	dest := filepath.Join(blockingPath, "file.txt")
	addition := resource.ResourceItem{
		Name: dest,
		FileExtra: &resource.FileItemExtra{
			Source:      "(inline)",
			Inline:      "content",
			Destination: dest,
			Mode:        "0644",
		},
	}
	results := p.applyGroupAddition(context.Background(), groupAdditionFrom(addition))

	// Should have an error
	if len(results) == 0 || results[0].Err == nil {
		t.Fatal("expected error when directory creation blocked, got none")
	}

	// Error should mention the parent directory
	errMsg := results[0].Err.Error()
	if !strings.Contains(errMsg, "parent directory") {
		t.Errorf("error message should mention 'parent directory', got: %v", errMsg)
	}
}

func TestFileProvider_Reconcile_EmitsInfoForDirectoryCreation(t *testing.T) {
	tmp := t.TempDir()
	// Create a nested path that doesn't exist
	dest := filepath.Join(tmp, "new", "nested", "path", "file.txt")

	item := resource.ResourceItem{
		Name: "testfile",
		FileExtra: &resource.FileItemExtra{
			Source:      "(inline)",
			Inline:      "test content",
			Destination: dest,
			Mode:        "0644",
		},
	}
	group := resource.ResourceGroup[any]{
		Kind:  "ManagedFile",
		Name:  "myfiles",
		Items: []resource.ResourceItem{item},
	}

	p := NewFileProvider("")
	plan := p.Reconcile(context.Background(), []resource.ResourceGroup[any]{group}, nil)

	// Should have an addition
	if len(plan.Additions) == 0 {
		t.Fatalf("expected additions, got none")
	}

	// Should have an info warning about directory creation
	var foundDirInfo bool
	for _, w := range plan.Warnings {
		if w.Severity == "info" && strings.Contains(w.Message, "Will create parent directory") {
			foundDirInfo = true
			// Verify the path is mentioned
			if !strings.Contains(w.Message, filepath.Dir(dest)) {
				t.Errorf("warning message missing expected directory path: %v", w.Message)
			}
			break
		}
	}
	if !foundDirInfo {
		t.Errorf("expected info warning about directory creation, got warnings: %+v", plan.Warnings)
	}
}

func TestFileProvider_Reconcile_NoDirInfoForExistingDir(t *testing.T) {
	tmp := t.TempDir()
	// Create the parent directory
	existingDir := filepath.Join(tmp, "existing")
	if err := os.MkdirAll(existingDir, 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	dest := filepath.Join(existingDir, "file.txt")

	item := resource.ResourceItem{
		Name: "testfile",
		FileExtra: &resource.FileItemExtra{
			Source:      "(inline)",
			Inline:      "test content",
			Destination: dest,
			Mode:        "0644",
		},
	}
	group := resource.ResourceGroup[any]{
		Kind:  "ManagedFile",
		Name:  "myfiles",
		Items: []resource.ResourceItem{item},
	}

	p := NewFileProvider("")
	plan := p.Reconcile(context.Background(), []resource.ResourceGroup[any]{group}, nil)

	// Should NOT have an info warning about directory creation
	for _, w := range plan.Warnings {
		if w.Severity == "info" && strings.Contains(w.Message, "Will create parent directory") {
			t.Errorf("unexpected directory creation warning for existing dir: %v", w.Message)
		}
	}
}

func TestFileProvider_Reconcile_EmitsWarningForExistingDestination(t *testing.T) {
	tmp := t.TempDir()
	dest := filepath.Join(tmp, ".zshrc")
	if err := os.WriteFile(dest, []byte("existing"), 0o644); err != nil {
		t.Fatalf("write dest: %v", err)
	}

	// Build desired resource group with one ManagedFile item pointing to dest
	item := resource.ResourceItem{
		Name:      "zshrc",
		FileExtra: &resource.FileItemExtra{Destination: dest},
	}
	group := resource.ResourceGroup[any]{
		Kind:  "ManagedFile",
		Name:  "myfiles",
		Items: []resource.ResourceItem{item},
	}

	// empty state (resource not tracked)
	// pass nil state (no saved state) to indicate resource is not tracked
	p := NewFileProvider("")
	plan := p.Reconcile(context.Background(), []resource.ResourceGroup[any]{group}, nil)

	// Expect an addition
	if len(plan.Additions) == 0 {
		t.Fatalf("expected additions when destination exists and resource not in state")
	}

	// Expect a warning
	if len(plan.Warnings) == 0 {
		t.Fatalf("expected warning when destination exists and resource not in state")
	}
	w := plan.Warnings[0]
	if !strings.Contains(w.Message, "Destination file already exists") {
		t.Fatalf("unexpected warning message: %v", w.Message)
	}
	if !strings.Contains(w.Suggestion, "nim state import") {
		t.Fatalf("unexpected suggestion: %v", w.Suggestion)
	}
}
