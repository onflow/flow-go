# Name of the cover profile
COVER_PROFILE := cover.out

# Run unit tests for test utilities in this module
.PHONY: test
test:
	GO111MODULE=on go test $(if $(VERBOSE),-v,) -tags relic -coverprofile=$(COVER_PROFILE) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) `go list ./... | grep -v -e integration/tests -e integration/benchmark`

# Run the integration test suite
.PHONY: integration-test
integration-test: access-tests common-tests execution-tests verification-tests collection-tests epochs-tests network-tests consensus-tests

.PHONY: ci-integration-test
ci-integration-test: access-tests common-tests epochs-tests consensus-tests execution-tests verification-tests network-tests collection-tests

.PHONY: access-tests
access-tests:
	GO111MODULE=on go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/access/api
	# GO111MODULE=on go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/access/unstaked

.PHONY: collection-tests
collection-tests:
	# flakey as it often hits port already allocated, since too many containers are created by the multi cluster test case
	# GO111MODULE=on go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/collection/proposal
	# GO111MODULE=on go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/collection/tx_ingress

.PHONY: consensus-tests
consensus-tests:
	GO111MODULE=on go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/consensus/inclusion
	GO111MODULE=on go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/consensus/sealing

.PHONY: epochs-tests
epochs-tests:
	GO111MODULE=on go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/epochs/epoch_static_transition
	GO111MODULE=on go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/epochs/epoch_join_and_leave_an
	GO111MODULE=on go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/epochs/epoch_join_and_leave_vn
	GO111MODULE=on go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/epochs/epoch_join_and_leave_ln
	GO111MODULE=on go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/epochs/epoch_join_and_leave_sn

.PHONY: common-tests
common-tests:
	GO111MODULE=on go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/common/ghost_node_send
	GO111MODULE=on go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/common/ghost_node_subscribe
	GO111MODULE=on go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/common/mvp
	GO111MODULE=on go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/common/mvp_bootstrap

.PHONY: execution-tests
execution-tests:
	GO111MODULE=on go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/execution/chunk_data_pack
	# GO111MODULE=on go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/execution/failing_tx_reverted
	GO111MODULE=on go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/execution/state_sync

.PHONY: verification-tests
verification-tests:
	GO111MODULE=on go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/verification/happypath
	GO111MODULE=on go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/verification/systemchunk
	GO111MODULE=on go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/verification/streamNegotiation

.PHONY: network-tests
network-tests:
	GO111MODULE=on go test $(if $(VERBOSE),-v,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/network

.PHONY: benchmark
benchmark:
	GO111MODULE=on go test -v -tags relic -count=1 -timeout 30m ./benchmark -run Benchmark

.PHONY: ci-benchmark
ci-benchmark:
	GO111MODULE=on ENV=TEAMCITY go test -v -tags relic -count=1 -timeout 15m ./benchmark -run Benchmark
