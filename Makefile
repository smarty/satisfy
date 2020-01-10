#!/usr/bin/make -f

simple-test:
	go fmt ./...
	go test -timeout=1s -count=1 -short ./...

test:
	go test -timeout=1s -race -coverprofile=coverage.txt -covermode=atomic ./...

compile:
	go build ./...

build: test compile

.PHONY: simple-test test compile build
