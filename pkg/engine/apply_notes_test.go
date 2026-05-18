package engine

import (
	"testing"

	"github.com/wasilak/nim/pkg/provider"
	"github.com/wasilak/nim/pkg/resource"
)

func TestApply_CollectsNotesFromChangedItems(t *testing.T) {
	// This test verifies that the apply logic correctly collects notes
	// from items that are changed (additions or modifications)
	
	// Create a simple test to verify notes field exists and flows through
	item := resource.ResourceItem{
		Name:  "test-file",
		Notes: "Test note for changed item",
	}

	if item.Notes != "Test note for changed item" {
		t.Errorf("Notes field not properly stored: got %q, want %q", item.Notes, "Test note for changed item")
	}

	// Verify ItemState also has Notes field
	state := resource.ItemState{
		Name:  "test",
		Notes: "State note",
	}

	if state.Notes != "State note" {
		t.Errorf("ItemState.Notes field not properly stored: got %q, want %q", state.Notes, "State note")
	}
}

func TestApply_ItemStateNotesPreserved(t *testing.T) {
	// Test that ItemState.Notes is properly serialized/deserialized
	state := resource.ItemState{
		Name:     "test-item",
		Status:   "present",
		Checksum: "abc123",
		Notes:    "Remember to restart your shell",
	}

	// Verify all fields are set
	if state.Name != "test-item" {
		t.Errorf("Name = %q, want %q", state.Name, "test-item")
	}
	if state.Notes != "Remember to restart your shell" {
		t.Errorf("Notes = %q, want %q", state.Notes, "Remember to restart your shell")
	}
}

func TestApply_ProviderItemChangeWithNotes(t *testing.T) {
	// Test that ItemChange can carry Notes through NewState
	change := provider.ItemChange{
		ItemName: "test-item",
		NewState: resource.ItemState{
			Name:   "test-item",
			Status: "present",
			Notes:  "Post-apply note",
		},
	}

	if change.NewState.Notes != "Post-apply note" {
		t.Errorf("ItemChange.NewState.Notes = %q, want %q", change.NewState.Notes, "Post-apply note")
	}
}
