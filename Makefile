export GO111MODULE := on

.PHONY: build-relic
build-relic:
	./pkg/crypto/relic_build.sh

.PHONY: install-tools
install-tools: build-relic
	go get github.com/psiemens/godoc2md@v1.0.1
	go get github.com/google/wire/cmd/wire@v0.3.0
	go get github.com/golang/protobuf/protoc-gen-go@v1.3.2
	go get github.com/uber/prototool/cmd/prototool@v1.8.0
	go get github.com/golang/mock/mockgen@v1.3.1

.PHONY: test
test:
	go test ./...

.PHONY: generate-godoc
generate-godoc:
	godoc2md github.com/dapperlabs/flow-go/internal/roles/collect/clusters > internal/roles/collect/clusters/README.md
	godoc2md github.com/dapperlabs/flow-go/internal/roles/collect/routing > internal/roles/collect/routing/README.md
	godoc2md github.com/dapperlabs/flow-go/internal/roles/collect/collections > internal/roles/collect/collections/README.md
	godoc2md github.com/dapperlabs/flow-go/internal/roles/collect/controller > internal/roles/collect/controller/README.md
	godoc2md github.com/dapperlabs/flow-go/internal/roles/verify/processor > internal/roles/verify/processor/README.md
	godoc2md github.com/dapperlabs/flow-go/pkg/data/keyvalue > pkg/data/keyvalue/README.md
	godoc2md github.com/dapperlabs/flow-go/sdk > sdk/README.md
	godoc2md github.com/dapperlabs/flow-go/sdk/accounts > sdk/accounts/README.md

.PHONY: generate-proto
generate-proto:
	prototool generate proto/

.PHONY: generate-wire
generate-wire:
	wire ./internal/roles/collect/
	wire ./internal/roles/consensus/
	wire ./internal/roles/execute/
	wire ./internal/roles/verify/

.PHONY: generate-mocks
generate-mocks:
	mockgen -destination=sdk/client/mocks/mock_client.go -package=mocks github.com/dapperlabs/flow-go/sdk/client RPCClient

.PHONY: generate
generate: generate-godoc generate-proto generate-wire generate-mocks

.PHONY: check-generated-code
check-generated-code:
	./scripts/check-generated-code.sh

.PHONY: build-cli
build-cli:
	go build -o flow ./cmd/flow/

.PHONY: install-cli
install-cli: build-relic
	go install ./cmd/flow

.PHONY: ci
ci: install-tools generate check-generated-code test
