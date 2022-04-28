package main

import (
	"bytes"
	"fmt"
	"go/types"
	"strconv"
	"strings"

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

func getBasicKind(a types.Type) types.BasicKind {
	a = getUnderlyingType(a)
	switch ua := a.(type) {

	case *types.Basic:
		return ua.Kind()
	}

	return 0
}

func isTypeInt(a types.Type) bool {
	a = getUnderlyingType(a)
	basic, ok := a.(*types.Basic)
	if !ok {
		return false
	}

	switch basic.Kind() {
	case types.Int, types.Int8, types.Int16, types.Int32, types.Int64:
		return true

	}

	return false
}

func isTypeString(a types.Type) bool {
	a = getUnderlyingType(a)
	basic, ok := a.(*types.Basic)
	if ok {
		return basic.Kind() == types.String
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

type Literal string

// toGoTupleString converts type to go literal tuple
func toGoTupleString(a interface{}) string {
	switch v := a.(type) {
	case []int:
		var ss []string
		for _, i := range v {
			ss = append(ss, strconv.Itoa(i))
		}
		return strings.Join(ss, ", ")

	case []string:
		var qs []string
		for _, s := range v {
			qs = append(qs, strconv.Quote(s))
		}
		return strings.Join(qs, ", ")

	case []Literal:
		var ss []string
		for _, s := range v {
			ss = append(ss, string(s))
		}
		return strings.Join(ss, ", ")

	default:
		panic(fmt.Errorf("unsupported type: %+v", v))

	}

	return "nil"
}

func typeParamsTuple(a types.Type) *types.Tuple {
	switch a := a.(type) {

	// pure signature callback, like:
	// for func(a bool, b bool) error
	// we return the "a bool, b bool"
	case *types.Signature:
		return a.Params()

	// named type callback, like: BookGenerator, RequestHandler, PositionUpdater
	case *types.Named:
		// fetch the underlying type and return the params tuple
		return typeParamsTuple(a.Underlying())

	default:
		return nil

	}
}
