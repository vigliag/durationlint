package analyzer

import (
	"go/ast"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "durationlint",
	Doc:  "disallows usage of untyped literals and constants as time.Duration",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		// NOTE: as struct and function call expressions can be nested in any
		// other assignment and call expressions, we want to always return true
		// to continue descending the tree

		ast.Inspect(file, func(node ast.Node) bool {
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
				if shouldExcludeCall(v) {
					return false
				}
				for _, arg := range v.Args {
					diag := checkArgument(pass, arg)
					if diag != nil {
						pass.Report(*diag)
					}
				}
				return true

			default:
				return true
			}
		})
	}
	return nil, nil
}

func checkArgument(pass *analysis.Pass, v ast.Expr) *analysis.Diagnostic {
	if pass.TypesInfo.TypeOf(v).String() != "time.Duration" {
		return nil
	}
	if !usesUntypedConstants(pass.TypesInfo, v) {
		return nil
	}
	return &analysis.Diagnostic{
		Pos:     v.Pos(),
		Message: "untyped constant in time.Duration argument",
	}
}

func checkAssignment(pass *analysis.Pass, l ast.Expr, r ast.Expr) *analysis.Diagnostic {
	lType := pass.TypesInfo.TypeOf(l)
	if lType == nil || lType.String() != "time.Duration" {
		return nil
	}
	if !usesUntypedConstants(pass.TypesInfo, r) {
		return nil
	}
	return &analysis.Diagnostic{Pos: r.Pos(), Message: "untyped constant in time.Duration assignment"}
}

func usesUntypedConstants(ti *types.Info, e ast.Expr) bool {
	switch v := e.(type) {
	case *ast.BasicLit: // ex: 1
		return v.Value != "0"
	case *ast.BinaryExpr:
		switch v.Op {
		case token.ADD: // ex: 1 + time.Seconds
			return usesUntypedConstants(ti, v.X) || usesUntypedConstants(ti, v.Y)
		case token.MUL: // ex: 1 * time.Seconds
			return usesUntypedConstants(ti, v.X) && usesUntypedConstants(ti, v.Y)
		}
	case *ast.Ident: // ex: someIdentifier
		return hasUntypedConstDeclaration(ti, v)
	}
	return false
}

// we only care about untyped `const Name = 123` declarations
// `var Name = 123`, and `a := 123` declarations are already type-checked
// by the compiler
func hasUntypedConstDeclaration(ti *types.Info, identifier *ast.Ident) bool {
	decl := identifier.Obj.Decl

	// TODO: we could ignore `var` statements altogether
	// Is there a way to distinguish them?
	vSpec, ok := decl.(*ast.ValueSpec) // `var` or `const` declaration
	if !ok {
		return false
	}

	// typed const or var declaration,
	// we can ignore it as it is already type-checked
	if vSpec.Type != nil {
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

// shouldExcludeCall prevents `time.Duration(10)` from being reported
func shouldExcludeCall(v *ast.CallExpr) bool {
	se, ok := v.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	// NOTE: we don't check the package name in the selector expression, as it
	// could have been aliased to something else
	return se.Sel.Name == "Duration"
}
