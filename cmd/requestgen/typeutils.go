package main

import (
	"bytes"
	"go/types"

	"github.com/sirupsen/logrus"
)

// typeTupleString converts Tuple types to string
func typeTupleString(tup *types.Tuple, variadic bool, qf types.Qualifier) string {
	buf := bytes.NewBuffer(nil)
	// buf.WriteByte('(')
	if tup != nil {

		for i := 0; i < tup.Len(); i++ {
			v := tup.At(i)
			if i > 0 {
				buf.WriteString(", ")
			}

			name := v.Name()
			if name != "" {
				buf.WriteString(name)
				buf.WriteByte(' ')
			}

			typ := v.Type()

			if variadic && i == tup.Len()-1 {
				if s, ok := typ.(*types.Slice); ok {
					buf.WriteString("...")
					typ = s.Elem()
				} else {
					// special case:
					// append(s, "foo"...) leads to signature func([]byte, string...)
					if t, ok := typ.Underlying().(*types.Basic); !ok || t.Kind() != types.String {
						panic("internal error: string type expected")
					}
					types.WriteType(buf, typ, qf)
					buf.WriteString("...")
					continue
				}
			}
			types.WriteType(buf, typ, qf)
		}
	}
	// buf.WriteByte(')')
	return buf.String()
}

func getUnderlyingType(a types.Type) types.Type {
	p, ok := a.(*types.Pointer)
	if ok {
		a = p.Elem()
	}

	for {
		if n, ok := a.(*types.Named); ok {
			a = n.Underlying()
		} else {
			break
		}
	}

	return a
}

func isTypeInt(a types.Type) bool {
	a = getUnderlyingType(a)
	switch ua := a.(type) {

	case *types.Basic:
		switch ua.Kind() {
		case types.Int, types.Int32, types.Int64:
			return true

		}

	}

	return false
}

func getBasicKind(a types.Type) types.BasicKind {
	a = getUnderlyingType(a)
	switch ua := a.(type) {

	case *types.Basic:
		return ua.Kind()
	}

	return 0
}

func isTypeString(a types.Type) bool {
	a = getUnderlyingType(a)
	switch ua := a.(type) {

	case *types.Basic:
		return ua.Kind() == types.String

	}

	return false
}

func debugUnderlying(k string, a types.Type) {
	underlying := a.Underlying()
	switch ua := underlying.(type) {
	case *types.Basic:
		logrus.Debugf("%s %+v underlying -> basic: %+v info: %+v kind: %+v", k, a, ua, ua.Info(), ua.Kind())
		switch ua.Kind() {
		case types.String:
		case types.Int:
		case types.Bool:

		}

	case *types.Struct:
		logrus.Debugf("%s %+v underlying -> struct: %+v numFields: %d", k, a, ua, ua.NumFields())

	default:
		logrus.Debugf("%s %+v underlying -> default: %+v", k, a, ua)

	}
}
