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

import "github.com/c9s/requestgen"

//go:generate requestgen -type PlaceOrderRequest
type PlaceOrderRequest struct {
	// client is an optional field to implement
	// if you add this field with the APIClient interface type, the Do() method will be generated
	// note, you will have to add flag "-url" and "-method" to specify your endpoint and the request method.
	client requestgen.APIClient
	
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

Or you can run generate command manually like this:

```shell
go run ./cmd/requestgen -type PlaceOrderRequest -method GET -url "/api/v1/bullet" -debug ./example/api 
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

## APIClient

You can implement your own http API client struct that satisfies the following interface (defined in `requestgen.APIClient`)

```
// APIClient defines the request builder method and request method for the API service
type APIClient interface {
	// NewAuthenticatedRequest builds up the http request for authentication-required endpoints
	NewAuthenticatedRequest(method, refURL string, params url.Values, payload interface{}) (*http.Request, error)

	// NewRequest builds up the http request for public endpoints
	NewRequest(method, refURL string, params url.Values, payload interface{}) (*http.Request, error)

	// SendRequest sends the request object to the api gateway
	SendRequest(req *http.Request) (*Response, error)
}
```

See a real example implementation of the APIClient interface. [kucoin exchange client](./example/api/client.go)

# See Also

- callbackgen <https://github.com/c9s/callbackgen>

# LICENSE

MIT License
