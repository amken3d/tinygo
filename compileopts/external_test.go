package compileopts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// createTestExternalPackage creates a temporary external package for testing.
func createTestExternalPackage(t *testing.T, name string) string {
	t.Helper()

	// Create temp directory
	dir := t.TempDir()

	// Create manifest
	manifest := ExternalManifest{
		Version:         "1.0",
		Name:            name,
		Description:     "Test external package",
		ChipFamilies:    []string{"testchip"},
		Targets:         []string{"targets/*.json"},
		MachinePackages: []string{"src/machine"},
		DevicePackages:  []string{"src/device"},
		LinkerScripts:   []string{"linker/*.ld"},
		PeripheralStatus: map[string]PeripheralStatus{
			"gpio": {
				Status:   "verified",
				TestTier: 2,
				Notes:    "Loopback tested",
			},
		},
	}

	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal manifest: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "tinygo-external.json"), manifestData, 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Create directories
	for _, subdir := range []string{"targets", "src/machine", "src/device", "linker"} {
		if err := os.MkdirAll(filepath.Join(dir, subdir), 0755); err != nil {
			t.Fatalf("failed to create directory %s: %v", subdir, err)
		}
	}

	// Create a test target JSON
	targetJSON := `{
  "inherits": ["cortex-m4"],
  "build-tags": ["testboard"],
  "linkerscript": "${PACKAGE_ROOT}/linker/testboard.ld",
  "flash-method": "openocd",
  "openocd-interface": "cmsis-dap",
  "openocd-target": "stm32f4x"
}`
	if err := os.WriteFile(filepath.Join(dir, "targets", "testboard.json"), []byte(targetJSON), 0644); err != nil {
		t.Fatalf("failed to write target JSON: %v", err)
	}

	// Create a test linker script
	linkerScript := `/* Test linker script */
MEMORY
{
  FLASH (rx) : ORIGIN = 0x08000000, LENGTH = 512K
  RAM (rwx) : ORIGIN = 0x20000000, LENGTH = 128K
}
`
	if err := os.WriteFile(filepath.Join(dir, "linker", "testboard.ld"), []byte(linkerScript), 0644); err != nil {
		t.Fatalf("failed to write linker script: %v", err)
	}

	return dir
}

func TestLoadExternalPackage(t *testing.T) {
	dir := createTestExternalPackage(t, "test-package")

	pkg, err := LoadExternalPackage(dir)
	if err != nil {
		t.Fatalf("failed to load external package: %v", err)
	}

	// Verify manifest loaded correctly
	if pkg.Manifest.Name != "test-package" {
		t.Errorf("expected name 'test-package', got '%s'", pkg.Manifest.Name)
	}

	if pkg.Manifest.Version != "1.0" {
		t.Errorf("expected version '1.0', got '%s'", pkg.Manifest.Version)
	}

	// Verify namespace
	if pkg.Namespace() != "test-package" {
		t.Errorf("expected namespace 'test-package', got '%s'", pkg.Namespace())
	}

	// Verify peripheral status
	if gpio, ok := pkg.Manifest.PeripheralStatus["gpio"]; !ok {
		t.Error("expected gpio peripheral status")
	} else {
		if gpio.Status != "verified" {
			t.Errorf("expected gpio status 'verified', got '%s'", gpio.Status)
		}
		if gpio.TestTier != 2 {
			t.Errorf("expected gpio test tier 2, got %d", gpio.TestTier)
		}
	}

	// Verify target directory resolved
	if len(pkg.ResolvedTargetDirs) == 0 {
		t.Error("expected at least one target directory")
	} else {
		expectedDir := filepath.Join(dir, "targets")
		if pkg.ResolvedTargetDirs[0] != expectedDir {
			t.Errorf("expected target dir '%s', got '%s'", expectedDir, pkg.ResolvedTargetDirs[0])
		}
	}
}

func TestLoadExternalPackageNotFound(t *testing.T) {
	_, err := LoadExternalPackage("/nonexistent/path")
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}

func TestLoadExternalPackageNoManifest(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadExternalPackage(dir)
	if err == nil {
		t.Error("expected error for missing manifest")
	}
}

func TestLoadExternalPackageInvalidManifest(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tinygo-external.json"), []byte("invalid json"), 0644); err != nil {
		t.Fatalf("failed to write invalid manifest: %v", err)
	}

	_, err := LoadExternalPackage(dir)
	if err == nil {
		t.Error("expected error for invalid manifest")
	}
}

func TestLoadExternalPackageMissingName(t *testing.T) {
	dir := t.TempDir()
	manifest := `{"version": "1.0"}`
	if err := os.WriteFile(filepath.Join(dir, "tinygo-external.json"), []byte(manifest), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	_, err := LoadExternalPackage(dir)
	if err == nil {
		t.Error("expected error for missing name")
	}
}

func TestExternalPackageDiscovery(t *testing.T) {
	// Clear cache before test
	ClearExternalPackageCache()
	defer ClearExternalPackageCache()

	// Create test package
	dir := createTestExternalPackage(t, "discovery-test")

	// Set environment variable
	oldEnv := os.Getenv("TINYGO_EXTERNAL_PACKAGES")
	os.Setenv("TINYGO_EXTERNAL_PACKAGES", dir)
	defer os.Setenv("TINYGO_EXTERNAL_PACKAGES", oldEnv)

	// Discover packages
	packages, err := GetExternalPackages()
	if err != nil {
		t.Fatalf("failed to discover packages: %v", err)
	}

	if len(packages) != 1 {
		t.Fatalf("expected 1 package, got %d", len(packages))
	}

	if packages[0].Manifest.Name != "discovery-test" {
		t.Errorf("expected name 'discovery-test', got '%s'", packages[0].Manifest.Name)
	}
}

func TestExternalPackageDiscoveryMultiple(t *testing.T) {
	// Clear cache before test
	ClearExternalPackageCache()
	defer ClearExternalPackageCache()

	// Create multiple test packages
	dir1 := createTestExternalPackage(t, "package-1")
	dir2 := createTestExternalPackage(t, "package-2")

	// Set environment variable with multiple paths
	oldEnv := os.Getenv("TINYGO_EXTERNAL_PACKAGES")
	os.Setenv("TINYGO_EXTERNAL_PACKAGES", dir1+":"+dir2)
	defer os.Setenv("TINYGO_EXTERNAL_PACKAGES", oldEnv)

	// Discover packages
	packages, err := GetExternalPackages()
	if err != nil {
		t.Fatalf("failed to discover packages: %v", err)
	}

	if len(packages) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(packages))
	}
}

func TestFindExternalTarget(t *testing.T) {
	// Clear cache before test
	ClearExternalPackageCache()
	defer ClearExternalPackageCache()

	// Create test package
	dir := createTestExternalPackage(t, "target-test")

	// Set environment variable
	oldEnv := os.Getenv("TINYGO_EXTERNAL_PACKAGES")
	os.Setenv("TINYGO_EXTERNAL_PACKAGES", dir)
	defer os.Setenv("TINYGO_EXTERNAL_PACKAGES", oldEnv)

	// Find target by simple name
	pkg, path, err := FindExternalTarget("testboard")
	if err != nil {
		t.Fatalf("failed to find target: %v", err)
	}
	if pkg == nil {
		t.Fatal("expected to find target")
	}
	if path == "" {
		t.Fatal("expected path to be set")
	}

	// Find target by qualified name
	pkg, path, err = FindExternalTarget("target-test/testboard")
	if err != nil {
		t.Fatalf("failed to find target by qualified name: %v", err)
	}
	if pkg == nil {
		t.Fatal("expected to find target by qualified name")
	}

	// Try non-existent target
	pkg, path, err = FindExternalTarget("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg != nil || path != "" {
		t.Error("expected nil result for non-existent target")
	}
}

func TestFindExternalTargetAmbiguous(t *testing.T) {
	// Clear cache before test
	ClearExternalPackageCache()
	defer ClearExternalPackageCache()

	// Create two packages with the same target name
	dir1 := createTestExternalPackage(t, "package-a")
	dir2 := createTestExternalPackage(t, "package-b")

	// Set environment variable
	oldEnv := os.Getenv("TINYGO_EXTERNAL_PACKAGES")
	os.Setenv("TINYGO_EXTERNAL_PACKAGES", dir1+":"+dir2)
	defer os.Setenv("TINYGO_EXTERNAL_PACKAGES", oldEnv)

	// Finding by simple name should error (ambiguous)
	_, _, err := FindExternalTarget("testboard")
	if err == nil {
		t.Error("expected error for ambiguous target name")
	}

	// Finding by qualified name should work
	pkg, _, err := FindExternalTarget("package-a/testboard")
	if err != nil {
		t.Fatalf("failed to find target by qualified name: %v", err)
	}
	if pkg == nil {
		t.Error("expected to find target by qualified name")
	}
	if pkg.Manifest.Name != "package-a" {
		t.Errorf("expected package-a, got %s", pkg.Manifest.Name)
	}
}

func TestResolveExternalPath(t *testing.T) {
	tests := []struct {
		packageRoot string
		path        string
		expected    string
	}{
		{"/pkg", "${PACKAGE_ROOT}/linker/test.ld", "/pkg/linker/test.ld"},
		{"/pkg", "src/file.go", "src/file.go"},
		{"/pkg", "/absolute/path.ld", "/absolute/path.ld"},
		{"/pkg", "${PACKAGE_ROOT}/a/${PACKAGE_ROOT}/b", "/pkg/a//pkg/b"},
	}

	for _, tc := range tests {
		result := resolveExternalPath(tc.packageRoot, tc.path)
		if result != tc.expected {
			t.Errorf("resolveExternalPath(%q, %q) = %q, expected %q",
				tc.packageRoot, tc.path, result, tc.expected)
		}
	}
}

func TestCustomNamespace(t *testing.T) {
	dir := t.TempDir()

	// Create manifest with custom namespace
	manifest := ExternalManifest{
		Version:   "1.0",
		Name:      "my-package",
		Namespace: "custom-ns",
	}

	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(filepath.Join(dir, "tinygo-external.json"), manifestData, 0644)

	pkg, err := LoadExternalPackage(dir)
	if err != nil {
		t.Fatalf("failed to load package: %v", err)
	}

	if pkg.Namespace() != "custom-ns" {
		t.Errorf("expected namespace 'custom-ns', got '%s'", pkg.Namespace())
	}
}
