# Disable go sum database lookup for private repos
GOPRIVATE=github.com/onflow/*

GOPATH ?= $(HOME)/go

# Ensure go bin path is in path (Especially for CI)
PATH := $(PATH):$(GOPATH)/bin

.PHONY: test
test:
	# test all packages
	GO111MODULE=on go test $(if $(JSON_OUTPUT),-json,) -parallel 8 ./...

.PHONY: install-tools
install-tools:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b ${GOPATH}/bin v1.26.0

.PHONY: lint
lint:
	golangci-lint run -v ./...

# this ensures there is no unused dependency being added by accident
.PHONY: tidy
tidy:
	go mod tidy; git diff --exit-code
