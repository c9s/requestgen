# requestgen

requestgen generates the cascade call for your request object

## Installation

```
go get github.com/c9s/requestgen/cmd/requestgen
```

## Usage

`requestgen` scans all the fields of the target struct, and generate setter methods and getParameters method.

```go
package api

//go:generate requestgen -type PlaceOrderRequest
type PlaceOrderRequest struct {
	// A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters.
	clientOrderID *string `param:"clientOid,required" defaultValuer:"uuid()"`

	symbol string `param:"symbol,required"`

	// A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 8 characters.
	tag *string `param:"tag"`

	// "buy" or "sell"
	side SideType `param:"side,required" validValues:"buy,sell"`

	orderType OrderType `param:"ordType" validValues:"limit,market"`

	size string `param:"size"`

	// limit order parameters
	price *string `param:"price,omitempty"`

	timeInForce *TimeInForceType `param:"timeInForce,omitempty" validValues:"GTC,GTT,FOK"`

	complexArg ComplexArg `param:"complexArg"`

	startTime *time.Time `param:"startTime,milliseconds" defaultValuer:"now()"`
}
```

Then you can do:

```go
req := &PlaceOrderRequest{}
err := req.Tag(..).
	OrderType(OrderTypeLimit).
	Side(SideTypeBuy).
	Do(ctx)
```

See the [generated example](./example/api/place_order_request_accessors.go)

Note that you need to implement Do() by yourself.

# See Also

- callbackgen <https://github.com/c9s/callbackgen>

# LICENSE

MIT License
