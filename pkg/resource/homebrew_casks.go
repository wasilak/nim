package resource

import (
	"log/slog"
)

// HomeBrewCasks defines Homebrew casks (GUI applications).
type HomeBrewCasks struct {
	BaseResource `yaml:",inline"`
	Spec         HomeBrewCasksSpec `yaml:"spec" validate:"required"`
}

// HomeBrewCasksSpec contains the HomeBrewCasks configuration.
type HomeBrewCasksSpec struct {
	Casks []Package `yaml:"casks,omitempty" validate:"dive"`
}

// Validate implements Resource.Validate.
func (r HomeBrewCasks) Validate() error {
	if err := ValidateStruct(r); err != nil {
		return err
	}
	if err := requireNonExecutable(r.Kind, r.Spec.Casks); err != nil {
		return err
	}
	return validateDependsOnAddresses(r.Metadata.DependsOn)
}

// ToGroup implements Resource.ToGroup.
func (r HomeBrewCasks) ToGroup() ResourceGroup[any] {
	items := make([]ResourceItem, 0)

	slog.Debug("HomeBrewCasks.ToGroup",
		"group", r.Metadata.Name,
		"casks", len(r.Spec.Casks),
	)

	for _, c := range r.Spec.Casks {
		// Use plain cask name; provider will treat casks based on group.Kind
		items = append(items, ResourceItem{
			Name:    c.Name,
			Version: c.Version,
		})
	}

	return ResourceGroup[any]{
		Kind:    r.Kind,
		Name:    r.Metadata.Name,
		Items:   items,
		RawSpec: r.Spec,
	}
}
