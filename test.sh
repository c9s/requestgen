#!/bin/bash
set -e
rm -f ./example/api/place_order_request_accessors.go

go run ./cmd/requestgen -type PlaceOrderRequest -method GET -url "/api/v1/bullet" -debug \
        ./example/api && \
            cat ./example/api/place_order_request_accessors.go && \
            go test ./example/api

go run ./cmd/requestgen -type PlaceOrderRequest -method GET -url "/api/v1/bullet" -debug \
        -responseType '"./example/api".Response' \
        -responseDataField Data \
        -responseDataType '"./example/api".Order' \
        ./example/api && \
            cat ./example/api/place_order_request_accessors.go && \
            go test ./example/api

go run ./cmd/requestgen -type PlaceOrderRequest -method GET -url "/api/v1/bullet" -debug \
        -responseType 'interface{}' \
        ./example/api && \
            cat ./example/api/place_order_request_accessors.go && \
            go test ./example/api
