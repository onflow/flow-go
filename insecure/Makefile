# Name of the cover profile
COVER_PROFILE := cover.out

# By default, this will run all tests in all packages, but we have a way to override this in CI so that we can
# dynamically split up CI jobs into smaller jobs that can be run in parallel
GO_TEST_PACKAGES := ./...

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
	$(CGO_FLAG) go test $(if $(VERBOSE),-v,) -coverprofile=$(COVER_PROFILE) $(RACE_FLAG) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) $(GO_TEST_PACKAGES)

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
