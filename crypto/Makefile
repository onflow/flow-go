# Name of the cover profile
COVER_PROFILE := cover.out

IMAGE_TAG := v0.0.7

ADX_SUPPORT := $(shell if ([ -f "/proc/cpuinfo" ] && grep -q -e '^flags.*\badx\b' /proc/cpuinfo); then echo 1; else echo 0; fi)

.PHONY: setup
setup:
	go generate

# test BLS-related functionalities requiring the Relic library (and hence relic Go build flag)
.PHONY: relic_tests
relic_tests: setup
ifeq ($(ADX_SUPPORT), 1)
	GO111MODULE=on go test -coverprofile=$(COVER_PROFILE) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) --tags relic $(if $(VERBOSE),-v,)
else
	CGO_CFLAGS="-D__BLST_PORTABLE__" GO111MODULE=on go test -coverprofile=$(COVER_PROFILE) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) --tags relic $(if $(VERBOSE),-v,)
endif

# test all packages that do not require Relic library (all functionalities except the BLS-related ones)
.PHONY: non_relic_tests
non_relic_tests:
# root package without relic 
	GO111MODULE=on go test -coverprofile=$(COVER_PROFILE) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) $(if $(VERBOSE),-v,)
# sub packages
	GO111MODULE=on go test -coverprofile=$(COVER_PROFILE) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) $(if $(VERBOSE),-v,) ./hash
	GO111MODULE=on go test -coverprofile=$(COVER_PROFILE) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) $(if $(VERBOSE),-v,) ./random


# runs all tests of the crypto package 
.PHONY: test
test: relic_tests non_relic_tests

# target used by the flakey test monitor 
# CAUTION: modifying the target name will break the flaky test monitor)
.PHONY: test-main
test-main: test

.PHONY: docker-build
docker-build:
	docker build -t gcr.io/dl-flow/golang-cmake:latest -t gcr.io/dl-flow/golang-cmake:$(IMAGE_TAG) .

.PHONY: docker-push
docker-push:
	docker push gcr.io/dl-flow/golang-cmake:latest 
	docker push "gcr.io/dl-flow/golang-cmake:$(IMAGE_TAG)"
