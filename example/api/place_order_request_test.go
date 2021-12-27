package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPlaceOrderRequest_GetParameters(t *testing.T) {
	client := NewClient()
	req := PlaceOrderRequest{client: client}
	req.Symbol("BTCUSDT").ClientOrderID("0aa1a0123").Side(SideTypeBuy).OrdType(OrderTypeLimit)
	params, err := req.GetParameters()
	assert.NoError(t, err)
	assert.NotNil(t, params)
}

func TestPlaceOrderRequest_GetQueryParameters(t *testing.T) {
	client := NewClient()
	req := PlaceOrderRequest{client: client}
	req.Symbol("BTCUSDT").ClientOrderID("0aa1a0123").Side(SideTypeBuy).OrdType(OrderTypeLimit).Page(20)
	params, err := req.GetQueryParameters()
	assert.NoError(t, err)
	assert.NotNil(t, params)
	assert.True(t, params.Has("page"))
}
