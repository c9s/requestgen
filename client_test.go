package requestgen

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClient(t *testing.T) {
	baseURL, err := url.Parse("https://api.binance.com")
	assert.NoError(t, err)

	ctx := context.Background()
	apiClient := &BaseAPIClient{
		BaseURL:    baseURL,
	}

	req, err := apiClient.NewRequest(ctx, "GET", "/api/v3/ping", nil , nil)
	if assert.NoError(t, err) {
		assert.NotNil(t, req)

		resp, err := apiClient.SendRequest(req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	}
}
