package requestgen

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"

	"golang.org/x/tools/go/ast/astutil"
)

type TypeSelector struct {
	pkg        string
	pkgMember  string
}

func ParseTypeSelector(main string) (*TypeSelector, error) {
	var spec TypeSelector

	e, _ := parser.ParseExpr(main)

	if pkg := parseImportPath(e); pkg != "" {
		// e.g. bytes or "encoding/json": a package
		spec.pkg = pkg
		return &spec, nil
	}

	if e, ok := e.(*ast.SelectorExpr); ok {
		x := unparen(e.X)

		// Strip off star constructor, if any.
		if star, ok := x.(*ast.StarExpr); ok {
			x = star.X
		}

		if pkg := parseImportPath(x); pkg != "" {
			// package member e.g. "encoding/json".HTMLEscape
			spec.pkg = pkg              // e.g. "encoding/json"
			spec.pkgMember = e.Sel.Name // e.g. "HTMLEscape"
			return &spec, nil
		}

		if x, ok := x.(*ast.SelectorExpr); ok {
			// field/method of type e.g. ("encoding/json".Decoder).Decode
			y := unparen(x.X)
			if pkg := parseImportPath(y); pkg != "" {
				spec.pkg = pkg               // e.g. "encoding/json"
				spec.pkgMember = x.Sel.Name  // e.g. "Decoder"
				return &spec, nil
			}
		}
	}

	return nil, errors.New("can not parse type")
}

func unparen(e ast.Expr) ast.Expr { return astutil.Unparen(e) }

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
