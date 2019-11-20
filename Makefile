# The short Git commit hash
SHORT_COMMIT := $(shell git rev-parse --short HEAD)
# The Git commit hash
COMMIT := $(shell git rev-parse HEAD)
# The tag of the current commit, otherwise empty
VERSION := $(shell git describe --tags --abbrev=0 --exact-match)

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
	GO111MODULE=on go get github.com/vektra/mockery/cmd/mockery@master

.PHONY: test
test:
	# test all packages with Relic library enabled
	GO111MODULE=on go test --tags relic ./...
	# test SDK package with Relic library disabled
	GO111MODULE=on go test -count 1 ./sdk/...

.PHONY: generate
generate: generate-godoc generate-proto generate-registries generate-mocks

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


.PHONY: check-generated-code
check-generated-code:
	./utils/scripts/check-generated-code.sh

.PHONY: lint-sdk
lint:
	GO111MODULE=on revive -config revive.toml ./...

.PHONY: ci
ci: install-tools generate check-generated-code lint test

.PHONY: docker-ci
docker-ci:
	docker run -v "$(CURDIR)":/go/flow -v "/tmp/.cache":"${HOME}/.cache" -v "/tmp/pkg":"${GOPATH}/pkg" -w "/go/flow" gcr.io/dl-flow/golang-cmake:latest make ci

.PHONY: install-cli
install-cli: crypto/relic/build
	GO111MODULE=on install ./cmd/flow

cmd/flow/flow: crypto/*.go $(shell find  cli/ -name '*.go') $(shell find cmd -name '*.go') $(shell find model -name '*.go') $(shell find proto -name '*.go') $(shell find sdk -name '*.go')
	GO111MODULE=on go build \
	    -ldflags \
	    "-X github.com/dapperlabs/flow-go/cli/flow/version.commit=$(COMMIT) -X github.com/dapperlabs/flow-go/cli/flow/version.version=$(VERSION)" \
	    -o ./cmd/flow/flow ./cmd/flow

.PHONY: docker-build-emulator
docker-build-emulator:
	docker build -f cmd/flow/emulator/Dockerfile -t gcr.io/dl-flow/emulator:latest -t "gcr.io/dl-flow/emulator:$(SHORT_COMMIT)" .

docker-push-emulator:
	docker push gcr.io/dl-flow/emulator:latest
	docker push "gcr.io/dl-flow/emulator:$(SHORT_COMMIT)"

.PHONY: docker-build-consensus
docker-build-consensus:
	docker build -f cmd/consensus/Dockerfile -t gcr.io/dl-flow/consensus:latest -t "gcr.io/dl-flow/consensus:$(SHORT_COMMIT)" .
