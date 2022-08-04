# Name of the cover profile
COVER_PROFILE := cover.out

# Run the integration test suite
.PHONY: integration-test
integration-test: access-tests ghost-tests mvp-tests execution-tests verification-tests collection-tests epochs-tests network-tests consensus-tests

.PHONY: ci-integration-test
ci-integration-test: access-tests ghost-tests mvp-tests epochs-tests consensus-tests execution-tests verification-tests network-tests collection-tests

.PHONY: benchmark
benchmark:
	go test -v -tags relic -count=1 -timeout 30m ./benchmark -run Benchmark

.PHONY: ci-benchmark
ci-benchmark:
	ENV=TEAMCITY go test -v -tags relic -count=1 -timeout 15m ./benchmark -run Benchmark

############################################################################################
# CAUTION: DO NOT MODIFY THE TARGETS BELOW! DOING SO WILL BREAK THE FLAKY TEST MONITOR
# In particular, do not skip tests by commenting them out here.

# Run unit tests for test utilities in this module
.PHONY: test
test:
	go test $(if $(VERBOSE),-v,) -tags relic -coverprofile=$(COVER_PROFILE) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) `go list ./... | grep -v -e integration/tests -e integration/benchmark`

.PHONY: access-tests
access-tests:
	go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/access/...

.PHONY: collection-tests
collection-tests:
	go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/collection/...

.PHONY: consensus-tests
consensus-tests:
	go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/consensus/...

.PHONY: epochs-tests
epochs-tests:
	go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/epochs/...

.PHONY: ghost-tests
ghost-tests:
	go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/ghost/...

.PHONY: mvp-tests
mvp-tests:
	go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/mvp/...

.PHONY: execution-tests
execution-tests:
	go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/execution/...

.PHONY: verification-tests
verification-tests:
	go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/verification/...

.PHONY: network-tests
network-tests:
	go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/network/...

.PHONY: observer-tests
observer-tests:
	go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/observer/...

# BFT tests need to be run sequentially (-p 1) due to interference between different Docker networks when tests are run in parallel
.PHONY: bft-tests
bft-tests:
	go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/bft/... -p 1


############################################################################################
