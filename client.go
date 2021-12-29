package requestgen

import (
	"context"
	"net/http"
	"net/url"
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
