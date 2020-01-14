# The short Git commit hash
SHORT_COMMIT := $(shell git rev-parse --short HEAD)
# The Git commit hash
COMMIT := $(shell git rev-parse HEAD)
# The tag of the current commit, otherwise empty
VERSION := $(shell git describe --tags --abbrev=0 --exact-match 2>/dev/null)
# Image tag
IMAGE_TAG := ${VERSION}
ifeq (${IMAGE_TAG},)
IMAGE_TAG := ${SHORT_COMMIT}
endif
# Name of the cover profile
COVER_PROFILE := cover.out
# Disable go sum database lookup for private repos
GOPRIVATE=github.com/dapperlabs/*
# OS
UNAME := $(shell uname)

# The location of the k8s YAML files
K8S_YAMLS_LOCATION_STAGING=./k8s/staging

export FLOW_ABI_EXAMPLES_DIR := $(CURDIR)/language/abi/examples/
export DOCKER_BUILDKIT := 1

crypto/relic:
	rm -rf crypto/relic
	git submodule update --init --recursive

crypto/relic/build: crypto/relic
	./crypto/relic_build.sh

cmd/collection/collection:
	go build -o cmd/collection/collection cmd/collection/main.go

.PHONY: install-tools
install-tools: crypto/relic/build check-go-version
ifeq ($(UNAME), Linux)
	sudo apt-get -y install capnproto
endif
ifeq ($(UNAME), Darwin)
	brew install capnp
endif
	cd ${GOPATH}; \
	GO111MODULE=on go get github.com/golang/protobuf/protoc-gen-go@v1.3.2; \
	GO111MODULE=on go get github.com/uber/prototool/cmd/prototool@v1.9.0; \
	GO111MODULE=on go get zombiezen.com/go/capnproto2@v0.0.0-20190505172156-0c36f8f86ab2; \
	GO111MODULE=on go get github.com/mgechev/revive@master; \
	GO111MODULE=on go get github.com/vektra/mockery/cmd/mockery@v0.0.0-20181123154057-e78b021dcbb5; \
	GO111MODULE=on go get github.com/golang/mock/mockgen@v1.3.1; \
	GO111MODULE=on go get golang.org/x/tools/cmd/stringer@master; \
	GO111MODULE=on go get github.com/kevinburke/go-bindata/...@v3.11.0;

.PHONY: test
test: generate-mocks
	# test all packages with Relic library enabled
	GO111MODULE=on go test -coverprofile=$(COVER_PROFILE) $(if $(JSON_OUTPUT),-json,) --tags relic ./...
	$(MAKE) -C crypto test
	$(MAKE) -C language test

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

.PHONY: generate-capnp
generate-capnp:
	capnp compile -I${GOPATH}/src/zombiezen.com/go/capnproto2/std -ogo schema/captain/*.capnp

.PHONY: generate-mocks
generate-mocks:
	GO111MODULE=on mockgen -destination=storage/mocks/storage.go -package=mocks github.com/dapperlabs/flow-go/storage Blocks,Collections,StateCommitments
	GO111MODULE=on mockgen -destination=module/mocks/network.go -package=mocks github.com/dapperlabs/flow-go/module Network,Local
	GO111MODULE=on mockgen -destination=network/mocks/conduit.go -package=mocks github.com/dapperlabs/flow-go/network Conduit
	GO111MODULE=on mockgen -destination=network/mocks/engine.go -package=mocks github.com/dapperlabs/flow-go/network Engine
	GO111MODULE=on mockgen -destination=protocol/mocks/state.go -package=mocks github.com/dapperlabs/flow-go/protocol State
	GO111MODULE=on mockgen -destination=protocol/mocks/snapshot.go -package=mocks github.com/dapperlabs/flow-go/protocol Snapshot
	mockery -name '.*' -dir=module -case=underscore -output="./module/mock" -outpkg="mock"
	mockery -name '.*' -dir=module/mempool -case=underscore -output="./module/mempool/mock" -outpkg="mempool"
	mockery -name '.*' -dir=network -case=underscore -output="./network/mock" -outpkg="mock"
	mockery -name '.*' -dir=storage -case=underscore -output="./storage/mock" -outpkg="mock"
	mockery -name '.*' -dir=protocol -case=underscore -output="./protocol/mock" -outpkg="mock"
	mockery -name '.*' -dir=engine/execution/execution/executor -case=underscore -output="./engine/execution/execution/executor/mock" -outpkg="mock"
	mockery -name '.*' -dir=engine/execution/execution/state -case=underscore -output="./engine/execution/execution/state/mock" -outpkg="mock"
	mockery -name '.*' -dir=engine/execution/execution/virtualmachine -case=underscore -output="./engine/execution/execution/virtualmachine/mock" -outpkg="mock"
	mockery -name 'Processor' -dir="./engine/consensus/eventdriven/components/pacemaker/events" -case=underscore -output="./engine/consensus/eventdriven/components/pacemaker/mock" -outpkg="mock"
	mockery -name '.*' -dir=network/gossip/libp2p/middleware -case=underscore -output="./network/gossip/libp2p/mock" -outpkg="mock"

.PHONY: lint
lint:
	GO111MODULE=on revive -config revive.toml ./...

.PHONY: ci
ci: install-tools lint test coverage

.PHONY: docker-ci
docker-ci:
	docker run --env COVER=$(COVER) --env JSON_OUTPUT=$(JSON_OUTPUT) \
		-v "$(CURDIR)":/go/flow -v "/tmp/.cache":"/root/.cache" -v "/tmp/pkg":"/go/pkg" \
		-w "/go/flow" gcr.io/dl-flow/golang-cmake:v0.0.5 \
		make ci

# This command is should only be used by Team City
# Includes a TeamCity specific git fix, ref:https://github.com/akkadotnet/akka.net/issues/2834#issuecomment-494795604
.PHONY: docker-ci-team-city
docker-ci-team-city:
	docker run --env COVER=$(COVER) --env JSON_OUTPUT=$(JSON_OUTPUT) \
		-v "$(CURDIR)":/go/flow -v "/tmp/.cache":"/root/.cache" -v "/tmp/pkg":"/go/pkg" -v /opt/teamcity/buildAgent/system/git:/opt/teamcity/buildAgent/system/git \
		-w "/go/flow" gcr.io/dl-flow/golang-cmake:v0.0.5 \
		make ci

.PHONY: docker-build-collection
docker-build-collection:
	docker build -f cmd/Dockerfile --build-arg TARGET=collection -t gcr.io/dl-flow/collection:latest -t "gcr.io/dl-flow/collection:$(SHORT_COMMIT)" -t gcr.io/dl-flow/collection:$(IMAGE_TAG) .

.PHONY: docker-build-consensus
docker-build-consensus:
	docker build -f cmd/Dockerfile --build-arg TARGET=consensus -t gcr.io/dl-flow/consensus:latest -t "gcr.io/dl-flow/consensus:$(SHORT_COMMIT)" -t "gcr.io/dl-flow/consensus:$(IMAGE_TAG)" .

.PHONY: docker-build-execution
docker-build-execution:
	docker build -f cmd/Dockerfile --build-arg TARGET=execution -t gcr.io/dl-flow/execution:latest -t "gcr.io/dl-flow/execution:$(SHORT_COMMIT)" -t "gcr.io/dl-flow/execution:$(IMAGE_TAG)" .

.PHONY: docker-build-verification
docker-build-verification:
	docker build -f cmd/Dockerfile --build-arg TARGET=verification -t gcr.io/dl-flow/verification:latest -t "gcr.io/dl-flow/verification:$(SHORT_COMMIT)" -t "gcr.io/dl-flow/verification:$(IMAGE_TAG)" .

.PHONY: docker-build-flow
docker-build-flow: docker-build-collection docker-build-consensus docker-build-execution docker-build-verification

.PHONY: docker-push-flow
docker-push-flow:
	echo gcr.io/dl-flow/{collection,consensus,execution,verification}:{$(SHORT_COMMIT),$(IMAGE_TAG)} | xargs -n 1 docker push

.PHONY: docker-run-collection
docker-run-collection:
	docker run -p 8080:8080 -p 3569:3569 gcr.io/dl-flow/collection:latest --nodeid 1234567890123456789012345678901234567890123456789012345678901234 --entries collection-1234567890123456789012345678901234567890123456789012345678901234@localhost:3569=1000

.PHONY: docker-run-consensus
docker-run-consensus:
	docker run -p 8080:8080 -p 3569:3569 gcr.io/dl-flow/consensus:latest --nodeid 1234567890123456789012345678901234567890123456789012345678901234 --entries consensus-1234567890123456789012345678901234567890123456789012345678901234@localhost:3569=1000

.PHONY: docker-run-execution
docker-run-execution:
	docker run -p 8080:8080 -p 3569:3569 gcr.io/dl-flow/execution:latest --nodeid 1234567890123456789012345678901234567890123456789012345678901234 --entries execution-1234567890123456789012345678901234567890123456789012345678901234@localhost:3569=1000

.PHONY: docker-run-verification
docker-run-verification:
	docker run -p 8080:8080 -p 3569:3569 gcr.io/dl-flow/verification:latest --nodeid 1234567890123456789012345678901234567890123456789012345678901234 --entries verification-1234567890123456789012345678901234567890123456789012345678901234@localhost:3569=1000

# Builds the VS Code extension
.PHONY: build-vscode-extension
build-vscode-extension:
	cd language/tools/vscode-extension && npm run package;

# Builds and promotes the VS Code extension. Promoting here means re-generating
# the embedded extension binary used in the CLI for installation.
.PHONY: promote-vscode-extension
promote-vscode-extension: build-vscode-extension
	cp language/tools/vscode-extension/cadence-*.vsix ./cli/flow/cadence/vscode/cadence.vsix;
	go generate ./cli/flow/cadence/vscode;

# Check if the go version is 1.13. flow-go only supports go 1.13
.PHONY: check-go-version
check-go-version:
	go version | grep 1.13

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
 		kubectl patch -f $$file -p '{"spec":{"template":{"spec":{"containers":[{"name":"${CONTAINER}","image":"gcr.io/dl-flow/'"$$node"':${IMAGE_TAG}"}]}}}}`' --local -o yaml > $$patched; \
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
