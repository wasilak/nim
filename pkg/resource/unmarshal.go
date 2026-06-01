package resource

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// UnmarshalYAML dynamically unmarshals a YAML resource based on its kind.
// It first extracts the apiVersion and kind fields, then creates the appropriate
// resource type and unmarshals the full document into it.
func UnmarshalYAML(data []byte) (Resource, error) {
	// First, extract just the type information
	type yamlTypeInfo struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
	}

	var typeInfo yamlTypeInfo
	if err := yaml.Unmarshal(data, &typeInfo); err != nil {
		return nil, fmt.Errorf("failed to parse resource type info: %w", err)
	}

	// Validate API version
	if !IsSupportedAPIVersion(typeInfo.APIVersion) {
		return nil, fmt.Errorf("unsupported apiVersion: %s (supported: %s)", typeInfo.APIVersion, SupportedAPIVersion)
	}

	// Create the appropriate resource type based on kind
	var resource Resource
	switch typeInfo.Kind {
	case KindHomeBrewPackages:
		resource = &HomeBrewPackages{}
	case KindHomeBrewCasks:
		resource = &HomeBrewCasks{}
	case KindHomeBrewTaps:
		resource = &HomeBrewTaps{}
	case KindNpmPackages:
		resource = &NpmPackages{}
	case KindGoPackages:
		resource = &GoPackages{}
	case KindCargoPackages:
		resource = &CargoPackages{}
	case KindManagedFile:
		resource = &ManagedFile{}
	case KindAISkillPackages:
		resource = &AISkillPackages{}
	case KindAppStoreApps:
		resource = &AppStoreApps{}
	case KindManagedFilePartial:
		resource = &ManagedFilePartial{}
	default:
		return nil, fmt.Errorf("unknown resource kind: %s", typeInfo.Kind)
	}

	// Unmarshal the full document into the resource
	if err := yaml.Unmarshal(data, resource); err != nil {
		return nil, fmt.Errorf("failed to unmarshal %s resource: %w", typeInfo.Kind, err)
	}

	// Compile namespace regex if the namespace uses /pattern/ syntax
	if err := resource.CompileNamespace(); err != nil {
		return nil, fmt.Errorf("failed to compile namespace for %s resource: %w", typeInfo.Kind, err)
	}

	return resource, nil
}

// ValidResourceKinds returns all valid resource kind strings.
func ValidResourceKinds() []string {
	return []string{
		KindHomeBrewPackages,
		KindHomeBrewCasks,
		KindHomeBrewTaps,
		KindNpmPackages,
		KindGoPackages,
		KindCargoPackages,
		KindManagedFile,
		KindAISkillPackages,
		KindAppStoreApps,
		KindManagedFilePartial,
	}
}
