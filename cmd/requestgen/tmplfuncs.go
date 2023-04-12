package main

import (
	"go/types"
	"strings"
	"text/template"

	log "github.com/sirupsen/logrus"
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
		"referenceByType": func(a types.Type) string {
			switch ua := a.Underlying().(type) {
			case *types.Slice, *types.Interface, *types.Map:
				log.Debugf("type %v is %T, do not use reference", ua, ua)
				return ""
			}
			return "&"
		},
		"toPointer": func(a types.Type) types.Type {
			switch ua := a.Underlying().(type) {
			case *types.Slice, *types.Interface, *types.Map:
				log.Debugf("type %v is %T, do not use pointer", ua, ua)
				return a
			}

			return types.NewPointer(a)
		},
		"typeString": func(a types.Type) interface{} {
			return types.TypeString(a, qf)
		},
	}
}
