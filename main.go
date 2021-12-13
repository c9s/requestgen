/*
requestgen generates the request builder methods.

1. it parses the struct of the given type
2. iterate and filter the fields with json tag.
3. build up the field object with the parsed metadata
4. generate the accessor method for each field
	1. pointer -> optional fields
	2. literal value -> required fields
5. parameter builder method should return one of the types:
	- url.Values
	- map[string]interface{}

*/
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/fatih/camelcase"
	"github.com/fatih/structtag"
	log "github.com/sirupsen/logrus"

	"golang.org/x/tools/go/packages"
)

var (
	typeNamesStr  = flag.String("type", "", "comma-separated list of type names; must be set")
	parameterType = flag.String("parameterType", "map", "the parameter type to build, valid: map or url, default: map")
	debug         = flag.Bool("debug", false, "debug mode")
	outputStdout  = flag.Bool("stdout", false, "output generated content to the stdout")
	output        = flag.String("output", "", "output file name; default srcdir/<type>_string.go")
	buildTags     = flag.String("tags", "", "comma-separated list of build tags to apply")
)

// File holds a single parsed file and associated data.
type File struct {
	pkg  *Package  // Package to which this file belongs.
	file *ast.File // Parsed AST.
}

type Package struct {
	name  string
	pkg   *packages.Package
	defs  map[*ast.Ident]types.Object
	files []*File
}

type Field struct {
	Name string

	Type types.Type

	// ArgType is the argument type of the setter
	ArgType types.Type

	ArgKind types.BasicKind

	IsString bool

	IsInt bool

	IsTime bool

	DefaultValuer string

	Default interface{}

	IsMillisecondsTime bool

	// SetterName is the method name of the setter
	SetterName string

	// JsonKey is the key that is used for setting the parameters
	JsonKey string

	// Optional - is this field an optional parameter?
	Optional bool

	// Required means we will check the interval value of the field, empty string or zero will be rejected
	Required bool

	File *ast.File

	ValidValues interface{}

	// StructName is the struct of the given request type
	StructName     string
	StructTypeName string
	StructType     *types.Struct
	ReceiverName   string
}

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

type Generator struct {
	buf bytes.Buffer // Accumulated output.
	pkg *Package     // Package we are scanning.

	// structTypeReceiverNames is used for collecting the receiver name of the given struct types
	structTypeReceiverNames map[string]string

	importPackages map[string]struct{}

	// the collected fields
	fields []Field
	queryFields []Field
}

// addPackage adds a type checked Package and its syntax files to the generator.
func (g *Generator) addPackage(pkg *packages.Package) {
	g.pkg = &Package{
		name:  pkg.Name,
		pkg:   pkg,
		defs:  pkg.TypesInfo.Defs,
		files: make([]*File, len(pkg.Syntax)),
	}

	for i, file := range pkg.Syntax {
		g.pkg.files[i] = &File{
			file: file,
			pkg:  g.pkg,
		}
	}
}

func (g *Generator) Newline() {
	fmt.Fprint(&g.buf, "\n")
}

func (g *Generator) Printf(format string, args ...interface{}) {
	fmt.Fprintf(&g.buf, format, args...)
}

func (g *Generator) registerReceiverNameOfType(decl *ast.FuncDecl) bool {
	// find the receiver and use the user-defined receiver name (not type)
	// skip functions that don't have receiver
	if decl.Recv == nil || len(decl.Recv.List) == 0 {
		return false
	}

	// there will be only one element in the receiver list
	receiver := decl.Recv.List[0]

	// skip if the typeAndValue is not defined in this parsed package
	receiverTypeValue, ok := g.pkg.pkg.TypesInfo.Types[receiver.Type]
	if !ok {
		return true
	}

	// there are 2 types of receiver type value (named type or pointer type)
	// here we record the type name -> receiver name mapping
	switch receiverType := receiverTypeValue.Type.(type) {
	case *types.Named:
		g.structTypeReceiverNames[receiverType.String()] = receiver.Names[0].String()

	case *types.Pointer:
		g.structTypeReceiverNames[receiverType.Elem().String()] = receiver.Names[0].String()
	}
	return true
}

func (g *Generator) parseStruct(file *ast.File, typeSpec *ast.TypeSpec, structType *ast.StructType) {
	typeDef := g.pkg.pkg.TypesInfo.Defs[typeSpec.Name]
	fullTypeName := typeDef.Type().String()
	_ = fullTypeName

	structTV := g.pkg.pkg.TypesInfo.Types[structType]
	_ = structTV

	// iterate the field list (by syntax)
	for _, field := range structType.Fields.List {

		// each struct field AST could have multiple names in one line
		if len(field.Names) == 1 {
			var optional = false
			var name = field.Names[0].Name
			var jsonKey = name

			var isExported = field.Names[0].IsExported()
			var setterName string

			// convert field name to the json key as the default json key
			var ss = camelcase.Split(name)

			if isExported {
				ss[0] = strings.ToLower(ss[0])
				setterName = "Set" + name
				jsonKey = strings.Join(ss, "")
			} else {
				ss[0] = strings.Title(ss[0])
				setterName = strings.Join(ss, "")
				jsonKey = name
			}

			if field.Tag == nil {
				continue
			}

			tag := field.Tag.Value
			tag = strings.Trim(tag, "`")
			tags, err := structtag.Parse(tag)
			if err != nil {
				log.WithError(err).Errorf("struct tag parse error, tag: %s", tag)
				continue
			}

			paramTag, err := tags.Get("param")
			if err != nil {
				continue
			}

			if len(paramTag.Name) > 0 {
				jsonKey = paramTag.Name
			}

			// The field.Type is an ast Type, we can't use that.
			// So we need to find the abstract type information from the types info
			typeValue, ok := g.pkg.pkg.TypesInfo.Types[field.Type]
			if !ok {
				continue
			}

			var argType types.Type
			var argKind types.BasicKind

			switch a := typeValue.Type.(type) {
			case *types.Pointer:
				optional = true
				argType = a.Elem()
			default:
				argType = a
			}

			argKind = getBasicKind(argType)
			isString := isTypeString(argType)
			isInt := isTypeInt(argType)
			isTime := argType.String() == "time.Time"
			required := paramTag.HasOption("required")
			isMillisecondsTime := paramTag.HasOption("milliseconds")
			isQuery := paramTag.HasOption("query")

			if isTime {
				g.importPackages["time"] = struct{}{}
				if isMillisecondsTime {
					g.importPackages["strconv"] = struct{}{}
				}
			}

			if !isTime && isMillisecondsTime {
				log.Errorf("milliseconds option is not valid for non time.Time type field")
				return
			}

			var defaultValuer string
			defaultTag, _ := tags.Get("defaultValuer")
			if defaultTag != nil {
				defaultValuer = defaultTag.Value()
				switch defaultValuer {
				case "now()":
					g.importPackages["time"] = struct{}{}
				case "uuid()":
					g.importPackages["github.com/google/uuid"] = struct{}{}

				}
			}

			fieldName := field.Names[0].Name
			debugUnderlying(fieldName, argType)

			var validValues interface{}
			validValuesTag, _ := tags.Get("validValues")
			if validValuesTag != nil {
				validValueList := strings.Split(validValuesTag.Value(), ",")

				log.Debugf("%s found valid values: %v", fieldName, validValueList)

				switch argKind {
				case types.Int, types.Int64, types.Int32:
					var slice []int
					for _, s := range validValueList {
						i, err := strconv.Atoi(s)
						if err != nil {
							return
						}
						slice = append(slice, i)
					}

				case types.String:
					validValues = validValueList

				}
			}

			receiverName, ok := g.structTypeReceiverNames[fullTypeName]
			if !ok {
				receiverName = strings.ToLower(string(typeSpec.Name.String()[0]))
			}
			f := Field{
				Name:               field.Names[0].Name,
				Type:               typeValue.Type,
				ArgType:            argType,
				SetterName:         setterName,
				IsString:           isString,
				IsInt:              isInt,
				IsTime:             isTime,
				IsMillisecondsTime: isMillisecondsTime,
				JsonKey:            jsonKey,
				Optional:           optional,
				Required:           required,
				ValidValues:        validValues,
				DefaultValuer:      defaultValuer,

				StructName:     typeSpec.Name.String(),
				StructTypeName: fullTypeName,
				StructType:     structTV.Type.(*types.Struct),
				ReceiverName:   receiverName,
				File:           file,
			}

			g.fields = append(g.fields, f)

			// query parameters
			if isQuery {
				g.queryFields = append(g.queryFields, f)
			}
		}
	}
}

// nodeParser returns an ast node iterator function for iterating the ast nodes
func (g *Generator) nodeParser(typeName string, file *File) func(ast.Node) bool {
	return func(node ast.Node) bool {
		switch decl := node.(type) {
		case *ast.ImportSpec:

		case *ast.FuncDecl:
			// TODO: should pull this out for the first round parsing, or we might not be able to find the receiver name
			return g.registerReceiverNameOfType(decl)

		case *ast.GenDecl:
			if decl.Tok != token.TYPE {
				// We only care about const declarations.
				return true
			}

			// find the struct type
			for _, spec := range decl.Specs {
				// see if the statement is declaring a type
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					// if not skip
					return true
				}

				// if the type name does not match, we should skip
				if typeSpec.Name.Name != typeName {
					return true
				}

				// if the matched type is not a struct type, we should skip
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					log.Errorf("type %s is not a StructType", typeName)

					// stop here
					return false
				}

				g.parseStruct(file.file, typeSpec, structType)
			}

		default:
			return true
		}

		return true
	}

}

func (g *Generator) generate(typeName string) {
	// collect the fields and types
	for _, file := range g.pkg.files {
		// Set the state for this run of the walker.
		if file.file == nil {
			continue
		}

		ast.Inspect(file.file, g.nodeParser(typeName, file))
	}

	if len(g.fields) == 0 {
		return
	}

	// conf := types.Config{Importer: importer.Default()}

	var usedImports = map[string]*types.Package{}

	var usedPkgNames []string
	for n := range g.importPackages {
		usedPkgNames = append(usedPkgNames, n)
	}

	usedPkg, err := parsePackage(usedPkgNames, nil)
	if err != nil {
		log.WithError(err).Errorf("parse package error")
		return
	}
	for _, pkg := range usedPkg {
		usedImports[pkg.Name] = pkg.Types
	}

	pkgTypes := g.pkg.pkg.Types
	qf := func(other *types.Package) string {

		log.Debugf("solving:%s current:%s", other.Path(), pkgTypes.Path())
		if pkgTypes == other {
			return "" // same package; unqualified
		}

		// solve imports
		for _, ip := range pkgTypes.Imports() {
			if other == ip {
				usedImports[ip.Name()] = ip
				return ip.Name()
			}
		}

		return other.Path()
	}

	// scan imports in the first run and use the qualifer to register the imports
	for _, field := range g.fields {
		// reference the types that we will use in our template
		types.TypeString(field.ArgType, qf)
	}

	type TemplateArgs struct {
		Field     Field
		Qualifier types.Qualifier
	}

	var funcMap = templateFuncs(qf)
	var setterFuncTemplate = template.Must(
		template.New("accessor").Funcs(funcMap).Parse(`
func ({{- .Field.ReceiverName }} *{{ .Field.StructName -}}) {{ .Field.SetterName }}( {{- .Field.Name }} {{ typeString .Field.ArgType -}} ) *{{ .Field.StructName -}} {
	{{ .Field.ReceiverName }}.{{ .Field.Name }} = {{ if .Field.Optional -}} & {{- end -}} {{ .Field.Name }}
	return {{ .Field.ReceiverName }}
}
`))

	if len(usedImports) > 0 {
		g.Printf("import (")
		g.Newline()
		for _, importedPkg := range usedImports {
			g.Printf("\t%q", importedPkg.Path())
			g.Newline()
		}
		g.Printf(")")
		g.Newline()
	}

	for _, field := range g.fields {
		err := setterFuncTemplate.Execute(&g.buf, TemplateArgs{
			Field:     field,
			Qualifier: qf,
		})
		if err != nil {
			log.Fatal(err)
		}
	}

	var parameterFuncTemplate *template.Template

	parameterFuncTemplate = template.Must(
		template.New("parameters").Funcs(funcMap).Parse(`
{{ $recv := .FirstField.ReceiverName }}
{{ $structType := .FirstField.StructName }}

{{- define "check-required" }}
{{- if .Required }}
	{{- if .IsString }}
	if len({{ .Name }}) == 0 {
		 return params, fmt.Errorf("{{ .JsonKey }} is required, empty string given")
	}
	{{- else if .IsInt }}
	if {{ .Name }} == 0 {
		 return params, fmt.Errorf("{{ .JsonKey }} is required, 0 given")
	}
	{{- end }}
{{- end }}
{{- end }}

{{- define "check-valid-values" }}
	{{- if .ValidValues }}
	switch {{ .Name }} {
		case {{ toGoTupleString .ValidValues }}:
			params[ "{{- .JsonKey -}}" ] = {{ .Name }}

		default:
			return params, fmt.Errorf("{{ .JsonKey }} value %v is invalid", {{ .Name }})

	}
	{{- end }}
{{- end }}

{{- define "assign" }}
	// assign parameter of {{ .Name }}
{{- if and .IsTime .IsMillisecondsTime }}
	// convert time.Time to milliseconds time
	params[ "{{- .JsonKey -}}" ] = strconv.FormatInt({{ .Name }}.UnixNano()/int64(time.Millisecond), 10)
{{- else }}
	params[ "{{- .JsonKey -}}" ] = {{ .Name }}
{{- end -}}
{{- end }}

{{- define "assign-default" }}
	// assign default of {{ .Name }}
	{{- if eq .DefaultValuer "now()" }}
	{{ .Name }} := time.Now()
	{{ template "assign" . }}
	{{- else if eq .DefaultValuer "uuid()" }}
	{{ .Name }} := uuid.New().String()
	{{ template "assign" . }}
	{{- end }}
{{- end }}

func ({{- $recv }} * {{- $structType -}} ) GetParameters() (map[string]interface{}, error) {
	var params = map[string]interface{}{}

{{- range .Fields }}

	// check {{ .Name }} field -> json key {{ .JsonKey }}
{{- if .Optional }}
	if {{ $recv }}.{{ .Name }} != nil {
		{{ .Name }} := *{{- $recv }}.{{ .Name }}

		{{ template "check-required" . }}

		{{ template "check-valid-values" . }}

		{{ template "assign" . }}
	} {{- if .DefaultValuer }} else {
		{{ template "assign-default" . }}
	} {{- end }}
{{- else }}
	{{ .Name }} := {{- $recv }}.{{ .Name }}

	{{ template "check-required" . }}

	{{ template "check-valid-values" . }}

	{{ template "assign" . }}
{{- end }}

{{- end }}

	return params, nil
}

func ({{- $recv }} * {{- $structType -}} ) GetParametersQuery() (url.Values, error) {
	query := url.Values{}

	params, err := {{ $recv }}.GetParameters()
	if err != nil {
		return query, err
	}

	for k, v := range params {
		query.Add(k, fmt.Sprintf("%v", v))
	}

	return query, nil
}

func ({{- $recv }} * {{- $structType -}} ) GetParametersJSON() ([]byte, error) {
	params, err := {{ $recv }}.GetParameters()
	if err != nil {
		return nil, err
	}

	return json.Marshal(params)
}

`))

	err = parameterFuncTemplate.Execute(&g.buf, struct {
		FirstField Field
		Fields     []Field
		Qualifier  types.Qualifier
	}{
		FirstField: g.fields[0],
		Fields:     g.fields,
		Qualifier:  qf,
	})
	if err != nil {
		log.Fatal(err)
	}

}

func main() {
	flag.Parse()
	if len(*typeNamesStr) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	if *debug {
		log.SetLevel(log.DebugLevel)
	}

	typeNames := strings.Split(*typeNamesStr, ",")
	var tags []string
	if len(*buildTags) > 0 {
		tags = strings.Split(*buildTags, ",")
	}

	// We accept either one directory or a list of files. Which do we have?
	args := flag.Args()
	if len(args) == 0 {
		// Default: process whole package in current directory.
		args = []string{"."}
	}

	// Parse the package once.
	var dir string

	// TODO(suzmue): accept other patterns for packages (directories, list of files, import paths, etc).
	if len(args) == 1 && isDirectory(args[0]) {
		dir = args[0]
	} else {
		if len(tags) != 0 {
			log.Fatal("-tags option applies only to directories, not when files are specified")
		}
		dir = filepath.Dir(args[0])
	}

	g := Generator{
		structTypeReceiverNames: map[string]string{},
		importPackages:          map[string]struct{}{
			"fmt": {},
			"net/url": {},
			"encoding/json": {},
		},
	}

	pkgs, err := parsePackage(args, tags)
	if err != nil {
		log.Fatal(err)
	}

	if len(pkgs) != 1 {
		log.Fatalf("error: %d packages found", len(pkgs))
	}

	g.addPackage(pkgs[0])

	g.Printf("// Code generated by \"requestgen %s\"; DO NOT EDIT.\n", strings.Join(os.Args[1:], " "))
	g.Newline()
	g.Newline()
	g.Printf("package %s", g.pkg.name)
	g.Newline()
	g.Newline()

	for _, typeName := range typeNames {
		g.generate(typeName)
	}

	// Format the output.
	src := formatBuffer(g.buf)

	if *outputStdout {
		_, err = fmt.Fprint(os.Stdout, string(src))
	} else {
		// Write to file.
		outputName := *output
		if outputName == "" {
			ss := camelcase.Split(typeNames[0])
			fn := strings.Join(ss, "_")
			baseName := fmt.Sprintf("%s_accessors.go", fn)
			outputName = filepath.Join(dir, strings.ToLower(baseName))
		}
		err = ioutil.WriteFile(outputName, src, 0644)
	}

	if err != nil {
		log.Fatalf("writing output: %s", err)
	}
}

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

func templateFuncs(qf types.Qualifier) template.FuncMap {
	return template.FuncMap{
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
		"typeString": func(a types.Type) interface{} {
			return types.TypeString(a, qf)
		},
	}
}

func formatBuffer(buf bytes.Buffer) []byte {
	src, err := format.Source(buf.Bytes())
	if err != nil {
		// Should never happen, but can arise when developing this code.
		// The user can compile the output to see the error.
		log.Printf("warning: internal error: invalid Go generated: %s", err)
		log.Printf("warning: compile the package to analyze the error")
		return buf.Bytes()
	}
	return src
}

func parsePackage(patterns []string, tags []string) ([]*packages.Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedTypes | packages.NeedTypesSizes |
			packages.NeedTypesInfo | packages.NeedFiles |
			packages.NeedSyntax | packages.NeedTypesInfo,
		// TODO: Need to think about constants in test files. Maybe write type_string_test.go
		// in a separate pass? For later.
		Tests:      false,
		BuildFlags: []string{fmt.Sprintf("-tags=%s", strings.Join(tags, " "))},
	}

	return packages.Load(cfg, patterns...)
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
		log.Debugf("%s %+v underlying -> basic: %+v info: %+v kind: %+v", k, a, ua, ua.Info(), ua.Kind())
		switch ua.Kind() {
		case types.String:
		case types.Int:
		case types.Bool:

		}

	case *types.Struct:
		log.Debugf("%s %+v underlying -> struct: %+v numFields: %d", k, a, ua, ua.NumFields())

	default:
		log.Debugf("%s %+v underlying -> default: %+v", k, a, ua)

	}
}
