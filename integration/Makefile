# Name of the cover profile
COVER_PROFILE := cover.out

.PHONY: test
test:
	# test all packages with Relic library enabled
	GO111MODULE=on go test -coverprofile=$(COVER_PROFILE) $(if $(JSON_OUTPUT),-json,) ./network

.PHONY: integration-test
integration-test:
	GO111MODULE=on go test $(go list ./... | grep -v /network/)
