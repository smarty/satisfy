#!/usr/bin/make -f

NAME  := satisfy
REPO  ?= $(or ${DOCKER_SERVER},smartystreets)
IMAGE := $(REPO)/$(NAME):$(or ${VERSION},local)
PKG   := bitbucket.org/smartystreets/$(NAME)/cmd/satisfy

test:
	go fmt ./... && go test -timeout=1s -count=1 -short ./...

coverage:
	go test -timeout=1s -race -covermode=atomic -short ./...

clean:
	rm -rf workspace/ coverage.txt

compile: clean
	GOOS="$(OS)" GOARCH="$(CPU)" CGO_ENABLED="0" go build -trimpath -ldflags "-X main.ldflagsSoftwareVersion=${VERSION}" -o workspace/app "$(PKG)"

build: coverage compile

install: coverage
	GOOS="$(OS)" GOARCH="$(CPU)" CGO_ENABLED="0" go install -trimpath -ldflags "-X main.ldflagsSoftwareVersion=${VERSION}" "$(PKG)"

##########################################################

image: OS  ?= linux
image: CPU ?= amd64
image: build
	docker build . --no-cache --rm -t "$(IMAGE)"

publish: image
	docker push "$(IMAGE)"

.PHONY: test coverage clean compile build install image publish
