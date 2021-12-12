# requestgen

requestgen generates the cascade call for your request object

## Installation

```
go get github.com/c9s/requestgen
```

## Usage

`requestgen` scans all the fields of the target struct, and generate setter methods and getParameters method.

```go
package api

//go:generate requestgen -type PlaceOrderRequest
type PlaceOrderRequest struct {
	// A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters.
	clientOrderID *string `param:"clientOid,required"`

	symbol string `param:"symbol,required"`

	// A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 8 characters.
	tag *string `param:"tag"`

	// "buy" or "sell"
	side SideType `param:"side,required" validValues:"buy,sell"`

	ordType OrderType `param:"ordType" validValues:"limit,market"`

	size string `param:"size"`

	// limit order parameters
	price *string `param:"price,omitempty"`

	timeInForce *TimeInForceType `param:"timeInForce,omitempty" validValues:"GTC,GTT,FOK"`

	complexArg ComplexArg `param:"complexArg"`
}
```

# LICENSE

MIT License
