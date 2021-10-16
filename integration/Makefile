# Name of the cover profile
COVER_PROFILE := cover.out

# The short Git commit hash
export SHORT_COMMIT := $(shell git rev-parse --short HEAD)

# Run unit tests for test utilities in this module
.PHONY: test
test:
	GO111MODULE=on go test -v -tags relic -coverprofile=$(COVER_PROFILE) $(if $(JSON_OUTPUT),-json,) `go list ./... | grep -v -e integration/tests -e integration/benchmark`

# Run the integration test suite
.PHONY: integration-test
integration-test: access-tests common-tests execution-tests verification-tests collection-tests consensus-tests epoch-tests

# NOTE: Currently skipping collection and consensus tests due to them not passing properly on CI
.PHONY: ci-integration-test
ci-integration-test: access-tests common-tests execution-tests epoch-tests verification-tests # collection-tests # consensus-tests

.PHONY: access-tests
access-tests:
	GO111MODULE=on go test -v -tags relic -count=1 ./tests/access

.PHONY: collection-tests
collection-tests:
	GO111MODULE=on go test -v -tags relic -count=1 ./tests/collection

.PHONY: consensus-tests
consensus-tests:
	GO111MODULE=on go test -v -tags relic -count=1 ./tests/consensus

.PHONY: epoch-tests
epoch-tests:
	GO111MODULE=on go test -v -tags relic -count=1 ./tests/epochs

.PHONY: common-tests
common-tests:
	GO111MODULE=on go test -v -tags relic -count=1 ./tests/common

.PHONY: execution-tests
execution-tests:
	GO111MODULE=on go test -v -tags relic -count=1 ./tests/execution

.PHONY: verification-tests
verification-tests:
	GO111MODULE=on go test -v -tags relic -count=1 ./tests/verification

.PHONY: benchmark
benchmark:
	GO111MODULE=on go test -v -tags relic -count=1 -timeout 30m ./benchmark -run Benchmark

.PHONY: ci-benchmark
ci-benchmark:
	GO111MODULE=on ENV=TEAMCITY go test -v -tags relic -count=1 -timeout 15m ./benchmark -run Benchmark
