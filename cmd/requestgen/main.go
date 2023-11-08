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
	"time"

	"github.com/fatih/camelcase"
	"github.com/fatih/structtag"
	log "github.com/sirupsen/logrus"

	"golang.org/x/tools/go/packages"

	"github.com/c9s/requestgen"
)

var (
	debug     = flag.Bool("debug", false, "debug mode")
	buildTags = flag.String("tags", "", "comma-separated list of build tags to apply")

	typeNamesStr   = flag.String("type", "", "comma-separated list of type names; must be set")
	apiMethodStr   = flag.String("method", "GET", "api method: GET, POST, PUT, DELETE, default to GET")
	apiUrlStr      = flag.String("url", "", "api url endpoint")
	useDynamicPath = flag.Bool("dynamicPath", false, "enable dynamic API path")

	parameterType       = flag.String("parameterType", "map", "the parameter type to build, valid: map or url, default: map")
	responseTypeSel     = flag.String("responseType", "interface{}", "the response type for decoding the API response, this type should be defined in the same package. if not given, interface{} will be used")
	responseDataTypeSel = flag.String("responseDataType", "", "the data type in the response. this is used to decode data with the response wrapper")
	responseDataField   = flag.String("responseDataField", "", "the field name of the inner data of the response type")

	outputStdout = flag.Bool("stdout", false, "output generated content to the stdout")
	output       = flag.String("output", "", "output file name; default srcdir/<type>_string.go")
)

var outputSuffix = "_requestgen.go"

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

type Generator struct {
	buf bytes.Buffer // Accumulated output.

	// structTypeReceiverNames is used for collecting the receiver name of the given struct types
	structTypeReceiverNames map[string]string

	// TODO: clean up the package structure it's redundant
	pkg            *Package // Package we are scanning.
	currentPackage *packages.Package
	importPackages map[string]struct{}

	responseType, responseDataType types.Type

	// apiClientField if the request defined the client field with APIClient,
	// it means we can generate the Do() method
	apiClientField         *string
	authenticatedApiClient bool
	structType             types.Type
	receiverName           string

	// the collected fields
	// fields is for post body
	fields []Field

	// queryFields means query string
	queryFields []Field

	slugs []Field

	simpleTypes          map[string]string
	simpleTypeValueNames map[string][]Literal
	stringTypeValues     map[string][]string
	intTypeValues        map[string][]int64
}

func (g *Generator) importPackage(pkg string) {
	log.Debugf("add package import: %s", pkg)
	g.importPackages[pkg] = struct{}{}
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

	if len(receiver.Names) == 0 {
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
		log.Debugf("found APIClient field %v -> %+v", field.Names[0], typeValue.Type.String())
		g.apiClientField = &field.Names[0].Name
	} else if typeValue.Type.String() == "github.com/c9s/requestgen.AuthenticatedAPIClient" {
		log.Debugf("found AuthenticatedAPIClient field %v -> %+v", field.Names[0], typeValue.Type.String())
		g.apiClientField = &field.Names[0].Name
		g.authenticatedApiClient = true
	}
}

func (g *Generator) parseStructFields(file *ast.File, typeSpec *ast.TypeSpec, structType *ast.StructType) {
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
		isSlug := paramTag.HasOption("slug")

		if isTime {
			g.importPackage("time")
			if isMillisecondsTime || isSecondsTime {
				g.importPackage("strconv")
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
				g.importPackage("time")
			case "uuid()":
				g.importPackage("github.com/google/uuid")
			default:
				log.Errorf("invalid default valuer: %v", defaultValuer)
				return
			}
		}

		var timeFormat string
		timeFormatTag, _ := tags.Get("timeFormat")
		if timeFormatTag != nil {
			timeFormat = timeFormatTag.Value()
			switch timeFormat {
			case "ANSIC":
				timeFormat = time.ANSIC
			case "RFC1123":
				timeFormat = time.RFC1123
			case "RFC3339":
				timeFormat = time.RFC3339
			case "RFC3339Nano":
				timeFormat = time.RFC3339Nano
			case "RFC850":
				timeFormat = time.RFC850
			case "RFC822":
				timeFormat = time.RFC822
			case "RubyDate":
				timeFormat = time.RubyDate
			}
		}

		fieldName := field.Names[0].Name
		debugUnderlying(fieldName, argType)

		validValues, err := parseValidValuesTag(tags, fieldName, argKind)
		if err != nil {
			return
		} else if validValues == nil {
			if values, ok := g.simpleTypeValueNames[argType.String()]; ok {
				validValues = values
			} else {
				// use built-in validator for simple types
				if isTypeString(argType) {
					if values, ok := g.stringTypeValues[argType.String()]; ok {
						validValues = values
					}
				} else if isTypeInt(argType) {
					if values, ok := g.intTypeValues[argType.String()]; ok {
						validValues = values
					}
				}
			}
		}

		defaultValue, err := parseDefaultTag(tags, fieldName, argKind)
		if err != nil {
			return
		}

		f := Field{
			Name:               field.Names[0].Name,
			Type:               typeValue.Type,
			IsSlug:             isSlug,
			ArgType:            argType,
			SetterName:         setterName,
			IsString:           isString,
			IsInt:              isInt,
			IsTime:             isTime,
			IsMillisecondsTime: isMillisecondsTime,
			IsSecondsTime:      isSecondsTime,
			TimeFormat:         timeFormat,
			JsonKey:            jsonKey,
			Optional:           optional,
			Required:           required,
			ValidValues:        validValues,
			Default:            defaultValue,
			DefaultValuer:      defaultValuer,

			File: file,
		}

		// query parameters
		if isSlug {
			g.slugs = append(g.slugs, f)
		} else if isQuery {
			g.queryFields = append(g.queryFields, f)
		} else {
			g.fields = append(g.fields, f)
		}
	}
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

func isIdent(expr ast.Expr) (string, bool) {
	switch dt := expr.(type) {
	case *ast.Ident:
		return dt.Name, true
	}

	return "", false
}

func (g *Generator) stringTypesCollectorWalker(typeName string, file *File) func(ast.Node) bool {
	return func(node ast.Node) bool {
		switch decl := node.(type) {
		case *ast.TypeSpec:
			log.Debugf("TypeSpec: name %s type: %+v\n", decl.Name.String(), decl.Type)
			if n, ok := isIdent(decl.Type); ok {
				switch n {
				case "string", "int", "int8", "int16", "int32", "int64":
					g.simpleTypes[decl.Name.String()] = n
					log.Debugf("simple type %s = %s", decl.Name.String(), n)
				}
			}

		case *ast.ValueSpec:
			log.Debugf("ValueSpec: parsing type %+v names: %+v values: %+v\n", decl.Type, decl.Names, decl.Values)
			typeValue, ok := g.pkg.pkg.TypesInfo.Types[decl.Type]
			if !ok {
				log.Debugf("types info %v (%v) not found", decl.Names, decl.Type)
				return false
			}

			fullQualifiedTypeName := typeValue.Type.String()
			for _, n := range decl.Names {
				g.simpleTypeValueNames[fullQualifiedTypeName] = append(g.simpleTypeValueNames[fullQualifiedTypeName], Literal(n.String()))
			}
			log.Debugf("simpleTypeValueNames %s = %+v", fullQualifiedTypeName, g.simpleTypeValueNames[fullQualifiedTypeName])

			if isTypeString(typeValue.Type) {
				for _, v := range decl.Values {
					if basic, ok := v.(*ast.BasicLit); ok {
						g.stringTypeValues[fullQualifiedTypeName] = append(g.stringTypeValues[fullQualifiedTypeName], basic.Value)
					}
				}
				log.Debugf("ValueSpec: type %+v values: %s %+v", decl.Type, fullQualifiedTypeName, g.stringTypeValues[fullQualifiedTypeName])
			} else if isTypeInt(typeValue.Type) {
				for _, v := range decl.Values {
					if basic, ok := v.(*ast.BasicLit); ok {
						ii, err := strconv.ParseInt(basic.Value, 10, 64)
						if err != nil {
							log.WithError(err).Errorf("can not parse int %s", basic.Value)
						} else {
							g.intTypeValues[fullQualifiedTypeName] = append(g.intTypeValues[fullQualifiedTypeName], ii)
						}
					}
				}
				log.Debugf("ValueSpec: type %+v values: %s %+v", decl.Type, fullQualifiedTypeName, g.intTypeValues[fullQualifiedTypeName])
			}

		case *ast.FuncDecl, *ast.FuncType:
			// ignore function blocks
			return false
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

				g.parseStructFields(file.file, typeSpec, structType)
			}

		default:
			return true
		}

		return true
	}
}

type Profile struct {
	task      string
	startTime time.Time
}

func (p *Profile) stop() {
	du := time.Now().Sub(p.startTime)
	log.Debugf("profile: %s: %s", p.task, du)
}

func newProfile(task string) *Profile {
	return &Profile{task: task, startTime: time.Now()}
}

func profile(task string, f func()) {
	p := &Profile{task: task, startTime: time.Now()}
	f()
	p.stop()
}

func (g *Generator) generate(typeName string) {
	p := newProfile(fmt.Sprintf("generate: %v", typeName))
	defer p.stop()

	// collect the fields and types
	profile("astTraverse", func() {
		for _, file := range g.pkg.files {
			if file.file == nil {
				continue
			}

			ast.Inspect(file.file, g.receiverNameWalker(typeName, file))
			ast.Inspect(file.file, g.stringTypesCollectorWalker(typeName, file))
		}

		profile("requestStructWalker", func() {
			for _, file := range g.pkg.files {
				if file.file == nil {
					continue
				}
				ast.Inspect(file.file, g.requestStructWalker(typeName, file))
			}
		})
	})

	// conf := types.Config{Importer: importer.Default()}

	var usedImports = map[string]*types.Package{}

	g.importPackage("fmt")
	g.importPackage("net/url")
	g.importPackage("encoding/json")
	g.importPackage("regexp")
	g.importPackage("reflect")

	if g.apiClientField != nil && (*apiUrlStr != "" || *useDynamicPath) {
		g.importPackage("net/url")
		g.importPackage("context")

		if *responseDataField != "" && g.responseDataType != nil {
			// json is used for unmarshalling the response data
			g.importPackage("encoding/json")
		}
	}

	var usedPkgNames []string
	for n := range g.importPackages {
		usedPkgNames = append(usedPkgNames, n)
	}

	if len(usedPkgNames) > 0 {
		usedPkg, err := loadPackages(usedPkgNames, nil)
		if err != nil {
			log.WithError(err).Errorf("parse package error")
			return
		}

		for _, pkg := range usedPkg {
			usedImports[pkg.Name] = pkg.Types
		}
	}

	qf := func(other *types.Package) string {
		var pkgTypes = g.pkg.pkg.Types
		var log = log.WithField("template-function", "qualifier")
		if pkgTypes == other {
			log.Debugf("importing %s from %s: same package object (pointer), no import", other.Path(), pkgTypes.Path())
			return "" // same package; unqualified
		}

		if other.Path() == g.currentPackage.PkgPath {
			log.Debugf("importing %s from %s: same package path, no import", other.Path(), pkgTypes.Path())
			return ""
		}

		// solve imports
		for _, ip := range pkgTypes.Imports() {
			log.Debugf("checking import %+v == other(%+v)", ip, other)
			// XXX: check Name() too?
			if other.Path() == ip.Path() {
				log.Debugf("importing %s from %s: found imported %s", other.Path(), pkgTypes.Path(), ip)
				usedImports[ip.Name()] = ip
				return ip.Name()
			}
		}

		log.Warnf("importing %s from %s, import not found, using %s", other.Path(), pkgTypes.Path(), other.Name())
		return other.Name()
	}

	// scan imports in the first run and use the qualifier to register the imports
	for _, field := range g.fields {
		// reference the types that we will use in our template
		types.TypeString(field.ArgType, qf)
	}
	types.TypeString(g.responseType, qf)
	types.TypeString(g.responseDataType, qf)

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

	log.Debugf("apiClientField: %v apiUrl: %v", g.apiClientField, apiUrlStr)
	if g.apiClientField != nil && (*apiUrlStr != "" || *useDynamicPath) {
		if err := g.generateDoMethod(funcMap); err != nil {
			log.Fatal(err)
		}
	}
}

func (g *Generator) generateDoMethod(funcMap template.FuncMap) error {
	var doFuncTemplate = template.Must(
		template.New("do").Funcs(funcMap).Parse(`
{{ $recv := .ReceiverName }}

// GetPath returns the request path of the API
func ({{- .ReceiverName }} * {{- typeString .StructType -}}) GetPath() string {
	return "{{ .ApiUrl }}"
}

// Do generates the request object and send the request object to the API endpoint
func ({{- .ReceiverName }} * {{- typeString .StructType -}}) Do(ctx context.Context) (
{{- if and .ResponseDataType .ResponseDataField -}}
	{{ typeString (toPointer .ResponseDataType) }}
{{- else -}}
	{{ typeString (toPointer .ResponseType) }}
{{- end -}}
	,error) {
    {{ $requestMethod := "NewRequest" }}
    {{- if .ApiAuthenticated -}}
    {{-    $requestMethod = "NewAuthenticatedRequest" }}
    {{- end -}}

{{- if not .HasParameters }}
    // no body params
	var params interface{}
{{- else if and .HasParameters (ne .ApiMethod "GET") }}
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
{{- else if and .HasParameters (eq .ApiMethod "GET") }}
	query, err := {{ $recv }}.GetParametersQuery()
	if err != nil {
		return nil, err
	}
{{- else }}
  	query := url.Values{}
{{- end }}


	var apiURL string

	{{- if .DynamicPath }}

	type dynamicPathProvider interface {
		GetDynamicPath() (string, error)
	}

	dpp := dynamicPathProvider({{ $recv }})
	if dPath, err := dpp.GetDynamicPath(); err != nil {
		return nil, err
	} else {
		apiURL = dPath
	}

	{{- else }}

	apiURL = {{ $recv }}.GetPath()

	{{- end }}

	{{- if .HasSlugs }}
	slugs, err := {{ $recv }}.GetSlugsMap()
	if err != nil {
		return nil, err
	}

	apiURL = {{ $recv }}.applySlugsToUrl(apiURL, slugs)
	{{- end }}

	req, err := {{ $recv }}.{{ .ApiClientField }}.{{ $requestMethod }}(ctx, "{{ .ApiMethod }}", apiURL, query, params)
	if err != nil {
		return nil, err
	}

	response, err := {{ $recv }}.{{ .ApiClientField }}.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse {{ typeString .ResponseType }}
	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}

	type responseValidator interface {
		Validate() error
	}
	validator, ok := interface{}(apiResponse).(responseValidator)
	if ok {
		if err := validator.Validate(); err != nil {
			return nil, err
		}	
	}

{{- if and .ResponseDataType .ResponseDataField }}
	var data {{ typeString .ResponseDataType }}
	if err := json.Unmarshal(apiResponse.{{ .ResponseDataField }}, &data) ; err != nil {
		return nil, err
	}
	return {{ referenceByType .ResponseDataType -}} data, nil
{{- else }}
	return {{ referenceByType .ResponseType -}} apiResponse, nil
{{- end }}
}
`))
	err := doFuncTemplate.Execute(&g.buf, struct {
		StructType                     types.Type
		ReceiverName                   string
		ApiClientField                 *string
		ApiMethod                      string
		ApiUrl                         string
		DynamicPath                    bool
		ApiAuthenticated               bool
		ResponseType, ResponseDataType types.Type
		ResponseDataField              string
		HasSlugs                       bool
		HasParameters                  bool
		HasQueryParameters             bool
	}{
		StructType:         g.structType,
		ReceiverName:       g.receiverName,
		ApiClientField:     g.apiClientField,
		ApiMethod:          *apiMethodStr,
		ApiUrl:             *apiUrlStr,
		DynamicPath:        *useDynamicPath,
		ApiAuthenticated:   g.authenticatedApiClient,
		ResponseType:       g.responseType,
		ResponseDataType:   g.responseDataType,
		ResponseDataField:  *responseDataField,
		HasSlugs:           len(g.slugs) > 0,
		HasParameters:      len(g.fields) > 0,
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
	// TEMPLATE check-required
	{{- if .IsString }}
	if len({{ .Name }}) == 0 {
		{{- if .Default }}
        {{ .Name }} = {{ .Default | printf "%q" }}
		{{- else }}
		return nil, fmt.Errorf("{{ .JsonKey }} is required, empty string given")
		{{- end }}
	}
	{{- else if .IsInt }}
	if {{ .Name }} == 0 {
		{{- if .Default }}
		{{ .Name }} = {{ .Default }}
		{{- else }}
		return nil, fmt.Errorf("{{ .JsonKey }} is required, 0 given")
		{{- end }}
	}
	{{- end }}
	// END TEMPLATE check-required
{{- end }}
{{- end }}

{{- define "check-valid-values" }}
	{{- if .ValidValues }}
	// TEMPLATE check-valid-values
	switch {{ .Name }} {
		case {{ toGoTupleString .ValidValues }}:
			params[ "{{- .JsonKey -}}" ] = {{ .Name }}

		default:
			return nil, fmt.Errorf("{{ .JsonKey }} value %v is invalid", {{ .Name }})

	}
	// END TEMPLATE check-valid-values
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
{{- else if and .IsTime .TimeFormat }}
	params[ "{{- .JsonKey -}}" ] = {{ .Name }}.Format("{{- .TimeFormat -}}")
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
	{{- template "assign" . }}
	{{- else if .Default }}
		{{- if .IsInt }}
			{{ .Name }} := {{ .Default }}
		{{- else if .IsString }}
			{{ .Name }} := {{ .Default | printf "%q" }}
		{{- end }}
		{{- template "assign" . }}
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
	} else {
		{{- if .DefaultValuer }}
			{{- template "assign-default" . }}
		{{- else if .Default }}
			{{- if .IsInt }}
			{{ .Name }} := {{ .Default }}
			{{- else if .IsString }}
			{{ .Name }} := {{ .Default | printf "%q" }}
			{{- end }}
		    {{ template "assign" . }}
		{{- end }}
	}

{{- else }}
	{{ .Name }} := {{- $recv }}.{{ .Name }}

	{{ template "check-required" . }}

	{{ template "check-valid-values" . }}

	{{ template "assign" . }}
{{- end }}

{{- end }}

	query := url.Values{}
	for _k, _v := range params {
		query.Add(_k, fmt.Sprintf("%v", _v))
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
	} else {
		{{- if .DefaultValuer }}
			{{- template "assign-default" . }}
		{{- else if .Default }}
			{{- if .IsInt }}
			{{ .Name }} := {{ .Default }}
			{{- else if .IsString }}
			{{ .Name }} := {{ .Default | printf "%q" }}
			{{- end }}
			{{ template "assign" . }}
		{{- end }}
	}

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

	for _k, _v := range params {
		if {{ $recv }}.isVarSlice(_v) {
			{{ $recv }}.iterateSlice(_v, func(it interface{}) {
				query.Add(_k + "[]", fmt.Sprintf("%v", it))
			})
		} else {
			query.Add(_k, fmt.Sprintf("%v", _v))
		}
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

// GetSlugParameters builds and checks the slug parameters and return the result in a map object
func ({{- $recv }} * {{- typeString .StructType -}} ) GetSlugParameters() (map[string]interface{}, error) {
	var params = map[string]interface{}{}

{{- range .Slugs }}
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

func ({{- $recv }} * {{- typeString .StructType -}} ) applySlugsToUrl(url string, slugs map[string]string) string {
	for _k, _v := range slugs {
		needleRE := regexp.MustCompile(":" + _k + "\\b")
		url = needleRE.ReplaceAllString(url, _v)
	}

	return url
}

func ({{- $recv }} * {{- typeString .StructType -}} ) iterateSlice(slice interface{}, _f func(it interface{})) {
	sliceValue := reflect.ValueOf(slice)
	for _i := 0; _i < sliceValue.Len(); _i++ {
		it := sliceValue.Index(_i).Interface() 
		_f(it)
	}
}

func ({{- $recv }} * {{- typeString .StructType -}} ) isVarSlice(_v interface{}) bool {
	rt := reflect.TypeOf(_v)
	switch rt.Kind() {
        case reflect.Slice:
			return true
	}
	return false
}


func ({{- $recv }} * {{- typeString .StructType -}} ) GetSlugsMap() (map[string]string, error) {
	slugs := map[string]string{}
	params, err := {{ $recv }}.GetSlugParameters()
	if err != nil {
		return slugs, nil
	}

	for _k, _v := range params {
		slugs[_k] = fmt.Sprintf("%v", _v)
	}

	return slugs, nil
}


`))

	err = parameterFuncTemplate.Execute(&g.buf, struct {
		StructType                 types.Type
		ReceiverName               string
		QueryFields, Fields, Slugs []Field
		Qualifier                  types.Qualifier
	}{
		StructType:   g.structType,
		ReceiverName: g.receiverName,
		Fields:       g.fields,
		QueryFields:  g.queryFields,
		Slugs:        g.slugs,
		Qualifier:    qf,
	})
	if err != nil {
		return err
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

	for _, field := range g.slugs {
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

	log.Debugf("args: %q", os.Args)

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
		importPackages:          map[string]struct{}{},
		simpleTypes:             make(map[string]string),
		simpleTypeValueNames:    make(map[string][]Literal),
		stringTypeValues:        make(map[string][]string),
		intTypeValues:           make(map[string][]int64),
	}

	pkgs, err := loadPackages(args, tags)
	if err != nil {
		log.Fatal(err)
	}

	if len(pkgs) != 1 {
		log.Fatalf("error: %d packages found", len(pkgs))
	}

	g.currentPackage = pkgs[0]
	g.addPackage(pkgs[0])

	// parse response type
	if responseTypeSel != nil && *responseTypeSel != "" {
		if *responseTypeSel == "interface{}" {
			g.responseType = types.NewInterfaceType(nil, nil)
		} else {
			o, ts, err := parseTypeSelector(*responseTypeSel, pkgs)
			if err != nil {
				log.Fatal(err)
			}

			if ts.IsSlice {
				g.responseType = types.NewSlice(o.Type())
			} else {
				g.responseType = o.Type()
			}
			if g.currentPackage.PkgPath != o.Pkg().Path() {
				g.importPackage(o.Pkg().Path())
			}

			log.Debugf("response type selector: %+v", ts)
		}
	}

	// parse response data type
	if responseDataTypeSel != nil && *responseDataTypeSel != "" {
		if *responseDataTypeSel == "interface{}" {
			g.responseDataType = types.NewInterfaceType(nil, nil)
		} else {
			o, ts, err := parseTypeSelector(*responseDataTypeSel, pkgs)
			if err != nil {
				log.Fatal(err)
			}

			if ts.IsSlice {
				g.responseDataType = types.NewSlice(o.Type())
			} else {
				g.responseDataType = o.Type()
			}

			if g.currentPackage.PkgPath != o.Pkg().Path() {
				g.importPackage(o.Pkg().Path())
			}
		}
	}

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
			baseName := fmt.Sprintf("%s%s", fn, outputSuffix)
			outputName = filepath.Join(dir, strings.ToLower(baseName))
		}
		err = ioutil.WriteFile(outputName, src, 0644)
	}

	if err != nil {
		log.Fatalf("writing output: %s", err)
	}
}

func locateObject(ts *requestgen.TypeSelector, selectedPkgs []*packages.Package) (types.Object, error) {
	log.Debugf("locating object: %#v", ts)

	var packages []*packages.Package
	if len(selectedPkgs) > 0 && (ts.Package == "." || ts.Package == selectedPkgs[0].PkgPath) {
		packages = selectedPkgs
	} else {
		var err error
		packages, err = loadPackages([]string{ts.Package}, []string{})
		if err != nil {
			return nil, err
		}
	}

	if len(packages) == 0 {
		return nil, fmt.Errorf("failed to load pacakges via the given type selector %+v", ts)
	}

	log.Debugf("loaded %d packages", len(packages))

	for _, pkg := range packages {
		log.Debugf("package %s (%s): %d defs", pkg.Name, pkg.PkgPath, len(pkg.TypesInfo.Defs))

		for ident, obj := range pkg.TypesInfo.Defs {
			log.Debugf("comparing ident %s <=> %s", ident.Name, ts.Member)

			if ident.Name != ts.Member {
				continue
			}

			log.Debugf("ident %s matches type selector member %s", ident.Name, ts.Member)

			log.Debugf("comparing package path %v == %v", obj.Pkg().Path(), ts.Package)
			if obj.Pkg().Path() == ts.Package {
				log.Debugf("package path matched")

				switch t := obj.Type().(type) {

				case *types.Named:
					log.Debugf("found named type: %+v", t)
					log.Debugf("found response type def: %+v -> %+v type:%+v import:%s", ident.Name, obj, obj.Type(), obj.Pkg().Path())
					return obj, nil

				case *types.Struct:
					log.Infof("found struct type: %+v", t)
					log.Debugf("found response type def: %+v -> %+v type:%+v import:%s", ident.Name, obj, obj.Type(), obj.Pkg().Path())
					return obj, nil

				default:
					log.Warnf("object type %T of %v is unexpected", t, t)
					continue
					// return nil, fmt.Errorf("can not parse type selector %v, unexpected type: %T %+v", ts, t, t)
				}
			}
		}

	}

	return nil, fmt.Errorf("can not find type matches the type selector %+v in the packages %+v", ts, packages)
}

func parseTypeSelector(sel string, selectedPkgs []*packages.Package) (types.Object, *requestgen.TypeSelector, error) {
	log.Debugf("parsing type selector: %s", sel)

	ts, err := requestgen.ParseTypeSelector(sel)
	if err != nil {
		return nil, nil, err
	}

	log.Debugf("parsed type selector: %#v", ts)

	if ts.Package == "." {
		ts.Package = selectedPkgs[0].PkgPath
	}

	o, err := locateObject(ts, selectedPkgs)
	if err != nil {
		return nil, ts, err
	}

	return o, ts, nil
}

func loadPackages(patterns []string, tags []string) ([]*packages.Package, error) {
	p := newProfile(fmt.Sprintf("loadPackages: %v", patterns))
	defer p.stop()

	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedImports |
			packages.NeedTypes |
			packages.NeedSyntax |
			packages.NeedTypesInfo |
			packages.NeedModule |
			packages.NeedDeps,
		Tests: false,
	}

	if *debug {
		cfg.Logf = log.Debugf
	}
	if len(tags) > 0 {
		cfg.BuildFlags = []string{fmt.Sprintf("-tags=%s", strings.Join(tags, " "))}
	}

	log.Debugf("loading packages: %+v", patterns)

	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, err
	}

	if len(pkgs) > 0 {
		log.Debugf("loaded package: %s (pkgPath %s) -> %#v", pkgs[0].Name, pkgs[0].PkgPath, pkgs[0])
	}

	return pkgs, nil
}
