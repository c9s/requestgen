package api

import "github.com/c9s/requestgen"

//go:generate go run ../../cmd/requestgen -type NoParamRequest -url /v1/bullet -method GET -debug
type NoParamRequest struct {
	client requestgen.APIClient
}
