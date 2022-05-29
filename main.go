package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(Analyzer)
}

// prior art:
// https://github.com/golang/lint/issues/130
// https://github.com/dominikh/go-staticcheck/issues/1
// https://github.com/charithe/durationcheck

// references
// https://disaev.me/p/writing-useful-go-analysis-linter/ (from author of golangci-lint)
// https://arslan.io/2019/06/13/using-go-analysis-to-write-a-custom-linter/

var Analyzer = &analysis.Analyzer{
	Name: "durationlint",
	Doc:  "report suspect assignments to time.Duration variables",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			var l, r ast.Expr
			switch v := node.(type) {
			case *ast.KeyValueExpr:
				l = v.Key
				r = v.Value
			case *ast.AssignStmt:
				l = v.Lhs[0]
				r = v.Rhs[0]
			default:
				return true
			}

			lType := pass.TypesInfo.TypeOf(l)
			if lType == nil || lType.String() != "time.Duration" {
				return true
			}

			if isSuspicious(pass.TypesInfo, r) {
				pass.Reportf(r.Pos(), "suspicious assignment to time.Duration")
				return true
			}

			return true
		})
	}
	return nil, nil
}

func isSuspicious(typesInfo *types.Info, e ast.Expr) bool {
	switch v := e.(type) {
	case *ast.BasicLit:
		return true
	case *ast.BinaryExpr:
		switch v.Op {
		case token.ADD:
			return isSuspicious(typesInfo, v.X) || isSuspicious(typesInfo, v.Y)
		case token.MUL:
			return isSuspicious(typesInfo, v.X) && isSuspicious(typesInfo, v.Y)
		}
	case *ast.Ident:
		decl := v.Obj.Decl
		if vSpec, ok := decl.(*ast.ValueSpec); ok {
			if vSpec.Type != nil {
				return false
			}

			vType := typesInfo.TypeOf(vSpec.Values[0])
			if vType.String() != "time.Duration" {
				return true
			}
		}
	}
	return false
}

func inspect(a interface{}) {
	fmt.Printf("%T %+v\n", a, a)
}
