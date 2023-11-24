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

# runs all unit tests of the insecure module
.PHONY: test
test:
	$(CGO_FLAG) go test $(if $(VERBOSE),-v,) -coverprofile=$(COVER_PROFILE) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) ./...

.PHONY: lint
lint: tidy
	# revive -config revive.toml -exclude storage/ledger/trie ./...
	golangci-lint run -v

# this ensures there is no unused dependency being added by accident
.PHONY: tidy
tidy:
	go mod tidy -v
	cd integration; go mod tidy -v
	cd crypto; go mod tidy -v
	cd cmd/testclient; go mod tidy -v
	cd insecure; go mod tidy -v
	git diff --exit-code
