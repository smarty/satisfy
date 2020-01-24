#!/usr/bin/make -f

NAME  := satisfy
REPO  ?= $(or ${DOCKER_SERVER},smartystreets)
IMAGE := $(REPO)/$(NAME):$(or ${VERSION},local)
PKG   := bitbucket.org/smartystreets/$(NAME)/cmd

simple-test:
	go fmt ./...
	go test -timeout=1s -count=1 -short ./...

test:
	go test -timeout=1s -race -coverprofile=coverage.txt -covermode=atomic -short ./...

clean:
	rm -rf workspace/

compile: clean
	GOOS="$(OS)" GOARCH="$(CPU)" CGO_ENABLED="0" go build -trimpath -ldflags "-X main.ldflagsSoftwareVersion=${VERSION}" -o workspace/app "$(PKG)"

build: test compile

install: test
	go install "$(PKG)"

##########################################################

image: OS  ?= linux
image: CPU ?= amd64
image: build
	docker build . --no-cache --rm -t "$(IMAGE)"

publish: image
	docker push "$(IMAGE)"

.PHONY: simple-test test clean compile build install image publish


.PHONY: simple-test test compile build
