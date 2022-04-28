// Code generated by "requestgen -type PlaceOrderRequest -method GET -url /api/v1/bullet -debug -responseType interface{} ./example/api"; DO NOT EDIT.

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"time"
)

func (p *PlaceOrderRequest) Page(page int64) *PlaceOrderRequest {
	p.page = &page
	return p
}

func (p *PlaceOrderRequest) ClientOrderID(clientOrderID string) *PlaceOrderRequest {
	p.clientOrderID = &clientOrderID
	return p
}

func (p *PlaceOrderRequest) Symbol(symbol string) *PlaceOrderRequest {
	p.symbol = symbol
	return p
}

func (p *PlaceOrderRequest) Tag(tag string) *PlaceOrderRequest {
	p.tag = &tag
	return p
}

func (p *PlaceOrderRequest) Side(side SideType) *PlaceOrderRequest {
	p.side = side
	return p
}

func (p *PlaceOrderRequest) OrdType(ordType OrderType) *PlaceOrderRequest {
	p.ordType = ordType
	return p
}

func (p *PlaceOrderRequest) Size(size string) *PlaceOrderRequest {
	p.size = size
	return p
}

func (p *PlaceOrderRequest) Price(price string) *PlaceOrderRequest {
	p.price = &price
	return p
}

func (p *PlaceOrderRequest) TimeInForce(timeInForce TimeInForceType) *PlaceOrderRequest {
	p.timeInForce = &timeInForce
	return p
}

func (p *PlaceOrderRequest) ComplexArg(complexArg ComplexArg) *PlaceOrderRequest {
	p.complexArg = complexArg
	return p
}

func (p *PlaceOrderRequest) StartTime(startTime time.Time) *PlaceOrderRequest {
	p.startTime = &startTime
	return p
}

// GetQueryParameters builds and checks the query parameters and returns url.Values
func (p *PlaceOrderRequest) GetQueryParameters() (url.Values, error) {
	var params = map[string]interface{}{}
	// check page field -> json key page
	if p.page != nil {
		page := *p.page

		// assign parameter of page
		params["page"] = page
	} else {
	}

	query := url.Values{}
	for k, v := range params {
		query.Add(k, fmt.Sprintf("%v", v))
	}

	return query, nil
}

// GetParameters builds and checks the parameters and return the result in a map object
func (p *PlaceOrderRequest) GetParameters() (map[string]interface{}, error) {
	var params = map[string]interface{}{}
	// check clientOrderID field -> json key clientOid
	if p.clientOrderID != nil {
		clientOrderID := *p.clientOrderID

		// TEMPLATE check-required
		if len(clientOrderID) == 0 {
			return nil, fmt.Errorf("clientOid is required, empty string given")
		}
		// END TEMPLATE check-required

		// assign parameter of clientOrderID
		params["clientOid"] = clientOrderID
	} else {
		// assign default of clientOrderID
		clientOrderID := uuid.New().String()
		// assign parameter of clientOrderID
		params["clientOid"] = clientOrderID
	}
	// check symbol field -> json key symbol
	symbol := p.symbol

	// TEMPLATE check-required
	if len(symbol) == 0 {
		return nil, fmt.Errorf("symbol is required, empty string given")
	}
	// END TEMPLATE check-required

	// assign parameter of symbol
	params["symbol"] = symbol
	// check tag field -> json key tag
	if p.tag != nil {
		tag := *p.tag

		// assign parameter of tag
		params["tag"] = tag
	} else {
	}
	// check side field -> json key side
	side := p.side

	// TEMPLATE check-required
	if len(side) == 0 {
		return nil, fmt.Errorf("side is required, empty string given")
	}
	// END TEMPLATE check-required

	// TEMPLATE check-valid-values
	switch side {
	case SideTypeBuy, SideTypeSell:
		params["side"] = side

	default:
		return nil, fmt.Errorf("side value %v is invalid", side)

	}
	// END TEMPLATE check-valid-values

	// assign parameter of side
	params["side"] = side
	// check ordType field -> json key ordType
	ordType := p.ordType

	// TEMPLATE check-required
	if len(ordType) == 0 {
		ordType = "limit"
	}
	// END TEMPLATE check-required

	// TEMPLATE check-valid-values
	switch ordType {
	case "limit", "market":
		params["ordType"] = ordType

	default:
		return nil, fmt.Errorf("ordType value %v is invalid", ordType)

	}
	// END TEMPLATE check-valid-values

	// assign parameter of ordType
	params["ordType"] = ordType
	// check size field -> json key size
	size := p.size

	// assign parameter of size
	params["size"] = size
	// check price field -> json key price
	if p.price != nil {
		price := *p.price

		// assign parameter of price
		params["price"] = price
	} else {
	}
	// check timeInForce field -> json key timeInForce
	if p.timeInForce != nil {
		timeInForce := *p.timeInForce

		// TEMPLATE check-valid-values
		switch timeInForce {
		case "GTC", "GTT", "FOK":
			params["timeInForce"] = timeInForce

		default:
			return nil, fmt.Errorf("timeInForce value %v is invalid", timeInForce)

		}
		// END TEMPLATE check-valid-values

		// assign parameter of timeInForce
		params["timeInForce"] = timeInForce
	} else {
	}
	// check complexArg field -> json key complexArg
	complexArg := p.complexArg

	// assign parameter of complexArg
	params["complexArg"] = complexArg
	// check startTime field -> json key startTime
	if p.startTime != nil {
		startTime := *p.startTime

		// assign parameter of startTime
		// convert time.Time to milliseconds time stamp
		params["startTime"] = strconv.FormatInt(startTime.UnixNano()/int64(time.Millisecond), 10)
	} else {
		// assign default of startTime
		startTime := time.Now()

		// assign parameter of startTime
		// convert time.Time to milliseconds time stamp
		params["startTime"] = strconv.FormatInt(startTime.UnixNano()/int64(time.Millisecond), 10)
	}

	return params, nil
}

// GetParametersQuery converts the parameters from GetParameters into the url.Values format
func (p *PlaceOrderRequest) GetParametersQuery() (url.Values, error) {
	query := url.Values{}

	params, err := p.GetParameters()
	if err != nil {
		return query, err
	}

	for k, v := range params {
		if p.isVarSlice(v) {
			p.iterateSlice(v, func(it interface{}) {
				query.Add(k+"[]", fmt.Sprintf("%v", it))
			})
		} else {
			query.Add(k, fmt.Sprintf("%v", v))
		}
	}

	return query, nil
}

// GetParametersJSON converts the parameters from GetParameters into the JSON format
func (p *PlaceOrderRequest) GetParametersJSON() ([]byte, error) {
	params, err := p.GetParameters()
	if err != nil {
		return nil, err
	}

	return json.Marshal(params)
}

// GetSlugParameters builds and checks the slug parameters and return the result in a map object
func (p *PlaceOrderRequest) GetSlugParameters() (map[string]interface{}, error) {
	var params = map[string]interface{}{}

	return params, nil
}

func (p *PlaceOrderRequest) applySlugsToUrl(url string, slugs map[string]string) string {
	for k, v := range slugs {
		needleRE := regexp.MustCompile(":" + k + "\\b")
		url = needleRE.ReplaceAllString(url, v)
	}

	return url
}

func (p *PlaceOrderRequest) iterateSlice(slice interface{}, f func(it interface{})) {
	sliceValue := reflect.ValueOf(slice)
	for i := 0; i < sliceValue.Len(); i++ {
		it := sliceValue.Index(i).Interface()
		f(it)
	}
}

func (p *PlaceOrderRequest) isVarSlice(v interface{}) bool {
	rt := reflect.TypeOf(v)
	switch rt.Kind() {
	case reflect.Slice:
		return true
	}
	return false
}

func (p *PlaceOrderRequest) GetSlugsMap() (map[string]string, error) {
	slugs := map[string]string{}
	params, err := p.GetSlugParameters()
	if err != nil {
		return slugs, nil
	}

	for k, v := range params {
		slugs[k] = fmt.Sprintf("%v", v)
	}

	return slugs, nil
}

func (p *PlaceOrderRequest) Do(ctx context.Context) (interface{}, error) {

	// empty params for GET operation
	var params interface{}
	query, err := p.GetQueryParameters()
	if err != nil {
		return nil, err
	}

	apiURL := "/api/v1/bullet"

	req, err := p.client.NewRequest(ctx, "GET", apiURL, query, params)
	if err != nil {
		return nil, err
	}

	response, err := p.client.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse interface{}
	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}
	return apiResponse, nil
}
