package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

const (
	ForbidImproperConversions = "forbid-improper-conversions"
)

var (
	fForbidImproperConversions bool
)

func init() {
	registerFlags()
}

func registerFlags() {
	Analyzer.Flags.BoolVar(
		&fForbidImproperConversions, ForbidImproperConversions, false,
		"report errors on conversion of integers via `time.Duration()` without multiplying them by proper units like `time.Second`",
	)
}

// reset flags to default values; useful for testing
func resetFlags() {
	fForbidImproperConversions = false
}

var Analyzer = &analysis.Analyzer{
	Name: "durationlint",
	Doc:  "disallows usage of untyped literals and constants as time.Duration",
	Run:  run,
}

// improperDurationContext is a helper to track context of `time.Duration(int)` conversions in a
// node and in its child nodes; it stores deferred diagnostics that are only reported once it is
// impossible to multiply all nested `time.Duration(int)` conversions by a time unit.
//
// We define a "improper duration" as a node whose type is duration and which has a nested
// `time.Duration(int)` conversion that hasn't been multiplied by a proper duration such as
// a duration variable or constant. A "proper duration" is any other node whose type is duration.
type improperDurationContext struct {
	node                 ast.Node
	isMultiplicationExpr bool
	// Stores whether the current node has type `time.Duration`
	isDurationType bool
	// Stores whether the node is itself an "improper" duration, i.e. one of the form
	// `time.Duration(someIntegerExpression)`.
	isImproperDurationNode bool
	// Stores whether the node has a child which is a proper duration
	hasProperDurationChild bool
	// Stores whether the node has a child which is an improper duration
	hasImproperDurationChild bool
	// Diagnostics which may be reported if no proper child is found or this is not
	// a multiplication
	deferredImproperDurationDiagnostics []*analysis.Diagnostic
}

func (c *improperDurationContext) isProperDuration() bool {
	if !c.isDurationType {
		return false
	}
	return !c.isImproperDuration()
}

func (c *improperDurationContext) isImproperDuration() bool {
	if !c.isDurationType {
		return false
	}
	return c.isImproperDurationNode ||
		(c.hasImproperDurationChild && !c.isMultiplicationExpr) ||
		(c.hasImproperDurationChild && !c.hasProperDurationChild)
}

// stack must be nonempty for any of its methods aside from `PushCurrent` to be called
type improperDurationContextStack struct {
	pass     *analysis.Pass
	sentinel improperDurationContext
	slice    []improperDurationContext
}

func (s *improperDurationContextStack) PushCurrent(node ast.Node) {
	isMultiplicationExpr := false
	expr, isExpr := node.(ast.Expr)
	if isExpr {
		if binaryExpr, isBinaryOp := expr.(*ast.BinaryExpr); isBinaryOp {
			isMultiplicationExpr = binaryExpr.Op == token.MUL
		}
	}
	isDurationExpr := false
	if isExpr {
		isDurationExpr = isDurationType(s.pass.TypesInfo.TypeOf(expr))
	}
	s.slice = append(s.slice, improperDurationContext{
		node:                 node,
		isMultiplicationExpr: isMultiplicationExpr,
		isDurationType:       isDurationExpr,
	})
}

func (s *improperDurationContextStack) PopCurrent() {
	current := s.current()
	parent := s.parent()
	isImproperDuration := current.isImproperDuration()
	isProperDuration := current.isProperDuration()
	if isImproperDuration {
		parent.hasImproperDurationChild = true
	}
	if isProperDuration {
		parent.hasProperDurationChild = true
	}
	switch {
	case isProperDuration:
		// nothing; it's been resolved
	case !current.isMultiplicationExpr:
		// immediately report all if any errors; this is not multiplication so it's too late to
		// fix conversions without a time unit, e.g. `(time.Duration(5) + time.Second)`
		for _, diagnostic := range current.deferredImproperDurationDiagnostics {
			s.pass.Report(*diagnostic)
		}
	default:
		// parent might still be able to resolve these improper durations via multiplication with
		// a proper duration, e.g. `(5 * time.Duration(someInt)) * time.Second`
		parent.deferredImproperDurationDiagnostics = append(
			parent.deferredImproperDurationDiagnostics,
			current.deferredImproperDurationDiagnostics...,
		)
	}
	// free up memory now that diagnostics are deferred or reported
	current.deferredImproperDurationDiagnostics = nil
	s.slice = s.slice[:len(s.slice)-1]
}

func (s *improperDurationContextStack) current() *improperDurationContext {
	return &s.slice[len(s.slice)-1]
}

func (s *improperDurationContextStack) parent() *improperDurationContext {
	if len(s.slice) == 1 {
		return &s.sentinel
	}
	return &s.slice[len(s.slice)-2]
}

func (s *improperDurationContextStack) ReportImproperDurationNode(diagnostic *analysis.Diagnostic) {
	s.current().isImproperDurationNode = true
	// parent might be able to resolve this improper duration via multiplication with a
	// proper duration, e.g. parent could be `time.Duration(5) * time.Second` when child is
	// `time.Duration(5)`
	parent := s.parent()
	parent.deferredImproperDurationDiagnostics = append(
		parent.deferredImproperDurationDiagnostics,
		diagnostic,
	)
}

func (s *improperDurationContextStack) Finish() {
	for _, diagnostic := range s.sentinel.deferredImproperDurationDiagnostics {
		s.ReportImproperDurationNode(diagnostic)
	}
	s.sentinel.deferredImproperDurationDiagnostics = nil
}

func run(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		// NOTE: as struct and function call expressions can be nested in any
		// other assignment and call expressions, we want to always return true
		// to continue descending the tree

		stack := improperDurationContextStack{
			pass: pass,
		}
		defer stack.Finish()
		ast.Inspect(file, func(node ast.Node) bool {
			isExitingNodeTraversal := node == nil
			if !isExitingNodeTraversal {
				stack.PushCurrent(node)
			} else {
				defer stack.PopCurrent()
			}
			switch v := node.(type) {
			case *ast.KeyValueExpr:
				diag := checkAssignment(pass, v.Key, v.Value)
				if diag != nil {
					pass.Report(*diag)
				}
				return true

			case *ast.AssignStmt:
				for i := range v.Lhs {
					diag := checkAssignment(pass, v.Lhs[i], v.Rhs[i])
					if diag != nil {
						pass.Report(*diag)
					}
				}
				return true

			case *ast.ValueSpec:
				if v.Type == nil {
					return true
				}
				for _, value := range v.Values {
					diag := checkAssignment(pass, v.Type, value)
					if diag != nil {
						pass.Report(*diag)
					}
				}
				return true

			case *ast.CallExpr:
				isConversion := isDurationConversion(v)
				if isConversion {
					if !fForbidImproperConversions {
						return false
					}
					diag := checkDurationConversionArgument(pass, v.Args[0])
					if diag != nil {
						stack.ReportImproperDurationNode(diag)
					}
					return true
				} else {
					for _, arg := range v.Args {
						diag := checkArgument(pass, arg)
						if diag != nil {
							pass.Report(*diag)
						}
					}
					return true
				}
			default:
				return true
			}
		})
	}
	return nil, nil
}

func checkArgument(pass *analysis.Pass, v ast.Expr) *analysis.Diagnostic {
	if !isDurationType(pass.TypesInfo.TypeOf(v)) {
		return nil
	}
	if !usesIntOrUntypedConstants(pass.TypesInfo, v) {
		return nil
	}
	return &analysis.Diagnostic{
		Pos:     v.Pos(),
		Message: "untyped constant in time.Duration argument",
	}
}

func checkDurationConversionArgument(pass *analysis.Pass, arg ast.Expr) *analysis.Diagnostic {
	argType := pass.TypesInfo.TypeOf(arg)
	implicitInt := usesIntOrUntypedConstants(pass.TypesInfo, arg)
	explicitInt := isIntegerType(argType)
	if !implicitInt && !explicitInt {
		return nil
	}
	return &analysis.Diagnostic{
		Pos:     arg.Pos(),
		Message: "converting integer via time.Duration() without multiplication by proper duration",
	}
}

func checkAssignment(pass *analysis.Pass, l ast.Expr, r ast.Expr) *analysis.Diagnostic {
	lType := pass.TypesInfo.TypeOf(l)
	if lType == nil || lType.String() != "time.Duration" {
		return nil
	}
	if !usesIntOrUntypedConstants(pass.TypesInfo, r) {
		return nil
	}
	return &analysis.Diagnostic{Pos: r.Pos(), Message: "untyped constant in time.Duration assignment"}
}

func usesIntOrUntypedConstants(ti *types.Info, e ast.Expr) bool {
	switch v := e.(type) {
	case *ast.BasicLit: // ex: 1
		return v.Value != "0"
	case *ast.BinaryExpr:
		switch v.Op {
		case token.ADD, token.SUB: // ex: 1 + time.Seconds
			return usesIntOrUntypedConstants(ti, v.X) || usesIntOrUntypedConstants(ti, v.Y)
		case token.MUL: // ex: 1 * time.Seconds
			return usesIntOrUntypedConstants(ti, v.X) && usesIntOrUntypedConstants(ti, v.Y)
		}
	case *ast.Ident: // ex: someIdentifier
		return hasIntOrUntypedConstDeclaration(ti, v)
	case *ast.ParenExpr:
		return usesIntOrUntypedConstants(ti, v.X)
	}
	return false
}

// we only care about untyped `const Name = 123` declarations
// `var Name = 123`, and `a := 123` declarations are already type-checked
// by the compiler
func hasIntOrUntypedConstDeclaration(ti *types.Info, identifier *ast.Ident) bool {
	decl := identifier.Obj.Decl

	// TODO: we could ignore `var` statements altogether
	// Is there a way to distinguish them?
	vSpec, ok := decl.(*ast.ValueSpec) // `var` or `const` declaration
	if !ok {
		return false
	}

	// typed const or var declaration,
	// we can ignore it if it isn't an integer
	if vSpec.Type != nil && !isIntegerType(ti.TypeOf(vSpec.Type)) {
		return false
	}

	// if it's a multiple declaration (eg. `const a, b = 10, time.Second`)
	// we need to find the correct identifier
	nameIdx := -1
	for i, name := range vSpec.Names {
		if name.Name == identifier.Name {
			nameIdx = i
			break
		}
	}

	if nameIdx == -1 {
		panic("logic error: identifier not found in its declaration")
	}

	// skip if the right-hand side is explicitly typed to time.Duration
	vType := ti.TypeOf(vSpec.Values[nameIdx])
	if vType.String() != "time.Duration" {
		return true
	}

	return false
}

// isDurationConversion recognizes `time.Duration(10)` in order either to not report it at all or
// to report it differently than other errors, depending on the value of `fForbidExplicitCast`
func isDurationConversion(v *ast.CallExpr) bool {
	se, ok := v.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if len(v.Args) != 1 {
		// Duration conversion should take exactly one argument; if not, it's some other call
		return false
	}

	// NOTE: we don't check the package name in the selector expression, as it
	// could have been aliased to something else
	return se.Sel.Name == "Duration"
}

// checks if a type is an integer (e.g. int64, int, int32, uint32)
func isIntegerType(typ types.Type) bool {
	switch typ := typ.(type) {
	case *types.Basic:
		if typ.Info()&types.IsInteger != 0 {
			return true
		}
	}
	return false
}

// checks if a type is `time.Duration`
func isDurationType(typ types.Type) bool {
	if typ == nil {
		return false
	}
	return typ.String() == "time.Duration"
}
