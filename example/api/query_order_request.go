package api

import (
	"github.com/c9s/requestgen"
)

//go:generate go run ../../cmd/requestgen -type QueryOrderRequest -responseType .Response -responseDataField Data -responseDataType []Order
type QueryOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	id []int `param:"id,query"` // Order IDs to query, can be multiple IDs separated by commas.
}
