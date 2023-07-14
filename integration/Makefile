# Name of the cover profile
COVER_PROFILE := cover.out

# allows CI to specify whether to have race detection on / off
ifeq ($(RACE_DETECTOR),1)
	RACE_FLAG := -race
else
	RACE_FLAG :=
endif

# Run the integration test suite
.PHONY: integration-test
integration-test: access-tests ghost-tests mvp-tests execution-tests verification-tests upgrades-tests collection-tests epochs-tests network-tests consensus-tests

.PHONY: ci-integration-test
ci-integration-test: access-tests ghost-tests mvp-tests epochs-tests consensus-tests execution-tests verification-tests upgrades-tests network-tests collection-tests

############################################################################################
# CAUTION: DO NOT MODIFY THE TARGETS BELOW! DOING SO WILL BREAK THE FLAKY TEST MONITOR
# In particular, do not skip tests by commenting them out here.

# Run unit tests for test utilities in this module
.PHONY: test
test:
	go test $(if $(VERBOSE),-v,) -tags relic -coverprofile=$(COVER_PROFILE) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) `go list ./... | grep -v -e integration/tests`

.PHONY: access-tests
access-tests:
	go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/access/...

.PHONY: collection-tests
collection-tests:
	go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/collection/...

.PHONY: consensus-tests
consensus-tests:
	go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/consensus/...

.PHONY: epochs-tests
epochs-tests:
	# Use a higher timeout of 20m for the suite of tests which span full epochs
	go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic -timeout 30m ./tests/epochs/...

.PHONY: ghost-tests
ghost-tests:
	go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/ghost/...

.PHONY: mvp-tests
mvp-tests:
	go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/mvp/...

.PHONY: execution-tests
execution-tests:
	go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/execution/...

.PHONY: verification-tests
verification-tests:
	go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/verification/...

# upgrades-tests tests need to be run sequentially (-p 1) due to interference between different Docker networks when tests are run in parallel
.PHONY: upgrades-tests
upgrades-tests:
	go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/upgrades/... -p 1

.PHONY: network-tests
network-tests:
	go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/network/...

# BFT tests need to be run sequentially (-p 1) due to interference between different Docker networks when tests are run in parallel
.PHONY: bft-tests
bft-tests:
	go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -tags relic ./tests/bft/... -p 1


############################################################################################
