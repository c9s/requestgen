package api

import "github.com/c9s/requestgen"

//go:generate go run ../../../cmd/requestgen -method GET -dynamicPath -debug -type DynamicPathRequest -responseType NoParamResponse
type DynamicPathRequest struct {
	client requestgen.APIClient
}

func (r *DynamicPathRequest) GetDynamicPath() (string, error) {
	return "/this/is/dynamic", nil
}
