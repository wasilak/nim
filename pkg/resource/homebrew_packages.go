package resource

import (
	"log/slog"
)

// HomeBrewPackages defines Homebrew formulae (command-line packages).
type HomeBrewPackages struct {
	BaseResource `yaml:",inline"`
	Spec         HomeBrewPackagesSpec `yaml:"spec" validate:"required"`
}

// HomeBrewPackagesSpec contains the HomeBrewPackages configuration.
type HomeBrewPackagesSpec struct {
	// Formulae to install (command-line tools)
	Formulae []Package `yaml:"formulae,omitempty" validate:"dive"`
}

// Validate implements Resource.Validate.
func (r HomeBrewPackages) Validate() error {
	if err := ValidateStruct(r); err != nil {
		return err
	}
	if err := requireNonExecutable(r.Kind, r.Spec.Formulae); err != nil {
		return err
	}
	return validateDependsOnAddresses(r.Metadata.DependsOn)
}

// ToGroup implements Resource.ToGroup.
func (r HomeBrewPackages) ToGroup() ResourceGroup[any] {
	items := make([]ResourceItem, 0)

	slog.Debug("HomeBrewPackages.ToGroup",
		"group", r.Metadata.Name,
		"formulae", len(r.Spec.Formulae),
	)

	// Add formulae as items
	for _, f := range r.Spec.Formulae {
		items = append(items, ResourceItem{
			Name:    f.Name,
			Version: f.Version,
		})
	}

	return ResourceGroup[any]{
		Kind:    r.Kind,
		Name:    r.Metadata.Name,
		Items:   items,
		RawSpec: r.Spec,
	}
}
