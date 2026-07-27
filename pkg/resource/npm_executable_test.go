package resource

import (
	"strings"
	"testing"
)

func newBase(kind string) BaseResource {
	return BaseResource{
		APIVersion: "github.com/wasilak/nim/v1",
		Kind:       kind,
		Metadata:   Metadata{Name: "test"},
	}
}

func TestNpmToGroup_ExecutableMetadata(t *testing.T) {
	r := NpmPackages{
		BaseResource: newBase(KindNpmPackages),
		Spec: NpmPackagesSpec{Packages: []Package{
			{Name: "typescript"},
			{Name: "claude-code-templates", Version: "latest", Executable: true, Args: []string{"--setting", "statusline/context-monitor"}},
		}},
	}

	group := r.ToGroup()
	if len(group.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(group.Items))
	}

	// Normal package carries no executable metadata.
	if group.Items[0].Metadata != nil {
		t.Errorf("normal package should have nil metadata, got %v", group.Items[0].Metadata)
	}

	// Executable package carries the flag and JSON-encoded args.
	exec := group.Items[1]
	if exec.Metadata[MetaExecutable] != "true" {
		t.Errorf("expected executable metadata true, got %q", exec.Metadata[MetaExecutable])
	}
	wantArgs := `["--setting","statusline/context-monitor"]`
	if exec.Metadata[MetaArgs] != wantArgs {
		t.Errorf("args metadata:\n got: %s\nwant: %s", exec.Metadata[MetaArgs], wantArgs)
	}
}

func TestNpmValidate_ArgsWithoutExecutable(t *testing.T) {
	r := NpmPackages{
		BaseResource: newBase(KindNpmPackages),
		Spec:         NpmPackagesSpec{Packages: []Package{{Name: "x", Args: []string{"--foo"}}}},
	}
	err := r.Validate()
	if err == nil || !strings.Contains(err.Error(), "args without executable") {
		t.Fatalf("expected args-without-executable error, got %v", err)
	}
}

func TestNonNpmKinds_RejectExecutable(t *testing.T) {
	cargo := CargoPackages{
		BaseResource: newBase(KindCargoPackages),
		Spec:         CargoPackagesSpec{Packages: []Package{{Name: "ripgrep", Executable: true}}},
	}
	if err := cargo.Validate(); err == nil || !strings.Contains(err.Error(), "only supported for NpmPackages") {
		t.Fatalf("CargoPackages should reject executable, got %v", err)
	}

	brew := HomeBrewPackages{
		BaseResource: newBase(KindHomeBrewPackages),
		Spec:         HomeBrewPackagesSpec{Formulae: []Package{{Name: "jq", Executable: true}}},
	}
	if err := brew.Validate(); err == nil || !strings.Contains(err.Error(), "only supported for NpmPackages") {
		t.Fatalf("HomeBrewPackages should reject executable, got %v", err)
	}
}
