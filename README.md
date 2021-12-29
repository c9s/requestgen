# requestgen

requestgen generates the cascade call for your request object

## Installation

```
go get github.com/c9s/requestgen/cmd/requestgen
```

## Usage

`requestgen` scans all the fields of the target struct, and generate setter
methods and getParameters method.

```go
package api

import "github.com/c9s/requestgen"

//go:generate requestgen -type PlaceOrderRequest
type PlaceOrderRequest struct {
	// client is an optional field to implement.
	// If the API needs authentication, the client type should be `AuthenticatedAPIClient`. Otherwise, `APIClient`.
	// The `Do()` method will be generated if the client field is provided.
	// note, you will have to add flag "-url" and "-method" to specify your endpoint and the request method.
	client requestgen.AuthenticatedAPIClient
	
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

## Command Options

`-responseType [responseTypeSelector]`

When `responseTypeSelector` is not given, `interface{}` will be used for decoding the response content from the API server.

You can define your own responseType struct that can decode the API response, like this, e.g.,

```
type Response struct {
	Code    string          `json:"code"`
	Message string          `json:"msg"`
	CurrentPage int `json:"currentPage"`
	PageSize    int `json:"pageSize"`
	TotalNum    int `json:"totalNum"`
	TotalPage   int `json:"totalPage"`
	Orders      []Orders `json:"orders"`
}
```

And then use the type selector like this:

```shell
# if the type is in a relative package
requestgen ... -responseType '"./example/api".Response'

# if the type is in the same package
requestgen ... -responseType '".".Response'
```


When using requestgen with go:generate, you should handle the quote escaping
for the type selector, for example:


```go
//go:generate requestgen -type PlaceOrderRequest -responseType "\".\".Response" -responseDataField Data -responseDataType "\".\"Order"
```

But don't worry about the escaping, the above selector can be simplified as:

```go
//go:generate requestgen -type PlaceOrderRequest -responseType .Response -responseDataField Data -responseDataType .Order
```

If you want to reference a type defined in an external package, you can pass
something like `"net/url".Response` as the type selector, but it needs to be
escaped like this:

```go
//go:generate requestgen -type PlaceOrderRequest -responseType "\"net/url\".Response"
```


`-responseDataField [dataField]`

When `dataField` is given, it means your data is inside the `responseType`, the field name is where you want to extract the data from.
Be sure to define `dataField` as a `json.RawMessage` so that the generated code can handle the decoding separately.

For example:

```
type Response struct {
	Code    string          `json:"code"`
	Message string          `json:"msg"`
	CurrentPage int `json:"currentPage"`
	PageSize    int `json:"pageSize"`
	TotalNum    int `json:"totalNum"`
	TotalPage   int `json:"totalPage"`
	Data        json.RawMessage `json:"data"`
}
```

`-responseDataType [dataType]` 

When `dataType` is given, it means your data is inside the `responseType`. the raw json message will be decoded with this given type.


## APIClient

You can implement your own http API client struct that satisfies the following
interface (defined in `requestgen.APIClient`)

```
// APIClient defines the request builder method and request method for the API service
type APIClient interface {
	// NewRequest builds up the http request for public endpoints
	NewRequest(ctx context.Context, method, refURL string, params url.Values, payload interface{}) (*http.Request, error)

	// SendRequest sends the request object to the api gateway
	SendRequest(req *http.Request) (*Response, error)
}

// AuthenticatedAPIClient defines the authenticated request builder
type AuthenticatedAPIClient interface {
	APIClient
	AuthenticatedRequestBuilder
}
```

See a real example implementation of the APIClient interface. [kucoin exchange client](./example/api/client.go)

# See Also

- callbackgen <https://github.com/c9s/callbackgen>

# LICENSE

MIT License
