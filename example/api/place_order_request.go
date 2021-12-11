package api

type SideType string

const (
	SideTypeBuy  SideType = "buy"
	SideTypeSell SideType = "sell"
)

type TimeInForceType string

const (
	// GTC Good Till Canceled orders remain open on the book until canceled. This is the default behavior if no policy is specified.
	TimeInForceGTC TimeInForceType = "GTC"

	// GTT Good Till Time orders remain open on the book until canceled or the allotted cancelAfter is depleted on the matching engine. GTT orders are guaranteed to cancel before any other order is processed after the cancelAfter seconds placed in order book.
	TimeInForceGTT TimeInForceType = "GTT"

	// FOK Fill Or Kill orders are rejected if the entire size cannot be matched.
	TimeInForceFOK TimeInForceType = "FOK"

	// IOC Immediate Or Cancel orders instantly cancel the remaining size of the limit order instead of opening it on the book.
	TimeInForceIOC TimeInForceType = "IOC"
)

type OrderType string

const (
	OrderTypeMarket OrderType = "market"
	OrderTypeLimit  OrderType = "limit"
)

//go:generate requestgen -type PlaceOrderRequest
type PlaceOrderRequest struct {
	// A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters.
	clientOrderID *string `json:"clientOid,omitempty"`

	symbol string `json:"symbol"`

	// A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 8 characters.
	tag *string `json:"tag"`

	// "buy" or "sell"
	side SideType `json:"side"`

	ordType OrderType `json:"ordType"`

	size string `json:"size"`

	// limit order parameters
	price *string `json:"price,omitempty"`

	timeInForce *TimeInForceType `json:"timeInForce,omitempty"`
}

