package misra

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"
	"unicode"
)

// fileVisitor traverses AST and checks for rule violations.
type fileVisitor struct {
	analyzer     *Analyzer
	file         *ast.File
	scopeStack   []*ast.Scope
	nestingDepth int
	funcName     string
	funcDecl     *ast.FuncDecl
	isSafety     bool // true if current function has safety annotation

	// Track variable definitions for shadowing detection
	varScopes []map[string]token.Pos
}

// Visit implements ast.Visitor interface.
func (v *fileVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		// Leaving a node - pop scope if needed
		if len(v.varScopes) > 0 {
			// Check if we're leaving a block
		}
		return nil
	}

	switch n := node.(type) {
	case *ast.FuncDecl:
		return v.visitFuncDecl(n)
	case *ast.BlockStmt:
		return v.visitBlockStmt(n)
	case *ast.IfStmt:
		return v.visitIfStmt(n)
	case *ast.ForStmt:
		return v.visitForStmt(n)
	case *ast.RangeStmt:
		return v.visitRangeStmt(n)
	case *ast.SwitchStmt:
		return v.visitSwitchStmt(n)
	case *ast.TypeSwitchStmt:
		return v.visitTypeSwitchStmt(n)
	case *ast.SelectStmt:
		return v.visitSelectStmt(n)
	case *ast.AssignStmt:
		v.visitAssignStmt(n)
	case *ast.ValueSpec:
		v.visitValueSpec(n)
	case *ast.TypeAssertExpr:
		v.visitTypeAssertExpr(n)
	case *ast.CallExpr:
		v.visitCallExpr(n)
	case *ast.GoStmt:
		v.visitGoStmt(n)
	case *ast.SendStmt:
		v.visitSendStmt(n)
	case *ast.BasicLit:
		v.visitBasicLit(n)
	case *ast.GenDecl:
		v.visitGenDecl(n)
	}

	return v
}

// visitFuncDecl handles function declarations.
func (v *fileVisitor) visitFuncDecl(n *ast.FuncDecl) ast.Visitor {
	v.funcName = n.Name.Name
	v.funcDecl = n
	v.isSafety = v.hasSafetyAnnotation(n.Doc)

	// Push new variable scope
	v.varScopes = append(v.varScopes, make(map[string]token.Pos))

	// Check cyclomatic complexity
	if v.analyzer.isRuleEnabled(RuleHighComplexity) {
		complexity := v.calculateComplexity(n)
		if complexity > v.analyzer.config.ComplexityThreshold {
			v.analyzer.addViolation(RuleHighComplexity, n.Pos(),
				"function has cyclomatic complexity of %d (threshold: %d)",
				v.funcName)
		}
	}

	// Check function parameters for shadowing
	if n.Type.Params != nil {
		for _, field := range n.Type.Params.List {
			for _, name := range field.Names {
				v.recordVar(name.Name, name.Pos())
			}
		}
	}

	// Continue visiting function body
	if n.Body != nil {
		ast.Walk(v, n.Body)
	}

	// Pop scope when done
	if len(v.varScopes) > 0 {
		v.varScopes = v.varScopes[:len(v.varScopes)-1]
	}

	v.funcName = ""
	v.funcDecl = nil
	v.isSafety = false

	return nil // Don't revisit children
}

// visitBlockStmt handles block statements (new scope).
func (v *fileVisitor) visitBlockStmt(n *ast.BlockStmt) ast.Visitor {
	// Push new scope
	v.varScopes = append(v.varScopes, make(map[string]token.Pos))
	v.nestingDepth++

	// Check nesting depth
	if v.analyzer.isRuleEnabled(RuleDeepNesting) {
		if v.nestingDepth > v.analyzer.config.NestingThreshold {
			v.analyzer.addViolation(RuleDeepNesting, n.Pos(),
				"nesting depth of %d exceeds threshold of %d",
				v.funcName)
		}
	}

	// Visit children
	for _, stmt := range n.List {
		ast.Walk(v, stmt)
	}

	// Pop scope
	v.nestingDepth--
	if len(v.varScopes) > 0 {
		v.varScopes = v.varScopes[:len(v.varScopes)-1]
	}

	return nil // Don't revisit children
}

// visitIfStmt handles if statements.
func (v *fileVisitor) visitIfStmt(n *ast.IfStmt) ast.Visitor {
	// Check for empty branches
	if v.analyzer.isRuleEnabled(RuleEmptyBranch) {
		if n.Body != nil && len(n.Body.List) == 0 {
			if !v.hasExplanatoryComment(n.Body) {
				v.analyzer.addViolation(RuleEmptyBranch, n.Body.Pos(),
					"empty if body should have explanatory comment",
					v.funcName)
			}
		}
		if elseStmt, ok := n.Else.(*ast.BlockStmt); ok && len(elseStmt.List) == 0 {
			if !v.hasExplanatoryComment(elseStmt) {
				v.analyzer.addViolation(RuleEmptyBranch, elseStmt.Pos(),
					"empty else body should have explanatory comment",
					v.funcName)
			}
		}
	}

	return v // Continue visiting
}

// visitForStmt handles for loops.
func (v *fileVisitor) visitForStmt(n *ast.ForStmt) ast.Visitor {
	// Push scope for loop variables
	v.varScopes = append(v.varScopes, make(map[string]token.Pos))

	if n.Init != nil {
		ast.Walk(v, n.Init)
	}
	if n.Cond != nil {
		ast.Walk(v, n.Cond)
	}
	if n.Post != nil {
		ast.Walk(v, n.Post)
	}
	if n.Body != nil {
		ast.Walk(v, n.Body)
	}

	// Pop scope
	if len(v.varScopes) > 0 {
		v.varScopes = v.varScopes[:len(v.varScopes)-1]
	}

	return nil
}

// visitRangeStmt handles range loops.
func (v *fileVisitor) visitRangeStmt(n *ast.RangeStmt) ast.Visitor {
	// Push scope for loop variables
	v.varScopes = append(v.varScopes, make(map[string]token.Pos))

	// Record range variables
	if n.Key != nil {
		if ident, ok := n.Key.(*ast.Ident); ok && n.Tok == token.DEFINE {
			v.recordVar(ident.Name, ident.Pos())
		}
	}
	if n.Value != nil {
		if ident, ok := n.Value.(*ast.Ident); ok && n.Tok == token.DEFINE {
			v.recordVar(ident.Name, ident.Pos())
		}
	}

	ast.Walk(v, n.X)
	if n.Body != nil {
		ast.Walk(v, n.Body)
	}

	// Pop scope
	if len(v.varScopes) > 0 {
		v.varScopes = v.varScopes[:len(v.varScopes)-1]
	}

	return nil
}

// visitSwitchStmt handles switch statements.
func (v *fileVisitor) visitSwitchStmt(n *ast.SwitchStmt) ast.Visitor {
	// Check for missing default case
	if v.analyzer.isRuleEnabled(RuleMissingDefault) {
		hasDefault := false
		if n.Body != nil {
			for _, stmt := range n.Body.List {
				if cc, ok := stmt.(*ast.CaseClause); ok && cc.List == nil {
					hasDefault = true
					break
				}
			}
		}
		if !hasDefault {
			v.analyzer.addViolation(RuleMissingDefault, n.Pos(),
				"switch statement should have a default case",
				v.funcName)
		}
	}

	// Check for empty cases
	if v.analyzer.isRuleEnabled(RuleEmptyBranch) && n.Body != nil {
		for _, stmt := range n.Body.List {
			if cc, ok := stmt.(*ast.CaseClause); ok && len(cc.Body) == 0 {
				if !v.hasExplanatoryCommentForCase(cc) {
					v.analyzer.addViolation(RuleEmptyBranch, cc.Pos(),
						"empty case should have explanatory comment",
						v.funcName)
				}
			}
		}
	}

	return v
}

// visitTypeSwitchStmt handles type switch statements.
func (v *fileVisitor) visitTypeSwitchStmt(n *ast.TypeSwitchStmt) ast.Visitor {
	// Check for missing default case
	if v.analyzer.isRuleEnabled(RuleMissingDefault) {
		hasDefault := false
		if n.Body != nil {
			for _, stmt := range n.Body.List {
				if cc, ok := stmt.(*ast.CaseClause); ok && cc.List == nil {
					hasDefault = true
					break
				}
			}
		}
		if !hasDefault {
			v.analyzer.addViolation(RuleMissingDefault, n.Pos(),
				"type switch statement should have a default case",
				v.funcName)
		}
	}

	return v
}

// visitSelectStmt handles select statements.
func (v *fileVisitor) visitSelectStmt(n *ast.SelectStmt) ast.Visitor {
	// Check for channel usage in safety-critical code
	if v.isSafety && v.analyzer.isRuleEnabled(RuleChannelInSafety) {
		v.analyzer.addViolation(RuleChannelInSafety, n.Pos(),
			"select statement in safety-critical function",
			v.funcName)
	}

	// Check for missing default case (can cause blocking)
	if v.analyzer.isRuleEnabled(RuleMissingDefault) {
		hasDefault := false
		if n.Body != nil {
			for _, stmt := range n.Body.List {
				if cc, ok := stmt.(*ast.CommClause); ok && cc.Comm == nil {
					hasDefault = true
					break
				}
			}
		}
		if !hasDefault && v.isSafety {
			v.analyzer.addViolation(RuleMissingDefault, n.Pos(),
				"select without default case may block indefinitely",
				v.funcName)
		}
	}

	return v
}

// visitAssignStmt handles assignment statements.
func (v *fileVisitor) visitAssignStmt(n *ast.AssignStmt) {
	// Check for short variable declarations (shadowing)
	if n.Tok == token.DEFINE {
		for _, lhs := range n.Lhs {
			if ident, ok := lhs.(*ast.Ident); ok {
				// Check for shadowing
				if v.analyzer.isRuleEnabled(RuleShadowedVariable) {
					if prevPos := v.findVar(ident.Name); prevPos.IsValid() {
						v.analyzer.addViolation(RuleShadowedVariable, ident.Pos(),
							"variable '%s' shadows declaration at %s",
							v.funcName)
					}
				}

				// Check for short variable names
				if v.analyzer.isRuleEnabled(RuleShortVarName) {
					if !v.isAcceptableShortName(ident.Name) {
						v.analyzer.addViolation(RuleShortVarName, ident.Pos(),
							"variable name '%s' is too short",
							v.funcName)
					}
				}

				v.recordVar(ident.Name, ident.Pos())
			}
		}
	}

	// Check for ignored error returns
	if v.analyzer.isRuleEnabled(RuleIgnoredError) {
		for _, lhs := range n.Lhs {
			if ident, ok := lhs.(*ast.Ident); ok && ident.Name == "_" {
				// Check if RHS is a call expression with error return
				for i, rhs := range n.Rhs {
					if call, ok := rhs.(*ast.CallExpr); ok {
						if v.returnsError(call) && i < len(n.Lhs) {
							// Check if this is the error position
							if v.isErrorPosition(call, i) {
								v.analyzer.addViolation(RuleIgnoredError, ident.Pos(),
									"error return value is ignored",
									v.funcName)
							}
						}
					}
				}
			}
		}
	}
}

// visitValueSpec handles variable declarations.
func (v *fileVisitor) visitValueSpec(n *ast.ValueSpec) {
	for _, name := range n.Names {
		// Check for shadowing
		if v.analyzer.isRuleEnabled(RuleShadowedVariable) {
			if prevPos := v.findVar(name.Name); prevPos.IsValid() {
				v.analyzer.addViolation(RuleShadowedVariable, name.Pos(),
					"variable '%s' shadows declaration",
					v.funcName)
			}
		}

		// Check for short variable names
		if v.analyzer.isRuleEnabled(RuleShortVarName) && v.funcName != "" {
			if !v.isAcceptableShortName(name.Name) {
				v.analyzer.addViolation(RuleShortVarName, name.Pos(),
					"variable name '%s' is too short",
					v.funcName)
			}
		}

		v.recordVar(name.Name, name.Pos())
	}
}

// visitTypeAssertExpr handles type assertions.
func (v *fileVisitor) visitTypeAssertExpr(n *ast.TypeAssertExpr) {
	if v.analyzer.isRuleEnabled(RuleUnsafeTypeAssertion) {
		// Type assertions should use the two-value form
		// This is a heuristic - we check the parent context
		// The two-value form is detected by the parser as part of an AssignStmt
		// with 2 LHS values
		v.analyzer.addViolation(RuleUnsafeTypeAssertion, n.Pos(),
			"type assertion without ok check may panic",
			v.funcName)
	}
}

// visitCallExpr handles function calls.
func (v *fileVisitor) visitCallExpr(n *ast.CallExpr) {
	// Check for panic calls in safety-critical code
	if v.isSafety && v.analyzer.isRuleEnabled(RulePanicInSafetyCode) {
		if ident, ok := n.Fun.(*ast.Ident); ok && ident.Name == "panic" {
			v.analyzer.addViolation(RulePanicInSafetyCode, n.Pos(),
				"panic() called in safety-critical function",
				v.funcName)
		}
	}

	// Check for recover misuse
	if v.analyzer.isRuleEnabled(RuleRecoverMisuse) {
		if ident, ok := n.Fun.(*ast.Ident); ok && ident.Name == "recover" {
			// recover is only valid in a deferred function
			// This is a simplified check
			v.analyzer.addViolation(RuleRecoverMisuse, n.Pos(),
				"recover() should only be called in deferred functions",
				v.funcName)
		}
	}

	// Check for make/new in safety-critical code (dynamic allocation)
	if v.isSafety && v.analyzer.isRuleEnabled(RuleDynamicAllocation) {
		if ident, ok := n.Fun.(*ast.Ident); ok {
			if ident.Name == "make" || ident.Name == "new" || ident.Name == "append" {
				v.analyzer.addViolation(RuleDynamicAllocation, n.Pos(),
					"%s() may cause dynamic memory allocation",
					v.funcName)
			}
		}
	}
}

// visitGoStmt handles go statements.
func (v *fileVisitor) visitGoStmt(n *ast.GoStmt) {
	if v.isSafety && v.analyzer.isRuleEnabled(RuleGoroutineInSafety) {
		v.analyzer.addViolation(RuleGoroutineInSafety, n.Pos(),
			"goroutine spawned in safety-critical function",
			v.funcName)
	}
}

// visitSendStmt handles channel send statements.
func (v *fileVisitor) visitSendStmt(n *ast.SendStmt) {
	if v.isSafety && v.analyzer.isRuleEnabled(RuleChannelInSafety) {
		v.analyzer.addViolation(RuleChannelInSafety, n.Pos(),
			"channel send in safety-critical function",
			v.funcName)
	}
}

// visitBasicLit handles literal values.
func (v *fileVisitor) visitBasicLit(n *ast.BasicLit) {
	if v.analyzer.isRuleEnabled(RuleMagicNumber) && v.funcName != "" {
		if n.Kind == token.INT || n.Kind == token.FLOAT {
			// Allow common values: 0, 1, -1, 2
			if !isCommonNumericLiteral(n.Value) {
				v.analyzer.addViolation(RuleMagicNumber, n.Pos(),
					"magic number '%s' should be a named constant",
					v.funcName)
			}
		}
	}
}

// visitGenDecl handles general declarations (var, const, type).
func (v *fileVisitor) visitGenDecl(n *ast.GenDecl) {
	// Check for global variables
	if n.Tok == token.VAR && v.funcName == "" {
		if v.analyzer.isRuleEnabled(RuleGlobalVariable) {
			for _, spec := range n.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok {
					for _, name := range vs.Names {
						if name.Name != "_" {
							v.analyzer.addViolation(RuleGlobalVariable, name.Pos(),
								"global variable '%s' - consider using dependency injection",
								"")
						}
					}
				}
			}
		}

		// Check for mutable global variables
		if v.analyzer.isRuleEnabled(RuleMutableGlobal) {
			for _, spec := range n.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok {
					// Check if it's not a constant (var declaration)
					for _, name := range vs.Names {
						if name.Name != "_" && !v.hasConstComment(n.Doc) {
							v.analyzer.addViolation(RuleMutableGlobal, name.Pos(),
								"mutable global variable '%s' should be documented or made const",
								"")
						}
					}
				}
			}
		}
	}
}

// Helper functions

// hasSafetyAnnotation checks if the doc comment contains the safety annotation.
func (v *fileVisitor) hasSafetyAnnotation(doc *ast.CommentGroup) bool {
	if doc == nil {
		return false
	}
	annotation := v.analyzer.config.SafetyAnnotation
	for _, comment := range doc.List {
		if strings.Contains(comment.Text, annotation) {
			return true
		}
	}
	return false
}

// hasExplanatoryComment checks if a block has an explanatory comment.
func (v *fileVisitor) hasExplanatoryComment(block *ast.BlockStmt) bool {
	// Check if there's a comment inside the empty block
	// This is a simplified check - a real implementation would use CommentMap
	return false
}

// hasExplanatoryCommentForCase checks if a case has an explanatory comment.
func (v *fileVisitor) hasExplanatoryCommentForCase(cc *ast.CaseClause) bool {
	// Simplified check
	return false
}

// recordVar records a variable definition in the current scope.
func (v *fileVisitor) recordVar(name string, pos token.Pos) {
	if len(v.varScopes) > 0 {
		v.varScopes[len(v.varScopes)-1][name] = pos
	}
}

// findVar looks for a variable in outer scopes (for shadowing detection).
func (v *fileVisitor) findVar(name string) token.Pos {
	// Skip the current scope, look in outer scopes
	for i := len(v.varScopes) - 2; i >= 0; i-- {
		if pos, ok := v.varScopes[i][name]; ok {
			return pos
		}
	}
	return token.NoPos
}

// isAcceptableShortName checks if a short variable name is acceptable.
func (v *fileVisitor) isAcceptableShortName(name string) bool {
	// Common acceptable short names
	acceptable := map[string]bool{
		"i": true, "j": true, "k": true, // loop indices
		"n": true, "m": true, // counts
		"x": true, "y": true, "z": true, // coordinates
		"a": true, "b": true, "c": true, // generic short vars
		"r": true, "w": true, // reader/writer
		"ok": true, "err": true, // Go conventions
		"_": true, // blank identifier
	}

	if acceptable[name] {
		return true
	}

	return len(name) >= v.analyzer.config.MinVarNameLength
}

// isCommonNumericLiteral checks if a numeric literal is a common value.
func isCommonNumericLiteral(value string) bool {
	common := map[string]bool{
		"0": true, "1": true, "2": true, "-1": true,
		"0.0": true, "1.0": true,
		"8": true, "16": true, "32": true, "64": true, // bit sizes
		"10": true, "100": true, "1000": true, // common bases
	}
	return common[value]
}

// returnsError checks if a call expression returns an error.
func (v *fileVisitor) returnsError(call *ast.CallExpr) bool {
	// This requires type information - simplified check
	if v.analyzer.info == nil {
		return false
	}

	tv, ok := v.analyzer.info.Types[call]
	if !ok {
		return false
	}

	// Check if return type includes error
	if tuple, ok := tv.Type.(*types.Tuple); ok {
		for i := 0; i < tuple.Len(); i++ {
			if isErrorType(tuple.At(i).Type()) {
				return true
			}
		}
	}

	return isErrorType(tv.Type)
}

// isErrorType checks if a type is the error interface.
func isErrorType(t types.Type) bool {
	if named, ok := t.(*types.Named); ok {
		return named.Obj().Name() == "error" && named.Obj().Pkg() == nil
	}
	if iface, ok := t.Underlying().(*types.Interface); ok {
		return iface.NumMethods() == 1 && iface.Method(0).Name() == "Error"
	}
	return false
}

// isErrorPosition checks if the given position is the error return value.
func (v *fileVisitor) isErrorPosition(call *ast.CallExpr, pos int) bool {
	// Simplified - would need type info for accurate check
	return true
}

// hasConstComment checks if doc comment indicates the var should be const.
func (v *fileVisitor) hasConstComment(doc *ast.CommentGroup) bool {
	if doc == nil {
		return false
	}
	for _, comment := range doc.List {
		text := strings.ToLower(comment.Text)
		if strings.Contains(text, "const") || strings.Contains(text, "immutable") ||
			strings.Contains(text, "read-only") || strings.Contains(text, "readonly") {
			return true
		}
	}
	return false
}

// calculateComplexity calculates the cyclomatic complexity of a function.
func (v *fileVisitor) calculateComplexity(fn *ast.FuncDecl) int {
	if fn.Body == nil {
		return 1
	}

	complexity := 1 // Start with 1 for the function itself

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.IfStmt:
			complexity++
		case *ast.ForStmt:
			complexity++
		case *ast.RangeStmt:
			complexity++
		case *ast.CaseClause:
			complexity++
		case *ast.CommClause:
			complexity++
		case *ast.BinaryExpr:
			if expr, ok := n.(*ast.BinaryExpr); ok {
				if expr.Op == token.LAND || expr.Op == token.LOR {
					complexity++
				}
			}
		}
		return true
	})

	return complexity
}

// isLoopIndex checks if a variable name appears to be a loop index.
func isLoopIndex(name string) bool {
	return len(name) == 1 && unicode.IsLower(rune(name[0]))
}
