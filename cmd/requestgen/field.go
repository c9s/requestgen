package main

import (
	"go/ast"
	"go/types"
	"strconv"
	"strings"

	"github.com/fatih/structtag"
	"github.com/sirupsen/logrus"
)

type Field struct {
	Name string

	// IsSlug is used in the url as a template placeholder (the field name will be the placeholder ID).
	IsSlug bool

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

	TimeFormat string

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
}

func parseDefaultTag(tags *structtag.Tags, fieldName string, argKind types.BasicKind) (interface{}, error) {
	defaultTag, _ := tags.Get("default")
	if defaultTag == nil {
		return nil, nil
	}

	var defaultValue interface{}
	var defaultValueStr = defaultTag.Value()

	logrus.Debugf("%s found default value: %v", fieldName, defaultValueStr)

	switch argKind {
	case types.Int, types.Int64, types.Int32:
		i, err := strconv.Atoi(defaultValueStr)
		if err != nil {
			return nil, err
		}
		defaultValue = i

	case types.String:
		defaultValue = defaultValueStr

	}

	return defaultValue, nil
}

func parseValidValuesTag(tags *structtag.Tags, fieldName string, argKind types.BasicKind) (interface{}, error) {
	validValuesTag, _ := tags.Get("validValues")
	if validValuesTag == nil {
		return nil, nil
	}

	var validValues interface{}
	validValueList := strings.Split(validValuesTag.Value(), ",")

	logrus.Debugf("%s found valid values: %v", fieldName, validValueList)

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
