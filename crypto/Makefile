# Name of the cover profile
COVER_PROFILE := cover.out

IMAGE_TAG := v0.0.7

# allows CI to specify whether to have race detection on / off
ifeq ($(RACE_DETECTOR),1)
	RACE_FLAG := -race
else
	RACE_FLAG :=
endif

ADX_SUPPORT := $(shell if ([ -f "/proc/cpuinfo" ] && grep -q -e '^flags.*\badx\b' /proc/cpuinfo); then echo 1; else echo 0; fi)

.PHONY: setup
setup:
	go generate

# test BLS-related functionalities requiring the Relic library (and hence relic Go build flag)
.PHONY: relic_tests
relic_tests:
ifeq ($(ADX_SUPPORT), 1)
	go test -coverprofile=$(COVER_PROFILE) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) --tags relic $(if $(VERBOSE),-v,)
else
	CGO_CFLAGS="-D__BLST_PORTABLE__" go test -coverprofile=$(COVER_PROFILE) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) --tags relic $(if $(VERBOSE),-v,)
endif

# test all packages that do not require Relic library (all functionalities except the BLS-related ones)
.PHONY: non_relic_tests
non_relic_tests:
# root package without relic 
	go test -coverprofile=$(COVER_PROFILE) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) $(if $(VERBOSE),-v,)
# sub packages
	go test -coverprofile=$(COVER_PROFILE) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) $(if $(VERBOSE),-v,) ./hash
	go test -coverprofile=$(COVER_PROFILE) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) $(if $(VERBOSE),-v,) ./random

############################################################################################
# CAUTION: DO NOT MODIFY THIS TARGET! DOING SO WILL BREAK THE FLAKY TEST MONITOR

# sets up the crypto module and runs all tests
.PHONY: test
test: setup unittest

# runs the unit tests of the module (assumes the module was set up)
.PHONY: unittest
unittest: relic_tests non_relic_tests

############################################################################################

.PHONY: docker-build
docker-build:
	docker build -t gcr.io/dl-flow/golang-cmake:latest -t gcr.io/dl-flow/golang-cmake:$(IMAGE_TAG) .

.PHONY: docker-push
docker-push:
	docker push gcr.io/dl-flow/golang-cmake:latest 
	docker push "gcr.io/dl-flow/golang-cmake:$(IMAGE_TAG)"
