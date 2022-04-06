FROM golang:1.17.8-alpine

RUN apk add --no-cache git

COPY . /go/src/github.com/c9s/requestgen
WORKDIR /go/src/github.com/c9s/requestgen
RUN go install ./cmd/requestgen
RUN go generate ./example/api
# RUN go generate github.com/c9s/requestgen/example/api
