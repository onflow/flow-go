REVISION := $(shell git rev-parse --short HEAD)

.PHONY: build-relic
build-relic:
	./pkg/crypto/relic_build.sh

.PHONY: install-tools
install-tools: build-relic
	cd ${GOPATH}; \
	GO111MODULE=on go get github.com/psiemens/godoc2md@v1.0.1; \
	GO111MODULE=on go get github.com/google/wire/cmd/wire@v0.3.0; \
	GO111MODULE=on go get github.com/golang/protobuf/protoc-gen-go@v1.3.2; \
	GO111MODULE=on go get github.com/uber/prototool/cmd/prototool@v1.9.0; \
	GO111MODULE=on go get github.com/golang/mock/mockgen@v1.3.1
	GO111MODULE=on go get golang.org/x/lint/golint@master

.PHONY: test
test:
	GO111MODULE=on go test ./...

.PHONY: generate-godoc
generate-godoc:
	godoc2md github.com/dapperlabs/flow-go/internal/roles/collect/clusters > internal/roles/collect/clusters/README.md
	godoc2md github.com/dapperlabs/flow-go/internal/roles/collect/routing > internal/roles/collect/routing/README.md
	godoc2md github.com/dapperlabs/flow-go/internal/roles/collect/collections > internal/roles/collect/collections/README.md
	godoc2md github.com/dapperlabs/flow-go/internal/roles/collect/controller > internal/roles/collect/controller/README.md
	godoc2md github.com/dapperlabs/flow-go/internal/roles/verify/processor > internal/roles/verify/processor/README.md
	godoc2md github.com/dapperlabs/flow-go/pkg/data/keyvalue > pkg/data/keyvalue/README.md
	godoc2md github.com/dapperlabs/flow-go/pkg/network/gossip/v1 > pkg/network/gossip/v1/README.md
	godoc2md github.com/dapperlabs/flow-go/sdk > sdk/README.md
	godoc2md github.com/dapperlabs/flow-go/sdk/templates > sdk/templates/README.md
	godoc2md github.com/dapperlabs/flow-go/sdk/keys > sdk/keys/README.md

.PHONY: generate-proto
generate-proto:
	prototool generate proto/

.PHONY: generate-wire
generate-wire:
	GO111MODULE=on wire ./internal/roles/collect/
	GO111MODULE=on wire ./internal/roles/execute/
	GO111MODULE=on wire ./internal/roles/verify/

.PHONY: generate-mocks
generate-mocks:
	GO111MODULE=on mockgen -destination=sdk/client/mocks/mock_client.go -package=mocks github.com/dapperlabs/flow-go/sdk/client RPCClient

.PHONY: generate-registries
generate-registries:
	GO111MODULE=on go build -o /tmp/registry-generator ./pkg/network/gossip/v1/scripts/
	find ./pkg/grpc/services -type f -iname "*pb.go" -exec /tmp/registry-generator -w {} \;
	rm /tmp/registry-generator

.PHONY: generate
generate: generate-godoc generate-proto generate-registries generate-wire generate-mocks

.PHONY: check-generated-code
check-generated-code:
	./check-generated-code.sh

.PHONY: build-cli
build-cli:
	go build -o flow ./cmd/flow/

.PHONY: install-cli
install-cli: build-relic
	go install ./cmd/flow

.PHONY: lint
lint:
	GO111MODULE=on golint ./sdk/emulator/... ./sdk/client/... ./sdk/templates/...

.PHONY: ci
ci: install-tools generate check-generated-code lint test

.PHONY: docker-build-emulator
docker-build-emulator:
	docker build -f cmd/emulator/Dockerfile -t gcr.io/dl-flow/emulator:latest -t gcr.io/dl-flow/emulator:$(REVISION) .

.PHONY: docker-build-collect
docker-build-collect:
	docker build -f cmd/collect/Dockerfile -t gcr.io/dl-flow/collect:latest -t gcr.io/dl-flow/collect:$(REVISION) .

.PHONY: docker-build-consensus
docker-build-consensus:
	docker build -f cmd/consensus/Dockerfile -t gcr.io/dl-flow/consensus:latest -t gcr.io/dl-flow/consensus:$(REVISION)" .

.PHONY: docker-build-execute
docker-build-execute:
	docker build -f cmd/execute/Dockerfile -t gcr.io/dl-flow/execute:latest -t gcr.io/dl-flow/execute:$(REVISION)" .

.PHONY: docker-build-observe
docker-build-observe:
	docker build -f cmd/observe/Dockerfile -t gcr.io/dl-flow/observe:latest -t gcr.io/dl-flow/observe:$(REVISION)" .

.PHONY: docker-build-verify
docker-build-verify:
	docker build -f cmd/verify/Dockerfile -t gcr.io/dl-flow/verify:latest -t gcr.io/dl-flow/verify:$(REVISION)" .
