package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCustomUnmarshalRequest(t *testing.T) {
	transport := &MockTransport{}
	transport.GET("/v1/bullet", func(req *http.Request) (*http.Response, error) {
		return BuildResponseJson(http.StatusOK, map[string]interface{}{
			"foo": "bar",
		}), nil
	})

	client := NewClient()
	client.HttpClient.Transport = transport

	ctx := context.Background()

	req := &CustomResponseUnmarshalerRequest{client: client}
	customResponse, err := req.Do(ctx)
	assert.NoError(t, err)
	assert.True(t, customResponse.Parsed)
	assert.Equal(t, `{"foo":"bar"}`, customResponse.Str)

	t.Logf("%+v", customResponse)
}
