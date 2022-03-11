
# The short Git commit hash
SHORT_COMMIT := $(shell git rev-parse --short HEAD)
# The Git commit hash
COMMIT := $(shell git rev-parse HEAD)
# The tag of the current commit, otherwise empty
VERSION := $(shell git describe --tags --abbrev=2 --match "v*" 2>/dev/null)

# Image tag: if image tag is not set, set it with version (or short commit if empty)
ifeq (${IMAGE_TAG},)
IMAGE_TAG := ${VERSION}
endif

ifeq (${IMAGE_TAG},)
IMAGE_TAG := ${SHORT_COMMIT}
endif

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

.PHONY: crypto/relic/build
crypto/relic/build:
# setup the crypto package under the GOPATH, needed to test packages importing flow-go/crypto
	bash crypto_setup.sh

cmd/collection/collection:
	go build -o cmd/collection/collection cmd/collection/main.go

cmd/util/util:
	go build -o cmd/util/util --tags relic cmd/util/main.go

.PHONY: install-mock-generators
install-mock-generators:
	cd ${GOPATH}; \
    GO111MODULE=on go install github.com/vektra/mockery/cmd/mockery@v1.1.2; \
    GO111MODULE=on go install github.com/golang/mock/mockgen@v1.3.1;

.PHONY: install-tools
install-tools: crypto/relic/build check-go-version install-mock-generators
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b ${GOPATH}/bin v1.41.1; \
	cd ${GOPATH}; \
	GO111MODULE=on go install github.com/golang/protobuf/protoc-gen-go@v1.3.2; \
	GO111MODULE=on go install github.com/uber/prototool/cmd/prototool@v1.9.0; \
	GO111MODULE=on go install github.com/gogo/protobuf/protoc-gen-gofast@latest; \
	GO111MODULE=on go install golang.org/x/tools/cmd/stringer@master;

.PHONY: unittest-main
unittest-main:
	# test all packages with Relic library enabled
	GO111MODULE=on go test -coverprofile=$(COVER_PROFILE) -covermode=atomic $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) --tags relic ./...

.PHONY: unittest
unittest: unittest-main
	$(MAKE) -C crypto test
	$(MAKE) -C integration test

.PHONY: emulator-build
emulator-build:
	# test the fvm package compiles with Relic library disabled (required for the emulator build)
	cd ./fvm && GO111MODULE=on go test ./... -run=NoTestHasThisPrefix

.PHONY: test
test: generate-mocks emulator-build unittest

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

.PHONY: generate
generate: generate-proto generate-mocks

.PHONY: generate-proto
generate-proto:
	prototool generate protobuf

.PHONY: generate-mocks
generate-mocks:
	GO111MODULE=on mockery -name '(Connector|PingInfoProvider)' -dir=network/p2p -case=underscore -output="./network/mocknetwork" -outpkg="mocknetwork"
	GO111MODULE=on mockgen -destination=storage/mocks/storage.go -package=mocks github.com/onflow/flow-go/storage Blocks,Headers,Payloads,Collections,Commits,Events,ServiceEvents,TransactionResults
	GO111MODULE=on mockgen -destination=module/mocks/network.go -package=mocks github.com/onflow/flow-go/module Local,Requester
	GO111MODULE=on mockgen -destination=network/mocknetwork/engine.go -package=mocknetwork github.com/onflow/flow-go/network Engine
	GO111MODULE=on mockgen -destination=network/mocknetwork/mock_network.go -package=mocknetwork github.com/onflow/flow-go/network Network
	GO111MODULE=on mockery -name '(ExecutionDataService|ExecutionDataCIDCache)' -dir=module/state_synchronization -case=underscore -output="./module/state_synchronization/mock" -outpkg="state_synchronization"
	GO111MODULE=on mockery -name 'ExecutionState' -dir=engine/execution/state -case=underscore -output="engine/execution/state/mock" -outpkg="mock"
	GO111MODULE=on mockery -name 'BlockComputer' -dir=engine/execution/computation/computer -case=underscore -output="engine/execution/computation/computer/mock" -outpkg="mock"
	GO111MODULE=on mockery -name 'ComputationManager' -dir=engine/execution/computation -case=underscore -output="engine/execution/computation/mock" -outpkg="mock"
	GO111MODULE=on mockery -name 'EpochComponentsFactory' -dir=engine/collection/epochmgr -case=underscore -output="engine/collection/epochmgr/mock" -outpkg="mock"
	GO111MODULE=on mockery -name 'Backend' -dir=engine/collection/rpc -case=underscore -output="engine/collection/rpc/mock" -outpkg="mock"
	GO111MODULE=on mockery -name 'ProviderEngine' -dir=engine/execution/provider -case=underscore -output="engine/execution/provider/mock" -outpkg="mock"
	(cd ./crypto && GO111MODULE=on mockery -name 'PublicKey' -case=underscore -output="../module/mock" -outpkg="mock")
	GO111MODULE=on mockery -name '.*' -dir=state/cluster -case=underscore -output="state/cluster/mock" -outpkg="mock"
	GO111MODULE=on mockery -name '.*' -dir=module -case=underscore -tags="relic" -output="./module/mock" -outpkg="mock"
	GO111MODULE=on mockery -name '.*' -dir=module/mempool -case=underscore -output="./module/mempool/mock" -outpkg="mempool"
	GO111MODULE=on mockery -name '.*' -dir=module/component -case=underscore -output="./module/component/mock" -outpkg="component"
	GO111MODULE=on mockery -name '.*' -dir=network -case=underscore -output="./network/mocknetwork" -outpkg="mocknetwork"
	GO111MODULE=on mockery -name '.*' -dir=storage -case=underscore -output="./storage/mock" -outpkg="mock"
	GO111MODULE=on mockery -name '.*' -dir="state/protocol" -case=underscore -output="state/protocol/mock" -outpkg="mock"
	GO111MODULE=on mockery -name '.*' -dir="state/protocol/events" -case=underscore -output="./state/protocol/events/mock" -outpkg="mock"
	GO111MODULE=on mockery -name '.*' -dir=engine/execution/computation/computer -case=underscore -output="./engine/execution/computation/computer/mock" -outpkg="mock"
	GO111MODULE=on mockery -name '.*' -dir=engine/execution/state -case=underscore -output="./engine/execution/state/mock" -outpkg="mock"
	GO111MODULE=on mockery -name '.*' -dir=engine/consensus -case=underscore -output="./engine/consensus/mock" -outpkg="mock"
	GO111MODULE=on mockery -name '.*' -dir=engine/consensus/approvals -case=underscore -output="./engine/consensus/approvals/mock" -outpkg="mock"
	GO111MODULE=on mockery -name '.*' -dir=fvm -case=underscore -output="./fvm/mock" -outpkg="mock"
	GO111MODULE=on mockery -name '.*' -dir=fvm/state -case=underscore -output="./fvm/mock/state" -outpkg="mock"
	GO111MODULE=on mockery -name '.*' -dir=ledger -case=underscore -output="./ledger/mock" -outpkg="mock"
	GO111MODULE=on mockery -name 'SubscriptionManager' -dir=network/ -case=underscore -output="./network/mocknetwork" -outpkg="mocknetwork"
	GO111MODULE=on mockery -name 'Vertex' -dir="./module/forest" -case=underscore -output="./module/forest/mock" -outpkg="mock"
	GO111MODULE=on mockery -name '.*' -dir="./consensus/hotstuff" -case=underscore -output="./consensus/hotstuff/mocks" -outpkg="mocks"
	GO111MODULE=on mockery -name '.*' -dir="./engine/access/wrapper" -case=underscore -output="./engine/access/mock" -outpkg="mock"
	GO111MODULE=on mockery -name 'API' -dir="./access" -case=underscore -output="./access/mock" -outpkg="mock"
	GO111MODULE=on mockery -name 'ConnectionFactory' -dir="./engine/access/rpc/backend" -case=underscore -output="./engine/access/rpc/backend/mock" -outpkg="mock"
	GO111MODULE=on mockery -name 'IngestRPC' -dir="./engine/execution/ingestion" -case=underscore -tags relic -output="./engine/execution/ingestion/mock" -outpkg="mock"
	GO111MODULE=on mockery -name '.*' -dir=model/fingerprint -case=underscore -output="./model/fingerprint/mock" -outpkg="mock"
	GO111MODULE=on mockery -name 'ExecForkActor' --structname 'ExecForkActorMock' -dir=module/mempool/consensus/mock/ -case=underscore -output="./module/mempool/consensus/mock/" -outpkg="mock"
	GO111MODULE=on mockery -name '.*' -dir=engine/verification/fetcher/ -case=underscore -output="./engine/verification/fetcher/mock" -outpkg="mockfetcher"
	GO111MODULE=on mockery -name '.*' -dir=insecure/ -case=underscore -output="./insecure/mock"  -outpkg="mockinsecure"
	GO111MODULE=on mockery -name '.*' -dir=./cmd/util/ledger/reporters -case=underscore -output="./cmd/util/ledger/reporters/mock" -outpkg="mock"




# this ensures there is no unused dependency being added by accident
.PHONY: tidy
tidy:
	go mod tidy
	cd integration; go mod tidy
	cd crypto; go mod tidy
	cd cmd/testclient; go mod tidy
	git diff --exit-code

.PHONY: lint
lint: tidy
	# GO111MODULE=on revive -config revive.toml -exclude storage/ledger/trie ./...
	golangci-lint run -v --build-tags relic ./...

.PHONY: fix-lint
fix-lint:
	# GO111MODULE=on revive -config revive.toml -exclude storage/ledger/trie ./...
	golangci-lint run -v --build-tags relic --fix ./...

# Runs unit tests, SKIP FOR NOW linter, coverage
.PHONY: ci
ci: install-tools tidy test # lint coverage

# Runs integration tests
.PHONY: ci-integration
ci-integration: crypto/relic/build
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
	docker run --env COVER=$(COVER) --env JSON_OUTPUT=$(JSON_OUTPUT) \
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
	docker build -f cmd/Dockerfile  --build-arg TARGET=collection --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG) --build-arg GOARCH=$(GOARCH) --target production \
		-t "$(CONTAINER_REGISTRY)/collection:latest" -t "$(CONTAINER_REGISTRY)/collection:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/collection:$(IMAGE_TAG)" .

.PHONY: docker-build-collection-debug
docker-build-collection-debug:
	docker build -f cmd/Dockerfile  --build-arg TARGET=collection --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG) --build-arg GOARCH=$(GOARCH) --target debug \
		-t "$(CONTAINER_REGISTRY)/collection-debug:latest" -t "$(CONTAINER_REGISTRY)/collection-debug:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/collection-debug:$(IMAGE_TAG)" .

.PHONY: docker-build-consensus
docker-build-consensus:
	docker build -f cmd/Dockerfile  --build-arg TARGET=consensus --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG) --build-arg GOARCH=$(GOARCH) --target production \
		-t "$(CONTAINER_REGISTRY)/consensus:latest" -t "$(CONTAINER_REGISTRY)/consensus:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/consensus:$(IMAGE_TAG)" .

.PHONY: docker-build-consensus-debug
docker-build-consensus-debug:
	docker build -f cmd/Dockerfile  --build-arg TARGET=consensus --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG) --build-arg GOARCH=$(GOARCH) --target debug \
		-t "$(CONTAINER_REGISTRY)/consensus-debug:latest" -t "$(CONTAINER_REGISTRY)/consensus-debug:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/consensus-debug:$(IMAGE_TAG)" .

.PHONY: docker-build-execution
docker-build-execution:
	docker build -f cmd/Dockerfile  --build-arg TARGET=execution --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG) --build-arg GOARCH=$(GOARCH) --target production \
		-t "$(CONTAINER_REGISTRY)/execution:latest" -t "$(CONTAINER_REGISTRY)/execution:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/execution:$(IMAGE_TAG)" .

.PHONY: docker-build-execution-debug
docker-build-execution-debug:
	docker build -f cmd/Dockerfile  --build-arg TARGET=execution --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG) --build-arg GOARCH=$(GOARCH) --target debug \
		-t "$(CONTAINER_REGISTRY)/execution-debug:latest" -t "$(CONTAINER_REGISTRY)/execution-debug:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/execution-debug:$(IMAGE_TAG)" .

.PHONY: docker-build-verification
docker-build-verification:
	docker build -f cmd/Dockerfile  --build-arg TARGET=verification --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG) --build-arg GOARCH=$(GOARCH) --target production \
		-t "$(CONTAINER_REGISTRY)/verification:latest" -t "$(CONTAINER_REGISTRY)/verification:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/verification:$(IMAGE_TAG)" .

.PHONY: docker-build-verification-debug
docker-build-verification-debug:
	docker build -f cmd/Dockerfile  --build-arg TARGET=verification --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG) --build-arg GOARCH=$(GOARCH) --target debug \
		-t "$(CONTAINER_REGISTRY)/verification-debug:latest" -t "$(CONTAINER_REGISTRY)/verification-debug:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/verification-debug:$(IMAGE_TAG)" .

.PHONY: docker-build-access
docker-build-access:
	docker build -f cmd/Dockerfile  --build-arg TARGET=access --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG) --build-arg GOARCH=$(GOARCH) --target production \
		-t "$(CONTAINER_REGISTRY)/access:latest" -t "$(CONTAINER_REGISTRY)/access:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/access:$(IMAGE_TAG)" .

.PHONY: docker-build-access-debug
docker-build-access-debug:
	docker build -f cmd/Dockerfile  --build-arg TARGET=access  --build-arg COMMIT=$(COMMIT) --build-arg VERSION=$(IMAGE_TAG) --build-arg GOARCH=$(GOARCH) --target debug \
		-t "$(CONTAINER_REGISTRY)/access-debug:latest" -t "$(CONTAINER_REGISTRY)/access-debug:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/access-debug:$(IMAGE_TAG)" .

# Observer is currently simply access node, this target is added for compatibility with deployment pipeline
# Once proper observer is separated in the code, we should just need to change TARGET parameter below
.PHONY: docker-build-observer
docker-build-observer:
	docker build -f cmd/Dockerfile  --build-arg TARGET=access --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG) --build-arg GOARCH=$(GOARCH) --target production \
		-t "$(CONTAINER_REGISTRY)/observer:latest" -t "$(CONTAINER_REGISTRY)/observer:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/observer:$(IMAGE_TAG)" .


.PHONY: docker-build-ghost
docker-build-ghost:
	docker build -f cmd/Dockerfile  --build-arg TARGET=ghost --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG) --build-arg GOARCH=$(GOARCH) --target production \
		-t "$(CONTAINER_REGISTRY)/ghost:latest" -t "$(CONTAINER_REGISTRY)/ghost:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/ghost:$(IMAGE_TAG)" .

.PHONY: docker-build-ghost-debug
docker-build-ghost-debug:
	docker build -f cmd/Dockerfile  --build-arg TARGET=ghost --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(IMAGE_TAG) --build-arg GOARCH=$(GOARCH) --target debug \
		-t "$(CONTAINER_REGISTRY)/ghost-debug:latest" -t "$(CONTAINER_REGISTRY)/ghost-debug:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/ghost-debug:$(IMAGE_TAG)" .

PHONY: docker-build-bootstrap
docker-build-bootstrap:
	docker build -f cmd/Dockerfile  --build-arg TARGET=bootstrap --build-arg GOARCH=$(GOARCH) --target production \
		-t "$(CONTAINER_REGISTRY)/bootstrap:latest" -t "$(CONTAINER_REGISTRY)/bootstrap:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/bootstrap:$(IMAGE_TAG)" .

PHONY: tool-bootstrap
tool-bootstrap: docker-build-bootstrap
	docker container create --name bootstrap $(CONTAINER_REGISTRY)/bootstrap:latest;docker container cp bootstrap:/bin/app ./bootstrap;docker container rm bootstrap

.PHONY: docker-build-bootstrap-transit
docker-build-bootstrap-transit:
	docker build -f cmd/Dockerfile  --build-arg TARGET=bootstrap/transit --build-arg COMMIT=$(COMMIT)  --build-arg VERSION=$(VERSION) --build-arg GOARCH=$(GOARCH) --no-cache \
	    --target production  \
		-t "$(CONTAINER_REGISTRY)/bootstrap-transit:latest" -t "$(CONTAINER_REGISTRY)/bootstrap-transit:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/bootstrap-transit:$(IMAGE_TAG)" .

PHONY: tool-transit
tool-transit: docker-build-bootstrap-transit
	docker container create --name transit $(CONTAINER_REGISTRY)/bootstrap-transit:latest;docker container cp transit:/bin/app ./transit;docker container rm transit

.PHONY: docker-build-loader
docker-build-loader:
	docker build -f ./integration/loader/Dockerfile --build-arg TARGET=loader --target production \
		-t "$(CONTAINER_REGISTRY)/loader:latest" -t "$(CONTAINER_REGISTRY)/loader:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/loader:$(IMAGE_TAG)" .

.PHONY: docker-build-flow
docker-build-flow: docker-build-collection docker-build-consensus docker-build-execution docker-build-verification docker-build-access docker-build-observer docker-build-ghost

.PHONY: docker-build-benchnet
docker-build-benchnet: docker-build-flow docker-build-loader

.PHONY: docker-push-collection
docker-push-collection:
	docker push "$(CONTAINER_REGISTRY)/collection:latest"
	docker push "$(CONTAINER_REGISTRY)/collection:$(SHORT_COMMIT)"
	docker push "$(CONTAINER_REGISTRY)/collection:$(IMAGE_TAG)"

.PHONY: docker-push-consensus
docker-push-consensus:
	docker push "$(CONTAINER_REGISTRY)/consensus:latest"
	docker push "$(CONTAINER_REGISTRY)/consensus:$(SHORT_COMMIT)"
	docker push "$(CONTAINER_REGISTRY)/consensus:$(IMAGE_TAG)"

.PHONY: docker-push-execution
docker-push-execution:
	docker push "$(CONTAINER_REGISTRY)/execution:latest"
	docker push "$(CONTAINER_REGISTRY)/execution:$(SHORT_COMMIT)"
	docker push "$(CONTAINER_REGISTRY)/execution:$(IMAGE_TAG)"

.PHONY: docker-push-verification
docker-push-verification:
	docker push "$(CONTAINER_REGISTRY)/verification:latest"
	docker push "$(CONTAINER_REGISTRY)/verification:$(SHORT_COMMIT)"
	docker push "$(CONTAINER_REGISTRY)/verification:$(IMAGE_TAG)"

.PHONY: docker-push-access
docker-push-access:
	docker push "$(CONTAINER_REGISTRY)/access:latest"
	docker push "$(CONTAINER_REGISTRY)/access:$(SHORT_COMMIT)"
	docker push "$(CONTAINER_REGISTRY)/access:$(IMAGE_TAG)"

.PHONY: docker-push-observer
docker-push-observer:
	docker push "$(CONTAINER_REGISTRY)/observer:latest"
	docker push "$(CONTAINER_REGISTRY)/observer:$(SHORT_COMMIT)"
	docker push "$(CONTAINER_REGISTRY)/observer:$(IMAGE_TAG)"

.PHONY: docker-push-ghost
docker-push-ghost:
	docker push "$(CONTAINER_REGISTRY)/ghost:latest"
	docker push "$(CONTAINER_REGISTRY)/ghost:$(SHORT_COMMIT)"
	docker push "$(CONTAINER_REGISTRY)/ghost:$(IMAGE_TAG)"

.PHONY: docker-push-loader
docker-push-loader:
	docker push "$(CONTAINER_REGISTRY)/loader:latest"
	docker push "$(CONTAINER_REGISTRY)/loader:$(SHORT_COMMIT)"
	docker push "$(CONTAINER_REGISTRY)/loader:$(IMAGE_TAG)"

.PHONY: docker-push-flow
docker-push-flow: docker-push-collection docker-push-consensus docker-push-execution docker-push-verification docker-push-access docker-push-observer

.PHONY: docker-push-benchnet
docker-push-benchnet: docker-push-flow docker-push-loader

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

.PHONY: docker-run-ghost
docker-run-ghost:
	docker run -p 9000:9000 -p 3569:3569 "$(CONTAINER_REGISTRY)/ghost:latest" --nodeid 1234567890123456789012345678901234567890123456789012345678901234 --entries ghost-1234567890123456789012345678901234567890123456789012345678901234@localhost:3569=1000

PHONY: docker-all-tools
docker-all-tools: tool-util tool-remove-execution-fork

PHONY: docker-build-util
docker-build-util:
	docker build -f cmd/Dockerfile --ssh default --build-arg TARGET=util --build-arg GOARCH=$(GOARCH) --target production \
		-t "$(CONTAINER_REGISTRY)/util:latest" -t "$(CONTAINER_REGISTRY)/util:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/util:$(IMAGE_TAG)" .

PHONY: tool-util
tool-util: docker-build-util
	docker container create --name util $(CONTAINER_REGISTRY)/util:latest;docker container cp util:/bin/app ./util;docker container rm util

PHONY: docker-build-remove-execution-fork
docker-build-remove-execution-fork:
	docker build -f cmd/Dockerfile --ssh default --build-arg TARGET=util/cmd/remove-execution-fork --build-arg GOARCH=$(GOARCH) --target production \
		-t "$(CONTAINER_REGISTRY)/remove-execution-fork:latest" -t "$(CONTAINER_REGISTRY)/remove-execution-fork:$(SHORT_COMMIT)" -t "$(CONTAINER_REGISTRY)/remove-execution-fork:$(IMAGE_TAG)" .

PHONY: tool-remove-execution-fork
tool-remove-execution-fork: docker-build-remove-execution-fork
	docker container create --name remove-execution-fork $(CONTAINER_REGISTRY)/remove-execution-fork:latest;docker container cp remove-execution-fork:/bin/app ./remove-execution-fork;docker container rm remove-execution-fork

# Check if the go version is 1.16 or higher. flow-go only supports go 1.16 and up.
.PHONY: check-go-version
check-go-version:
	go version | grep '1.1[6-9]'

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
	files=$$(find ${K8S_YAMLS_LOCATION_STAGING} -type f \( -name "*.yml" -or -name "*.yaml" \)); \
	echo "$$files" | xargs -I {} kubectl --kubeconfig=$$kconfig apply -f {}

# Deployment YAMLs must have 'deployment' in their name.
.PHONY: update-deployment-image-name-staging
update-deployment-image-name-staging: CONTAINER=flow-test-net
update-deployment-image-name-staging:
	@files=$$(find ${K8S_YAMLS_LOCATION_STAGING} -type f \( -name "*.yml" -or -name "*.yaml" \) | grep deployment); \
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
