# modified version of https://gist.github.com/kwilczynski/ab451a357fa59b9377b7d946e557ee79
SHELL := /bin/bash

TARGET := rattlesnakeos-stack
VERSION := $(shell cat VERSION)

OS := darwin linux windows
ARCH := amd64

.PHONY: \
	help \
	default \
	clean \
	clean-artifacts \
	clean-vendor \
	tools \
	deps \
	test \
	coverage \
	vet \
	errors \
	lint \
	fmt \
	env \
	build \
	build-all \
	doc \
	check \
	version

all: fmt lint vet build-all

help:
	@echo 'Usage: make <OPTIONS> ... <TARGETS>'
	@echo ''
	@echo 'Available targets are:'
	@echo ''
	@echo '    help               Show this help screen.'
	@echo '    clean              Remove binaries, artifacts and releases.'
	@echo '    clean-artifacts    Remove build artifacts only.'
	@echo '    clean-vendor       Remove content of the vendor directory.'
	@echo '    tools              Install tools needed by the project.'
	@echo '    deps               Download and install build time dependencies.'
	@echo '    test               Run unit tests.'
	@echo '    coverage           Report code tests coverage.'
	@echo '    vet                Run go vet.'
	@echo '    lint               Run golint.'
	@echo '    fmt                Run go fmt.'
	@echo '    env                Display Go environment.'
	@echo '    build              Build project for current platform.'
	@echo '    build-all          Build project for all supported platforms.'
	@echo '    doc                Start Go documentation server on port 8080.'
	@echo '    check              Verify compiled binary.'
	@echo '    version            Display Go version.'
	@echo ''
	@echo 'Targets run by default are: imports, fmt, lint, vet, errors and build.'
	@echo ''

print-%:
	@echo $* = $($*)

clean: clean-artifacts
	go clean -i ./...
	rm -vf $(CURDIR)/coverage.*

clean-artifacts:
	rm -Rf artifacts

clean-vendor:
	find $(CURDIR)/vendor -type d -print0 2>/dev/null | xargs -0 rm -Rf

clean-all: clean clean-artifacts clean-vendor

tools:
	go get github.com/golang/lint/golint
	go get github.com/axw/gocov/gocov
	go get github.com/matm/gocov-html
	go get github.com/tools/godep
	go get github.com/mitchellh/gox

deps:
	dep ensure

test:
	go test -v ./...

coverage: 
	gocov test ./... > $(CURDIR)/coverage.out 2>/dev/null
	gocov report $(CURDIR)/coverage.out
	if test -z "$$CI"; then \
	  gocov-html $(CURDIR)/coverage.out > $(CURDIR)/coverage.html; \
	  if which open &>/dev/null; then \
	    open $(CURDIR)/coverage.html; \
	  fi; \
	fi

vet:
	go vet -v ./...

lint:
	golint $(go list ./... | grep -v /vendor/)

fmt:
	go fmt ./...

env:
	@go env

build:
	go build -race -ldflags "-X main.version=$(VERSION)" -v -o "$(TARGET)" .

build-all:
	mkdir -v -p $(CURDIR)/artifacts/$(VERSION)
	gox -verbose -ldflags "-X main.version=$(VERSION)" \
	    -os "$(OS)" -arch "$(ARCH)" \
	    -output "$(CURDIR)/artifacts/$(VERSION)/{{.OS}}/$(TARGET)" .
	cp -v -f \
	   $(CURDIR)/artifacts/$(VERSION)/$$(go env GOOS)/$(TARGET) .

doc:
	godoc -http=:8080 -index

check:
	@test -x $(CURDIR)/$(TARGET) || exit 1
	if $(CURDIR)/$(TARGET) --version | grep -qF '$(VERSION)'; then \
	  echo "$(CURDIR)/$(TARGET): OK"; \
	else \
	  exit 1; \
	fi

version:
	@go version

zip: all
	mkdir -p artifacts/zips
	pushd artifacts/$(VERSION)/darwin && zip -r ../../../artifacts/zips/rattlesnakeos-stack-osx-${VERSION}.zip $(TARGET) && popd
	pushd artifacts/$(VERSION)/windows && zip -r ../../../artifacts/zips/rattlesnakeos-stack-windows-${VERSION}.zip $(TARGET).exe && popd
	pushd artifacts/$(VERSION)/linux && zip -r ../../../artifacts/zips/rattlesnakeos-stack-linux-${VERSION}.zip $(TARGET) && popd