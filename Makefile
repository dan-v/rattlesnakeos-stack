# modified version of https://gist.github.com/kwilczynski/ab451a357fa59b9377b7d946e557ee79
SHELL := /bin/bash

TARGET := rattlesnakeos-stack
VERSION := $(shell cat VERSION)

OS := darwin linux windows
ARCH := amd64
PKGS := $(shell go list ./internal/... ./cmd...)

.PHONY: \
	help \
	clean \
	tools \
	deps \
	test \
	vet \
	lint \
	fmt \
	build \
	build-all \
	version

all: fmt lint vet shellcheck test build-all

help:
	@echo 'Usage: make <OPTIONS> ... <TARGETS>'
	@echo ''
	@echo 'Available targets are:'
	@echo ''
	@echo '    help               Show this help screen.'
	@echo '    clean              Remove binaries, artifacts and releases.'
	@echo '    tools              Install tools needed by the project.'
	@echo '    deps               Download and install build time dependencies.'
	@echo '    test               Run unit tests.'
	@echo '    vet                Run go vet.'
	@echo '    lint               Run golint.'
	@echo '    fmt                Run go fmt.'
	@echo '    env                Display Go environment.'
	@echo '    build              Build project for current platform.'
	@echo '    build-all          Build project for all supported platforms.'
	@echo ''

print-%:
	@echo $* = $($*)

clean:
	rm -Rf build

tools:
	go get github.com/mitchellh/gox

deps:
	go mod tidy

test:
	go test ${PKGS}

vet:
	go vet ${PKGS}

lint:
	golangci-lint run cmd/... internal/... || true

fmt:
	go fmt ${PKGS}

shellcheck:
	shellcheck --severity=warning templates/build.sh || true

build:
	go build -race -ldflags "-X github.com/dan-v/rattlesnakeos-stack/cli.version=$(VERSION)" -v -o "$(TARGET)" .

build-all:
	mkdir -v -p $(CURDIR)/build/$(VERSION)
	gox -verbose -ldflags "-X github.com/dan-v/rattlesnakeos-stack/cli.version=$(VERSION)" \
	    -os "$(OS)" -arch "$(ARCH)" \
	    -output "$(CURDIR)/build/$(VERSION)/{{.OS}}/$(TARGET)" .
	cp -v -f \
	   $(CURDIR)/build/$(VERSION)/$$(go env GOOS)/$(TARGET) .

zip: all
	mkdir -p build/zips
	pushd build/$(VERSION)/darwin && zip -r ../../../build/zips/rattlesnakeos-stack-osx-${VERSION}.zip $(TARGET) && popd
	pushd build/$(VERSION)/windows && zip -r ../../../build/zips/rattlesnakeos-stack-windows-${VERSION}.zip $(TARGET).exe && popd
	pushd build/$(VERSION)/linux && zip -r ../../../build/zips/rattlesnakeos-stack-linux-${VERSION}.zip $(TARGET) && popd