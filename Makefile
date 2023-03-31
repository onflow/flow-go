# The short Git commit hash
SHORT_COMMIT := $(shell git rev-parse --short HEAD)
BRANCH_NAME:=$(shell git rev-parse --abbrev-ref HEAD | tr '/' '-')
# The Git commit hash
COMMIT := $(shell git rev-parse HEAD)
# The tag of the current commit, otherwise empty
VERSION := $(shell git describe --tags --abbrev=2 --match "v*" --match "secure-cadence*" 2>/dev/null)

# By default, this will run all tests in all packages, but we have a way to override this in CI so that we can
# dynamically split up CI jobs into smaller jobs that can be run in parallel
GO_TEST_PACKAGES := ./...

FLOW_GO_TAG := v0.28.15


# Image tag: if image tag is not set, set it with version (or short commit if empty)
ifeq (${IMAGE_TAG},)
IMAGE_TAG := ${VERSION}
endif

ifeq (${IMAGE_TAG},)
IMAGE_TAG := ${SHORT_COMMIT}
endif

IMAGE_TAG_NO_NETGO := $(IMAGE_TAG)-without_netgo

# Name of the cover profile
COVER_PROFILE := coverage.txt
# Disable go sum database lookup for private repos
GOPRIVATE=github.com/dapperlabs/*
# OS
UNAME := $(shell uname)

# Used when building within docker
GOARCH := $(shell go env GOARCH)

# The location of the k8s YAML files
K8S_YAMLS_LOCATION_STAGING=./k8s/staging


# docker container registry
export CONTAINER_REGISTRY := gcr.io/flow-container-registry
export DOCKER_BUILDKIT := 1

# setup the crypto package under the GOPATH: needed to test packages importing flow-go/crypto
.PHONY: crypto_setup_gopath
crypto_setup_gopath:
	bash crypto_setup.sh

cmd/collection/collection:
	go build -o cmd/collection/collection cmd/collection/main.go

cmd/util/util:
	go build -o cmd/util/util --tags relic cmd/util/main.go

.PHONY: update-core-contracts-version
update-core-contracts-version:
	./scripts/update-core-contracts.sh $(CC_VERSION)

############################################################################################
# CAUTION: DO NOT MODIFY THESE TARGETS! DOING SO WILL BREAK THE FLAKY TEST MONITOR

.PHONY: unittest-main
unittest-main:
	# test all packages with Relic library enabled
	go test $(if $(VERBOSE),-v,) -coverprofile=$(COVER_PROFILE) -covermode=atomic $(if $(RACE_DETECTOR),-race,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) --tags relic $(GO_TEST_PACKAGES)

.PHONY: install-mock-generators
install-mock-generators:
	cd ${GOPATH}; \
    go install github.com/vektra/mockery/v2@v2.21.4; \
    go install github.com/golang/mock/mockgen@v1.6.0;

.PHONY: install-tools
install-tools: crypto_setup_gopath check-go-version install-mock-generators
	cd ${GOPATH}; \
	go install github.com/golang/protobuf/protoc-gen-go@v1.3.2; \
	go install github.com/uber/prototool/cmd/prototool@v1.9.0; \
	go install github.com/gogo/protobuf/protoc-gen-gofast@latest; \
	go install golang.org/x/tools/cmd/stringer@master;

.PHONY: verify-mocks
verify-mocks: generate-mocks
	git diff --exit-code

############################################################################################

.PHONY: emulator-norelic-check
emulator-norelic-check:
	# test the fvm package compiles with Relic library disabled (required for the emulator build)
	cd ./fvm && go test ./... -run=NoTestHasThisPrefix

.PHONY: fuzz-fvm
fuzz-fvm:
	# run fuzz tests in the fvm package
	cd ./fvm && go test -fuzz=Fuzz -run ^$$ --tags relic

.PHONY: test
test: verify-mocks unittest-main

.PHONY: integration-test
integration-test: docker-build-flow
	$(MAKE) -C integration integration-test

.PHONY: benchmark
benchmark: docker-build-flow
	$(MAKE) -C integration benchmark

.PHONY: coverage
coverage:
ifeq ($(COVER), true)
	# Cover summary has to produce cover.json
	COVER_PROFILE=$(COVER_PROFILE) ./cover-summary.sh
	# file has to be called index.html
	gocov-html cover.json > index.html
	# coverage.zip will automatically be picked up by teamcity
	zip coverage.zip index.html
endif

.PHONY: generate-openapi
generate-openapi:
	swagger-codegen generate -l go -i https://raw.githubusercontent.com/onflow/flow/master/openapi/access.yaml -D packageName=models,modelDocs=false,models -o engine/access/rest/models;
	go fmt ./engine/access/rest/models

.PHONY: generate
generate: generate-proto generate-mocks generate-fvm-env-wrappers

.PHONY: generate-proto
generate-proto:
	prototool generate protobuf

.PHONY: generate-fvm-env-wrappers
generate-fvm-env-wrappers:
	go run ./fvm/environment/generate-wrappers fvm/environment/parse_restricted_checker.go

.PHONY: generate-mocks
generate-mocks: install-mock-generators
	mockery --name '(Connector|PingInfoProvider)' --dir=network/p2p --case=underscore --output="./network/mocknetwork" --outpkg="mocknetwork"
	mockgen -destination=storage/mocks/storage.go -package=mocks github.com/onflow/flow-go/storage Blocks,Headers,Payloads,Collections,Commits,Events,ServiceEvents,TransactionResults
	mockgen -destination=module/mocks/network.go -package=mocks github.com/onflow/flow-go/module Local,Requester
	mockgen -destination=network/mocknetwork/mock_network.go -package=mocknetwork github.com/onflow/flow-go/network Network
	mockery --name='.*' --dir=integration/benchmark/mocksiface --case=underscore --output="integration/benchmark/mock" --outpkg="mock"
	mockery --name=ExecutionDataStore --dir=module/executiondatasync/execution_data --case=underscore --output="./module/executiondatasync/execution_data/mock" --outpkg="mock"
	mockery --name=Downloader --dir=module/executiondatasync/execution_data --case=underscore --output="./module/executiondatasync/execution_data/mock" --outpkg="mock"
	mockery --name 'ExecutionDataRequester' --dir=module/state_synchronization --case=underscore --output="./module/state_synchronization/mock" --outpkg="state_synchronization"
	mockery --name 'ExecutionState' --dir=engine/execution/state --case=underscore --output="engine/execution/state/mock" --outpkg="mock"
	mockery --name 'BlockComputer' --dir=engine/execution/computation/computer --case=underscore --output="engine/execution/computation/computer/mock" --outpkg="mock"
	mockery --name 'ComputationManager' --dir=engine/execution/computation --case=underscore --output="engine/execution/computation/mock" --outpkg="mock"
	mockery --name 'EpochComponentsFactory' --dir=engine/collection/epochmgr --case=underscore --output="engine/collection/epochmgr/mock" --outpkg="mock"
	mockery --name 'Backend' --dir=engine/collection/rpc --case=underscore --output="engine/collection/rpc/mock" --outpkg="mock"
	mockery --name 'ProviderEngine' --dir=engine/execution/provider --case=underscore --output="engine/execution/provider/mock" --outpkg="mock"
	(cd ./crypto && mockery --name 'PublicKey' --case=underscore --output="../module/mock" --outpkg="mock")
	mockery --name '.*' --dir=state/cluster --case=underscore --output="state/cluster/mock" --outpkg="mock"
	mockery --name '.*' --dir=module --case=underscore --tags="relic" --output="./module/mock" --outpkg="mock"
	mockery --name '.*' --dir=module/mempool --case=underscore --output="./module/mempool/mock" --outpkg="mempool"
	mockery --name '.*' --dir=module/component --case=underscore --output="./module/component/mock" --outpkg="component"
	mockery --name '.*' --dir=network --case=underscore --output="./network/mocknetwork" --outpkg="mocknetwork"
	mockery --name '.*' --dir=storage --case=underscore --output="./storage/mock" --outpkg="mock"
	mockery --name '.*' --dir="state/protocol" --case=underscore --output="state/protocol/mock" --outpkg="mock"
	mockery --name '.*' --dir="state/protocol/events" --case=underscore --output="./state/protocol/events/mock" --outpkg="mock"
	mockery --name '.*' --dir=engine/execution/computation/computer --case=underscore --output="./engine/execution/computation/computer/mock" --outpkg="mock"
	mockery --name '.*' --dir=engine/execution/state --case=underscore --output="./engine/execution/state/mock" --outpkg="mock"
	mockery --name '.*' --dir=engine/collection --case=underscore --output="./engine/collection/mock" --outpkg="mock"
	mockery --name '.*' --dir=engine/common/follower/cache --case=underscore --output="./engine/common/follower/cache/mock" --outpkg="mock"
	mockery --name '.*' --dir=engine/consensus --case=underscore --output="./engine/consensus/mock" --outpkg="mock"
	mockery --name '.*' --dir=engine/consensus/approvals --case=underscore --output="./engine/consensus/approvals/mock" --outpkg="mock"
	rm -rf ./fvm/mock
	mockery --name '.*' --dir=fvm --case=underscore --output="./fvm/mock" --outpkg="mock"
	rm -rf ./fvm/environment/mock
	mockery --name '.*' --dir=fvm/environment --case=underscore --output="./fvm/environment/mock" --outpkg="mock"
	mockery --name '.*' --dir=ledger --case=underscore --output="./ledger/mock" --outpkg="mock"
	mockery --name 'ViolationsConsumer' --dir=network/slashing --case=underscore --output="./network/mocknetwork" --outpkg="mocknetwork"
	mockery --name '.*' --dir=network/p2p/ --case=underscore --output="./network/p2p/mock" --outpkg="mockp2p"
	mockery --name 'Vertex' --dir="./module/forest" --case=underscore --output="./module/forest/mock" --outpkg="mock"
	mockery --name '.*' --dir="./consensus/hotstuff" --case=underscore --output="./consensus/hotstuff/mocks" --outpkg="mocks"
	mockery --name '.*' --dir="./engine/access/wrapper" --case=underscore --output="./engine/access/mock" --outpkg="mock"
	mockery --name 'API' --dir="./access" --case=underscore --output="./access/mock" --outpkg="mock"
	mockery --name 'API' --dir="./engine/protocol" --case=underscore --output="./engine/protocol/mock" --outpkg="mock"
	mockery --name 'API' --dir="./engine/access/state_stream" --case=underscore --output="./engine/access/state_stream/mock" --outpkg="mock"
	mockery --name 'ConnectionFactory' --dir="./engine/access/rpc/backend" --case=underscore --output="./engine/access/rpc/backend/mock" --outpkg="mock"
	mockery --name 'IngestRPC' --dir="./engine/execution/ingestion" --case=underscore --tags relic --output="./engine/execution/ingestion/mock" --outpkg="mock"
	mockery --name '.*' --dir=model/fingerprint --case=underscore --output="./model/fingerprint/mock" --outpkg="mock"
	mockery --name 'ExecForkActor' --structname 'ExecForkActorMock' --dir=module/mempool/consensus/mock/ --case=underscore --output="./module/mempool/consensus/mock/" --outpkg="mock"
	mockery --name '.*' --dir=engine/verification/fetcher/ --case=underscore --output="./engine/verification/fetcher/mock" --outpkg="mockfetcher"
	mockery --name '.*' --dir=./cmd/util/ledger/reporters --case=underscore --output="./cmd/util/ledger/reporters/mock" --outpkg="mock"
	mockery --name 'Storage' --dir=module/executiondatasync/tracker --case=underscore --output="module/executiondatasync/tracker/mock" --outpkg="mocktracker"

	#temporarily make insecure/ a non-module to allow mockery to create mocks
	mv insecure/go.mod insecure/go2.mod
	mockery --name '.*' --dir=insecure/ --case=underscore --output="./insecure/mock"  --outpkg="mockinsecure"
	mv insecure/go2.mod insecure/go.mod

# this ensures there is no unused dependency being added by accident
.PHONY: tidy
tidy:
	go mod tidy -v
	cd integration; go mod tidy -v
	cd crypto; go mod tidy -v
	cd cmd/testclient; go mod tidy -v
	cd insecure; go mod tidy -v
	git diff --exit-code

.PHONY: lint
lint: tidy
	# revive -config revive.toml -exclude storage/ledger/trie ./...
	golangci-lint run -v --build-tags relic ./...

.PHONY: fix-lint
fix-lint:
	# revive -config revive.toml -exclude storage/ledger/trie ./...
	golangci-lint run -v --build-tags relic --fix ./...

# Runs unit tests with different list of packages as passed by CI so they run in parallel
.PHONY: ci
ci: install-tools test

# Runs integration tests
.PHONY: ci-integration
ci-integration: crypto_setup_gopath
	$(MAKE) -C integration ci-integration-test

# Runs benchmark tests
# NOTE: we do not need `docker-build-flow` as this is run as a separate step
# on Teamcity
.PHONY: ci-benchmark
ci-benchmark: install-tools
	$(MAKE) -C integration ci-benchmark

# Runs unit tests, test coverage, lint in Docker (for mac)
.PHONY: docker-ci
docker-ci:
	docker run --env RACE_DETECTOR=$(RACE_DETECTOR) --env COVER=$(COVER) --env JSON_OUTPUT=$(JSON_OUTPUT) \
		-v /run/host-services/ssh-auth.sock:/run/host-services/ssh-auth.sock -e SSH_AUTH_SOCK="/run/host-services/ssh-auth.sock" \
		-v "$(CURDIR)":/go/flow -v "/tmp/.cache":"/root/.cache" -v "/tmp/pkg":"/go/pkg" \
		-w "/go/flow" "$(CONTAINER_REGISTRY)/golang-cmake:v0.0.7" \
		make ci

# Runs integration tests in Docker  (for mac)
.PHONY: docker-ci-integration
docker-ci-integration:
	rm -rf crypto/relic
	docker run \
		--env DOCKER_API_VERSION='1.39' \
		--network host \
		-v "$(CURDIR)":/go/flow -v "/tmp/.cache":"/root/.cache" -v "/tmp/pkg":"/go/pkg" \
		-v /tmp:/tmp \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v /run/host-services/ssh-auth.sock:/run/host-services/ssh-auth.sock -e SSH_AUTH_SOCK="/run/host-services/ssh-auth.sock" \
		-w "/go/flow" "$(CONTAINER_REGISTRY)/golang-cmake:v0.0.7" \
		make ci-integration

.PHONY: docker-build-collection
docker-build-collection:
	docker build -f cmd/Dockerfile  --build-arg TARGET=./cmd/collection --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG) --build-arg GOARCH=$(GOARCH) --target production \
		--label "git_commit=${COMMIT}" --label "git_tag=${IMAGE_TAG}" \
		-t "$(CONTAINER_REGISTRY)/collection:latest" -t "$(CONTAINER_REGISTRY)/collection:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/collection:$(IMAGE_TAG)" -t  "$(CONTAINER_REGISTRY)/collection:$(FLOW_GO_TAG)" .

.PHONY: docker-build-collection-without-netgo
docker-build-collection-without-netgo:
	docker build -f cmd/Dockerfile  --build-arg TAGS=relic --build-arg TARGET=./cmd/collection --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG_NO_NETGO) --build-arg GOARCH=$(GOARCH) --target production \
		--label "git_commit=${COMMIT}" --label "git_tag=$(IMAGE_TAG_NO_NETGO)" -t "$(CONTAINER_REGISTRY)/collection:$(IMAGE_TAG_NO_NETGO)"  .

.PHONY: docker-build-collection-debug
docker-build-collection-debug:
	docker build -f cmd/Dockerfile  --build-arg TARGET=./cmd/collection --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG) --build-arg GOARCH=$(GOARCH) --target debug \
		-t "$(CONTAINER_REGISTRY)/collection-debug:latest" -t "$(CONTAINER_REGISTRY)/collection-debug:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/collection-debug:$(IMAGE_TAG)" .

.PHONY: docker-build-consensus
docker-build-consensus:
	docker build -f cmd/Dockerfile  --build-arg TARGET=./cmd/consensus --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG) --build-arg GOARCH=$(GOARCH) --target production \
		--label "git_commit=${COMMIT}" --label "git_tag=${IMAGE_TAG}" \
		-t "$(CONTAINER_REGISTRY)/consensus:latest" -t "$(CONTAINER_REGISTRY)/consensus:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/consensus:$(IMAGE_TAG)" -t  "$(CONTAINER_REGISTRY)/consensus:$(FLOW_GO_TAG)" .

.PHONY: docker-build-consensus-without-netgo
docker-build-consensus-without-netgo:
	docker build -f cmd/Dockerfile  --build-arg TAGS=relic --build-arg TARGET=./cmd/consensus --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG_NO_NETGO) --build-arg GOARCH=$(GOARCH) --target production \
		--label "git_commit=${COMMIT}" --label "git_tag=$(IMAGE_TAG_NO_NETGO)" -t "$(CONTAINER_REGISTRY)/consensus:$(IMAGE_TAG_NO_NETGO)" .

.PHONY: docker-build-consensus-debug
docker-build-consensus-debug:
	docker build -f cmd/Dockerfile  --build-arg TARGET=./cmd/consensus --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG) --build-arg GOARCH=$(GOARCH) --target debug \
		-t "$(CONTAINER_REGISTRY)/consensus-debug:latest" -t "$(CONTAINER_REGISTRY)/consensus-debug:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/consensus-debug:$(IMAGE_TAG)" .

.PHONY: docker-build-execution
docker-build-execution:
	docker build -f cmd/Dockerfile  --build-arg TARGET=./cmd/execution --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG) --build-arg GOARCH=$(GOARCH) --target production \
		--label "git_commit=${COMMIT}" --label "git_tag=${IMAGE_TAG}" \
		-t "$(CONTAINER_REGISTRY)/execution:latest" -t "$(CONTAINER_REGISTRY)/execution:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/execution:$(IMAGE_TAG)" -t  "$(CONTAINER_REGISTRY)/execution:$(FLOW_GO_TAG)" .

.PHONY: docker-build-execution-without-netgo
docker-build-execution-without-netgo:
	docker build -f cmd/Dockerfile  --build-arg TAGS=relic --build-arg TARGET=./cmd/execution --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG_NO_NETGO) --build-arg GOARCH=$(GOARCH) --target production \
		--label "git_commit=${COMMIT}" --label "git_tag=$(IMAGE_TAG_NO_NETGO)" -t "$(CONTAINER_REGISTRY)/execution:$(IMAGE_TAG_NO_NETGO)" .

.PHONY: docker-build-execution-debug
docker-build-execution-debug:
	docker build -f cmd/Dockerfile  --build-arg TARGET=./cmd/execution --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG) --build-arg GOARCH=$(GOARCH) --target debug \
		-t "$(CONTAINER_REGISTRY)/execution-debug:latest" -t "$(CONTAINER_REGISTRY)/execution-debug:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/execution-debug:$(IMAGE_TAG)" .

# build corrupt execution node for BFT testing
.PHONY: docker-build-execution-corrupt
docker-build-execution-corrupt:
	# temporarily make insecure/ a non-module to allow Docker to use corrupt builders there
	./insecure/cmd/mods_override.sh
	docker build -f cmd/Dockerfile  --build-arg TARGET=./insecure/cmd/execution --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG) --build-arg GOARCH=$(GOARCH) --target production \
		--label "git_commit=${COMMIT}" --label "git_tag=${IMAGE_TAG}" \
		-t "$(CONTAINER_REGISTRY)/execution-corrupted:latest" -t "$(CONTAINER_REGISTRY)/execution-corrupted:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/execution-corrupted:$(IMAGE_TAG)" .
	./insecure/cmd/mods_restore.sh

.PHONY: docker-build-verification
docker-build-verification:
	docker build -f cmd/Dockerfile  --build-arg TARGET=./cmd/verification --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG) --build-arg GOARCH=$(GOARCH) --target production \
		--label "git_commit=${COMMIT}" --label "git_tag=${IMAGE_TAG}" \
		-t "$(CONTAINER_REGISTRY)/verification:latest" -t "$(CONTAINER_REGISTRY)/verification:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/verification:$(IMAGE_TAG)" -t  "$(CONTAINER_REGISTRY)/verification:$(FLOW_GO_TAG)" .

.PHONY: docker-build-verification-without-netgo
docker-build-verification-without-netgo:
	docker build -f cmd/Dockerfile  --build-arg TAGS=relic --build-arg TARGET=./cmd/verification --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG_NO_NETGO) --build-arg GOARCH=$(GOARCH) --target production \
		--label "git_commit=${COMMIT}" --label "git_tag=$(IMAGE_TAG_NO_NETGO)" -t "$(CONTAINER_REGISTRY)/verification:$(IMAGE_TAG_NO_NETGO)" .

.PHONY: docker-build-verification-debug
docker-build-verification-debug:
	docker build -f cmd/Dockerfile  --build-arg TARGET=./cmd/verification --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG) --build-arg GOARCH=$(GOARCH) --target debug \
		-t "$(CONTAINER_REGISTRY)/verification-debug:latest" -t "$(CONTAINER_REGISTRY)/verification-debug:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/verification-debug:$(IMAGE_TAG)" .

# build corrupt verification node for BFT testing
.PHONY: docker-build-verification-corrupt
docker-build-verification-corrupt:
	# temporarily make insecure/ a non-module to allow Docker to use corrupt builders there
	./insecure/cmd/mods_override.sh
	docker build -f cmd/Dockerfile  --build-arg TARGET=./insecure/cmd/verification --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG) --build-arg GOARCH=$(GOARCH) --target production \
		--label "git_commit=${COMMIT}" --label "git_tag=${IMAGE_TAG}" \
		-t "$(CONTAINER_REGISTRY)/verification-corrupted:latest" -t "$(CONTAINER_REGISTRY)/verification-corrupted:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/verification-corrupted:$(IMAGE_TAG)" .
	./insecure/cmd/mods_restore.sh

.PHONY: docker-build-access
docker-build-access:
	docker build -f cmd/Dockerfile  --build-arg TARGET=./cmd/access --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG) --build-arg GOARCH=$(GOARCH) --target production \
		--label "git_commit=${COMMIT}" --label "git_tag=${IMAGE_TAG}" \
		-t "$(CONTAINER_REGISTRY)/access:latest" -t "$(CONTAINER_REGISTRY)/access:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/access:$(IMAGE_TAG)" -t  "$(CONTAINER_REGISTRY)/access:$(FLOW_GO_TAG)" .

.PHONY: docker-build-access-without-netgo
docker-build-access-without-netgo:
	docker build -f cmd/Dockerfile  --build-arg TAGS=relic --build-arg TARGET=./cmd/access --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG_NO_NETGO) --build-arg GOARCH=$(GOARCH) --target production \
		--label "git_commit=${COMMIT}" --label "git_tag=$(IMAGE_TAG_NO_NETGO)" -t "$(CONTAINER_REGISTRY)/access:$(IMAGE_TAG_NO_NETGO)" .

.PHONY: docker-build-access-debug
docker-build-access-debug:
	docker build -f cmd/Dockerfile  --build-arg TARGET=./cmd/access  --build-arg COMMIT=$(COMMIT) --build-arg VERSION=$(IMAGE_TAG) --build-arg GOARCH=$(GOARCH) --target debug \
		-t "$(CONTAINER_REGISTRY)/access-debug:latest" -t "$(CONTAINER_REGISTRY)/access-debug:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/access-debug:$(IMAGE_TAG)" .

# build corrupt access node for BFT testing
.PHONY: docker-build-access-corrupt
docker-build-access-corrupt:
	#temporarily make insecure/ a non-module to allow Docker to use corrupt builders there
	./insecure/cmd/mods_override.sh
	docker build -f cmd/Dockerfile  --build-arg TARGET=./insecure/cmd/access --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG) --build-arg GOARCH=$(GOARCH) --target production \
		--label "git_commit=${COMMIT}" --label "git_tag=${IMAGE_TAG}" \
		-t "$(CONTAINER_REGISTRY)/access-corrupted:latest" -t "$(CONTAINER_REGISTRY)/access-corrupted:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/access-corrupted:$(IMAGE_TAG)" .
	./insecure/cmd/mods_restore.sh

.PHONY: docker-build-observer
docker-build-observer:
	docker build -f cmd/Dockerfile --build-arg TARGET=./cmd/observer --build-arg COMMIT=$(COMMIT) --build-arg VERSION=$(IMAGE_TAG) --build-arg GOARCH=$(GOARCH) --target production \
		--label "git_commit=${COMMIT}" --label "git_tag=${IMAGE_TAG}" \
		-t "$(CONTAINER_REGISTRY)/observer:latest" -t "$(CONTAINER_REGISTRY)/observer:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/observer:$(IMAGE_TAG)" .

.PHONY: docker-build-observer-without-netgo
docker-build-observer-without-netgo:
	docker build -f cmd/Dockerfile  --build-arg TAGS=relic --build-arg TARGET=./cmd/observer --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG_NO_NETGO) --build-arg GOARCH=$(GOARCH) --target production \
		--label "git_commit=${COMMIT}" --label "git_tag=$(IMAGE_TAG_NO_NETGO)" -t "$(CONTAINER_REGISTRY)/observer:$(IMAGE_TAG_NO_NETGO)" .


.PHONY: docker-build-ghost
docker-build-ghost:
	docker build -f cmd/Dockerfile  --build-arg TARGET=./cmd/ghost --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG) --build-arg GOARCH=$(GOARCH) --target production \
		--label "git_commit=${COMMIT}" --label "git_tag=${IMAGE_TAG}" \
		-t "$(CONTAINER_REGISTRY)/ghost:latest" -t "$(CONTAINER_REGISTRY)/ghost:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/ghost:$(IMAGE_TAG)" .

.PHONY: docker-build-ghost-debug
docker-build-ghost-debug:
	docker build -f cmd/Dockerfile  --build-arg TARGET=./cmd/ghost --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG) --build-arg GOARCH=$(GOARCH) --target debug \
		-t "$(CONTAINER_REGISTRY)/ghost-debug:latest" -t "$(CONTAINER_REGISTRY)/ghost-debug:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/ghost-debug:$(IMAGE_TAG)" .

PHONY: docker-build-bootstrap
docker-build-bootstrap:
	docker build -f cmd/Dockerfile  --build-arg TARGET=./cmd/bootstrap --build-arg GOARCH=$(GOARCH) --target production \
		--label "git_commit=${COMMIT}" --label "git_tag=${IMAGE_TAG}" \
		-t "$(CONTAINER_REGISTRY)/bootstrap:latest" -t "$(CONTAINER_REGISTRY)/bootstrap:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/bootstrap:$(IMAGE_TAG)" .

PHONY: tool-bootstrap
tool-bootstrap: docker-build-bootstrap
	docker container create --name bootstrap $(CONTAINER_REGISTRY)/bootstrap:latest;docker container cp bootstrap:/bin/app ./bootstrap;docker container rm bootstrap

.PHONY: docker-build-bootstrap-transit
docker-build-bootstrap-transit:
	docker build -f cmd/Dockerfile  --build-arg TARGET=./cmd/bootstrap/transit --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(VERSION) --build-arg GOARCH=$(GOARCH) --no-cache \
	    --target production  \
		-t "$(CONTAINER_REGISTRY)/bootstrap-transit:latest" -t "$(CONTAINER_REGISTRY)/bootstrap-transit:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/bootstrap-transit:$(IMAGE_TAG)" .

PHONY: tool-transit
tool-transit: docker-build-bootstrap-transit
	docker container create --name transit $(CONTAINER_REGISTRY)/bootstrap-transit:latest;docker container cp transit:/bin/app ./transit;docker container rm transit

.PHONY: docker-build-loader
docker-build-loader:
	docker build -f ./integration/benchmark/cmd/manual/Dockerfile --build-arg TARGET=./benchmark/cmd/manual --target production \
		--label "git_commit=${COMMIT}" --label "git_tag=${IMAGE_TAG}" \
		-t "$(CONTAINER_REGISTRY)/loader:latest" -t "$(CONTAINER_REGISTRY)/loader:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/loader:$(IMAGE_TAG)" .

.PHONY: docker-build-flow
docker-build-flow: docker-build-collection docker-build-consensus docker-build-execution docker-build-verification docker-build-access docker-build-observer docker-build-ghost

.PHONY: docker-build-flow-without-netgo
docker-build-flow-without-netgo: docker-build-collection-without-netgo docker-build-consensus-without-netgo docker-build-execution-without-netgo docker-build-verification-without-netgo docker-build-access-without-netgo docker-build-observer-without-netgo

.PHONY: docker-build-flow-corrupt
docker-build-flow-corrupt: docker-build-execution-corrupt docker-build-verification-corrupt docker-build-access-corrupt

.PHONY: docker-build-benchnet
docker-build-benchnet: docker-build-flow docker-build-loader

.PHONY: docker-push-collection
docker-push-collection:
	docker push "$(CONTAINER_REGISTRY)/collection:$(SHORT_COMMIT)"
	docker push "$(CONTAINER_REGISTRY)/collection:$(IMAGE_TAG)"
	docker push "$(CONTAINER_REGISTRY)/collection:$(FLOW_GO_TAG)"

.PHONY: docker-push-collection-without-netgo
docker-push-collection-without-netgo:
	docker push "$(CONTAINER_REGISTRY)/collection:$(IMAGE_TAG_NO_NETGO)"

.PHONY: docker-push-collection-latest
docker-push-collection-latest: docker-push-collection
	docker push "$(CONTAINER_REGISTRY)/collection:latest"

.PHONY: docker-push-consensus
docker-push-consensus:
	docker push "$(CONTAINER_REGISTRY)/consensus:$(SHORT_COMMIT)"
	docker push "$(CONTAINER_REGISTRY)/consensus:$(IMAGE_TAG)"
	docker push "$(CONTAINER_REGISTRY)/consensus:$(FLOW_GO_TAG)"

.PHONY: docker-push-consensus-without-netgo
docker-push-consensus-without-netgo:
	docker push "$(CONTAINER_REGISTRY)/consensus:$(IMAGE_TAG_NO_NETGO)"

.PHONY: docker-push-consensus-latest
docker-push-consensus-latest: docker-push-consensus
	docker push "$(CONTAINER_REGISTRY)/consensus:latest"

.PHONY: docker-push-execution
docker-push-execution:
	docker push "$(CONTAINER_REGISTRY)/execution:$(SHORT_COMMIT)"
	docker push "$(CONTAINER_REGISTRY)/execution:$(IMAGE_TAG)"
	docker push "$(CONTAINER_REGISTRY)/execution:$(FLOW_GO_TAG)"

.PHONY: docker-push-execution-corrupt
docker-push-execution-corrupt:
	docker push "$(CONTAINER_REGISTRY)/execution-corrupted:$(SHORT_COMMIT)"
	docker push "$(CONTAINER_REGISTRY)/execution-corrupted:$(IMAGE_TAG)"


.PHONY: docker-push-execution-without-netgo
docker-push-execution-without-netgo:
	docker push "$(CONTAINER_REGISTRY)/execution:$(IMAGE_TAG_NO_NETGO)"

.PHONY: docker-push-execution-latest
docker-push-execution-latest: docker-push-execution
	docker push "$(CONTAINER_REGISTRY)/execution:latest"

.PHONY: docker-push-verification
docker-push-verification:
	docker push "$(CONTAINER_REGISTRY)/verification:$(SHORT_COMMIT)"
	docker push "$(CONTAINER_REGISTRY)/verification:$(IMAGE_TAG)"
	docker push "$(CONTAINER_REGISTRY)/verification:$(FLOW_GO_TAG)"

.PHONY: docker-push-verification-corrupt
docker-push-verification-corrupt:
	docker push "$(CONTAINER_REGISTRY)/verification-corrupted:$(SHORT_COMMIT)"
	docker push "$(CONTAINER_REGISTRY)/verification-corrupted:$(IMAGE_TAG)"

.PHONY: docker-push-verification-without-netgo
docker-push-verification-without-netgo:
	docker push "$(CONTAINER_REGISTRY)/verification:$(IMAGE_TAG_NO_NETGO)"

.PHONY: docker-push-verification-latest
docker-push-verification-latest: docker-push-verification
	docker push "$(CONTAINER_REGISTRY)/verification:latest"

.PHONY: docker-push-access
docker-push-access:
	docker push "$(CONTAINER_REGISTRY)/access:$(SHORT_COMMIT)"
	docker push "$(CONTAINER_REGISTRY)/access:$(IMAGE_TAG)"
	docker push "$(CONTAINER_REGISTRY)/access:$(FLOW_GO_TAG)"

.PHONY: docker-push-access-corrupt
docker-push-access-corrupt:
	docker push "$(CONTAINER_REGISTRY)/access-corrupted:$(SHORT_COMMIT)"
	docker push "$(CONTAINER_REGISTRY)/access-corrupted:$(IMAGE_TAG)"

.PHONY: docker-push-access-without-netgo
docker-push-access-without-netgo:
	docker push "$(CONTAINER_REGISTRY)/access:$(IMAGE_TAG_NO_NETGO)"

.PHONY: docker-push-access-latest
docker-push-access-latest: docker-push-access
	docker push "$(CONTAINER_REGISTRY)/access:latest"
	

.PHONY: docker-push-observer
docker-push-observer:
	docker push "$(CONTAINER_REGISTRY)/observer:$(SHORT_COMMIT)"
	docker push "$(CONTAINER_REGISTRY)/observer:$(IMAGE_TAG)"

.PHONY: docker-push-observer-without-netgo
docker-push-observer-without-netgo:
	docker push "$(CONTAINER_REGISTRY)/observer:$(IMAGE_TAG_NO_NETGO)"

.PHONY: docker-push-observer-latest
docker-push-observer-latest: docker-push-observer
	docker push "$(CONTAINER_REGISTRY)/observer:latest"

.PHONY: docker-push-ghost
docker-push-ghost:
	docker push "$(CONTAINER_REGISTRY)/ghost:$(SHORT_COMMIT)"
	docker push "$(CONTAINER_REGISTRY)/ghost:$(IMAGE_TAG)"

.PHONY: docker-push-ghost-latest
docker-push-ghost-latest: docker-push-ghost
	docker push "$(CONTAINER_REGISTRY)/ghost:latest"

.PHONY: docker-push-loader
docker-push-loader:
	docker push "$(CONTAINER_REGISTRY)/loader:$(SHORT_COMMIT)"
	docker push "$(CONTAINER_REGISTRY)/loader:$(IMAGE_TAG)"

.PHONY: docker-push-loader-latest
docker-push-loader-latest: docker-push-loader
	docker push "$(CONTAINER_REGISTRY)/loader:latest"

.PHONY: docker-push-flow
docker-push-flow: docker-push-collection docker-push-consensus docker-push-execution docker-push-verification docker-push-access docker-push-observer

.PHONY: docker-push-flow-without-netgo
docker-push-flow-without-netgo: docker-push-collection-without-netgo docker-push-consensus-without-netgo docker-push-execution-without-netgo docker-push-verification-without-netgo docker-push-access-without-netgo docker-push-observer-without-netgo

.PHONY: docker-push-flow-latest
docker-push-flow-latest: docker-push-collection-latest docker-push-consensus-latest docker-push-execution-latest docker-push-verification-latest docker-push-access-latest docker-push-observer-latest

.PHONY: docker-push-flow-corrupt
docker-push-flow-corrupt: docker-push-access-corrupt docker-push-execution-corrupt docker-push-verification-corrupt

.PHONY: docker-push-benchnet
docker-push-benchnet: docker-push-flow docker-push-loader

.PHONY: docker-push-benchnet-latest
docker-push-benchnet-latest: docker-push-flow-latest docker-push-loader-latest

.PHONY: docker-run-collection
docker-run-collection:
	docker run -p 8080:8080 -p 3569:3569 "$(CONTAINER_REGISTRY)/collection:latest" --nodeid 1234567890123456789012345678901234567890123456789012345678901234 --entries collection-1234567890123456789012345678901234567890123456789012345678901234@localhost:3569=1000

.PHONY: docker-run-consensus
docker-run-consensus:
	docker run -p 8080:8080 -p 3569:3569 "$(CONTAINER_REGISTRY)/consensus:latest" --nodeid 1234567890123456789012345678901234567890123456789012345678901234 --entries consensus-1234567890123456789012345678901234567890123456789012345678901234@localhost:3569=1000

.PHONY: docker-run-execution
docker-run-execution:
	docker run -p 8080:8080 -p 3569:3569 "$(CONTAINER_REGISTRY)/execution:latest" --nodeid 1234567890123456789012345678901234567890123456789012345678901234 --entries execution-1234567890123456789012345678901234567890123456789012345678901234@localhost:3569=1000

.PHONY: docker-run-verification
docker-run-verification:
	docker run -p 8080:8080 -p 3569:3569 "$(CONTAINER_REGISTRY)/verification:latest" --nodeid 1234567890123456789012345678901234567890123456789012345678901234 --entries verification-1234567890123456789012345678901234567890123456789012345678901234@localhost:3569=1000

.PHONY: docker-run-access
docker-run-access:
	docker run -p 9000:9000 -p 3569:3569 -p 8080:8080  -p 8000:8000 "$(CONTAINER_REGISTRY)/access:latest" --nodeid 1234567890123456789012345678901234567890123456789012345678901234 --entries access-1234567890123456789012345678901234567890123456789012345678901234@localhost:3569=1000

.PHONY: docker-run-observer
docker-run-observer:
	docker run -p 9000:9000 -p 3569:3569 -p 8080:8080  -p 8000:8000 "$(CONTAINER_REGISTRY)/observer:latest" --nodeid 1234567890123456789012345678901234567890123456789012345678901234 --entries observer-1234567890123456789012345678901234567890123456789012345678901234@localhost:3569=1000

.PHONY: docker-run-ghost
docker-run-ghost:
	docker run -p 9000:9000 -p 3569:3569 "$(CONTAINER_REGISTRY)/ghost:latest" --nodeid 1234567890123456789012345678901234567890123456789012345678901234 --entries ghost-1234567890123456789012345678901234567890123456789012345678901234@localhost:3569=1000

PHONY: docker-all-tools
docker-all-tools: tool-util tool-remove-execution-fork

PHONY: docker-build-util
docker-build-util:
	docker build -f cmd/Dockerfile --build-arg TARGET=./cmd/util --build-arg GOARCH=$(GOARCH) --target production \
		-t "$(CONTAINER_REGISTRY)/util:latest" -t "$(CONTAINER_REGISTRY)/util:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/util:$(IMAGE_TAG)" .

PHONY: tool-util
tool-util: docker-build-util
	docker container create --name util $(CONTAINER_REGISTRY)/util:latest;docker container cp util:/bin/app ./util;docker container rm util

PHONY: docker-build-remove-execution-fork
docker-build-remove-execution-fork:
	docker build -f cmd/Dockerfile --ssh default --build-arg TARGET=./cmd/util/cmd/remove-execution-fork --build-arg GOARCH=$(GOARCH) --target production \
		-t "$(CONTAINER_REGISTRY)/remove-execution-fork:latest" -t "$(CONTAINER_REGISTRY)/remove-execution-fork:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/remove-execution-fork:$(IMAGE_TAG)" .

PHONY: tool-remove-execution-fork
tool-remove-execution-fork: docker-build-remove-execution-fork
	docker container create --name remove-execution-fork $(CONTAINER_REGISTRY)/remove-execution-fork:latest;docker container cp remove-execution-fork:/bin/app ./remove-execution-fork;docker container rm remove-execution-fork

.PHONY: check-go-version
check-go-version:
	@bash -c '\
		MINGOVERSION=1.18; \
		function ver { printf "%d%03d%03d%03d" $$(echo "$$1" | tr . " "); }; \
		GOVER=$$(go version | sed -rne "s/.* go([0-9.]+).*/\1/p" ); \
		if [ "$$(ver $$GOVER)" -lt "$$(ver $$MINGOVERSION)" ]; then \
			echo "go $$GOVER is too old. flow-go only supports go $$MINGOVERSION and up."; \
			exit 1; \
		fi; \
		'

#----------------------------------------------------------------------
# CD COMMANDS
#----------------------------------------------------------------------

.PHONY: deploy-staging
deploy-staging: update-deployment-image-name-staging apply-staging-files monitor-rollout

# Staging YAMLs must have 'staging' in their name.
.PHONY: apply-staging-files
apply-staging-files:
	kconfig=$$(uuidgen); \
	echo "$$KUBECONFIG_STAGING" > $$kconfig; \
	files=$$(find ${K8S_YAMLS_LOCATION_STAGING} -type f \( --name "*.yml" -or --name "*.yaml" \)); \
	echo "$$files" | xargs -I {} kubectl --kubeconfig=$$kconfig apply -f {}

# Deployment YAMLs must have 'deployment' in their name.
.PHONY: update-deployment-image-name-staging
update-deployment-image-name-staging: CONTAINER=flow-test-net
update-deployment-image-name-staging:
	@files=$$(find ${K8S_YAMLS_LOCATION_STAGING} -type f \( --name "*.yml" -or --name "*.yaml" \) | grep deployment); \
	for file in $$files; do \
		patched=`openssl rand -hex 8`; \
		node=`echo "$$file" | grep -oP 'flow-\K\w+(?=-node-deployment.yml)'`; \
		kubectl patch -f $$file -p '{"spec":{"template":{"spec":{"containers":[{"name":"${CONTAINER}","image":"$(CONTAINER_REGISTRY)/'"$$node"':${IMAGE_TAG}"}]}}}}`' --local -o yaml > $$patched; \
		mv -f $$patched $$file; \
	done

.PHONY: monitor-rollout
monitor-rollout:
	kconfig=$$(uuidgen); \
	echo "$$KUBECONFIG_STAGING" > $$kconfig; \
	kubectl --kubeconfig=$$kconfig rollout status statefulsets.apps flow-collection-node-v1; \
	kubectl --kubeconfig=$$kconfig rollout status statefulsets.apps flow-consensus-node-v1; \
	kubectl --kubeconfig=$$kconfig rollout status statefulsets.apps flow-execution-node-v1; \
	kubectl --kubeconfig=$$kconfig rollout status statefulsets.apps flow-verification-node-v1