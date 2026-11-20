#!/usr/bin/make -f

NAME  := satisfy
REPO  ?= $(or ${DOCKER_SERVER},smartystreets)
IMAGE := $(REPO)/$(NAME):$(or ${VERSION},current)
IMAGEARCH := $(REPO)/$(NAME)-$(CPU):$(or ${VERSION},current)
PKG   := github.com/smarty/$(NAME)/v2/cmd/satisfy
# all development happens in the v2 module; the root (v0) module is frozen
GO    := go -C v2
# the version reported by `satisfy version`; defaults to the git tag when VERSION is not set
SOFTWARE_VERSION := $(or ${VERSION},$(shell git describe --tags --always --dirty 2>/dev/null))

test: fmt
	$(GO) test -timeout=5s -short -cover ./...

fmt:
	$(GO) mod tidy && $(GO) fmt ./...

coverage:
	$(GO) test -timeout=5s -race -covermode=atomic -short ./...

clean:
	rm -rf workspace/ coverage.txt

compile: clean
	GOOS="$(OS)" GOARCH="$(CPU)" GOAMD64="v3" CGO_ENABLED="0" $(GO) build -trimpath -ldflags "-X main.ldflagsSoftwareVersion=$(SOFTWARE_VERSION)" -o "$(CURDIR)/workspace/satisfy" "$(PKG)"

build: coverage compile

install: coverage
	GOOS="$(OS)" GOARCH="$(CPU)" CGO_ENABLED="0" $(GO) install -trimpath -ldflags "-X main.ldflagsSoftwareVersion=$(SOFTWARE_VERSION)" "$(PKG)"

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
