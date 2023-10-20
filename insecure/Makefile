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

# runs all unit tests of the insecure module
.PHONY: test
test:
	$(CGO_FLAG) go test $(if $(VERBOSE),-v,) -coverprofile=$(COVER_PROFILE) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) ./...
