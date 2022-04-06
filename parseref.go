package requestgen

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/tools/go/ast/astutil"
)

type TypeSelector struct {
	Package string
	Member  string
	IsSlice bool
}

func sanitizeImport(ts *TypeSelector) (*TypeSelector, error) {
	buildCtx := build.Default

	cwd, err := os.Getwd()
	if err != nil {
		return ts, err
	}

	// parse the package file structure and information (this call does not build or compile)
	bp, err := buildCtx.Import(ts.Package, cwd, build.FindOnly)
	if err != nil {
		return ts, fmt.Errorf("can find package %q", ts.Package)
	}

	if bp.ImportPath == "." {
		return ts, fmt.Errorf("can not resolve the package import path %s", bp.ImportPath)
	}

	ts.Package = bp.ImportPath
	return ts, nil
}

func ParseTypeSelector(main string) (*TypeSelector, error) {
	if len(main) == 0 {
		return nil, errors.New("empty expression")
	}

	var spec TypeSelector

	// dot references the current package
	if main[0] == '.' {
		main = `"."` + main
	} else if strings.HasPrefix(main, ".[]") {
		main = main[3:]
		spec.IsSlice = true
	} else if strings.HasPrefix(main, "[]") {
		main = main[2:]
		spec.IsSlice = true
	}

	e, err := parser.ParseExpr(main)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid expression: %s", main)
	}

	switch e := e.(type) {
	case *ast.Ident:
		spec.Package = "."
		spec.Member = e.Name
		return sanitizeImport(&spec)

	case *ast.SelectorExpr:
		x := unparen(e.X)

		// Strip off star constructor, if any.
		if star, ok := x.(*ast.StarExpr); ok {
			x = star.X
		}

		if pkg := parseImportPath(x); pkg != "" {
			// package member e.g. "encoding/json".HTMLEscape
			spec.Package = pkg       // e.g. "encoding/json"
			spec.Member = e.Sel.Name // e.g. "HTMLEscape"
			return sanitizeImport(&spec)
		}

		if x, ok := x.(*ast.SelectorExpr); ok {
			// field/method of type e.g. ("encoding/json".Decoder).Decode
			y := unparen(x.X)
			if pkg := parseImportPath(y); pkg != "" {
				spec.Package = pkg       // e.g. "encoding/json"
				spec.Member = x.Sel.Name // e.g. "Decoder"
				return sanitizeImport(&spec)
			}
		}
	default:
		return nil, fmt.Errorf("expression is not an ident, selector expr or slice expr, %+v given", e)
	}

	return nil, fmt.Errorf("can not parse type selector: %s", main)
}

func unparen(e ast.Expr) ast.Expr {
	return astutil.Unparen(e)
}

func parseImportPath(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.Ident:
		return e.Name // e.g. bytes

	case *ast.BasicLit:
		if e.Kind == token.STRING {
			pkgname, _ := strconv.Unquote(e.Value)
			return pkgname // e.g. "encoding/json"
		}
	}
	return ""
}
