# Name of the cover profile
COVER_PROFILE := cover.out

# allows CI to specify whether to have race detection on / off
ifeq ($(RACE_DETECTOR),1)
	RACE_FLAG := -race
else
	RACE_FLAG :=
endif

include ../crypto_adx_flag.mk

CGO_FLAG := CGO_CFLAGS=$(CRYPTO_FLAG)

# Run the integration test suite
.PHONY: integration-test
integration-test: access-cohort1-tests access-cohort2-tests access-cohort3-tests ghost-tests mvp-tests execution-tests verification-tests upgrades-tests collection-tests epochs-an-tests epochs-ln-tests epochs-sn-tests epochs-static-tests epochs-vn-tests network-tests consensus-tests

.PHONY: ci-integration-test
ci-integration-test: access-cohort1-tests access-cohort2-tests access-cohort3-tests ghost-tests mvp-tests epochs-an-tests epochs-ln-tests epochs-sn-tests epochs-static-tests epochs-vn-tests consensus-tests execution-tests verification-tests upgrades-tests network-tests collection-tests

# Run unit tests for test utilities in this module
.PHONY: test
test:
	$(CGO_FLAG) go test $(if $(VERBOSE),-v,) -coverprofile=$(COVER_PROFILE) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) `go list ./... | grep -v -e integration/tests`

.PHONY: access-cohort1-tests
access-cohort1-tests:
	$(CGO_FLAG) go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) ./tests/access/cohort1/...

.PHONY: access-cohort2-tests
access-cohort2-tests:
	$(CGO_FLAG) go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) ./tests/access/cohort2/...

.PHONY: access-cohort3-tests
access-cohort3-tests:
	$(CGO_FLAG) go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) ./tests/access/cohort3/...


.PHONY: collection-tests
collection-tests:
	$(CGO_FLAG) go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) ./tests/collection/...

.PHONY: consensus-tests
consensus-tests:
	$(CGO_FLAG) go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) ./tests/consensus/...

.PHONY: epochs-an-tests
epochs-an-tests:
	# Use a higher timeout of 20m for the suite of tests which span full epochs
	$(CGO_FLAG) go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -timeout 20m ./tests/epochs/an/...

.PHONY: epochs-ln-tests
epochs-ln-tests:
	# Use a higher timeout of 20m for the suite of tests which span full epochs
	$(CGO_FLAG) go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -timeout 20m ./tests/epochs/ln/...

.PHONY: epochs-sn-tests
epochs-sn-tests:
	# Use a higher timeout of 20m for the suite of tests which span full epochs
	$(CGO_FLAG) go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -timeout 20m ./tests/epochs/sn/...

.PHONY: epochs-static-tests
epochs-static-tests:
	# Use a higher timeout of 20m for the suite of tests which span full epochs
	$(CGO_FLAG) go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -timeout 20m ./tests/epochs/static/...

.PHONY: epochs-vn-tests
epochs-vn-tests:
	# Use a higher timeout of 20m for the suite of tests which span full epochs
	$(CGO_FLAG) go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -timeout 20m ./tests/epochs/vn/...

.PHONY: ghost-tests
ghost-tests:
	$(CGO_FLAG) go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) ./tests/ghost/...

.PHONY: mvp-tests
mvp-tests:
	$(CGO_FLAG) go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) ./tests/mvp/...

.PHONY: execution-tests
execution-tests:
	$(CGO_FLAG) go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) ./tests/execution/...

.PHONY: verification-tests
verification-tests:
	$(CGO_FLAG) go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) ./tests/verification/...

# upgrades-tests tests need to be run sequentially (-p 1) due to interference between different Docker networks when tests are run in parallel
.PHONY: upgrades-tests
upgrades-tests:
	$(CGO_FLAG) go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) ./tests/upgrades/... -p 1

.PHONY: network-tests
network-tests:
	$(CGO_FLAG) go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) ./tests/network/...

# BFT tests need to be run sequentially (-p 1) due to interference between different Docker networks when tests are run in parallel
.PHONY: bft-framework-tests
bft-framework-tests:
	$(CGO_FLAG) go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) ./tests/bft/framework/... -p 1
.PHONY: bft-protocol-tests
bft-protocol-tests:
	$(CGO_FLAG) go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) ./tests/bft/protocol/... -p 1

.PHONY: bft-gossipsub-rpc-inspector-tests
bft-gossipsub-rpc-inspector-tests:
	$(CGO_FLAG) go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) ./tests/bft/gossipsub/rpc_inspector/... -p 1

.PHONY: bft-gossipsub-signature-tests
bft-gossipsub-signature-tests:
	$(CGO_FLAG) go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) ./tests/bft/gossipsub/signature/... -p 1

.PHONY: bft-gossipsub-topic-validator-tests
bft-gossipsub-topic-validator-tests:
	$(CGO_FLAG) go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) ./tests/bft/gossipsub/topicvalidator/... -p 1

.PHONY: bft-tests
bft-tests: bft-framework-tests bft-protocol-tests bft-gossipsub-rpc-inspector-tests bft-gossipsub-signature-tests bft-gossipsub-topic-validator-tests
