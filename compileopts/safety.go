// Package compileopts contains safety-related configuration for TinyGo builds.
package compileopts

import (
	"fmt"
	"strings"
)

// Valid safety analysis options
var (
	validSafetyLevelOptions  = []string{"none", "standard", "enhanced", "strict"}
	validSafetyFormatOptions = []string{"text", "json", "sarif"}
)

// SafetyOptions contains configuration for functional safety analysis.
type SafetyOptions struct {
	// Level specifies the safety analysis strictness level.
	// Valid values: none, standard, enhanced, strict
	Level string

	// EnabledRules specifies which MISRA-Go rules to enable.
	// If empty, all rules at the configured level are enabled.
	EnabledRules []string

	// DisabledRules specifies which rules to skip.
	DisabledRules []string

	// TreatWarningsAsErrors makes all violations fail the build.
	TreatWarningsAsErrors bool

	// OutputFormat specifies the output format for the analysis report.
	// Valid values: text, json, sarif
	OutputFormat string

	// OutputFile specifies the file to write the safety report to.
	// If empty, the report is written to stderr.
	OutputFile string

	// ComplexityThreshold is the maximum cyclomatic complexity allowed.
	ComplexityThreshold int

	// NestingThreshold is the maximum nesting depth allowed.
	NestingThreshold int

	// SafetyAnnotation is the comment annotation marking safety-critical functions.
	SafetyAnnotation string
}

// DefaultSafetyOptions returns sensible defaults for safety analysis.
func DefaultSafetyOptions() SafetyOptions {
	return SafetyOptions{
		Level:               "none",
		OutputFormat:        "text",
		ComplexityThreshold: 10,
		NestingThreshold:    4,
		SafetyAnnotation:    "go:safety",
	}
}

// Validate checks the safety options for valid values.
func (o *SafetyOptions) Validate() error {
	if o.Level != "" && !isInArray(validSafetyLevelOptions, o.Level) {
		return fmt.Errorf("invalid safety level '%s': valid values are %s",
			o.Level, strings.Join(validSafetyLevelOptions, ", "))
	}

	if o.OutputFormat != "" && !isInArray(validSafetyFormatOptions, o.OutputFormat) {
		return fmt.Errorf("invalid safety output format '%s': valid values are %s",
			o.OutputFormat, strings.Join(validSafetyFormatOptions, ", "))
	}

	if o.ComplexityThreshold < 0 {
		return fmt.Errorf("complexity threshold must be non-negative, got %d", o.ComplexityThreshold)
	}

	if o.NestingThreshold < 0 {
		return fmt.Errorf("nesting threshold must be non-negative, got %d", o.NestingThreshold)
	}

	return nil
}

// IsEnabled returns true if safety analysis is enabled.
func (o *SafetyOptions) IsEnabled() bool {
	return o.Level != "" && o.Level != "none"
}

// SafetyLevel returns the configured safety level for a Config.
func (c *Config) SafetyLevel() string {
	if c.Options.Safety.Level != "" {
		return c.Options.Safety.Level
	}
	return "none"
}

// SafetyEnabled returns true if safety analysis is enabled.
func (c *Config) SafetyEnabled() bool {
	return c.Options.Safety.IsEnabled()
}

// SafetyTreatWarningsAsErrors returns whether warnings should fail the build.
func (c *Config) SafetyTreatWarningsAsErrors() bool {
	return c.Options.Safety.TreatWarningsAsErrors
}

// SafetyOutputFormat returns the output format for safety analysis.
func (c *Config) SafetyOutputFormat() string {
	if c.Options.Safety.OutputFormat != "" {
		return c.Options.Safety.OutputFormat
	}
	return "text"
}

// SafetyOutputFile returns the output file for safety analysis.
// Returns empty string if output should go to stderr.
func (c *Config) SafetyOutputFile() string {
	return c.Options.Safety.OutputFile
}

// SafetyComplexityThreshold returns the maximum allowed cyclomatic complexity.
func (c *Config) SafetyComplexityThreshold() int {
	if c.Options.Safety.ComplexityThreshold > 0 {
		return c.Options.Safety.ComplexityThreshold
	}
	return 10 // default
}

// SafetyNestingThreshold returns the maximum allowed nesting depth.
func (c *Config) SafetyNestingThreshold() int {
	if c.Options.Safety.NestingThreshold > 0 {
		return c.Options.Safety.NestingThreshold
	}
	return 4 // default
}

// SafetyAnnotation returns the comment annotation for safety-critical functions.
func (c *Config) SafetyAnnotation() string {
	if c.Options.Safety.SafetyAnnotation != "" {
		return c.Options.Safety.SafetyAnnotation
	}
	return "go:safety"
}

// GetEnabledRules returns the list of enabled rules based on safety level.
func (o *SafetyOptions) GetEnabledRules() []string {
	if len(o.EnabledRules) > 0 {
		return o.EnabledRules
	}

	// Return rules based on safety level
	switch o.Level {
	case "strict":
		return getAllRules()
	case "enhanced":
		return getEnhancedRules()
	case "standard":
		return getStandardRules()
	default:
		return nil
	}
}

// getAllRules returns all available rule IDs.
func getAllRules() []string {
	return []string{
		"MISRA-GO-1.1", "MISRA-GO-1.2", "MISRA-GO-1.3",
		"MISRA-GO-2.1",
		"MISRA-GO-3.1", "MISRA-GO-3.2",
		"MISRA-GO-4.1", "MISRA-GO-4.2", "MISRA-GO-4.3",
		"MISRA-GO-5.1", "MISRA-GO-5.2",
		"MISRA-GO-6.1", "MISRA-GO-6.2", "MISRA-GO-6.3",
		"MISRA-GO-7.1",
		"MISRA-GO-8.1", "MISRA-GO-8.2",
		"MISRA-GO-9.1", "MISRA-GO-9.2",
		"MISRA-GO-10.1", "MISRA-GO-10.2",
	}
}

// getEnhancedRules returns rules for enhanced safety level.
func getEnhancedRules() []string {
	return []string{
		"MISRA-GO-1.1", "MISRA-GO-1.2", // Unreachable code, unused vars
		"MISRA-GO-2.1",                 // Shadowed variables
		"MISRA-GO-3.1", "MISRA-GO-3.2", // Type safety
		"MISRA-GO-4.1", "MISRA-GO-4.2", // Recursion, complexity
		"MISRA-GO-5.2",                 // Mutable globals
		"MISRA-GO-6.1", "MISRA-GO-6.3", // Panic, ignored errors
		"MISRA-GO-8.1", "MISRA-GO-8.2", // Concurrency
		"MISRA-GO-9.1", "MISRA-GO-9.2", // Control flow
	}
}

// getStandardRules returns rules for standard safety level.
func getStandardRules() []string {
	return []string{
		"MISRA-GO-1.1", // Unreachable code
		"MISRA-GO-2.1", // Shadowed variables
		"MISRA-GO-3.2", // Unsafe type assertion
		"MISRA-GO-4.1", // Unbounded recursion
		"MISRA-GO-6.1", // Panic in safety code
		"MISRA-GO-6.3", // Ignored errors
		"MISRA-GO-8.1", // Goroutines in safety code
		"MISRA-GO-9.2", // Missing default
	}
}
