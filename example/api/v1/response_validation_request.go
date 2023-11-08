package api

import (
	"fmt"

	"github.com/c9s/requestgen"
)

type ResponseValidator struct {
	RetCode int    `json:"ret_code"`
	Msg     string `json:"msg"`
}

func (n ResponseValidator) Validate() error {
	if n.RetCode != 0 {
		return fmt.Errorf("unexpected response, retCode: %d, msg: %s", n.RetCode, n.Msg)
	}
	return nil
}

//go:generate go run ../../../cmd/requestgen -method GET -url /v1/bullet -debug -type ResponseValidatorRequest -responseType ResponseValidator
type ResponseValidatorRequest struct {
	client requestgen.APIClient
}
