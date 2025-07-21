# requestgen

<!--
[![Go Report Card](https://goreportcard.com/badge/github.com/c9s/requestgen)](https://goreportcard.com/report/github.com/c9s/requestgen)
-->

requestgen generates the cascade call for your request object

## Installation

```
go install github.com/c9s/requestgen/cmd/requestgen
```

## Synopsis


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

	// Set side parameter with valid values "buy", "sell"
	// "required" means the parameter is required.
	side SideType `param:"side,required" validValues:"buy,sell"`

    // Set order type parameter with valid values "limit", "market"
	orderType OrderType `param:"ordType" validValues:"limit,market"`

	size string `param:"size"`

    // For optional fields, you can use pointer to indicate that the field is optional.
    // 
	// price is an optional field by using pointer
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

See the [generated example](./example/api/place_order_request_requestgen.go)

See the [real world application](https://github.com/c9s/bbgo/blob/main/pkg/exchange/kucoin/kucoinapi/marketdata.go#L3)

## Usage

`requestgen` let you define http request parameters in a struct as struct field tags, and generate the request methods for you.

### Implementing your HTTP API Client

#### Interacting with Private APIs

For user-specific (APIs needs authentication) HTTP API requests, you can implement your own HTTP API client that satisfies the `AuthenticatedAPIClient` interface, 
which is a combination of `APIClient` and `AuthenticatedRequestBuilder`. The interface is defined in the `requestgen` package:

```go
type AuthenticatedAPIClient interface {
	APIClient
	AuthenticatedRequestBuilder
}

type AuthenticatedRequestBuilder interface {
    // NewAuthenticatedRequest builds up the http request for authentication-required endpoints
    NewAuthenticatedRequest(
        ctx context.Context, method, refURL string, params url.Values, payload interface{},
    ) (*http.Request, error)
}
```

The `NewAuthenticatedRequest` is a request builder, which is used to create authenticated requests.

Your `NewAuthenticatedRequest` method should attach the authentication headers to the request, such as API keys or bearer token.

#### Interacting with Public APIs

For public HTTP API requests, you can implement your own HTTP API client that satisfies the `APIClient` interface, which is defined in the `requestgen` package:

```go
type APIClient interface {
    // NewRequest builds up the http request for public endpoints
    NewRequest(ctx context.Context, method, refURL string, params url.Values, payload interface{}) (*http.Request, error)

    // SendRequest sends the request object to the api gateway
    SendRequest(req *http.Request) (*Response, error)
}
```

You can implement the `NewRequest` method to create a new HTTP request for public endpoints, and the `SendRequest` method to send the request and receive the response.

You can also use `requestgen.BaseAPIClient` as a base implementation for your API client, which provides a basic implementation of the `APIClient` interface.

```go
package api

import (
    "context"
    "net/http"
    "net/url"
    "github.com/c9s/requestgen"
)

type RestClient struct {
    requestgen.BaseAPIClient // Embedding BaseAPIClient to use its methods
}

func (c *RestClient) NewAuthenticatedRequest(ctx context.Context, method, refURL string, params url.Values, payload interface{}) (*http.Request, error) {
    // Implement your authentication logic here
    // For example, add API key or bearer token to the request headers
    req, err := c.BaseAPIClient.NewRequest(ctx, method, refURL, params, payload)
    if err != nil {
        return nil, err
    }
    // Add authentication headers
    req.Header.Set("Authorization", "Bearer YOUR_TOKEN_HERE")
    return req, nil
}
```


### Embedding Your API Client In Your Request Struct

Define your request struct with the `requestgen` tags to specify the endpoint URL and HTTP method,
like this a file `example/api/your_request.go`:

```go
//go:generate requestgen -method GET -url "/api/v1/test" -type YourRequest
type YourRequest struct {
    // If the API needs authentication, the client type should be `AuthenticatedAPIClient`. Otherwise, `APIClient`.
    // The `Do()` method will be generated if the client field is provided.
    // 
    // note, you will have to add flag "-url" and "-method" in the command to specify your endpoint and the request method.
    client requestgen.AuthenticatedAPIClient
}
```

Once you have defined your request struct, you can use the `go:generate` directive to generate the request methods automatically.
Or, run the `go generate` command to generate the request methods:

```shell
go generate ./example/api
```

And then you can define a helper method to create a new request object:

```go
func (c *RestClient) NewYourRequest() *YourRequest {
    return &YourRequest{
        client: c, // c is your API client that implements AuthenticatedAPIClient
    }
}
```

### Defining Request Parameters

You can define request parameters in the struct fields using the `param` tag. The tag format is `param:"name,options"`,
where `name` is the parameter name and `options` can include:
- `required`: Indicates that the parameter is required.
- `query`: Indicates that the parameter should be placed in the query string.
- `slug`: Indicates that the parameter should be slugified (e.g., converted to lowercase and hyphenated).

For example, you can define a request parameter like this:

```go
//go:generate requestgen -type GetAccountRequest -url "/api/v1/accounts/:accountID" -method GET
type GetAccountRequest struct {
    client requestgen.AuthenticatedAPIClient
    accountID string `param:"accountID,required,slug"` // This will be placed in the URL path
}
```
You can also use the `defaultValuer` tag to specify a default value for the parameter, which can be a function call or a constant value.

```go
//go:generate requestgen -type GetAccountRequest -url "/api/v1/accounts/:accountID" -method GET
type GetAccountRequest struct {
    client requestgen.AuthenticatedAPIClient
    accountID string `param:"accountID,required,slug" defaultValuer:"uuid()"`
}
```

### Generating Request Methods

After defining your request struct and its parameters, you can generate the request methods using the `go:generate` directive or by running the `requestgen` command manually.

```shell
go run ./cmd/requestgen -type GetAccountRequest -url "/api/v1/accounts/:accountID" -method GET
```

### Using the Generated Request Methods

Once the request methods are generated, you can use them in your code like this:

```go
package main

import (
    "context"
    "fmt"
    "github.com/c9s/requestgen/example/api"
)

func main() {
    // Create your API client
    client := &api.RestClient{
        // Initialize your API client here
    }

    // Create a new request
    req := client.NewGetAccountRequest("12345")

    // Call the Do method to send the request
    resp, err := req.Do(context.Background())
    if err != nil {
        fmt.Println("Error:", err)
        return
    }

    // Handle the response
    fmt.Println("Response:", resp)
}
```


### Embedding parameter in the URL

You can use the `slug` attribute to embed the parameter into the url:

```go
//go:generate GetRequest -url "/api/v1/accounts/:accountID" -type NewGetAccountRequest -responseDataType []Account
type NewGetAccountRequest struct {
	client requestgen.AuthenticatedAPIClient
	accountID string `param:"accountID,slug"`
}
```

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

## Placing parameter in the request query

```
//go:generate GetRequest -url "/api/orders" -type GetOpenOrdersRequest -responseDataType []Order
type GetOpenOrdersRequest struct {
	client requestgen.AuthenticatedAPIClient
	market string `param:"market,query"`
}

func (c *RestClient) NewGetOpenOrdersRequest(market string) *GetOpenOrdersRequest {
	return &GetOpenOrdersRequest{
		client: c,
		market: market,
	}
}
```

## Placing parameter in the request path

```
//go:generate requestgen -method DELETE -url "/api/orders/:orderID" -type CancelOrderRequest -responseType .APIResponse
type CancelOrderRequest struct {
	client  requestgen.AuthenticatedAPIClient
	orderID string `param:"orderID,required,slug"`
}

func (c *RestClient) NewCancelOrderRequest(orderID string) *CancelOrderRequest {
	return &CancelOrderRequest{
		client:  c,
		orderID: orderID,
	}
}
```







## APIClient

requestgen provides a base HTTP client, if your application does not need to get authenticated, you can use it directly:

```
baseURL, err := url.Parse("https://api.binance.com")
ctx := context.Background()
apiClient := &BaseAPIClient{
    BaseURL:    baseURL,
}

req, err := apiClient.NewRequest(ctx, "GET", "/api/v3/ping", nil , nil)
resp, err := apiClient.SendRequest(req)
```

You can also embed requestgen.BaseAPIClient into your own APIClient struct.


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

## Handling Response Error

You can handle the response error by casting the err to `*requestgen.ErrResponse`:

```go
if err != nil {
    if respErr, ok := err.(*requestgen.ErrResponse); ok {
        // handle the error response
        response := respErr.Response // this is the http.Response object
        body := respErr.Body // this is the response body as a byte slice []byte
        log.Printf("Response: %s, Body: %s, Code: %s\n", response, body, respErr.StatusCode)
    } else {
        // handle other errors
        log.Println("An error occurred:", err)
    }
}
```

# See Also

- callbackgen <https://github.com/c9s/callbackgen>

# LICENSE

MIT License
