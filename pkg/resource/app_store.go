package resource

import "fmt"

// AppStoreApps defines Mac App Store applications to manage via the mas CLI.
type AppStoreApps struct {
	BaseResource `yaml:",inline"`
	Spec         AppStoreAppsSpec `yaml:"spec" validate:"required"`
}

// AppStoreAppsSpec contains the AppStoreApps configuration.
type AppStoreAppsSpec struct {
	// Apps to install from the Mac App Store.
	Apps []AppStoreApp `yaml:"apps" validate:"required,dive"`
}

// AppStoreApp represents a Mac App Store application identified by its ADAM ID.
type AppStoreApp struct {
	// ID is the numeric Mac App Store identifier (ADAM ID).
	ID int `yaml:"id" validate:"required"`

	// Name is an optional human-readable label for documentation.
	Name string `yaml:"name,omitempty"`
}

// Validate implements Resource.Validate.
func (r AppStoreApps) Validate() error {
	if err := ValidateStruct(r); err != nil {
		return err
	}
	return validateDependsOnAddresses(r.Metadata.DependsOn)
}

// ToGroup implements Resource.ToGroup.
func (r AppStoreApps) ToGroup() ResourceGroup[any] {
	items := make([]ResourceItem, 0, len(r.Spec.Apps))

	for _, app := range r.Spec.Apps {
		item := ResourceItem{
			Name: fmt.Sprintf("%d", app.ID),
		}
		if app.Name != "" {
			item.Metadata = map[string]string{
				"display_name": app.Name,
			}
		}
		items = append(items, item)
	}

	return ResourceGroup[any]{
		Kind:    r.Kind,
		Name:    r.Metadata.Name,
		Items:   items,
		RawSpec: r.Spec,
	}
}