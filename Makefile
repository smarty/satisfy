#!/usr/bin/make -f

NAME  := satisfy
REPO  ?= $(or ${DOCKER_SERVER},smartystreets)
IMAGE := $(REPO)/$(NAME):$(or ${VERSION},current)
IMAGEARCH := $(REPO)/$(NAME)-$(CPU):$(or ${VERSION},current)
PKG   := github.com/smarty/$(NAME)/cmd/satisfy

test: fmt
	GORACE="atexit_sleep_ms=50" go test -timeout=1s -count=1 -short -cover ./...

fmt:
	go mod tidy && go fmt ./...

coverage:
	GORACE="atexit_sleep_ms=50" go test -timeout=1s -race -covermode=atomic -short ./...

clean:
	rm -rf workspace/ coverage.txt

compile: clean
	GOOS="$(OS)" GOARCH="$(CPU)" GOAMD64="v3" CGO_ENABLED="0" go build -trimpath -ldflags "-X main.ldflagsSoftwareVersion=${VERSION}" -o workspace/satisfy "$(PKG)"

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

publish-amd: build
	docker build . --no-cache --rm -t "$(IMAGEARCH)"
	docker push "$(IMAGEARCH)"

.PHONY: test fmt coverage clean compile build install image publish
