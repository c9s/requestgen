package api

import "github.com/c9s/requestgen"

type CustomUnmarshalerResponse struct {
	Str    string
	Parsed bool
}

func (r *CustomUnmarshalerResponse) Unmarshal(data []byte) error {
	r.Str = string(data)
	r.Parsed = true
	return nil
}

//go:generate go run ../../cmd/requestgen -type CustomResponseUnmarshalerRequest -url /v1/bullet -method GET -debug -responseType .CustomUnmarshalerResponse
type CustomResponseUnmarshalerRequest struct {
	client requestgen.APIClient
}
