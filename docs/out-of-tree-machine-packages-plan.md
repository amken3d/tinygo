# Out-of-Tree Machine Package Support for TinyGo

## Executive Summary

This document outlines a plan to implement out-of-tree machine package support for TinyGo, allowing chip families and boards to be maintained separately from mainline TinyGo while remaining fully compatible with the TinyGo build system.

## Goals

1. Allow developers to maintain chip/board support packages outside the main TinyGo repository
2. Enable using external machine packages with standard `tinygo build` commands
3. Support the full chip support lifecycle: device definitions, machine packages, targets, and linker scripts
4. Maintain compatibility with TinyGo updates
5. Provide a clean interface for package discovery and loading

---

## Architecture Overview

### Current TinyGo Structure

```
tinygo/
├── targets/           # Target JSON files (pico.json, stm32f4.json, etc.)
├── src/
│   ├── machine/       # Machine packages (GPIO, UART, SPI, I2C, etc.)
│   ├── device/        # Device definitions (register maps from SVD)
│   └── runtime/       # Runtime support
├── lib/
│   ├── cmsis-svd/     # SVD files for device generation
│   └── stm32-svd/     # STM32-specific SVD files
└── tools/
    └── gen-device-svd/ # SVD to Go code generator
```

### Proposed External Package Structure

```
my-chip-support/
├── tinygo-external.json    # Package manifest (discovery/config)
├── targets/                 # Target JSON files
│   ├── mychip.json
│   └── myboard.json
├── src/
│   ├── machine/             # Machine package files
│   │   ├── machine_mychip.go
│   │   ├── machine_mychip_gpio.go
│   │   └── board_myboard.go
│   └── device/
│       └── mychip/          # Generated device definitions
│           └── mychip.go
├── lib/
│   └── svd/                 # SVD files for device generation
│       └── mychip.svd
└── linker/
    └── mychip.ld            # Linker scripts
```

---

## Implementation Plan

### Phase 1: Package Discovery and Configuration

#### 1.1 External Package Manifest (`tinygo-external.json`)

Create a manifest format that describes external packages:

```json
{
  "version": "1.0",
  "name": "my-chip-support",
  "description": "Support package for MyChip family",
  "author": "Your Name",
  "license": "BSD-3-Clause",

  "chip-families": ["mychip"],
  "targets": ["targets/*.json"],
  "machine-packages": ["src/machine"],
  "device-packages": ["src/device"],
  "linker-scripts": ["linker/*.ld"],
  "extra-files": ["src/device/mychip/*.s"],

  "dependencies": {
    "tinygo-min-version": "0.32.0",
    "inherits-from": ["cortex-m4"]
  }
}
```

#### 1.2 Package Discovery Mechanism

**Option A: Environment Variable (Recommended for simplicity)**
```bash
export TINYGO_EXTERNAL_PACKAGES="/path/to/my-chip-support:/path/to/another-package"
tinygo build -target myboard main.go
```

**Option B: Configuration File**
```yaml
# ~/.config/tinygo/external-packages.yaml
packages:
  - path: /path/to/my-chip-support
  - path: /path/to/another-package
  - git: https://github.com/user/chip-support.git
    version: v1.0.0
```

**Option C: Command-line flag**
```bash
tinygo build -target myboard -external-pkg /path/to/my-chip-support main.go
```

#### 1.3 Implementation: Modify `compileopts/target.go`

Add external package discovery to the target loading system:

```go
// New file: compileopts/external.go

type ExternalPackage struct {
    Path           string
    Manifest       ExternalManifest
    TargetDir      string
    MachineDir     string
    DeviceDir      string
    LinkerScripts  []string
}

func DiscoverExternalPackages() ([]ExternalPackage, error) {
    var packages []ExternalPackage

    // Check environment variable
    envPath := os.Getenv("TINYGO_EXTERNAL_PACKAGES")
    if envPath != "" {
        for _, p := range strings.Split(envPath, ":") {
            pkg, err := LoadExternalPackage(p)
            if err != nil {
                return nil, err
            }
            packages = append(packages, pkg)
        }
    }

    return packages, nil
}

func LoadExternalPackage(path string) (ExternalPackage, error) {
    manifestPath := filepath.Join(path, "tinygo-external.json")
    data, err := os.ReadFile(manifestPath)
    if err != nil {
        return ExternalPackage{}, fmt.Errorf("no tinygo-external.json found in %s", path)
    }

    var manifest ExternalManifest
    if err := json.Unmarshal(data, &manifest); err != nil {
        return ExternalPackage{}, err
    }

    return ExternalPackage{
        Path:     path,
        Manifest: manifest,
        // ... resolve directories
    }, nil
}
```

---

### Phase 2: Target Resolution with External Packages

#### 2.1 Modify Target Loading (`compileopts/target.go`)

Update `LoadTarget()` to search external packages:

```go
func LoadTarget(options *Options) (*TargetSpec, error) {
    target := options.Target

    // 1. Try built-in targets first
    spec, err := loadBuiltinTarget(target)
    if err == nil {
        return spec, nil
    }

    // 2. Search external packages
    externalPkgs, err := DiscoverExternalPackages()
    if err != nil {
        return nil, err
    }

    for _, pkg := range externalPkgs {
        spec, err := loadExternalTarget(pkg, target)
        if err == nil {
            // Mark this target as external for later resolution
            spec.ExternalPackage = &pkg
            return spec, nil
        }
    }

    // 3. Try as file path (existing behavior)
    return loadTargetFromFile(target)
}
```

#### 2.2 Target JSON Extensions

Allow external targets to reference files relative to their package:

```json
{
  "inherits": ["cortex-m4"],
  "build-tags": ["mychip", "mycorp"],
  "linkerscript": "${PACKAGE_ROOT}/linker/mychip.ld",
  "extra-files": [
    "${PACKAGE_ROOT}/src/device/mychip/startup.s"
  ],
  "external-machine-package": "${PACKAGE_ROOT}/src/machine",
  "external-device-package": "${PACKAGE_ROOT}/src/device"
}
```

---

### Phase 3: Loader Integration

#### 3.1 Modify Package Loading (`loader/loader.go`)

Update the loader to include external machine and device packages:

```go
func (p *Program) loadMachinePackage() error {
    // Standard machine package path
    machinePaths := []string{
        filepath.Join(p.TINYGOROOT, "src", "machine"),
    }

    // Add external machine packages
    if p.config.Target.ExternalPackage != nil {
        extMachine := p.config.Target.ExternalPackage.MachineDir
        machinePaths = append(machinePaths, extMachine)
    }

    // Create overlay for merged machine package
    return p.loadMergedPackage("machine", machinePaths)
}
```

#### 3.2 Package Overlay System

Create a mechanism to overlay external packages on top of built-in ones:

```go
// New file: loader/overlay.go

type PackageOverlay struct {
    BaseDir     string   // TinyGo's built-in package
    OverlayDirs []string // External package directories
}

func (o *PackageOverlay) CollectFiles(buildTags []string) ([]string, error) {
    var allFiles []string

    // Collect from base directory
    baseFiles, _ := collectGoFiles(o.BaseDir, buildTags)
    allFiles = append(allFiles, baseFiles...)

    // Overlay external files (can override base files)
    for _, overlayDir := range o.OverlayDirs {
        overlayFiles, _ := collectGoFiles(overlayDir, buildTags)
        allFiles = mergeFiles(allFiles, overlayFiles)
    }

    return allFiles, nil
}
```

---

### Phase 4: Device Package Generation

#### 4.1 External SVD Processing

Extend `gen-device-svd` to work with external packages:

```bash
# Generate device files for external package
tinygo generate-device \
    -svd /path/to/my-chip-support/lib/svd/mychip.svd \
    -output /path/to/my-chip-support/src/device/mychip/
```

#### 4.2 Device Package Import Resolution

Modify the compiler to resolve device imports from external packages:

```go
// In user code:
import "device/mychip"

// Resolution order:
// 1. Check external packages for device/mychip
// 2. Fall back to built-in src/device/mychip
```

---

### Phase 5: Build Integration

#### 5.1 Linker Script Resolution

Modify the builder to find linker scripts in external packages:

```go
func (b *Builder) resolveLinkerScript() (string, error) {
    script := b.config.Target.LinkerScript

    // Expand ${PACKAGE_ROOT} for external packages
    if strings.HasPrefix(script, "${PACKAGE_ROOT}") {
        if b.config.Target.ExternalPackage != nil {
            script = strings.Replace(script, "${PACKAGE_ROOT}",
                b.config.Target.ExternalPackage.Path, 1)
        }
    }

    return script, nil
}
```

#### 5.2 Extra Files Resolution

Handle assembly files and other extra files from external packages:

```go
func (b *Builder) collectExtraFiles() ([]string, error) {
    var files []string

    for _, f := range b.config.Target.ExtraFiles {
        resolved := b.resolvePath(f)
        files = append(files, resolved)
    }

    return files, nil
}
```

---

### Phase 6: Development Tooling

#### 6.1 Package Initialization Command

```bash
# Initialize a new external chip support package
tinygo init-external-package my-chip-support

# Creates:
# my-chip-support/
# ├── tinygo-external.json
# ├── targets/
# ├── src/machine/
# ├── src/device/
# └── linker/
```

#### 6.2 Package Validation Command

```bash
# Validate external package structure and compatibility
tinygo validate-external-package /path/to/my-chip-support

# Checks:
# - Manifest format
# - Target JSON validity
# - Build tag consistency
# - Linker script syntax
# - Go file compilation
```

#### 6.3 Device Generation Command

```bash
# Generate device definitions from SVD
tinygo generate-device \
    --svd ./lib/svd/mychip.svd \
    --output ./src/device/mychip/ \
    --package mychip
```

---

## File-by-File Implementation Guide

### Files to Modify in TinyGo

| File | Changes |
|------|---------|
| `compileopts/options.go` | Add `ExternalPackages` field to Options |
| `compileopts/target.go` | Modify `LoadTarget()` to search external packages |
| `compileopts/config.go` | Add external package resolution to Config |
| `loader/loader.go` | Add external package overlay to package loading |
| `loader/goroot.go` | Extend synthetic GOROOT with external packages |
| `builder/build.go` | Resolve linker scripts and extra files from external packages |
| `main.go` | Add external package commands and flags |

### New Files to Create

| File | Purpose |
|------|---------|
| `compileopts/external.go` | External package discovery and loading |
| `loader/overlay.go` | Package overlay/merge functionality |
| `cmd/external.go` | CLI commands for external packages |

---

## Example: Creating an External Package

### Step 1: Initialize Package Structure

```
my-mychip-support/
├── tinygo-external.json
├── targets/
│   ├── mychip.json
│   └── myboard.json
├── src/
│   ├── machine/
│   │   ├── machine_mychip.go
│   │   ├── machine_mychip_gpio.go
│   │   ├── machine_mychip_uart.go
│   │   └── board_myboard.go
│   └── device/
│       └── mychip/
│           └── mychip.go
└── linker/
    └── mychip.ld
```

### Step 2: Create Target JSON

```json
// targets/mychip.json
{
  "inherits": ["cortex-m4"],
  "build-tags": ["mychip"],
  "linkerscript": "${PACKAGE_ROOT}/linker/mychip.ld",
  "extra-files": [
    "${PACKAGE_ROOT}/src/device/mychip/startup.s"
  ],
  "flash-method": "openocd",
  "openocd-interface": "cmsis-dap"
}

// targets/myboard.json
{
  "inherits": ["mychip"],
  "build-tags": ["myboard"],
  "serial-port": ["1234:5678"]
}
```

### Step 3: Create Machine Package

```go
// src/machine/machine_mychip.go
//go:build mychip

package machine

import (
    "device/mychip"
    "runtime/volatile"
)

type Pin uint8

func (p Pin) Configure(config PinConfig) {
    // Implementation using device/mychip registers
    port := mychip.GPIO
    // ...
}

func (p Pin) Set(high bool) {
    // ...
}

func (p Pin) Get() bool {
    // ...
}
```

```go
// src/machine/board_myboard.go
//go:build myboard

package machine

const (
    LED  = PA5
    BTN  = PA0

    UART_TX = PA2
    UART_RX = PA3
)
```

### Step 4: Generate Device Definitions

```bash
tinygo generate-device \
    --svd ./lib/svd/mychip.svd \
    --output ./src/device/mychip/
```

### Step 5: Use the Package

```bash
export TINYGO_EXTERNAL_PACKAGES=/path/to/my-mychip-support
tinygo build -target myboard -o firmware.elf main.go
```

---

## Compatibility Considerations

### Build Tag Conflicts

External packages must not reuse existing TinyGo build tags unless they're intentionally extending that chip family:

```go
// WRONG: Conflicts with built-in nrf52
//go:build nrf52

// CORRECT: Uses unique tag
//go:build mychip
```

### API Stability

External packages should target specific TinyGo versions and may need updates when TinyGo's internal APIs change. The manifest includes version constraints:

```json
{
  "dependencies": {
    "tinygo-min-version": "0.32.0",
    "tinygo-max-version": "0.35.x"
  }
}
```

### Inheritance Safety

External targets can only inherit from stable, documented base targets:
- `cortex-m0`, `cortex-m0plus`, `cortex-m3`, `cortex-m4`, `cortex-m7`, `cortex-m33`
- `riscv32`, `riscv64`
- `avr`, `xtensa`

---

## Implementation Phases Summary

| Phase | Description | Complexity | Dependencies |
|-------|-------------|------------|--------------|
| **1** | Package discovery and manifest | Medium | None |
| **2** | Target resolution from external packages | Medium | Phase 1 |
| **3** | Loader integration (machine/device packages) | High | Phases 1, 2 |
| **4** | Device generation tooling | Medium | Phase 1 |
| **5** | Build integration (linker, extra files) | Medium | Phases 1-3 |
| **6** | Development tooling (init, validate) | Low | Phases 1-5 |

---

## Alternative Approaches Considered

### A. Go Module-based Approach

Use standard Go modules for external packages:

```go
import "github.com/user/mychip-support/machine"
```

**Pros:** Standard Go tooling, familiar to Go developers
**Cons:** Doesn't integrate with target system, requires code changes in user programs

### B. Plugin-based Approach

Load external packages as shared libraries:

```go
plugin.Open("/path/to/mychip.so")
```

**Pros:** Dynamic loading, no TinyGo modifications needed
**Cons:** Complex, platform-specific, doesn't work well with cross-compilation

### C. Overlay Directory Approach (Recommended)

Use directory overlays with manifest-based discovery (this plan).

**Pros:** Simple, familiar pattern, integrates cleanly with existing build system
**Cons:** Requires TinyGo modifications, but changes are minimal and well-contained

---

## Open Questions

1. **Namespace isolation**: How to prevent collisions between multiple external packages defining the same target name?

2. **Versioning**: Should external packages be versioned independently, or tied to TinyGo versions?

3. **Distribution**: Should there be a registry for external packages (like pkg.go.dev)?

4. **Testing**: How should external packages be tested against TinyGo updates?

5. **Documentation**: How to integrate external package documentation with TinyGo docs?

---

## Next Steps

1. Prototype Phase 1 (package discovery) to validate the approach
2. Gather feedback from the TinyGo community
3. Create RFC for upstream TinyGo contribution
4. Implement remaining phases incrementally
5. Document and publish example external packages
