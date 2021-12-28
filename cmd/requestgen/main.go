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
	debug     = flag.Bool("debug", false, "debug mode")
	buildTags = flag.String("tags", "", "comma-separated list of build tags to apply")

	typeNamesStr = flag.String("type", "", "comma-separated list of type names; must be set")
	apiMethodStr = flag.String("method", "GET", "api method: GET, POST, PUT, DELETE, default to GET")
	apiUrlStr    = flag.String("url", "", "api url endpoint")

	parameterType     = flag.String("parameterType", "map", "the parameter type to build, valid: map or url, default: map")
	responseType      = flag.String("responseType", "interface{}", "the response type for decoding the API response, this type should be defined in the same package. if not given, interface{} will be used")
	responseDataType  = flag.String("responseDataType", "", "the data type in the response. this is used to decode data with the response wrapper")
	responseDataField = flag.String("responseDataField", "", "the field name of the inner data of the response type")

	outputStdout = flag.Bool("stdout", false, "output generated content to the stdout")
	output       = flag.String("output", "", "output file name; default srcdir/<type>_string.go")
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

	IsMillisecondsTime, IsSecondsTime bool

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

	// apiClientField if the request defined the client field with APIClient,
	// it means we can generate the Do() method
	apiClientField *string
	structType     types.Type
	receiverName   string

	// the collected fields
	// fields is for post body
	fields []Field

	// queryFields means query string
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

func (g *Generator) newline() {
	fmt.Fprint(&g.buf, "\n")
}

func (g *Generator) printf(format string, args ...interface{}) {
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
		return false
	}

	// use ident to look up type
	// typeDef := g.pkg.pkg.TypesInfo.Defs[receiver.Names[0]]

	// there are 2 types of receiver type value (named type or pointer type)
	// here we record the type name -> receiver name mapping
	switch receiverType := receiverTypeValue.Type.(type) {
	case *types.Named:
		g.structTypeReceiverNames[receiverType.String()] = receiver.Names[0].String()

	case *types.Pointer:
		g.structTypeReceiverNames[receiverType.Elem().String()] = receiver.Names[0].String()
	}

	return false
}

func (g *Generator) checkClientInterface(field *ast.Field) {
	typeValue, ok := g.pkg.pkg.TypesInfo.Types[field.Type]
	if !ok {
		return
	}

	// github.com/c9s/requestgen.APIClient
	if typeValue.Type.String() == "github.com/c9s/requestgen.APIClient" {
		log.Debugf("found APIClient field %v -> %+v", field.Names, typeValue.Type.String())

		g.apiClientField = &field.Names[0].Name
		g.importPackages["context"] = struct{}{}
	}
}

func (g *Generator) parseStruct(file *ast.File, typeSpec *ast.TypeSpec, structType *ast.StructType) {
	typeDef := g.pkg.pkg.TypesInfo.Defs[typeSpec.Name]
	fullTypeName := typeDef.Type().String()

	// structTV := g.pkg.pkg.TypesInfo.Types[structType]

	g.structType = typeDef.Type()

	receiverName, ok := g.structTypeReceiverNames[fullTypeName]
	if !ok {
		// use default
		receiverName = strings.ToLower(string(typeSpec.Name.String()[0]))
	}
	g.receiverName = receiverName

	// iterate the field list (by syntax)
	for _, field := range structType.Fields.List {
		// each struct field AST could have multiple names in one line
		if len(field.Names) > 1 {
			continue
		}

		g.checkClientInterface(field)

		if field.Tag == nil {
			continue
		}

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
		isSecondsTime := paramTag.HasOption("seconds")
		isQuery := paramTag.HasOption("query")

		if isTime {
			g.importPackages["time"] = struct{}{}
			if isMillisecondsTime || isSecondsTime {
				g.importPackages["strconv"] = struct{}{}
			}
		}

		if !isTime && (isMillisecondsTime || isSecondsTime) {
			log.Errorf("milliseconds/seconds option is not valid for non time.Time type field")
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

		validValues, err := parseValidValuesTag(tags, fieldName, argKind)
		if err != nil {
			return
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
			IsSecondsTime:      isSecondsTime,
			JsonKey:            jsonKey,
			Optional:           optional,
			Required:           required,
			ValidValues:        validValues,
			DefaultValuer:      defaultValuer,

			File: file,
		}

		// query parameters
		if isQuery {
			g.queryFields = append(g.queryFields, f)
		} else {
			g.fields = append(g.fields, f)
		}
	}
}

func parseValidValuesTag(tags *structtag.Tags, fieldName string, argKind types.BasicKind) (interface{}, error) {
	validValuesTag, _ := tags.Get("validValues")
	if validValuesTag == nil {
		return nil, nil
	}

	var validValues interface{}
	validValueList := strings.Split(validValuesTag.Value(), ",")

	log.Debugf("%s found valid values: %v", fieldName, validValueList)

	switch argKind {
	case types.Int, types.Int64, types.Int32:
		var slice []int
		for _, s := range validValueList {
			i, err := strconv.Atoi(s)
			if err != nil {
				return nil, err
			}

			slice = append(slice, i)
		}

	case types.String:
		validValues = validValueList

	}

	return validValues, nil
}

func (g *Generator) receiverNameWalker(typeName string, file *File) func(ast.Node) bool {
	return func(node ast.Node) bool {
		switch decl := node.(type) {
		case *ast.FuncDecl:
			// TODO: should pull this out for the first round parsing, or we might not be able to find the receiver name
			return g.registerReceiverNameOfType(decl)
		}

		return true
	}
}

// requestStructWalker returns an ast node iterator function for iterating the ast nodes
func (g *Generator) requestStructWalker(typeName string, file *File) func(ast.Node) bool {
	return func(node ast.Node) bool {
		switch decl := node.(type) {
		case *ast.ImportSpec:

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
		if file.file == nil {
			continue
		}
		ast.Inspect(file.file, g.receiverNameWalker(typeName, file))
	}

	for _, file := range g.pkg.files {
		if file.file == nil {
			continue
		}
		ast.Inspect(file.file, g.requestStructWalker(typeName, file))
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

	var funcMap = templateFuncs(qf)

	if len(usedImports) > 0 {
		g.printf("import (")
		g.newline()
		for _, importedPkg := range usedImports {
			g.printf("\t%q", importedPkg.Path())
			g.newline()
		}
		g.printf(")")
		g.newline()
	}

	if err := g.generateSetters(funcMap, qf); err != nil {
		log.Fatal(err)
	}

	if err := g.generateParameterMethods(funcMap, qf); err != nil {
		log.Fatal(err)
	}

	if g.apiClientField != nil && *apiUrlStr != "" {
		if err := g.generateDoMethod(funcMap); err != nil {
			log.Fatal(err)
		}
	}
}

func (g *Generator) generateDoMethod(funcMap template.FuncMap) error {
	var doFuncTemplate = template.Must(
		template.New("do").Funcs(funcMap).Parse(`
func ({{- .ReceiverName }} * {{- typeString .StructType -}}) Do(ctx context.Context) (
{{- if and .ResponseDataType .ResponseDataField -}}
	{{ typeReference .ResponseDataType }}
{{- else -}}
	{{ typeReference .ResponseTypeName }}
{{- end -}}
	,error) {
	{{ $recv := .ReceiverName }}

{{- if ne .ApiMethod "GET" }}
	params, err := {{ $recv }}.GetParameters()
	if err != nil {
		return nil, err
	}
{{- else }}
  // empty params for GET operation
	var params interface{}
{{- end }}

{{- if .HasQueryParameters }}
	query, err := {{ $recv }}.GetQueryParameters()
	if err != nil {
		return nil, err
	}
{{- else }}
  query := url.Values{}
{{- end }}

	req, err := {{ $recv }}.{{ .ApiClientField }}.NewRequest("{{ .ApiMethod }}", "{{ .ApiUrl }}", query, params)
	if err != nil {
		return nil, err
	}

	response, err := {{ $recv }}.{{ .ApiClientField }}.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse {{ .ResponseTypeName }}
	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}

{{- if and .ResponseDataType .ResponseDataField }}
	var data {{ .ResponseDataType }}
	if err := json.Unmarshal(apiResponse.{{ .ResponseDataField }}, &data) ; err != nil {
		return nil, err
	}
	return &data, nil
{{- else }}
	return &apiResponse, nil
{{- end }}
}
`))
	err := doFuncTemplate.Execute(&g.buf, struct {
		StructType         types.Type
		ReceiverName       string
		ApiClientField     string
		ApiMethod          string
		ApiUrl             string
		ResponseTypeName   string
		ResponseDataType   string
		ResponseDataField  string
		HasQueryParameters bool
	}{
		StructType:         g.structType,
		ReceiverName:       g.receiverName,
		ApiClientField:     *g.apiClientField,
		ApiMethod:          *apiMethodStr,
		ApiUrl:             *apiUrlStr,
		ResponseTypeName:   *responseType,
		ResponseDataType:   *responseDataType,
		ResponseDataField:  *responseDataField,
		HasQueryParameters: len(g.queryFields) > 0,
	})

	return err
}

func (g *Generator) generateParameterMethods(funcMap template.FuncMap, qf func(other *types.Package) string) error {
	var err error
	var parameterFuncTemplate *template.Template
	parameterFuncTemplate = template.Must(
		template.New("parameters").Funcs(funcMap).Parse(`
{{ $recv := .ReceiverName }}

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
	// convert time.Time to milliseconds time stamp
	params[ "{{- .JsonKey -}}" ] = strconv.FormatInt({{ .Name }}.UnixNano()/int64(time.Millisecond), 10)
{{- else if and .IsTime .IsSecondsTime }}
	// convert time.Time to seconds time stamp
	params[ "{{- .JsonKey -}}" ] = strconv.FormatInt({{ .Name }}.Unix(), 10)
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

// GetQueryParameters builds and checks the query parameters and returns url.Values
func ({{- $recv }} * {{- typeString .StructType -}} ) GetQueryParameters() (url.Values, error) {
	var params = map[string]interface{}{}

{{- range .QueryFields }}
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

	query := url.Values{}
	for k, v := range params {
		query.Add(k, fmt.Sprintf("%v", v))
	}

	return query, nil
}

// GetParameters builds and checks the parameters and return the result in a map object
func ({{- $recv }} * {{- typeString .StructType -}} ) GetParameters() (map[string]interface{}, error) {
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

// GetParametersQuery converts the parameters from GetParameters into the url.Values format
func ({{- $recv }} * {{- typeString .StructType -}} ) GetParametersQuery() (url.Values, error) {
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

// GetParametersJSON converts the parameters from GetParameters into the JSON format
func ({{- $recv }} *{{ typeString .StructType -}} ) GetParametersJSON() ([]byte, error) {
	params, err := {{ $recv }}.GetParameters()
	if err != nil {
		return nil, err
	}

	return json.Marshal(params)
}

`))

	if len(g.fields) > 0 {
		err = parameterFuncTemplate.Execute(&g.buf, struct {
			StructType   types.Type
			ReceiverName string
			Fields       []Field
			QueryFields  []Field
			Qualifier    types.Qualifier
		}{
			StructType:   g.structType,
			ReceiverName: g.receiverName,
			Fields:       g.fields,
			QueryFields:  g.queryFields,
			Qualifier:    qf,
		})
		if err != nil {
			return err
		}
	}

	return err
}

func (g *Generator) generateSetters(funcMap template.FuncMap, qf func(other *types.Package) string) error {
	type accessorTemplateArgs struct {
		StructType   types.Type
		ReceiverName string
		Field        Field
		Qualifier    types.Qualifier
	}

	var setterFuncTemplate = template.Must(
		template.New("accessor").Funcs(funcMap).Parse(`
func ({{- .ReceiverName }} * {{- typeString .StructType -}} ) {{ .Field.SetterName }}( {{- .Field.Name }} {{ typeString .Field.ArgType -}} ) * {{- typeString .StructType }} {
	{{ .ReceiverName }}.{{ .Field.Name }} = {{ if .Field.Optional -}} & {{- end -}} {{ .Field.Name }}
	return {{ .ReceiverName }}
}
`))
	for _, field := range g.queryFields {
		err := setterFuncTemplate.Execute(&g.buf, accessorTemplateArgs{
			Field:        field,
			Qualifier:    qf,
			StructType:   g.structType,
			ReceiverName: g.receiverName,
		})
		if err != nil {
			return err
		}
	}

	for _, field := range g.fields {
		err := setterFuncTemplate.Execute(&g.buf, accessorTemplateArgs{
			Field:        field,
			Qualifier:    qf,
			StructType:   g.structType,
			ReceiverName: g.receiverName,
		})
		if err != nil {
			return err
		}
	}

	return nil
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
		importPackages: map[string]struct{}{
			"fmt":           {},
			"net/url":       {},
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

	g.printf("// Code generated by \"requestgen %s\"; DO NOT EDIT.\n", strings.Join(os.Args[1:], " "))
	g.newline()
	g.newline()
	g.printf("package %s", g.pkg.name)
	g.newline()
	g.newline()

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

func parsePackage(patterns []string, tags []string) ([]*packages.Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedTypes | packages.NeedTypesSizes |
			packages.NeedSyntax | packages.NeedTypesInfo |
			packages.NeedDeps,
		// TODO: Need to think about constants in test files. Maybe write type_string_test.go
		// in a separate pass? For later.
		Tests:      false,
		BuildFlags: []string{fmt.Sprintf("-tags=%s", strings.Join(tags, " "))},
	}

	return packages.Load(cfg, patterns...)
}
