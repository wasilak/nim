package resource

import "fmt"

// Package represents a generic named package with optional version.
type Package struct {
	Name      string   `yaml:"name"`
	Version   string   `yaml:"version,omitempty"`
	DependsOn []string `yaml:"dependsOn,omitempty"`

	// Executable, when true, means the package is run ephemerally via a runner
	// (npm → npx) instead of being installed globally. Only supported for
	// NpmPackages; other package kinds reject it via requireNonExecutable.
	Executable bool `yaml:"executable,omitempty"`

	// Args are extra arguments passed to the runner after the package name.
	// Only meaningful together with Executable.
	Args []string `yaml:"args,omitempty"`
}

// requireNonExecutable rejects Executable/Args on package kinds that do not
// support ephemeral execution, so a stray `executable: true` fails loudly
// instead of being silently ignored.
func requireNonExecutable(kind string, pkgs []Package) error {
	for _, p := range pkgs {
		if p.Executable {
			return fmt.Errorf("%s: package %q sets executable: true, which is only supported for NpmPackages", kind, p.Name)
		}
		if len(p.Args) > 0 {
			return fmt.Errorf("%s: package %q sets args, which is only supported for executable NpmPackages", kind, p.Name)
		}
	}
	return nil
}

// Tap represents a Homebrew tap entry.
type Tap struct {
	Name      string   `yaml:"name"`
	Version   string   `yaml:"version,omitempty"`
	DependsOn []string `yaml:"dependsOn,omitempty"`
}
