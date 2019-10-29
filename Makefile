REVISION := $(shell git rev-parse --short HEAD)

.PHONY: build-relic
build-relic:
	./crypto/relic_build.sh

.PHONY: install-tools
install-tools: build-relic
	cd ${GOPATH}; \
	GO111MODULE=on go get github.com/davecheney/godoc2md@master; \
	GO111MODULE=on go get github.com/golang/protobuf/protoc-gen-go@v1.3.2; \
	GO111MODULE=on go get github.com/uber/prototool/cmd/prototool@v1.9.0; \
	GO111MODULE=on go get github.com/golang/mock/mockgen@v1.3.1; \
	GO111MODULE=on go get golang.org/x/lint/golint@master

.PHONY: test
test:
	GO111MODULE=on go test ./...

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

.PHONY: install-cli
install-cli: build-relic
	go install ./cmd/flow

.PHONY: lint-sdk
lint-sdk:
	GO111MODULE=on golint ./sdk/emulator/... ./sdk/client/... ./sdk/templates/...

.PHONY: ci
ci: install-tools generate check-generated-code lint-sdk test

.PHONY: docker-build-emulator
docker-build-emulator:
	docker build -f cmd/emulator/Dockerfile -t gcr.io/dl-flow/emulator:latest -t "gcr.io/dl-flow/emulator:$(REVISION)" .

.PHONY: docker-build-consensus
docker-build-consensus:
	docker build -f cmd/consensus/Dockerfile -t gcr.io/dl-flow/consensus:latest -t "gcr.io/dl-flow/consensus:$(REVISION)" .
