package resource

// ResourceGroup represents a level-2 entity (e.g., "core-tools" containing multiple packages).
// The type parameter T is an extension point for provider-specific spec data; current providers
// do not use RawSpec and T is always any at the Provider interface boundary.
type ResourceGroup[T any] struct {
	Kind    string
	Name    string
	Items   []ResourceItem
	RawSpec T
}

// ResourceItem represents a level-3 entity (e.g., "ripgrep" within "core-tools")
type ResourceItem struct {
	Name      string
	Version   string
	Checksum  string
	FileExtra *FileItemExtra    // non-nil only for ManagedFile items
	Metadata  map[string]string // provider-specific item metadata
	Notes     string            // post-apply notes to display (only for changed resources)
}

// ItemState represents the state of an individual item
type ItemState struct {
	Name      string         `json:"name"`
	Version   string         `json:"version,omitempty"`
	Checksum  string         `json:"checksum,omitempty"`
	Status    string         `json:"status,omitempty"` // present, absent, etc.
	FileExtra *FileItemExtra `json:"extra,omitempty"`  // json key "extra" preserves existing state-file format
	Notes     string         `json:"notes,omitempty"`  // post-apply notes
}

// ToGroup is a convenience helper for resources that carry no provider-specific spec.
func (br *BaseResource) ToGroup(items []ResourceItem) ResourceGroup[any] {
	return ResourceGroup[any]{
		Kind:  br.Kind,
		Name:  br.Metadata.Name,
		Items: items,
	}
}
