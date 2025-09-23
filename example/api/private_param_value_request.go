package api

import "github.com/c9s/requestgen"

//go:generate go run ../../cmd/requestgen -type PrivateParamValueRequest -url /v1/bullet -method GET -debug
type PrivateParamValueRequest struct {
	client requestgen.APIClient

	// symbol is the trading pair symbol, e.g., "BTC-USDT", "ETH-BTC".
	symbol string `param:"symbol,private" default:"BTC-USDT"`

	// side is the order side, e.g., "0", "1".
	side int `param:"side,private"`
}
