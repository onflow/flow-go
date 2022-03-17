# Name of the cover profile
COVER_PROFILE := cover.out

IMAGE_TAG := v0.0.7

ADX_SUPPORT := $(shell if ([ -f "/proc/cpuinfo" ] && grep -q -e '^flags.*\badx\b' /proc/cpuinfo); then echo 1; else echo 0; fi)

############################################################################################
# CAUTION: DO NOT MODIFY THESE TARGETS! DOING SO WILL BREAK THE FLAKY TEST MONITOR

.PHONY: setup
setup:
	go generate

.PHONY: test-main
test-main:
	GO111MODULE=on go test -coverprofile=$(COVER_PROFILE) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) $(if $(VERBOSE),-v,) ./...

############################################################################################

.PHONY: test
test: setup
	test-main
ifeq ($(ADX_SUPPORT), 1)
	GO111MODULE=on go test -coverprofile=$(COVER_PROFILE) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) --tags relic $(if $(VERBOSE),-v,)
else
	CGO_CFLAGS="-D__BLST_PORTABLE__" GO111MODULE=on go test -coverprofile=$(COVER_PROFILE) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) --tags relic $(if $(VERBOSE),-v,)
endif

.PHONY: docker-build
docker-build:
	docker build -t gcr.io/dl-flow/golang-cmake:latest -t gcr.io/dl-flow/golang-cmake:$(IMAGE_TAG) .

.PHONY: docker-push
docker-push:
	docker push gcr.io/dl-flow/golang-cmake:latest 
	docker push "gcr.io/dl-flow/golang-cmake:$(IMAGE_TAG)"
