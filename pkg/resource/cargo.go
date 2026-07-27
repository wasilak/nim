package resource

// CargoPackages defines Rust crates to install via `cargo install`.
type CargoPackages struct {
	BaseResource `yaml:",inline"`
	Spec         CargoPackagesSpec `yaml:"spec" validate:"required"`
}

// CargoPackagesSpec contains the CargoPackages configuration.
type CargoPackagesSpec struct {
	// Packages to install
	Packages []Package `yaml:"packages" validate:"required,dive"`
}

// Validate implements Resource.Validate.
func (r CargoPackages) Validate() error {
	if err := ValidateStruct(r); err != nil {
		return err
	}
	if err := requireNonExecutable(r.Kind, r.Spec.Packages); err != nil {
		return err
	}
	return validateDependsOnAddresses(r.Metadata.DependsOn)
}

// ToGroup implements Resource.ToGroup.
func (r CargoPackages) ToGroup() ResourceGroup[any] {
	items := make([]ResourceItem, 0, len(r.Spec.Packages))

	for _, p := range r.Spec.Packages {
		items = append(items, ResourceItem{
			Name:    p.Name,
			Version: p.Version,
		})
	}

	return ResourceGroup[any]{
		Kind:    r.Kind,
		Name:    r.Metadata.Name,
		Items:   items,
		RawSpec: r.Spec,
	}
}
