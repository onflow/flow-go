REVISION := $(shell git rev-parse --short HEAD)

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
	GO111MODULE=on go get golang.org/x/lint/golint@master

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

.PHONY: check-generated-code
check-generated-code:
	./utils/scripts/check-generated-code.sh

.PHONY: lint-sdk
lint-sdk:
	GO111MODULE=on golint ./sdk/emulator/... ./sdk/client/... ./sdk/templates/...

.PHONY: ci
ci: install-tools generate check-generated-code lint-sdk test

.PHONY: install-cli
install-cli: crypto/relic/build
	GO111MODULE=on install ./cmd/flow

cmd/flow/flow: cli cmd crypto model proto sdk
	GO111MODULE=on go build -o ./cmd/flow/flow ./cmd/flow

.PHONY: docker-build-emulator
docker-build-emulator:
	docker build -f cmd/flow/emulator/Dockerfile -t gcr.io/dl-flow/emulator:latest -t "gcr.io/dl-flow/emulator:$(REVISION)" .

.PHONY: docker-build-consensus
docker-build-consensus:
	docker build -f cmd/consensus/Dockerfile -t gcr.io/dl-flow/consensus:latest -t "gcr.io/dl-flow/consensus:$(REVISION)" .
