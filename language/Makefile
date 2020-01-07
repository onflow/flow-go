# Name of the cover profile
COVER_PROFILE := cover.out

.PHONY: test
test:
	# test all packages with Relic library enabled
	GO111MODULE=on go test -coverprofile=$(COVER_PROFILE) $(if $(JSON_OUTPUT),-json,) --tags relic ./...
