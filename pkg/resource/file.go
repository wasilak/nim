package resource

import "fmt"

// FileItemExtra holds the file-provider-specific metadata for a ResourceItem.
// FileProvider is the only provider that populates ResourceItem.FileExtra.
type FileItemExtra struct {
	Source      string `json:"source,omitempty"`
	Inline      string `json:"inline,omitempty"`
	Template    bool   `json:"template,omitempty"`
	Mode        string `json:"mode,omitempty"`
	Destination string `json:"destination,omitempty"`
}

// ManagedFile defines a single file to manage.
// The file can be templated or static, with content from inline source or external file.
type ManagedFile struct {
	BaseResource `yaml:",inline"`
	Spec         ManagedFileSpec `yaml:"spec" validate:"required"`
}

// ManagedFileSpec contains the ManagedFile configuration.
// Exactly one of Source (inline) or SourceFile (external file) must be specified,
// unless Generator is set — in which case all other source fields must be absent.
type ManagedFileSpec struct {
	// Source is inline content (use | in YAML for multi-line)
	// Example:
	//   source: |
	//     # My config
	//     export EDITOR=vim
	Source string `yaml:"source,omitempty"`

	// SourceFile is a path to an external file (relative to dotfiles root)
	// Example: sourceFile: shell/zshrc.sh
	SourceFile string `yaml:"sourceFile,omitempty"`

	// Destination is the absolute path where the file should be placed
	// Can contain template expressions like "{{ .Env.HOME }}/.zshrc"
	Destination string `yaml:"destination,omitempty" validate:"omitempty"`

	// Template indicates if the source should be processed as a Go template
	Template bool `yaml:"template,omitempty"`

	// Mode is the file permissions (e.g., "0644", "0755")
	// Defaults to 0644 if not specified
	Mode string `yaml:"mode,omitempty" validate:"omitempty,file_mode"`

	// Files is a list of files to manage (new list-based syntax).
	// When populated, this takes precedence over the single-file fields above.
	// Each entry represents one file to manage.
	Files []FileSpec `yaml:"files,omitempty" validate:"omitempty,dive"`

	// Generator expands a values list into multiple files at load time.
	// Mutually exclusive with Source, SourceFile, and Files.
	Generator *GeneratorSpec `yaml:"generator,omitempty"`

	// Vars holds manifest-level variables that extend the template context as .Vars.*
	// when template: true is set. Values here take precedence over nothing — they are
	// additive alongside .Values, .Env, and .OS.
	Vars map[string]any `yaml:"vars,omitempty"`

	// Notes is an optional message to display after the resource is applied.
	// Use this for post-install caveats or reminders (like Homebrew caveats).
	// Notes are only displayed when a resource is created or modified.
	Notes string `yaml:"notes,omitempty"`
}

// GeneratorSpec defines how to generate multiple files from a values list.
type GeneratorSpec struct {
	// SourceKey is a dot-notation path into .Values resolving to a list (e.g. "skills" or "agents.skills")
	SourceKey string `yaml:"sourceKey" validate:"required"`

	// Template is a Go text/template used to render file content for each item.
	// Mutually exclusive with SourceFilePattern.
	Template string `yaml:"template,omitempty"`

	// SourceFilePattern is a Go text/template that renders to a file path (relative to the
	// resource file's directory) whose content is used as-is for each item.
	// Mutually exclusive with Template.
	SourceFilePattern string `yaml:"sourceFilePattern,omitempty"`

	// DestinationPattern is a Go text/template used to render each file's destination path
	DestinationPattern string `yaml:"destinationPattern" validate:"required"`

	// Mode is the file permissions for generated files (e.g., "0644")
	Mode string `yaml:"mode,omitempty" validate:"omitempty,file_mode"`

	// DependsOn lists resource names this generator depends on.
	DependsOn []string `yaml:"dependsOn,omitempty"`
}

// FileSpec represents a single file in the list-based ManagedFile structure.
type FileSpec struct {
	// Source is inline content for this file
	Source string `yaml:"source,omitempty"`

	// SourceFile is a path to an external file for this file
	SourceFile string `yaml:"sourceFile,omitempty"`

	// Destination is the absolute path for this file
	Destination string `yaml:"destination" validate:"required"`

	// Template indicates if the source should be processed as a Go template
	Template bool `yaml:"template,omitempty"`

	// Mode is the file permissions for this file
	Mode string `yaml:"mode,omitempty" validate:"omitempty,file_mode"`

	// DependsOn lists resource names this file depends on.
	DependsOn []string `yaml:"dependsOn,omitempty"`

	// Vars holds per-file variables that extend the template context as .Vars.*
	// when template: true is set.
	Vars map[string]any `yaml:"vars,omitempty"`
}

// Validate implements Resource.Validate.
func (r ManagedFile) Validate() error {
	// Standard struct validation
	if err := ValidateStruct(r); err != nil {
		return err
	}

	hasGenerator := r.Spec.Generator != nil
	hasSource := r.Spec.Source != ""
	hasSourceFile := r.Spec.SourceFile != ""
	hasFiles := len(r.Spec.Files) > 0

	// Generator is mutually exclusive with all other source fields
	if hasGenerator {
		if hasSource || hasSourceFile || hasFiles {
			return fmt.Errorf("ManagedFile.spec: 'generator' is mutually exclusive with 'source', 'sourceFile', and 'files'")
		}
		gen := r.Spec.Generator
		hasTemplate := gen.Template != ""
		hasSourceFilePattern := gen.SourceFilePattern != ""
		if !hasTemplate && !hasSourceFilePattern {
			return fmt.Errorf("ManagedFile.spec.generator: exactly one of 'template' or 'sourceFilePattern' is required")
		}
		if hasTemplate && hasSourceFilePattern {
			return fmt.Errorf("ManagedFile.spec.generator: 'template' and 'sourceFilePattern' are mutually exclusive")
		}
		return nil
	}

	// Non-generator mode: require destination
	if r.Spec.Destination == "" && !hasFiles {
		return fmt.Errorf("ManagedFile.spec: 'destination' is required")
	}

	// Single-file mode: exactly one of Source or SourceFile must be set
	if !hasFiles {
		if !hasSource && !hasSourceFile {
			return fmt.Errorf("ManagedFile.spec: exactly one of 'source' (inline) or 'sourceFile' (external file) is required")
		}
		if hasSource && hasSourceFile {
			return fmt.Errorf("ManagedFile.spec: 'source' and 'sourceFile' are mutually exclusive, use exactly one")
		}
	}

	return validateDependsOnAddresses(r.Metadata.DependsOn)
}

// GetFiles returns the list of files to manage with ~ expanded in all destinations.
// If Files is populated (new syntax), returns that.
// Otherwise, converts the single-file fields to a list (backward compatibility).
func (r ManagedFile) GetFiles() []FileSpec {
	var files []FileSpec

	if len(r.Spec.Files) > 0 {
		files = make([]FileSpec, len(r.Spec.Files))
		copy(files, r.Spec.Files)
	} else {
		files = []FileSpec{{
			Source:      r.Spec.Source,
			SourceFile:  r.Spec.SourceFile,
			Destination: r.Spec.Destination,
			Template:    r.Spec.Template,
			Mode:        r.Spec.Mode,
		}}
	}

	for i := range files {
		files[i].Destination = expandTilde(files[i].Destination)
	}

	return files
}

// ToGroup implements Resource.ToGroup.
func (r ManagedFile) ToGroup() ResourceGroup[any] {
	files := r.GetFiles()
	items := make([]ResourceItem, 0, len(files))

	for i, f := range files {
		itemName := f.Destination
		if itemName == "" {
			itemName = fmt.Sprintf("file-%d", i)
		}

		source := f.SourceFile
		inline := ""
		if source == "" {
			// Mark source as inline and preserve inline content
			source = "(inline)"
			inline = f.Source
		}

		items = append(items, ResourceItem{
			Name:  itemName,
			Notes: r.Spec.Notes, // Copy notes from spec to each item
			FileExtra: &FileItemExtra{
				Source:      source,
				Inline:      inline,
				Template:    f.Template,
				Mode:        f.Mode,
				Destination: f.Destination,
			},
		})
	}

	return ResourceGroup[any]{
		Kind:    r.Kind,
		Name:    r.Metadata.Name,
		Items:   items,
		RawSpec: r.Spec,
	}
}
