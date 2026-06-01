package resource

const (
	// Homebrew resource kinds (greenfield)
	KindHomeBrewPackages = "HomeBrewPackages"
	KindHomeBrewCasks    = "HomeBrewCasks"
	KindHomeBrewTaps     = "HomeBrewTaps"
	KindNpmPackages      = "NpmPackages"
	KindGoPackages       = "GoPackages"
	KindCargoPackages    = "CargoPackages"
	KindManagedFile      = "ManagedFile"
	KindAISkillPackages    = "AISkillPackages"
	KindAppStoreApps       = "AppStoreApps"
	KindManagedFilePartial = "ManagedFilePartial"
)

// IsBrewKind reports whether the provided kind corresponds to the
// Brew/Homebrew package resource types (legacy or new name).
func IsBrewKind(k string) bool {
	return k == KindHomeBrewPackages || k == KindHomeBrewCasks || k == KindHomeBrewTaps
}
