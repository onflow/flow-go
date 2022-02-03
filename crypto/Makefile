# Name of the cover profile
COVER_PROFILE := cover.out

IMAGE_TAG := v0.0.7

ADX_SUPPORT := $(shell if ([ -f "/proc/cpuinfo" ] && grep -q -e '^flags.*\badx\b' /proc/cpuinfo); then echo 1; else echo 0; fi)

.PHONY: setup
setup:
	go generate

.PHONY: test
test: setup
	# test all packages with Relic library disabled
	GO111MODULE=on go test -coverprofile=$(COVER_PROFILE) $(if $(TEST_LONG),,-short) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) $(if $(VERBOSE),-v,) ./...
	# test BLS-related packages (with Relic library enabled)
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
