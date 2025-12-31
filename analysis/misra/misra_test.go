// Package misra implements MISRA-Go safety analysis rules for TinyGo.
// These rules are inspired by MISRA-C guidelines but adapted for Go's semantics.
// They help identify potential issues in safety-critical embedded systems code.
package misra

import (
	"go/ast"
	"go/token"
	"go/types"
	"sort"

	"golang.org/x/tools/go/ssa"
)

// Severity indicates the importance of a rule violation.
type Severity int

const (
	// SeverityMandatory violations must be fixed for safety-critical code.
	SeverityMandatory Severity = iota
	// SeverityRequired violations should be fixed unless a documented deviation exists.
	SeverityRequired
	// SeverityAdvisory violations are recommended best practices.
	SeverityAdvisory
)

func (s Severity) String() string {
	switch s {
	case SeverityMandatory:
		return "mandatory"
	case SeverityRequired:
		return "required"
	case SeverityAdvisory:
		return "advisory"
	default:
		return "unknown"
	}
}

// Category groups related rules together.
type Category string

const (
	CategoryUnusedCode    Category = "unused-code"
	CategoryControlFlow   Category = "control-flow"
	CategoryTypeSystem    Category = "type-system"
	CategoryMemory        Category = "memory"
	CategoryConcurrency   Category = "concurrency"
	CategoryErrorHandling Category = "error-handling"
	CategoryComplexity    Category = "complexity"
	CategoryNaming        Category = "naming"
)

// RuleID uniquely identifies a MISRA-Go rule.
type RuleID string

// Rule IDs for all implemented rules.
const (
	RuleUnreachableCode     RuleID = "MISRA-GO-1.1"
	RuleUnusedVariable      RuleID = "MISRA-GO-1.2"
	RuleUnusedParameter     RuleID = "MISRA-GO-1.3"
	RuleShadowedVariable    RuleID = "MISRA-GO-2.1"
	RuleImplicitConversion  RuleID = "MISRA-GO-3.1"
	RuleUnsafeTypeAssertion RuleID = "MISRA-GO-3.2"
	RuleUnboundedRecursion  RuleID = "MISRA-GO-4.1"
	RuleHighComplexity      RuleID = "MISRA-GO-4.2"
	RuleDeepNesting         RuleID = "MISRA-GO-4.3"
	RuleGlobalVariable      RuleID = "MISRA-GO-5.1"
	RuleMutableGlobal       RuleID = "MISRA-GO-5.2"
	RulePanicInSafetyCode   RuleID = "MISRA-GO-6.1"
	RuleRecoverMisuse       RuleID = "MISRA-GO-6.2"
	RuleIgnoredError        RuleID = "MISRA-GO-6.3"
	RuleDynamicAllocation   RuleID = "MISRA-GO-7.1"
	RuleGoroutineInSafety   RuleID = "MISRA-GO-8.1"
	RuleChannelInSafety     RuleID = "MISRA-GO-8.2"
	RuleEmptyBranch         RuleID = "MISRA-GO-9.1"
	RuleMissingDefault      RuleID = "MISRA-GO-9.2"
	RuleMagicNumber         RuleID = "MISRA-GO-10.1"
	RuleShortVarName        RuleID = "MISRA-GO-10.2"
)

// RuleInfo contains metadata about a specific rule.
type RuleInfo struct {
	ID          RuleID
	Name        string
	Description string
	Severity    Severity
	Category    Category
}

// AllRules contains information about all implemented rules.
var AllRules = map[RuleID]RuleInfo{
	RuleUnreachableCode: {
		ID:          RuleUnreachableCode,
		Name:        "Unreachable Code",
		Description: "Code that can never be executed shall not be present",
		Severity:    SeverityMandatory,
		Category:    CategoryUnusedCode,
	},
	RuleUnusedVariable: {
		ID:          RuleUnusedVariable,
		Name:        "Unused Variable",
		Description: "Variables that are declared but never used shall not be present",
		Severity:    SeverityRequired,
		Category:    CategoryUnusedCode,
	},
	RuleUnusedParameter: {
		ID:          RuleUnusedParameter,
		Name:        "Unused Parameter",
		Description: "Function parameters that are never used should be removed or documented",
		Severity:    SeverityAdvisory,
		Category:    CategoryUnusedCode,
	},
	RuleShadowedVariable: {
		ID:          RuleShadowedVariable,
		Name:        "Shadowed Variable",
		Description: "Variables shall not shadow variables from outer scopes",
		Severity:    SeverityMandatory,
		Category:    CategoryNaming,
	},
	RuleImplicitConversion: {
		ID:          RuleImplicitConversion,
		Name:        "Implicit Type Conversion",
		Description: "Type conversions that may lose precision should be explicit",
		Severity:    SeverityRequired,
		Category:    CategoryTypeSystem,
	},
	RuleUnsafeTypeAssertion: {
		ID:          RuleUnsafeTypeAssertion,
		Name:        "Unsafe Type Assertion",
		Description: "Type assertions should use the two-value form to check success",
		Severity:    SeverityRequired,
		Category:    CategoryTypeSystem,
	},
	RuleUnboundedRecursion: {
		ID:          RuleUnboundedRecursion,
		Name:        "Unbounded Recursion",
		Description: "Recursive functions shall have a clearly defined termination condition",
		Severity:    SeverityMandatory,
		Category:    CategoryControlFlow,
	},
	RuleHighComplexity: {
		ID:          RuleHighComplexity,
		Name:        "High Cyclomatic Complexity",
		Description: "Functions shall not have cyclomatic complexity exceeding the configured threshold",
		Severity:    SeverityRequired,
		Category:    CategoryComplexity,
	},
	RuleDeepNesting: {
		ID:          RuleDeepNesting,
		Name:        "Deep Nesting",
		Description: "Code shall not be nested more than the configured depth",
		Severity:    SeverityAdvisory,
		Category:    CategoryComplexity,
	},
	RuleGlobalVariable: {
		ID:          RuleGlobalVariable,
		Name:        "Global Variable Usage",
		Description: "Global variables should be minimized in safety-critical code",
		Severity:    SeverityAdvisory,
		Category:    CategoryMemory,
	},
	RuleMutableGlobal: {
		ID:          RuleMutableGlobal,
		Name:        "Mutable Global Variable",
		Description: "Global variables that are modified shall be documented",
		Severity:    SeverityRequired,
		Category:    CategoryMemory,
	},
	RulePanicInSafetyCode: {
		ID:          RulePanicInSafetyCode,
		Name:        "Panic in Safety-Critical Code",
		Description: "Panic shall not be used in safety-critical functions",
		Severity:    SeverityMandatory,
		Category:    CategoryErrorHandling,
	},
	RuleRecoverMisuse: {
		ID:          RuleRecoverMisuse,
		Name:        "Recover Misuse",
		Description: "Recover should only be used to handle unexpected panics, not for control flow",
		Severity:    SeverityRequired,
		Category:    CategoryErrorHandling,
	},
	RuleIgnoredError: {
		ID:          RuleIgnoredError,
		Name:        "Ignored Error",
		Description: "Error return values shall not be ignored",
		Severity:    SeverityMandatory,
		Category:    CategoryErrorHandling,
	},
	RuleDynamicAllocation: {
		ID:          RuleDynamicAllocation,
		Name:        "Dynamic Memory Allocation",
		Description: "Dynamic memory allocation should be avoided in safety-critical code",
		Severity:    SeverityAdvisory,
		Category:    CategoryMemory,
	},
	RuleGoroutineInSafety: {
		ID:          RuleGoroutineInSafety,
		Name:        "Goroutine in Safety-Critical Code",
		Description: "Goroutines shall not be spawned in safety-critical functions",
		Severity:    SeverityMandatory,
		Category:    CategoryConcurrency,
	},
	RuleChannelInSafety: {
		ID:          RuleChannelInSafety,
		Name:        "Channel in Safety-Critical Code",
		Description: "Channel operations should be carefully reviewed in safety-critical code",
		Severity:    SeverityRequired,
		Category:    CategoryConcurrency,
	},
	RuleEmptyBranch: {
		ID:          RuleEmptyBranch,
		Name:        "Empty Branch",
		Description: "Empty if/else/case branches shall have a comment explaining why",
		Severity:    SeverityRequired,
		Category:    CategoryControlFlow,
	},
	RuleMissingDefault: {
		ID:          RuleMissingDefault,
		Name:        "Missing Default Case",
		Description: "Switch statements shall have a default case",
		Severity:    SeverityRequired,
		Category:    CategoryControlFlow,
	},
	RuleMagicNumber: {
		ID:          RuleMagicNumber,
		Name:        "Magic Number",
		Description: "Numeric literals should be replaced with named constants",
		Severity:    SeverityAdvisory,
		Category:    CategoryNaming,
	},
	RuleShortVarName: {
		ID:          RuleShortVarName,
		Name:        "Short Variable Name",
		Description: "Variable names should be descriptive (except for loop indices)",
		Severity:    SeverityAdvisory,
		Category:    CategoryNaming,
	},
}

// Violation represents a single rule violation found during analysis.
type Violation struct {
	Rule     RuleID
	Severity Severity
	Pos      token.Position
	EndPos   token.Position // Optional end position for range
	Message  string
	Context  string // Additional context (e.g., function name)
}

// Config holds configuration options for the MISRA analyzer.
type Config struct {
	// EnabledRules specifies which rules to check. If empty, all rules are enabled.
	EnabledRules []RuleID

	// DisabledRules specifies rules to skip.
	DisabledRules []RuleID

	// MinSeverity is the minimum severity level to report.
	MinSeverity Severity

	// ComplexityThreshold is the maximum allowed cyclomatic complexity.
	ComplexityThreshold int

	// NestingThreshold is the maximum allowed nesting depth.
	NestingThreshold int

	// MinVarNameLength is the minimum variable name length (excluding loop indices).
	MinVarNameLength int

	// SafetyAnnotation is the comment annotation that marks safety-critical functions.
	SafetyAnnotation string

	// TreatWarningsAsErrors makes all violations return errors.
	TreatWarningsAsErrors bool
}

// DefaultConfig returns the default analyzer configuration.
func DefaultConfig() *Config {
	return &Config{
		MinSeverity:         SeverityAdvisory,
		ComplexityThreshold: 10,
		NestingThreshold:    4,
		MinVarNameLength:    2,
		SafetyAnnotation:    "go:safety",
	}
}

// Analyzer performs MISRA-Go analysis on Go code.
type Analyzer struct {
	config     *Config
	fset       *token.FileSet
	pkg        *types.Package
	info       *types.Info
	ssaPkg     *ssa.Package
	violations []Violation
}

// NewAnalyzer creates a new MISRA-Go analyzer.
func NewAnalyzer(config *Config) *Analyzer {
	if config == nil {
		config = DefaultConfig()
	}
	return &Analyzer{
		config: config,
	}
}

// Analyze performs MISRA-Go analysis on the given package.
func (a *Analyzer) Analyze(fset *token.FileSet, pkg *types.Package, info *types.Info, files []*ast.File, ssaPkg *ssa.Package) []Violation {
	a.fset = fset
	a.pkg = pkg
	a.info = info
	a.ssaPkg = ssaPkg
	a.violations = nil

	// Run all enabled checkers
	for _, file := range files {
		a.checkFile(file)
	}

	// SSA-based checks
	if ssaPkg != nil {
		a.checkSSA(ssaPkg)
	}

	// Sort violations by file and line
	sort.Slice(a.violations, func(i, j int) bool {
		if a.violations[i].Pos.Filename != a.violations[j].Pos.Filename {
			return a.violations[i].Pos.Filename < a.violations[j].Pos.Filename
		}
		return a.violations[i].Pos.Line < a.violations[j].Pos.Line
	})

	return a.violations
}

// isRuleEnabled checks if a rule should be checked.
func (a *Analyzer) isRuleEnabled(rule RuleID) bool {
	// Check if explicitly disabled
	for _, r := range a.config.DisabledRules {
		if r == rule {
			return false
		}
	}

	// Check if rule list is specified and rule is included
	if len(a.config.EnabledRules) > 0 {
		for _, r := range a.config.EnabledRules {
			if r == rule {
				return true
			}
		}
		return false
	}

	// Check minimum severity
	info, ok := AllRules[rule]
	if !ok {
		return false
	}
	return info.Severity <= a.config.MinSeverity
}

// addViolation records a rule violation.
func (a *Analyzer) addViolation(rule RuleID, pos token.Pos, message string, context string) {
	if !a.isRuleEnabled(rule) {
		return
	}

	info, ok := AllRules[rule]
	if !ok {
		return
	}

	a.violations = append(a.violations, Violation{
		Rule:     rule,
		Severity: info.Severity,
		Pos:      a.fset.Position(pos),
		Message:  message,
		Context:  context,
	})
}

// checkFile runs AST-based checks on a single file.
func (a *Analyzer) checkFile(file *ast.File) {
	// Create visitor for AST traversal
	v := &fileVisitor{
		analyzer:     a,
		file:         file,
		scopeStack:   make([]*ast.Scope, 0),
		nestingDepth: 0,
		funcName:     "",
	}

	ast.Walk(v, file)
}

// checkSSA runs SSA-based checks.
func (a *Analyzer) checkSSA(pkg *ssa.Package) {
	for _, member := range pkg.Members {
		if fn, ok := member.(*ssa.Function); ok {
			a.checkSSAFunction(fn)
		}
	}
}

// checkSSAFunction analyzes a single SSA function.
func (a *Analyzer) checkSSAFunction(fn *ssa.Function) {
	if fn.Blocks == nil {
		return // External function
	}

	// Check for unreachable code
	a.checkUnreachableBlocks(fn)

	// Check for unbounded recursion
	a.checkRecursion(fn)
}

// checkUnreachableBlocks finds unreachable basic blocks.
func (a *Analyzer) checkUnreachableBlocks(fn *ssa.Function) {
	if !a.isRuleEnabled(RuleUnreachableCode) {
		return
	}

	// Simple reachability analysis
	reachable := make(map[*ssa.BasicBlock]bool)
	var visit func(*ssa.BasicBlock)
	visit = func(b *ssa.BasicBlock) {
		if reachable[b] {
			return
		}
		reachable[b] = true
		for _, succ := range b.Succs {
			visit(succ)
		}
	}

	if len(fn.Blocks) > 0 {
		visit(fn.Blocks[0])
	}

	for _, block := range fn.Blocks {
		if !reachable[block] && len(block.Instrs) > 0 {
			// Find the first instruction with a position
			for _, instr := range block.Instrs {
				if pos := instr.Pos(); pos.IsValid() {
					a.addViolation(RuleUnreachableCode, pos,
						"unreachable code detected",
						fn.Name())
					break
				}
			}
		}
	}
}

// checkRecursion detects potentially unbounded recursion.
func (a *Analyzer) checkRecursion(fn *ssa.Function) {
	if !a.isRuleEnabled(RuleUnboundedRecursion) {
		return
	}

	// Check if function calls itself
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if call, ok := instr.(*ssa.Call); ok {
				if callee := call.Call.StaticCallee(); callee == fn {
					// Found self-recursion, check if there's a clear base case
					// For now, just report it as a warning
					a.addViolation(RuleUnboundedRecursion, call.Pos(),
						"recursive function detected - ensure termination condition is clear",
						fn.Name())
				}
			}
		}
	}
}
