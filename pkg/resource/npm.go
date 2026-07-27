package resource

import (
	"encoding/json"
	"fmt"
)

// Metadata keys used to carry executable-package details through ResourceItem.Metadata.
const (
	MetaExecutable = "executable" // value "true" when the package runs via npx
	MetaArgs       = "args"       // JSON-encoded []string of extra runner args
)

// NpmPackages defines global npm packages to install.
type NpmPackages struct {
	BaseResource `yaml:",inline"`
	Spec         NpmPackagesSpec `yaml:"spec" validate:"required"`
}

// NpmPackagesSpec contains the NpmPackages configuration.
type NpmPackagesSpec struct {
	// Packages to install globally (or run via npx when executable: true)
	Packages []Package `yaml:"packages" validate:"required,dive"`
}

// Validate implements Resource.Validate.
func (r NpmPackages) Validate() error {
	if err := ValidateStruct(r); err != nil {
		return err
	}
	for _, p := range r.Spec.Packages {
		if len(p.Args) > 0 && !p.Executable {
			return fmt.Errorf("npm package %q sets args without executable: true", p.Name)
		}
	}
	return validateDependsOnAddresses(r.Metadata.DependsOn)
}

// ToGroup implements Resource.ToGroup.
func (r NpmPackages) ToGroup() ResourceGroup[any] {
	items := make([]ResourceItem, 0, len(r.Spec.Packages))

	for _, p := range r.Spec.Packages {
		item := ResourceItem{
			Name:    p.Name,
			Version: p.Version,
		}
		if p.Executable {
			item.Metadata = map[string]string{MetaExecutable: "true"}
			if len(p.Args) > 0 {
				if encoded, err := json.Marshal(p.Args); err == nil {
					item.Metadata[MetaArgs] = string(encoded)
				}
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
