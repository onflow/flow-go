# Name of the cover profile
COVER_PROFILE := cover.out

# Run unit tests for test utilities in this module
.PHONY: test
test:
	GO111MODULE=on go test -tags relic -coverprofile=$(COVER_PROFILE) $(if $(JSON_OUTPUT),-json,) `go list ./... | grep -v -e integration/tests -e integration/benchmark`

# Run the integration test suite
.PHONY: integration-test
integration-test: collection-tests consensus-tests common-tests execution-tests verification-tests

.PHONY: collection-tests
collection-tests:
	GO111MODULE=on go test -tags relic -count=1 ./tests/collection

.PHONY: consensus-tests
consensus-tests:
	GO111MODULE=on go test -tags relic -count=1 ./tests/consensus

.PHONY: common-tests
common-tests:
	GO111MODULE=on go test -tags relic -count=1 ./tests/common

.PHONY: execution-tests
execution-tests:
	GO111MODULE=on go test -tags relic -count=1 ./tests/execution

.PHONY: verification-tests
verification-tests:
	GO111MODULE=on go test -tags relic -count=1 ./tests/verification

.PHONY: benchmark
benchmark:
	GO111MODULE=on go test -tags relic -count=1 -timeout 30m ./benchmark -run Benchmark
