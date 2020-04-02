# Name of the cover profile
COVER_PROFILE := cover.out

# Run unit tests for test utilities in this module
.PHONY: test
test:
	GO111MODULE=on go test -tags relic -coverprofile=$(COVER_PROFILE) $(if $(JSON_OUTPUT),-json,) `go list ./... | grep -v integration/tests`

# Run the integration test suite
.PHONY: integration-test
integration-test:
	GO111MODULE=on go test -tags relic -count=1 `go list ./... | grep integration/tests`
