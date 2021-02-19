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
	tools \
	deps \
	test \
	coverage \
	vet \
	errors \
	lint \
	fmt \
	build \
	build-all \
	doc \
	check \
	version

all: fmt lint vet shellcheck build-all

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
	@echo '    check              Verify compiled binary.'
	@echo ''

print-%:
	@echo $* = $($*)

clean: clean-artifacts
	rm -vf $(CURDIR)/coverage.*

clean-artifacts:
	rm -Rf build

clean-all: clean clean-artifacts clean-vendor

tools:
	go get golang.org/x/lint/golint
	go get github.com/axw/gocov/gocov
	go get github.com/matm/gocov-html
	go get github.com/tools/godep
	go get github.com/mitchellh/gox

deps:
	go mod tidy

test:
	go test -v $(go list ./internal/...)

coverage: 
	gocov test $(go list ./internal/...) > $(CURDIR)/coverage.out 2>/dev/null
	gocov report $(CURDIR)/coverage.out
	if test -z "$$CI"; then \
	  gocov-html $(CURDIR)/coverage.out > $(CURDIR)/coverage.html; \
	  if which open &>/dev/null; then \
	    open $(CURDIR)/coverage.html; \
	  fi; \
	fi

vet:
	go vet $(go list ./cmd/...)
	go vet $(go list ./internal/...)

lint:
	golint $(go list ./cmd/...)
	golint $(go list ./internal/...)

fmt:
	go fmt $(go list ./cmd/...)
	go fmt $(go list ./internal/...)

shellcheck:
	shellcheck --severity=warning templates/build.sh

build:
	go build -race -ldflags "-X github.com/dan-v/rattlesnakeos-stack/cli.version=$(VERSION)" -v -o "$(TARGET)" .

build-all:
	mkdir -v -p $(CURDIR)/build/$(VERSION)
	gox -verbose -ldflags "-X github.com/dan-v/rattlesnakeos-stack/cli.version=$(VERSION)" \
	    -os "$(OS)" -arch "$(ARCH)" \
	    -output "$(CURDIR)/build/$(VERSION)/{{.OS}}/$(TARGET)" .
	cp -v -f \
	   $(CURDIR)/build/$(VERSION)/$$(go env GOOS)/$(TARGET) .

check:
	@test -x $(CURDIR)/$(TARGET) || exit 1
	if $(CURDIR)/$(TARGET) --version | grep -qF '$(VERSION)'; then \
	  echo "$(CURDIR)/$(TARGET): OK"; \
	else \
	  exit 1; \
	fi

zip: all
	mkdir -p build/zips
	pushd build/$(VERSION)/darwin && zip -r ../../../build/zips/rattlesnakeos-stack-osx-${VERSION}.zip $(TARGET) && popd
	pushd build/$(VERSION)/windows && zip -r ../../../build/zips/rattlesnakeos-stack-windows-${VERSION}.zip $(TARGET).exe && popd
	pushd build/$(VERSION)/linux && zip -r ../../../build/zips/rattlesnakeos-stack-linux-${VERSION}.zip $(TARGET) && popd