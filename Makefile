# The short Git commit hash
SHORT_COMMIT := $(shell git rev-parse --short HEAD)
# The Git commit hash
COMMIT := $(shell git rev-parse HEAD)
# The tag of the current commit, otherwise empty
VERSION := $(shell git describe --tags --abbrev=0 --exact-match)
# Name of the cover profile
COVER_PROFILE := cover.out

export FLOW_ABI_EXAMPLES_DIR := $(CURDIR)/language/abi/examples/

crypto/relic:
	rm -rf crypto/relic
	git submodule update --init --recursive

crypto/relic/build: crypto/relic
	./crypto/relic_build.sh

.PHONY: install-tools
install-tools: crypto/relic/build
	cd ${GOPATH}; \
	GO111MODULE=on go get github.com/davecheney/godoc2md@master; \
	GO111MODULE=on go get github.com/golang/protobuf/protoc-gen-go@v1.3.2; \
	GO111MODULE=on go get github.com/uber/prototool/cmd/prototool@v1.9.0; \
	GO111MODULE=on go get github.com/golang/mock/mockgen@v1.3.1; \
	GO111MODULE=on go get github.com/mgechev/revive@master; \
	GO111MODULE=on go get github.com/vektra/mockery/cmd/mockery@master; \
	GO111MODULE=on go get golang.org/x/tools/cmd/stringer@master; \
	GO111MODULE=on go get github.com/kevinburke/go-bindata/...@v3.11.0;

.PHONY: test
test: generate-bindata
	# test all packages with Relic library enabled
	GO111MODULE=on go test -coverprofile=$(COVER_PROFILE) -json --tags relic ./...
	# test SDK package with Relic library disabled
	GO111MODULE=on go test -count 1 ./sdk/...

.PHONY: coverage
coverage:
ifeq ($(COVER), true)
	# file has to be called index.html
	gocov convert $(COVER_PROFILE) | gocov-html > index.html
	# coverage.zip will automatically be picked up by teamcity
	zip coverage.zip index.html
endif

.PHONY: generate
generate: generate-godoc generate-proto generate-registries generate-mocks generate-bindata

.PHONY: generate-godoc
generate-godoc:
	# godoc2md github.com/dapperlabs/flow-go/network/gossip > network/gossip/README.md
	# godoc2md github.com/dapperlabs/flow-go/sdk > sdk/README.md
	# godoc2md github.com/dapperlabs/flow-go/sdk/templates > sdk/templates/README.md
	# godoc2md github.com/dapperlabs/flow-go/sdk/keys > sdk/keys/README.md

.PHONY: generate-proto
generate-proto:
	prototool generate proto

.PHONY: generate-registries
generate-registries:
	GO111MODULE=on go build -o /tmp/registry-generator ./network/gossip/scripts/
	find ./proto/services -type f -iname "*pb.go" -exec /tmp/registry-generator -w {} \;
	rm /tmp/registry-generator

.PHONY: generate-mocks
generate-mocks:
	GO111MODULE=on mockgen -destination=sdk/client/mocks/mock_client.go -package=mocks github.com/dapperlabs/flow-go/sdk/client RPCClient
	mockery -name '.*' -dir=module -case=underscore -output="./module/mock" -outpkg="mock"
	mockery -name '.*' -dir=network -case=underscore -output="./network/mock" -outpkg="mock"
	GO111MODULE=on mockgen -destination=sdk/emulator/mocks/emulated_blockchain_api.go -package=mocks github.com/dapperlabs/flow-go/sdk/emulator EmulatedBlockchainAPI

.PHONY: generate-bindata
generate-bindata:
	go generate ./language/abi;

.PHONY: check-generated-code
check-generated-code:
	./utils/scripts/check-generated-code.sh

.PHONY: lint
lint:
	GO111MODULE=on revive -config revive.toml -exclude cli/flow/cadence/vscode/cadence_bin.go ./...

.PHONY: ci
ci: install-tools generate check-generated-code lint test coverage

.PHONY: docker-ci
docker-ci:
	docker run --env COVER=$(COVER) -v "$(CURDIR)":/go/flow -v "/tmp/.cache":"${HOME}/.cache" -v "/tmp/pkg":"${GOPATH}/pkg" -w "/go/flow" gcr.io/dl-flow/golang-cmake:v0.0.2 make ci

cmd/flow/flow: crypto/*.go $(shell find  cli/ -name '*.go') $(shell find cmd -name '*.go') $(shell find model -name '*.go') $(shell find proto -name '*.go') $(shell find sdk -name '*.go')
	GO111MODULE=on go build \
	    -ldflags \
	    "-X github.com/dapperlabs/flow-go/cli/flow/version.commit=$(COMMIT) -X github.com/dapperlabs/flow-go/cli/flow/version.version=$(VERSION)" \
	    -o ./cmd/flow/flow ./cmd/flow

sdk/abi/generation/generation: $(shell find sdk -name '*.go')
	GO111MODULE=on go build \
    	    -ldflags \
    	    "-X github.com/dapperlabs/flow-go/cli/flow/version.commit=$(COMMIT) -X github.com/dapperlabs/flow-go/cli/flow/version.version=$(VERSION)" \
    	    -o ./sdk/abi/generation/generation ./sdk/abi/generation

.PHONY: install-cli
install-cli: cmd/flow/flow
	cp cmd/flow/flow $$GOPATH/bin/

.PHONY: docker-build-emulator
docker-build-emulator:
	docker build -f cmd/flow/emulator/Dockerfile -t gcr.io/dl-flow/emulator:latest -t "gcr.io/dl-flow/emulator:$(SHORT_COMMIT)" .

docker-push-emulator:
	docker push gcr.io/dl-flow/emulator:latest
	docker push "gcr.io/dl-flow/emulator:$(SHORT_COMMIT)"

.PHONY: docker-build-consensus
docker-build-consensus:
	docker build -f cmd/consensus/Dockerfile -t gcr.io/dl-flow/consensus:latest -t "gcr.io/dl-flow/consensus:$(SHORT_COMMIT)" .

# Builds the VS Code extension
.PHONY: build-vscode-extension
build-vscode-extension:
	cd language/tools/vscode-extension && npm run package;

# Builds and promotes the VS Code extension. Promoting here means re-generating
# the embedded extension binary used in the CLI for installation.
.PHONY: promote-vscode-extension
promote-vscode-extension: build-vscode-extension
	cp language/tools/vscode-extension/cadence-*.vsix ./cli/flow/cadence/vscode/cadence.vsix;
	go generate ./cli/flow/cadence/vscode;

