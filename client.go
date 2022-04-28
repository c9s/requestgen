package requestgen

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/pkg/errors"
)

type AuthenticatedRequestBuilder interface {
	// NewAuthenticatedRequest builds up the http request for authentication-required endpoints
	NewAuthenticatedRequest(ctx context.Context, method, refURL string, params url.Values, payload interface{}) (*http.Request, error)
}

// APIClient defines the request builder method and request method for the API service
type APIClient interface {
	// NewRequest builds up the http request for public endpoints
	NewRequest(ctx context.Context, method, refURL string, params url.Values, payload interface{}) (*http.Request, error)

	// SendRequest sends the request object to the api gateway
	SendRequest(req *http.Request) (*Response, error)
}

type AuthenticatedAPIClient interface {
	APIClient
	AuthenticatedRequestBuilder
}

const defaultHTTPTimeout = time.Second * 30

var defaultHttpClient = &http.Client{
	Timeout: defaultHTTPTimeout,
}

// BaseAPIClient provides the basic public API transport and the request builder.
// Usage
/*
	client := &BaseAPIClient{
		BaseURL:    u,
		HttpClient: defaultHttpClient,
	}
*/
type BaseAPIClient struct {
	BaseURL    *url.URL
	HttpClient *http.Client
}

// NewRequest create new API request. Relative url can be provided in refURL.
func (c *BaseAPIClient) NewRequest(ctx context.Context, method, refPath string, params url.Values, payload interface{}) (*http.Request, error) {
	body, err := castPayload(payload)
	if err != nil {
		return nil, err
	}

	ref, err := url.Parse(refPath)
	if err != nil {
		return nil, err
	}

	pathURL := c.BaseURL.ResolveReference(ref)
	if params != nil {
		pathURL.RawQuery = params.Encode()
	}

	return http.NewRequestWithContext(ctx, method, pathURL.String(), bytes.NewReader(body))
}

// SendRequest sends the request to the API server and handle the response
func (c *BaseAPIClient) SendRequest(req *http.Request) (*Response, error) {
	if c.HttpClient == nil {
		c.HttpClient = defaultHttpClient
	}

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}

	// newResponse reads the response body and return a new Response object
	response, err := NewResponse(resp)
	if err != nil {
		return response, err
	}

	// Check error, if there is an error, return the ErrorResponse struct type
	if response.IsError() {
		return response, errors.New(string(response.Body))
	}

	return response, nil
}

func castPayload(payload interface{}) ([]byte, error) {
	if payload != nil {
		switch v := payload.(type) {
		case string:
			return []byte(v), nil

		case []byte:
			return v, nil

		default:
			body, err := json.Marshal(v)
			return body, err
		}
	}

	return nil, nil
}
