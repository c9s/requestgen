package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type RoundTripFunc func(req *http.Request) (*http.Response, error)

type MockTransport struct {
	getHandlers    map[string]RoundTripFunc
	postHandlers   map[string]RoundTripFunc
	deleteHandlers map[string]RoundTripFunc
	putHandlers    map[string]RoundTripFunc
}

func (transport *MockTransport) GET(path string, f RoundTripFunc) {
	if transport.getHandlers == nil {
		transport.getHandlers = make(map[string]RoundTripFunc)
	}

	transport.getHandlers[path] = f
}

func (transport *MockTransport) POST(path string, f RoundTripFunc) {
	if transport.postHandlers == nil {
		transport.postHandlers = make(map[string]RoundTripFunc)
	}

	transport.postHandlers[path] = f
}

func (transport *MockTransport) DELETE(path string, f RoundTripFunc) {
	if transport.deleteHandlers == nil {
		transport.deleteHandlers = make(map[string]RoundTripFunc)
	}

	transport.deleteHandlers[path] = f
}

func (transport *MockTransport) PUT(path string, f RoundTripFunc) {
	if transport.putHandlers == nil {
		transport.putHandlers = make(map[string]RoundTripFunc)
	}

	transport.putHandlers[path] = f
}

func (transport *MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var handlers map[string]RoundTripFunc

	switch strings.ToUpper(req.Method) {

	case "GET":
		handlers = transport.getHandlers
	case "POST":
		handlers = transport.postHandlers
	case "DELETE":
		handlers = transport.deleteHandlers
	case "PUT":
		handlers = transport.putHandlers

	default:
		return nil, fmt.Errorf("unsupported mock transport request method: %s", req.Method)

	}

	f, ok := handlers[req.URL.Path]
	if !ok {
		return nil, fmt.Errorf("roundtrip mock to %s %s is not defined", req.Method, req.URL.Path)
	}

	return f(req)
}

func MockWithJsonReply(url string, rawData interface{}) *http.Client {
	tripFunc := func(_ *http.Request) (*http.Response, error) {
		return BuildResponseJson(http.StatusOK, rawData), nil
	}

	transport := &MockTransport{}
	transport.DELETE(url, tripFunc)
	transport.GET(url, tripFunc)
	transport.POST(url, tripFunc)
	transport.PUT(url, tripFunc)
	return &http.Client{Transport: transport}
}

func BuildResponse(code int, payload []byte) *http.Response {
	return &http.Response{
		StatusCode:    code,
		Body:          io.NopCloser(bytes.NewBuffer(payload)),
		ContentLength: int64(len(payload)),
	}
}

func BuildResponseString(code int, payload string) *http.Response {
	b := []byte(payload)
	return &http.Response{
		StatusCode: code,
		Body: io.NopCloser(
			bytes.NewBuffer(b),
		),
		ContentLength: int64(len(b)),
	}
}

func BuildResponseJson(code int, payload interface{}) *http.Response {
	data, err := json.Marshal(payload)
	if err != nil {
		return BuildResponseString(http.StatusInternalServerError, `{error: "httptesting.MockTransport error calling json.Marshal()"}`)
	}

	resp := BuildResponse(code, data)
	resp.Header = http.Header{}
	resp.Header.Set("Content-Type", "application/json")
	return resp
}
