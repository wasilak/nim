// Package resource provides the core resource types and interfaces for nim.
//
// Resources follow a Kubernetes-style declarative model where each resource
// has apiVersion, kind, metadata, and a kind-specific spec.
//
// Example resource YAML:
//
//	apiVersion: nim/v1
//	kind: BrewPackages
//	metadata:
//	  name: core-tools
//	  namespace: default
//	spec:
//	  formulae:
//	    - name: ripgrep
//	    - name: fd
package resource

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

// init registers custom validators.
func init() {
	// Register file_mode validator (e.g., "0644", "0755")
	validate.RegisterValidation("file_mode", validateFileMode)
}

// fileModeRegex matches valid Unix file modes (4-digit octal like 0644, 0755)
var fileModeRegex = regexp.MustCompile(`^[0-7]{4}$`)

// validateFileMode validates a file mode string.
func validateFileMode(fl validator.FieldLevel) bool {
	mode := fl.Field().String()
	if mode == "" {
		return true // Empty is allowed (will use default)
	}
	return fileModeRegex.MatchString(mode)
}

// Resource is the interface implemented by all resource types.
// It provides common access to resource metadata and validation.
type Resource interface {
	// GetAPIVersion returns the API version (e.g. "github.com/wasilak/nim/v1")
	GetAPIVersion() string

	// GetKind returns the resource kind (e.g. "HomeBrewPackages")
	GetKind() string

	// GetMetadata returns the resource metadata
	GetMetadata() Metadata

	// Validate validates the resource spec and returns any validation errors
	Validate() error

	// ToGroup converts this resource to a ResourceGroup representation
	// This extracts items from the spec and creates the 3-level hierarchy
	ToGroup() ResourceGroup[any]

	// CompileNamespace compiles the namespace regex if it uses /pattern/ syntax.
	// Must be called after YAML unmarshal.
	CompileNamespace() error

	// MatchesNamespace reports whether this resource matches the given active namespace.
	// Implements three-branch logic: regex match (if /pattern/), implicit default
	// (if empty), or exact string match.
	MatchesNamespace(activeNS string) bool
}

// Metadata contains common metadata for all resources.
type Metadata struct {
	// Name is the unique name for this resource within its namespace
	Name string `yaml:"name" validate:"required,min=1,max=253"`

	// Namespace is a logical grouping (defaults to "default")
	Namespace string `yaml:"namespace,omitempty" validate:"omitempty,min=1,max=253"`

	// Labels are optional key-value pairs for resource organization
	Labels map[string]string `yaml:"labels,omitempty"`

	// Annotations are optional metadata for tooling
	Annotations map[string]string `yaml:"annotations,omitempty"`

	// DependsOn lists resource names (namespace/name or name) this resource depends on.
	DependsOn []string `yaml:"dependsOn,omitempty"`

	// namespaceRe is the compiled regex for /pattern/ namespace values.
	// It is nil for bare strings or when Namespace is empty.
	namespaceRe *regexp.Regexp
}

// GetNamespace returns the namespace or "default" if not set.
func (m Metadata) GetNamespace() string {
	if m.Namespace == "" {
		return "default"
	}
	return m.Namespace
}

// CompileNamespace compiles the Namespace field into namespaceRe if it uses
// /pattern/ syntax. Must be called after YAML unmarshal. Returns an error
// for invalid regex patterns (fail-fast at parse time).
func (m *Metadata) CompileNamespace() error {
	if len(m.Namespace) >= 2 && m.Namespace[0] == '/' && m.Namespace[len(m.Namespace)-1] == '/' {
		pattern := m.Namespace[1 : len(m.Namespace)-1]
		re, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			return fmt.Errorf("parsing namespace %q: %w", m.Namespace, err)
		}
		m.namespaceRe = re
		if !strings.HasPrefix(pattern, "^") || !strings.HasSuffix(pattern, "$") {
			slog.Warn("namespace regex uses substring matching — add ^ and $ anchors for exact match",
				"namespace", m.Namespace, "resource", m.Name)
		}
	}
	return nil
}

// ResourceID returns a unique identifier for the resource (namespace/name).
// For regex namespaces the raw pattern is used as-is (it already ends with "/").
func (m Metadata) ResourceID() string {
	ns := m.GetNamespace()
	if ns[len(ns)-1] == '/' {
		return ns + m.Name
	}
	return fmt.Sprintf("%s/%s", ns, m.Name)
}

// BaseResource provides common fields embedded in all resource structs.
// It partially implements the Resource interface.
type BaseResource struct {
	APIVersion string   `yaml:"apiVersion" validate:"required"`
	Kind       string   `yaml:"kind" validate:"required"`
	Metadata   Metadata `yaml:"metadata" validate:"required"`
}

// GetAPIVersion implements Resource.GetAPIVersion.
func (r BaseResource) GetAPIVersion() string {
	return r.APIVersion
}

// GetKind implements Resource.GetKind.
func (r BaseResource) GetKind() string {
	return r.Kind
}

// GetMetadata implements Resource.GetMetadata.
func (r BaseResource) GetMetadata() Metadata {
	return r.Metadata
}

// CompileNamespace delegates to the embedded Metadata's CompileNamespace.
func (r *BaseResource) CompileNamespace() error {
	return r.Metadata.CompileNamespace()
}

// MatchesNamespaceFilter reports whether res should be included for activeNS.
// It handles the case where activeNS itself is a /pattern/ regex (e.g. when
// NIM_NAMESPACE="/(default|work)/"), which MatchesNamespace cannot handle
// because that method expects a plain string and matches in the opposite direction.
//
// When activeNS is a /pattern/ regex:
//   - Plain-namespace resources: included if the active regex matches their namespace.
//   - Regex-namespace resources: matched against candidate namespaces extracted from
//     the active pattern (handles common alternations like "(default|work)");
//     falls back to conservative include for complex patterns.
//
// When activeNS is a plain string, delegates to res.MatchesNamespace.
func MatchesNamespaceFilter(res Resource, activeNS string) bool {
	if len(activeNS) < 2 || activeNS[0] != '/' || activeNS[len(activeNS)-1] != '/' {
		return res.MatchesNamespace(activeNS)
	}

	pattern := activeNS[1 : len(activeNS)-1]
	activeRe, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		// Invalid active-NS regex: no match rather than panic.
		return false
	}

	meta := res.GetMetadata()

	if meta.namespaceRe == nil {
		// Plain-namespace resource: check if active regex matches its namespace.
		return activeRe.MatchString(meta.GetNamespace())
	}

	// Regex-namespace resource: try to extract literal alternation candidates
	// from the active pattern so we can ask "does this resource match any of them?"
	candidates := extractAlternatives(pattern)
	if candidates == nil {
		// Complex active pattern — include conservatively.
		return true
	}
	for _, c := range candidates {
		if res.MatchesNamespace(c) {
			return true
		}
	}
	return false
}

// extractAlternatives parses literal alternation tokens from simple regex patterns
// like "(default|work)" or "^(default|work|home)$". Returns nil for patterns
// containing regex metacharacters that prevent literal extraction.
func extractAlternatives(pattern string) []string {
	// Strip anchors and outer parens.
	p := strings.TrimPrefix(pattern, "^")
	p = strings.TrimSuffix(p, "$")
	p = strings.TrimPrefix(p, "(")
	p = strings.TrimSuffix(p, ")")
	// Bail on any regex metacharacter beyond simple alternation.
	if strings.ContainsAny(p, `*+?.[\{`) {
		return nil
	}
	parts := strings.Split(p, "|")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// MatchesNamespace reports whether this resource should be included when the
// active namespace is activeNS. Three-branch logic per D-07:
//  1. If namespaceRe != nil: return namespaceRe.MatchString(activeNS) (regex match)
//  2. Else if Namespace == "": return activeNS == "default" (implicit default)
//  3. Else: return Namespace == activeNS (exact match)
func (r BaseResource) MatchesNamespace(activeNS string) bool {
	if r.Metadata.namespaceRe != nil {
		return r.Metadata.namespaceRe.MatchString(activeNS)
	}
	if r.Metadata.Namespace == "" {
		return activeNS == "default"
	}
	return r.Metadata.Namespace == activeNS
}

// validate is a shared validator instance.
var validate = validator.New()

// ValidateStruct validates a struct using go-playground/validator.
// This is a helper for resource implementations to use in their Validate() method.
func ValidateStruct(s any) error {
	if err := validate.Struct(s); err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			return fmt.Errorf("validation failed: %v", validationErrors)
		}
		return err
	}
	return nil
}

// validateDependsOnAddresses checks that each entry in a DependsOn slice is a
// syntactically valid resource address (parseable by ParseResourceID).
// Cross-resource existence checks are deferred to the graph build phase.
func validateDependsOnAddresses(deps []string) error {
	for _, addr := range deps {
		if _, err := ParseResourceID(addr); err != nil {
			return fmt.Errorf("dependsOn: invalid address %q: %w", addr, err)
		}
	}
	return nil
}

// SupportedAPIVersion is the current supported API version.
const SupportedAPIVersion = "github.com/wasilak/nim/v1"

// IsSupportedAPIVersion checks if the given API version is supported.
func IsSupportedAPIVersion(version string) bool {
	return version == SupportedAPIVersion
}
