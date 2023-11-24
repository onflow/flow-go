# Name of the cover profile
COVER_PROFILE := cover.out

# allows CI to specify whether to have race detection on / off
ifeq ($(RACE_DETECTOR),1)
	RACE_FLAG := -race
else
	RACE_FLAG :=
endif

# `ADX_SUPPORT` is 1 if ADX instructions are supported and 0 otherwise.
ifeq ($(shell uname -s),Linux)
# detect ADX support on the CURRENT linux machine.
	ADX_SUPPORT := $(shell if ([ -f "/proc/cpuinfo" ] && grep -q -e '^flags.*\badx\b' /proc/cpuinfo); then echo 1; else echo 0; fi)
else
# on non-linux machines, set the flag to 1 by default
	ADX_SUPPORT := 1
endif

# the crypto package uses BLST source files underneath which may use ADX insructions.
ifeq ($(ADX_SUPPORT), 1)
# if ADX insructions are supported, default is to use a fast ADX BLST implementation 
	CRYPTO_FLAG := ""
else
# if ADX insructions aren't supported, this CGO flags uses a slower non-ADX BLST implementation 
	CRYPTO_FLAG := "-O -D__BLST_PORTABLE__"
endif
CGO_FLAG := CGO_CFLAGS=$(CRYPTO_FLAG)

# Run the integration test suite
.PHONY: integration-test
integration-test: access-tests ghost-tests mvp-tests execution-tests verification-tests upgrades-tests collection-tests epochs-cohort1-tests epochs-cohort2-tests network-tests consensus-tests

.PHONY: ci-integration-test
ci-integration-test: access-tests ghost-tests mvp-tests epochs-cohort1-tests epochs-cohort2-tests consensus-tests execution-tests verification-tests upgrades-tests network-tests collection-tests

############################################################################################
# CAUTION: DO NOT MODIFY THE TARGETS BELOW! DOING SO WILL BREAK THE FLAKY TEST MONITOR
# In particular, do not skip tests by commenting them out here.

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

.PHONY: epochs-cohort1-tests
epochs-cohort1-tests:
	# Use a higher timeout of 20m for the suite of tests which span full epochs
	$(CGO_FLAG) go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -timeout 20m ./tests/epochs/cohort1/...

.PHONY: epochs-cohort2-tests
epochs-cohort2-tests:
	# Use a higher timeout of 20m for the suite of tests which span full epochs
	$(CGO_FLAG) go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) -timeout 20m ./tests/epochs/cohort2/...

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
.PHONY: bft-gossipsub-tests
bft-gossipsub-tests:
	$(CGO_FLAG) go test $(if $(VERBOSE),-v,) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) ./tests/bft/gossipsub/... -p 1

.PHONY: bft-tests
bft-tests: bft-framework-tests bft-protocol-tests bft-gossipsub-tests
############################################################################################
