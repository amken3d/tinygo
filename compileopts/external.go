// Package compileopts contains support for external machine packages.
// External packages allow chip families and boards to be maintained
// separately from mainline TinyGo.

package compileopts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ExternalManifest represents the tinygo-external.json manifest file
// that describes an external machine package.
type ExternalManifest struct {
	// Version of the manifest format (currently "1.0")
	Version string `json:"version"`

	// Name is the unique identifier for this package
	Name string `json:"name"`

	// Namespace for target names (defaults to Name if not specified)
	// Used to prevent collisions: namespace/target
	Namespace string `json:"namespace,omitempty"`

	// Human-readable description
	Description string `json:"description,omitempty"`

	// Package author
	Author string `json:"author,omitempty"`

	// License identifier (e.g., "BSD-3-Clause")
	License string `json:"license,omitempty"`

	// ChipFamilies lists the chip families supported by this package
	ChipFamilies []string `json:"chip-families,omitempty"`

	// Targets is a glob pattern for target JSON files (e.g., "targets/*.json")
	Targets []string `json:"targets,omitempty"`

	// MachinePackages lists directories containing machine package Go files
	MachinePackages []string `json:"machine-packages,omitempty"`

	// DevicePackages lists directories containing device definition Go files
	DevicePackages []string `json:"device-packages,omitempty"`

	// LinkerScripts is a glob pattern for linker scripts
	LinkerScripts []string `json:"linker-scripts,omitempty"`

	// ExtraFiles lists additional files (assembly, etc.)
	ExtraFiles []string `json:"extra-files,omitempty"`

	// Compatibility specifies version constraints
	Compatibility ExternalCompatibility `json:"compatibility,omitempty"`

	// PeripheralStatus documents the testing status of each peripheral
	PeripheralStatus map[string]PeripheralStatus `json:"peripheral-status,omitempty"`
}

// ExternalCompatibility specifies version compatibility constraints.
type ExternalCompatibility struct {
	// MachineAPI is the machine package interface version
	MachineAPI string `json:"machine-api,omitempty"`

	// TargetSpec is the target JSON schema version
	TargetSpec string `json:"target-spec,omitempty"`

	// MinTinyGo is the minimum TinyGo version required
	MinTinyGo string `json:"min-tinygo,omitempty"`

	// MaxTinyGo is the maximum known-good TinyGo version
	MaxTinyGo string `json:"max-tinygo,omitempty"`
}

// PeripheralStatus documents the testing status of a peripheral.
type PeripheralStatus struct {
	// Status is one of: "verified", "implemented", "not-implemented"
	Status string `json:"status"`

	// TestTier indicates the verification level (0-4)
	// 0: Compiles, 1: Smoke test, 2: Loopback, 3: Integration, 4: Application
	TestTier int `json:"test-tier,omitempty"`

	// TestFile is the path to the test file
	TestFile string `json:"test-file,omitempty"`

	// LastVerified is the date of last verification (YYYY-MM-DD)
	LastVerified string `json:"last-verified,omitempty"`

	// TinyGoVersion is the TinyGo version used for verification
	TinyGoVersion string `json:"tinygo-version,omitempty"`

	// Notes contains additional information
	Notes string `json:"notes,omitempty"`
}

// ExternalPackage represents a loaded external package.
type ExternalPackage struct {
	// Path is the root directory of the external package
	Path string

	// Manifest is the parsed tinygo-external.json
	Manifest ExternalManifest

	// ResolvedTargetDirs contains absolute paths to target directories
	ResolvedTargetDirs []string

	// ResolvedMachineDirs contains absolute paths to machine package directories
	ResolvedMachineDirs []string

	// ResolvedDeviceDirs contains absolute paths to device package directories
	ResolvedDeviceDirs []string

	// ResolvedLinkerScripts contains absolute paths to linker scripts
	ResolvedLinkerScripts []string
}

// Namespace returns the effective namespace for this package.
func (p *ExternalPackage) Namespace() string {
	if p.Manifest.Namespace != "" {
		return p.Manifest.Namespace
	}
	return p.Manifest.Name
}

// externalPackageCache caches loaded external packages
var externalPackageCache []ExternalPackage

// externalPackagesLoaded tracks whether packages have been loaded
var externalPackagesLoaded bool

// GetExternalPackages returns all discovered external packages.
// Results are cached after the first call.
func GetExternalPackages() ([]ExternalPackage, error) {
	if externalPackagesLoaded {
		return externalPackageCache, nil
	}

	packages, err := discoverExternalPackages()
	if err != nil {
		return nil, err
	}

	externalPackageCache = packages
	externalPackagesLoaded = true
	return packages, nil
}

// ClearExternalPackageCache clears the cached external packages.
// Useful for testing.
func ClearExternalPackageCache() {
	externalPackageCache = nil
	externalPackagesLoaded = false
}

// discoverExternalPackages finds external packages from environment variables
// and other configured locations.
func discoverExternalPackages() ([]ExternalPackage, error) {
	var packages []ExternalPackage

	// Check TINYGO_EXTERNAL_PACKAGES environment variable
	// Format: colon-separated list of paths (or semicolon on Windows)
	envPath := os.Getenv("TINYGO_EXTERNAL_PACKAGES")
	if envPath != "" {
		separator := ":"
		if os.PathSeparator == '\\' {
			separator = ";"
		}

		for _, p := range strings.Split(envPath, separator) {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}

			pkg, err := LoadExternalPackage(p)
			if err != nil {
				return nil, fmt.Errorf("failed to load external package from %s: %w", p, err)
			}
			packages = append(packages, pkg)
		}
	}

	return packages, nil
}

// LoadExternalPackage loads an external package from the given directory.
func LoadExternalPackage(path string) (ExternalPackage, error) {
	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return ExternalPackage{}, fmt.Errorf("failed to resolve path %s: %w", path, err)
	}

	// Check if directory exists
	info, err := os.Stat(absPath)
	if err != nil {
		return ExternalPackage{}, fmt.Errorf("external package directory not found: %s", absPath)
	}
	if !info.IsDir() {
		return ExternalPackage{}, fmt.Errorf("external package path is not a directory: %s", absPath)
	}

	// Load manifest
	manifestPath := filepath.Join(absPath, "tinygo-external.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return ExternalPackage{}, fmt.Errorf("failed to read manifest at %s: %w", manifestPath, err)
	}

	var manifest ExternalManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return ExternalPackage{}, fmt.Errorf("failed to parse manifest at %s: %w", manifestPath, err)
	}

	// Validate required fields
	if manifest.Name == "" {
		return ExternalPackage{}, fmt.Errorf("manifest at %s missing required 'name' field", manifestPath)
	}

	pkg := ExternalPackage{
		Path:     absPath,
		Manifest: manifest,
	}

	// Resolve target directories
	for _, pattern := range manifest.Targets {
		resolved := resolvePackagePath(absPath, pattern)
		// Get directory from pattern (e.g., "targets/*.json" -> "targets")
		dir := filepath.Dir(resolved)
		if !containsString(pkg.ResolvedTargetDirs, dir) {
			if _, err := os.Stat(dir); err == nil {
				pkg.ResolvedTargetDirs = append(pkg.ResolvedTargetDirs, dir)
			}
		}
	}

	// Resolve machine package directories
	for _, dir := range manifest.MachinePackages {
		resolved := resolvePackagePath(absPath, dir)
		if _, err := os.Stat(resolved); err == nil {
			pkg.ResolvedMachineDirs = append(pkg.ResolvedMachineDirs, resolved)
		}
	}

	// Resolve device package directories
	for _, dir := range manifest.DevicePackages {
		resolved := resolvePackagePath(absPath, dir)
		if _, err := os.Stat(resolved); err == nil {
			pkg.ResolvedDeviceDirs = append(pkg.ResolvedDeviceDirs, resolved)
		}
	}

	// Resolve linker scripts
	for _, pattern := range manifest.LinkerScripts {
		resolved := resolvePackagePath(absPath, pattern)
		matches, _ := filepath.Glob(resolved)
		pkg.ResolvedLinkerScripts = append(pkg.ResolvedLinkerScripts, matches...)
	}

	return pkg, nil
}

// resolvePackagePath resolves a path relative to the package root.
// It handles ${PACKAGE_ROOT} substitution.
func resolvePackagePath(packageRoot, path string) string {
	// Replace ${PACKAGE_ROOT} with actual path
	path = strings.ReplaceAll(path, "${PACKAGE_ROOT}", packageRoot)

	// If not absolute, make it relative to package root
	if !filepath.IsAbs(path) {
		path = filepath.Join(packageRoot, path)
	}

	return path
}

// containsString checks if a slice contains a string.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// FindExternalTarget searches external packages for a target by name.
// Returns the package and target file path if found.
// The targetName can be either:
// - Simple name: "myboard" (searches all packages, error if ambiguous)
// - Qualified name: "namespace/myboard" (searches specific package)
func FindExternalTarget(targetName string) (*ExternalPackage, string, error) {
	packages, err := GetExternalPackages()
	if err != nil {
		return nil, "", err
	}

	// Check if target name is qualified (namespace/target)
	var namespace, simpleName string
	if idx := strings.Index(targetName, "/"); idx != -1 {
		namespace = targetName[:idx]
		simpleName = targetName[idx+1:]
	} else {
		simpleName = targetName
	}

	var matches []struct {
		pkg      *ExternalPackage
		filePath string
	}

	for i := range packages {
		pkg := &packages[i]

		// If namespace specified, only search matching package
		if namespace != "" && pkg.Namespace() != namespace {
			continue
		}

		// Search target directories for this target
		for _, targetDir := range pkg.ResolvedTargetDirs {
			targetFile := filepath.Join(targetDir, simpleName+".json")
			if _, err := os.Stat(targetFile); err == nil {
				matches = append(matches, struct {
					pkg      *ExternalPackage
					filePath string
				}{pkg, targetFile})
			}
		}
	}

	if len(matches) == 0 {
		return nil, "", nil // Not found in external packages
	}

	if len(matches) > 1 && namespace == "" {
		// Ambiguous - multiple packages have this target
		var pkgNames []string
		for _, m := range matches {
			pkgNames = append(pkgNames, m.pkg.Namespace())
		}
		return nil, "", fmt.Errorf(
			"target %q found in multiple external packages: %s\nUse qualified name: %s/%s",
			targetName,
			strings.Join(pkgNames, ", "),
			pkgNames[0],
			targetName,
		)
	}

	return matches[0].pkg, matches[0].filePath, nil
}

// ResolveExternalPath resolves a path that may contain ${PACKAGE_ROOT}.
// If pkg is nil, returns the path unchanged.
func ResolveExternalPath(pkg *ExternalPackage, path string) string {
	if pkg == nil {
		return path
	}
	return resolvePackagePath(pkg.Path, path)
}

// GetExternalPackageByNamespace returns the external package with the given namespace.
func GetExternalPackageByNamespace(namespace string) (*ExternalPackage, error) {
	packages, err := GetExternalPackages()
	if err != nil {
		return nil, err
	}

	for i := range packages {
		if packages[i].Namespace() == namespace {
			return &packages[i], nil
		}
	}

	return nil, nil
}
