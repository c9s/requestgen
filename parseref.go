package requestgen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
)

type TypeSelector struct {
	Package string
	Member  string
	IsSlice bool
}

func loadPackageFast(pattern string, tags []string) (*packages.Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedImports |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedModule |
			packages.NeedDeps,
		Tests: false,
		Logf:  log.Debugf,
	}

	if len(tags) > 0 {
		cfg.BuildFlags = []string{fmt.Sprintf("-tags=%s", strings.Join(tags, " "))}
	}

	log.Debugf("loading package: %s", pattern)

	pkgs, err := packages.Load(cfg, pattern)
	if err != nil {
		return nil, err
	}

	if len(pkgs) == 0 {
		return nil, nil
	}

	log.Debugf("loaded package: %s (pkgPath %s) -> %#v", pkgs[0].Name, pkgs[0].PkgPath, pkgs[0])
	return pkgs[0], nil
}

func sanitizeImport(ts *TypeSelector) (*TypeSelector, error) {
	log.Debugf("sanitizing import: %#v", ts)

	pkg, err := loadPackageFast(ts.Package, nil)
	if err != nil {
		return nil, err
	}

	origPath := ts.Package
	ts.Package = pkg.PkgPath

	log.Debugf("sanitized import: %s => %s", origPath, ts.Package)
	return ts, nil
}

func ParseTypeSelector(expr string) (*TypeSelector, error) {
	if len(expr) == 0 {
		return nil, errors.New("empty expression")
	}

	var spec TypeSelector

	// dot references the current package
	if expr[0] == '.' {
		expr = `"."` + expr
	} else if strings.HasPrefix(expr, ".[]") {
		expr = expr[3:]
		spec.IsSlice = true
	} else if strings.HasPrefix(expr, "[]") {
		expr = expr[2:]
		spec.IsSlice = true
	}

	e, err := parser.ParseExpr(expr)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid expression: %s", expr)
	}

	switch e := e.(type) {
	case *ast.Ident:
		spec.Package = "."
		spec.Member = e.Name
		// return spec
		return &spec, nil
		// return sanitizeImport(&spec)

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

	return nil, fmt.Errorf("can not parse type selector: %s", expr)
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
