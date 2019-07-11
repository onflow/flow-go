GODOC2MD = GO111MODULE=on go run github.com/lanre-ade/godoc2md

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
	docker-compose up --build test

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
	GO111MODULE=on go run github.com/uber/prototool/cmd/prototool generate proto/

.PHONY: generate
generate: generate-godoc generate-proto
