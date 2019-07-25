export GO111MODULE := on

.PHONY: install-tools
install-tools:
	go get github.com/psiemens/godoc2md@v1.0.1
	go get github.com/google/wire/cmd/wire@v0.3.0
	go get github.com/golang/protobuf/protoc-gen-go@v1.3.2
	go get github.com/uber/prototool/cmd/prototool@v1.8.0

.PHONY: test-setup
test-setup:
	docker-compose up --build start_collect_dependencies
	docker-compose up --build start_consensus_dependencies
	docker-compose up --build start_execute_dependencies
	docker-compose up --build start_verify_dependencies
	docker-compose up --build start_test_dependencies

.PHONY: test-run
test-run:
	docker-compose up --build --exit-code-from test test

.PHONY: test-teardown
test-teardown:
	docker-compose down

.PHONY: test
test: test-setup test-run test-teardown

.PHONY: generate-godoc
generate-godoc:
	godoc2md github.com/dapperlabs/bamboo-node/internal/roles/collect/clusters > internal/roles/collect/clusters/README.md
	godoc2md github.com/dapperlabs/bamboo-node/internal/roles/collect/routing > internal/roles/collect/routing/README.md
	godoc2md github.com/dapperlabs/bamboo-node/internal/roles/collect/collections > internal/roles/collect/collections/README.md
	godoc2md github.com/dapperlabs/bamboo-node/pkg/data/keyvalue > pkg/data/keyvalue/README.md
	godoc2md github.com/dapperlabs/bamboo-node/internal/roles/verify/processor > internal/roles/verify/processor/README.md

.PHONY: generate-proto
generate-proto:
	prototool generate proto/

.PHONY: generate-wire
generate-wire:
	wire ./internal/roles/collect/
	wire ./internal/roles/consensus/
	wire ./internal/roles/execute/
	wire ./internal/roles/verify/

.PHONY: generate
generate: generate-godoc generate-proto generate-wire

.PHONY: check-generated-code
check-generated-code:
	./scripts/check-generated-code.sh

.PHONY: build-bamboo
build-bamboo:
	go build -o bamboo ./cmd/bamboo/

.PHONY: ci
ci: install-tools generate check-generated-code test
