package api

import (
	"encoding/json"
	"time"

	"github.com/c9s/requestgen"
)

type WalletType int

const (
	WalletTypeSpot WalletType = 0
	WalletTypeMargin WalletType = 1
	WalletTypeFunding WalletType = 2
)

type SideType string

const (
	SideTypeBuy  SideType = "buy"
	SideTypeSell SideType = "sell"
)

type TimeInForceType string

const (
	// TimeInForceGTC GTC Good Till Canceled orders remain open on the book until canceled. This is the default behavior if no policy is specified.
	TimeInForceGTC TimeInForceType = "GTC"

	// TimeInForceGTT GTT Good Till Time orders remain open on the book until canceled or the allotted cancelAfter is depleted on the matching engine. GTT orders are guaranteed to cancel before any other order is processed after the cancelAfter seconds placed in order book.
	TimeInForceGTT TimeInForceType = "GTT"

	// TimeInForceFOK FOK Fill Or Kill orders are rejected if the entire size cannot be matched.
	TimeInForceFOK TimeInForceType = "FOK"

	// TimeInForceIOC IOC Immediate Or Cancel orders instantly cancel the remaining size of the limit order instead of opening it on the book.
	TimeInForceIOC TimeInForceType = "IOC"
)

type OrderType string

const (
	OrderTypeMarket OrderType = "market"
	OrderTypeLimit  OrderType = "limit"
)

type ComplexArg struct {
	A, B int
}

type Response struct {
	Code    string          `json:"code"`
	Message string          `json:"msg"`
	CurrentPage int `json:"currentPage"`
	PageSize    int `json:"pageSize"`
	TotalNum    int `json:"totalNum"`
	TotalPage   int `json:"totalPage"`
	Data    json.RawMessage `json:"data"`
}

type Order struct {
	Id            string `json:"id"`
	Symbol        string `json:"symbol"`
	OpType        string `json:"opType"`
	Type          string `json:"type"`
	Side          string `json:"side"`
	Price         string `json:"price"`
	Size          string `json:"size"`
	Funds         string `json:"funds"`
	DealFunds     string `json:"dealFunds"`
	DealSize      string `json:"dealSize"`
	Fee           string `json:"fee"`
	FeeCurrency   string `json:"feeCurrency"`
	Stp           string `json:"stp"`
	Stop          string `json:"stop"`
	StopTriggered bool   `json:"stopTriggered"`
	StopPrice     string `json:"stopPrice"`
	TimeInForce   string `json:"timeInForce"`
	PostOnly      bool   `json:"postOnly"`
	Hidden        bool   `json:"hidden"`
	Iceberg       bool   `json:"iceberg"`
	VisibleSize   string `json:"visibleSize"`
	CancelAfter   int    `json:"cancelAfter"`
	Channel       string `json:"channel"`
	ClientOid     string `json:"clientOid"`
	Remark        string `json:"remark"`
	Tags          string `json:"tags"`
	IsActive      bool   `json:"isActive"`
	CancelExist   bool   `json:"cancelExist"`
	CreatedAt     int64  `json:"createdAt"`
	TradeType     string `json:"tradeType"`
}

//go:generate go run ../../cmd/requestgen -debug -type PlaceOrderRequest -responseType .Response -responseDataField Data -responseDataType .Order
type PlaceOrderRequest struct {
	client requestgen.APIClient

	// A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters.
	clientOrderID *string `param:"clientOid,required" defaultValuer:"uuid()"`

	symbol string `param:"symbol,required"`

	// A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 8 characters.
	tag *string `param:"tag"`

	// "buy" or "sell"
	side SideType `param:"side,required"`

	ordType OrderType `param:"ordType,required" validValues:"limit,market" default:"limit"`

	size string `param:"size"`

	// limit order parameters
	price *string `param:"price,omitempty"`

	timeInForce *TimeInForceType `param:"timeInForce,omitempty" validValues:"GTC,GTT,FOK"`

	complexArg ComplexArg `param:"complexArg"`

	startTime *time.Time `param:"startTime,milliseconds" defaultValuer:"now()"`

	// page defines the query parameters for something like '?page=123'
	page *int64 `param:"page,query"`
}
