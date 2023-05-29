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

# test all packages
.PHONY: test
test:
# root package (it uses BLST source files underneath which requires testing for ADX support)
ifeq ($(ADX_SUPPORT), 1)
	go test -coverprofile=$(COVER_PROFILE) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) $(if $(VERBOSE),-v,)
else
	CGO_CFLAGS="-O -D__BLST_PORTABLE__" go test -coverprofile=$(COVER_PROFILE) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) $(if $(VERBOSE),-v,)
endif
# sub packages
	go test -coverprofile=$(COVER_PROFILE) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) $(if $(VERBOSE),-v,) ./hash
	go test -coverprofile=$(COVER_PROFILE) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) $(if $(VERBOSE),-v,) ./random

.PHONY: docker-build
docker-build:
	docker build -t gcr.io/dl-flow/golang-cmake:latest -t gcr.io/dl-flow/golang-cmake:$(IMAGE_TAG) .

.PHONY: docker-push
docker-push:
	docker push gcr.io/dl-flow/golang-cmake:latest 
	docker push "gcr.io/dl-flow/golang-cmake:$(IMAGE_TAG)"
