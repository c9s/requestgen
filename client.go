package requestgen

import (
	"net/http"
	"net/url"
)

// APIClient defines the request builder method and request method for the API service
type APIClient interface {
	// NewAuthenticatedRequest builds up the http request for authentication-required endpoints
	NewAuthenticatedRequest(method, refURL string, params url.Values, payload interface{}) (*http.Request, error)

	// NewRequest builds up the http request for public endpoints
	NewRequest(method, refURL string, params url.Values, payload interface{}) (*http.Request, error)

	// SendRequest sends the request object to the api gateway
	SendRequest(req *http.Request) (*Response, error)
}
