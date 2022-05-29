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
	ti := pass.TypesInfo
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			var l, r ast.Expr
			switch v := node.(type) {
			case *ast.KeyValueExpr:
				l = v.Key
				r = v.Value
			case *ast.AssignStmt:
				// TODO: in multiple assignments, we are currently only checking
				//  the first one
				l = v.Lhs[0]
				r = v.Rhs[0]
			case *ast.CallExpr:
				if shouldExcludeCall(v) {
					return false
				}
				for _, arg := range v.Args {
					if ti.TypeOf(arg).String() == "time.Duration" && usesUntypedConstants(ti, arg) {
						pass.Reportf(v.Pos(), "untyped constant in time.Duration argument")
						return false
					}
				}
				return true
			default:
				return true
			}

			lType := ti.TypeOf(l)
			if lType == nil || lType.String() != "time.Duration" {
				// NOTE: we need to explore the subtree even if the lvalue is
				// not a time.Duration, so to detect `a := MyStruct { Duration: ... }`
				return true
			}

			if usesUntypedConstants(ti, r) {
				pass.Reportf(r.Pos(), "untyped constant in time.Duration assignment")
				return false
			}

			return true
		})
	}
	return nil, nil
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
func hasUntypedConstDeclaration(ti *types.Info, v *ast.Ident) bool {
	decl := v.Obj.Decl

	vSpec, ok := decl.(*ast.ValueSpec) // `var` or `const` declaration
	if !ok {
		return false
	}

	// TODO: how to distinguish var and const

	if vSpec.Type != nil { // `const name type = ...` declaration
		return false
	}

	// `const name = something` declaration
	// where `something` is a time.Duration
	vType := ti.TypeOf(vSpec.Values[0])
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
