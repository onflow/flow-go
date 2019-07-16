GODOC2MD = GO111MODULE=on go run github.com/lanre-ade/godoc2md
WIRE = GO111MODULE=on go run github.com/google/wire/cmd/wire

.PHONY: test-setup
test-setup:
	docker-compose up --build start_collect_dependencies
	docker-compose up --build start_consensus_dependencies
	docker-compose up --build start_execute_dependencies
	docker-compose up --build start_verify_dependencies
	docker-compose up --build start_seal_dependencies
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
	$(GODOC2MD) github.com/dapperlabs/bamboo-node/internal/roles/collect/clusters > internal/roles/collect/clusters/README.md
	$(GODOC2MD) github.com/dapperlabs/bamboo-node/internal/roles/collect/routing > internal/roles/collect/routing/README.md
	$(GODOC2MD) github.com/dapperlabs/bamboo-node/internal/roles/collect/collections > internal/roles/collect/collections/README.md

.PHONY: generate-proto
generate-proto:
	GO111MODULE=on go get github.com/golang/protobuf/protoc-gen-go
	GO111MODULE=on go run github.com/uber/prototool/cmd/prototool generate proto/

.PHONY: generate-wire
generate-wire:
	$(WIRE) ./internal/roles/collect/
	$(WIRE) ./internal/roles/consensus/
	$(WIRE) ./internal/roles/execute/
	$(WIRE) ./internal/roles/verify/
	$(WIRE) ./internal/roles/seal/

.PHONY: generate
generate: generate-godoc generate-proto generate-wire

.PHONY: check-generated-code
check-generated-code: generate
	./scripts/check-generated-code.sh

.PHONY: build-bamboo
build-bamboo:
	GO111MODULE=on go build -o bamboo ./cmd/bamboo/

.PHONY: ci
ci: check-generated-code test
