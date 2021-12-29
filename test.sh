#!/bin/bash
set -e
rm -f ./example/api/place_order_request_accessors.go

go run ./cmd/requestgen -type PlaceOrderRequest -method GET -url "/api/v1/bullet" -debug \
        ./example/api && \
            go test ./example/api || cat ./example/api/place_order_request_accessors.go

go run ./cmd/requestgen -type PlaceOrderRequest -method GET -url "/api/v1/bullet" -debug \
        -responseType '"./example/api".Response' \
        -responseDataField Data \
        -responseDataType '"./example/api".Order' \
        ./example/api && \
            go test ./example/api || cat ./example/api/place_order_request_accessors.go

go run ./cmd/requestgen -type PlaceOrderRequest -method GET -url "/api/v1/bullet" -debug \
        -responseType 'interface{}' \
        ./example/api && \
            go test ./example/api || cat ./example/api/place_order_request_accessors.go

go run ./cmd/requestgen -type NoParamRequest -url /v1/bullet -method GET -debug ./example/api && go test ./example/api
