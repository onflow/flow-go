# runs all unit tests of the insecure module
.PHONY: test
test:
	go test -coverprofile=$(COVER_PROFILE) $(if $(RACE_DETECTOR),-race,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) --tags relic $(if $(VERBOSE),-v,) ./...
