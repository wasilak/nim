package resource

// ManagedFilePartial manages a subset of keys in a JSON or YAML file.
// Unlike ManagedFile which manages entire files, this resource only manages
// the declared keys and leaves all other content untouched.
type ManagedFilePartial struct {
	BaseResource `yaml:",inline"`
	Spec         ManagedFilePartialSpec `yaml:"spec" validate:"required"`
}

// ManagedFilePartialSpec defines the desired state of a partial file.
type ManagedFilePartialSpec struct {
	// Path is the absolute path to the file to manage
	// Supports template expressions like "{{ .Env.HOME }}/.config/tool.yaml"
	Path string `yaml:"path" validate:"required"`

	// Keys is the list of key-value pairs to manage in the file
	Keys []PartialKey `yaml:"keys" validate:"required,min=1,dive"`
}

// PartialKey represents a single key-value pair to manage.
// The value is rendered as a Go template before being applied.
type PartialKey struct {
	Key   string `yaml:"key" validate:"required"`
	Value string `yaml:"value"` // Can be empty string or a template
}

// Validate implements Resource.Validate.
func (r ManagedFilePartial) Validate() error {
	if err := ValidateStruct(r); err != nil {
		return err
	}
	return validateDependsOnAddresses(r.Metadata.DependsOn)
}

// ToGroup implements Resource.ToGroup.
func (r ManagedFilePartial) ToGroup() ResourceGroup[any] {
	items := make([]ResourceItem, 0, len(r.Spec.Keys))

	for _, k := range r.Spec.Keys {
		items = append(items, ResourceItem{
			Name: k.Key,
			Metadata: map[string]string{
				"value": k.Value,
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
