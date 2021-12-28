package main

import (
	"go/types"
	"strings"
	"text/template"
)

func templateFuncs(qf types.Qualifier) template.FuncMap {
	return template.FuncMap{
		"typeReference": func(a string) string {
			if a == "interface{}" {
				return a
			}

			return "*" + a
		},
		"camelCase": func(a string) interface{} {
			return strings.ToLower(string(a[0])) + string(a[1:])
		},
		"join": func(sep string, a []string) interface{} {
			return strings.Join(a, sep)
		},
		"toGoTupleString": toGoTupleString,
		"typeTupleString": func(a *types.Tuple) interface{} {
			return typeTupleString(a, false, qf)
		},
		"toPointer": func(a types.Type) types.Type {
			switch a.(type) {
			case *types.Interface:
				return a
			}
			return types.NewPointer(a)
		},
		"typeString": func(a types.Type) interface{} {
			return types.TypeString(a, qf)
		},
	}
}
