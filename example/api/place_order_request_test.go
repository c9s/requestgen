package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPlaceOrderRequest_GetParameters(t *testing.T) {
	client := NewClient()
	req := PlaceOrderRequest{client: client}
	req.OdrReqSymbol("BTCUSDT").OdrReqClientOrderID("0aa1a0123").OdrReqSide(SideTypeBuy).OdrReqOrdType(OrderTypeLimit)
	params, err := req.GetParameters()
	assert.NoError(t, err)
	assert.NotNil(t, params)
}

func TestPlaceOrderRequest_GetQueryParameters(t *testing.T) {
	client := NewClient()
	req := PlaceOrderRequest{client: client}
	req.OdrReqSymbol("BTCUSDT").OdrReqClientOrderID("0aa1a0123").OdrReqSide(SideTypeBuy).OdrReqOrdType(OrderTypeLimit).OdrReqPage(20)
	params, err := req.GetQueryParameters()
	assert.NoError(t, err)
	assert.NotNil(t, params)
	assert.True(t, params.Has("page"))
}
