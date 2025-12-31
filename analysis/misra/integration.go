package misra

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io"
	"os"

	"github.com/tinygo-org/tinygo/compileopts"
	"golang.org/x/tools/go/ssa"
)

// RunAnalysis runs MISRA-Go safety analysis on the given package.
// It returns violations found and whether the build should fail.
func RunAnalysis(
	config *compileopts.Config,
	fset *token.FileSet,
	pkg *types.Package,
	info *types.Info,
	files []*ast.File,
	ssaPkg *ssa.Package,
) ([]Violation, bool) {
	if !config.SafetyEnabled() {
		return nil, false
	}

	// Create analyzer configuration
	analyzerConfig := ConfigFromCompileOpts(config)

	// Create and run analyzer
	analyzer := NewAnalyzer(analyzerConfig)
	violations := analyzer.Analyze(fset, pkg, info, files, ssaPkg)

	// Generate report
	report := NewReport(violations, analyzerConfig)

	// Determine output destination
	var output io.Writer = os.Stderr
	outputFile := config.SafetyOutputFile()
	if outputFile != "" {
		f, err := os.Create(outputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: could not create safety report file: %v\n", err)
		} else {
			defer f.Close()
			output = f
		}
	}

	// Output report based on format
	switch config.SafetyOutputFormat() {
	case "json":
		report.WriteJSON(output)
	case "sarif":
		report.WriteSARIF(output)
	default:
		report.WriteTo(output)
	}

	return violations, report.ShouldFail()
}

// ConfigFromCompileOpts creates a MISRA Config from TinyGo compile options.
func ConfigFromCompileOpts(config *compileopts.Config) *Config {
	safetyOpts := &config.Options.Safety

	misraConfig := &Config{
		MinSeverity:           severityFromLevel(config.SafetyLevel()),
		ComplexityThreshold:   config.SafetyComplexityThreshold(),
		NestingThreshold:      config.SafetyNestingThreshold(),
		SafetyAnnotation:      config.SafetyAnnotation(),
		TreatWarningsAsErrors: config.SafetyTreatWarningsAsErrors(),
	}

	// Set enabled/disabled rules
	if len(safetyOpts.EnabledRules) > 0 {
		for _, r := range safetyOpts.EnabledRules {
			misraConfig.EnabledRules = append(misraConfig.EnabledRules, RuleID(r))
		}
	} else {
		// Use rules based on safety level
		for _, r := range safetyOpts.GetEnabledRules() {
			misraConfig.EnabledRules = append(misraConfig.EnabledRules, RuleID(r))
		}
	}

	for _, r := range safetyOpts.DisabledRules {
		misraConfig.DisabledRules = append(misraConfig.DisabledRules, RuleID(r))
	}

	return misraConfig
}

// severityFromLevel converts a safety level string to a Severity threshold.
func severityFromLevel(level string) Severity {
	switch level {
	case "strict":
		return SeverityAdvisory // All severities
	case "enhanced":
		return SeverityRequired // Required and Mandatory
	case "standard":
		return SeverityMandatory // Mandatory only
	default:
		return SeverityMandatory
	}
}

// AnalyzePackage is a convenience function that runs analysis on a package
// and returns a structured result.
type AnalysisResult struct {
	PackagePath string
	Violations  []Violation
	ShouldFail  bool
	Summary     map[Severity]int
}

// AnalyzePackageResult runs analysis and returns a structured result.
func AnalyzePackageResult(
	config *compileopts.Config,
	fset *token.FileSet,
	pkg *types.Package,
	info *types.Info,
	files []*ast.File,
	ssaPkg *ssa.Package,
) *AnalysisResult {
	violations, shouldFail := RunAnalysis(config, fset, pkg, info, files, ssaPkg)

	result := &AnalysisResult{
		PackagePath: pkg.Path(),
		Violations:  violations,
		ShouldFail:  shouldFail,
		Summary:     make(map[Severity]int),
	}

	for _, v := range violations {
		result.Summary[v.Severity]++
	}

	return result
}
