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

type ComplexArg struct {
	A, B int
}


//go:generate requestgen -type PlaceOrderRequest
type PlaceOrderRequest struct {
	// A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters.
	clientOrderID *string `param:"clientOid,omitempty"`

	symbol string `param:"symbol"`

	// A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 8 characters.
	tag *string `param:"tag"`

	// "buy" or "sell"
	side SideType `param:"side" validValues:"buy,sell"`

	ordType OrderType `param:"ordType" validValues:"limit,market"`

	size string `param:"size"`

	// limit order parameters
	price *string `param:"price,omitempty"`

	timeInForce *TimeInForceType `param:"timeInForce,omitempty" validValues:"GTC,GTT,FOK"`

	complexArg ComplexArg `param:"complexArg"`
}

