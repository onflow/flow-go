
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

export DOCKER_BUILDKIT := 1

crypto/relic:
	rm -rf crypto/relic
	git submodule update --init --recursive

crypto/relic/build: crypto/relic
	./crypto/relic_build.sh

crypto/relic/update:
	git submodule update --recursive

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
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b ${GOPATH}/bin v1.23.8; \
	cd ${GOPATH}; \
	GO111MODULE=on go get github.com/golang/protobuf/protoc-gen-go@v1.3.2; \
	GO111MODULE=on go get github.com/uber/prototool/cmd/prototool@v1.9.0; \
	GO111MODULE=on go get zombiezen.com/go/capnproto2@v0.0.0-20190505172156-0c36f8f86ab2; \
	GO111MODULE=on go get github.com/vektra/mockery/cmd/mockery@v0.0.0-20181123154057-e78b021dcbb5; \
	GO111MODULE=on go get github.com/golang/mock/mockgen@v1.3.1; \
	GO111MODULE=on go get golang.org/x/tools/cmd/stringer@master; \
	GO111MODULE=on go get github.com/kevinburke/go-bindata/...@v3.11.0;

.PHONY: unittest
unittest:
	# test all packages with Relic library enabled
	GO111MODULE=on go test -coverprofile=$(COVER_PROFILE) $(if $(JSON_OUTPUT),-json,) --tags relic ./...
	$(MAKE) -C crypto test
	$(MAKE) -C integration test

.PHONY: test
test: generate-mocks unittest

.PHONY: integration-test
integration-test: docker-build-flow
	$(MAKE) -C integration integration-test

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
	GO111MODULE=on mockgen -destination=storage/mocks/storage.go -package=mocks github.com/dapperlabs/flow-go/storage Blocks,Payloads,Collections,Commits
	GO111MODULE=on mockgen -destination=module/mocks/network.go -package=mocks github.com/dapperlabs/flow-go/module Network,Local
	GO111MODULE=on mockgen -destination=network/mocks/conduit.go -package=mocks github.com/dapperlabs/flow-go/network Conduit
	GO111MODULE=on mockgen -destination=network/mocks/engine.go -package=mocks github.com/dapperlabs/flow-go/network Engine
	GO111MODULE=on mockgen -destination=protocol/mocks/state.go -package=mocks github.com/dapperlabs/flow-go/protocol State
	GO111MODULE=on mockgen -destination=consensus/hotstuff/mocks/sig_verifier.go -package=mocks github.com/dapperlabs/flow-go/consensus/hotstuff SigVerifier
	GO111MODULE=on mockgen -destination=consensus/hotstuff/mocks/sig_aggregator.go -package=mocks github.com/dapperlabs/flow-go/consensus/hotstuff SigAggregator
	GO111MODULE=on mockgen -destination=protocol/mocks/snapshot.go -package=mocks github.com/dapperlabs/flow-go/protocol Snapshot
	GO111MODULE=on mockgen -destination=protocol/mocks/mutator.go -package=mocks github.com/dapperlabs/flow-go/protocol Mutator
	GO111MODULE=on mockery -name 'ExecutionState' -dir=engine/execution/state -case=underscore -output="engine/execution/state/mock" -outpkg="mock"
	GO111MODULE=on mockery -name 'BlockComputer' -dir=engine/execution/computation/computer -case=underscore -output="engine/execution/computation/computer/mock" -outpkg="mock"
	GO111MODULE=on mockery -name 'ComputationManager' -dir=engine/execution/computation -case=underscore -output="engine/execution/computation/mock" -outpkg="mock"
	GO111MODULE=on mockery -name 'ProviderEngine' -dir=engine/execution/provider -case=underscore -output="engine/execution/provider/mock" -outpkg="mock"
	GO111MODULE=on mockery -name '.*' -dir=cluster -case=underscore -output="cluster/mock" -outpkg="mock"
	GO111MODULE=on mockery -name '.*' -dir=module -case=underscore -output="./module/mock" -outpkg="mock"
	GO111MODULE=on mockery -name '.*' -dir=module/mempool -case=underscore -output="./module/mempool/mock" -outpkg="mempool"
	GO111MODULE=on mockery -name '.*' -dir=module/trace -case=underscore -output="./module/trace/mock" -outpkg="mock"
	GO111MODULE=on mockery -name '.*' -dir=network -case=underscore -output="./network/mock" -outpkg="mock"
	GO111MODULE=on mockery -name '.*' -dir=storage -case=underscore -output="./storage/mock" -outpkg="mock"
	GO111MODULE=on mockery -name '.*' -dir=protocol -case=underscore -output="./protocol/mock" -outpkg="mock"
	GO111MODULE=on mockery -name '.*' -dir=engine/execution/computation/computer -case=underscore -output="./engine/execution/computation/computer/mock" -outpkg="mock"
	GO111MODULE=on mockery -name '.*' -dir=engine/execution/state -case=underscore -output="./engine/execution/state/mock" -outpkg="mock"
	GO111MODULE=on mockery -name '.*' -dir=engine/execution/computation/virtualmachine -case=underscore -output="./engine/execution/computation/virtualmachine/mock" -outpkg="mock"
	GO111MODULE=on mockery -name '.*' -dir=network/gossip/libp2p/middleware -case=underscore -output="./network/gossip/libp2p/mock" -outpkg="mock"
	GO111MODULE=on mockery -name 'Consumer' -dir="./consensus/hotstuff/notifications/" -case=underscore -output="./consensus/hotstuff/notifications/mock" -outpkg="mock"
	GO111MODULE=on mockery -name 'Vertex' -dir="./consensus/hotstuff/forks/finalizer/forest" -case=underscore -output="./consensus/hotstuff/forks/finalizer/forest/mock" -outpkg="mock"
	GO111MODULE=on mockery -name '.*' -dir="./consensus/hotstuff" -case=underscore -output="./consensus/hotstuff/mock" -outpkg="mock"
	GO111MODULE=on mockery -name 'ObserveServiceClient' -dir=protobuf/services/observation -case=underscore -output="./engine/observation/mock" -outpkg="mock"


# this ensures there is no unused dependency being added by accident
.PHONY: tidy
tidy:
	go mod tidy; git diff --exit-code

.PHONY: lint
lint:
	# GO111MODULE=on revive -config revive.toml -exclude storage/ledger/trie ./...
	 golangci-lint run -v --build-tags relic ./...

# Runs unit tests, coverage, linter
.PHONY: ci
ci: install-tools tidy lint test coverage

# Runs integration tests
# NOTE: we do not need `docker-build-flow` as this is run as a separate step
# on Teamcity
.PHONY: ci-integration
ci-integration: install-tools
	$(MAKE) -C integration integration-test

# Runs unit tests, test coverage, lint in Docker
.PHONY: docker-ci
docker-ci:
	docker run --env COVER=$(COVER) --env JSON_OUTPUT=$(JSON_OUTPUT) \
		-v "$(CURDIR)":/go/flow -v "/tmp/.cache":"/root/.cache" -v "/tmp/pkg":"/go/pkg" \
		-w "/go/flow" gcr.io/dl-flow/golang-cmake:v0.0.7 \
		make ci

# This command is should only be used by Team City
# Includes a TeamCity specific git fix, ref:https://github.com/akkadotnet/akka.net/issues/2834#issuecomment-494795604
.PHONY: docker-ci-team-city
docker-ci-team-city:
	docker run --env COVER=$(COVER) --env JSON_OUTPUT=$(JSON_OUTPUT) \
		-v ${SSH_AUTH_SOCK}:/tmp/ssh_auth_sock -e SSH_AUTH_SOCK="/tmp/ssh_auth_sock" \
		-v "$(CURDIR)":/go/flow -v "/tmp/.cache":"/root/.cache" -v "/tmp/pkg":"/go/pkg" \
		-v /opt/teamcity/buildAgent/system/git:/opt/teamcity/buildAgent/system/git \
		-w "/go/flow" gcr.io/dl-flow/golang-cmake:v0.0.7 \
		make ci

# Runs integration tests in Docker
.PHONY: docker-ci-integration
docker-ci-integration:
	docker run \
		-v "$(CURDIR)":/go/flow -v "/tmp/.cache":"/root/.cache" -v "/tmp/pkg":"/go/pkg" \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-w "/go/flow" gcr.io/dl-flow/golang-cmake:v0.0.7 \
		make ci-integration

# This command is should only be used by Team City
# Includes a TeamCity specific git fix, ref:https://github.com/akkadotnet/akka.net/issues/2834#issuecomment-494795604
.PHONY: docker-ci-integration-team-city
docker-ci-integration-team-city:
	docker run \
		-v ${SSH_AUTH_SOCK}:/tmp/ssh_auth_sock -e SSH_AUTH_SOCK="/tmp/ssh_auth_sock" \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v "$(CURDIR)":/go/flow -v "/tmp/.cache":"/root/.cache" -v "/tmp/pkg":"/go/pkg" \
		-v /opt/teamcity/buildAgent/system/git:/opt/teamcity/buildAgent/system/git \
		-w "/go/flow" gcr.io/dl-flow/golang-cmake:v0.0.7 \
		make ci-integration

.PHONY: docker-build-collection
docker-build-collection:
	docker build --ssh default -f cmd/Dockerfile --build-arg TARGET=collection --target production \
		-t gcr.io/dl-flow/collection:latest -t "gcr.io/dl-flow/collection:$(SHORT_COMMIT)" -t gcr.io/dl-flow/collection:$(IMAGE_TAG) .

.PHONY: docker-build-collection-debug
docker-build-collection-debug:
	docker build --ssh default -f cmd/Dockerfile --build-arg TARGET=collection --target debug \
		-t gcr.io/dl-flow/collection-debug:latest -t "gcr.io/dl-flow/collection-debug:$(SHORT_COMMIT)" -t gcr.io/dl-flow/collection-debug:$(IMAGE_TAG) .

.PHONY: docker-build-consensus
docker-build-consensus:
	docker build --ssh default -f cmd/Dockerfile --build-arg TARGET=consensus --target production \
		-t gcr.io/dl-flow/consensus:latest -t "gcr.io/dl-flow/consensus:$(SHORT_COMMIT)" -t "gcr.io/dl-flow/consensus:$(IMAGE_TAG)" .

.PHONY: docker-build-consensus-debug
docker-build-consensus-debug:
	docker build --ssh default -f cmd/Dockerfile --build-arg TARGET=consensus --target debug \
		-t gcr.io/dl-flow/consensus-debug:latest -t "gcr.io/dl-flow/consensus-debug:$(SHORT_COMMIT)" -t "gcr.io/dl-flow/consensus-debug:$(IMAGE_TAG)" .

.PHONY: docker-build-execution
docker-build-execution:
	docker build --ssh default -f cmd/Dockerfile --build-arg TARGET=execution --target production \
		-t gcr.io/dl-flow/execution:latest -t "gcr.io/dl-flow/execution:$(SHORT_COMMIT)" -t "gcr.io/dl-flow/execution:$(IMAGE_TAG)" .

.PHONY: docker-build-execution-debug
docker-build-execution-debug:
	docker build --ssh default -f cmd/Dockerfile --build-arg TARGET=execution --target debug \
		-t gcr.io/dl-flow/execution-debug:latest -t "gcr.io/dl-flow/execution-debug:$(SHORT_COMMIT)" -t "gcr.io/dl-flow/execution-debug:$(IMAGE_TAG)" .

.PHONY: docker-build-verification
docker-build-verification:
	docker build --ssh default -f cmd/Dockerfile --build-arg TARGET=verification --target production \
		-t gcr.io/dl-flow/verification:latest -t "gcr.io/dl-flow/verification:$(SHORT_COMMIT)" -t "gcr.io/dl-flow/verification:$(IMAGE_TAG)" .

.PHONY: docker-build-verification-debug
docker-build-verification-debug:
	docker build --ssh default -f cmd/Dockerfile --build-arg TARGET=verification --target debug \
		-t gcr.io/dl-flow/verification-debug:latest -t "gcr.io/dl-flow/verification-debug:$(SHORT_COMMIT)" -t "gcr.io/dl-flow/verification-debug:$(IMAGE_TAG)" .

.PHONY: docker-build-observation
docker-build-observation:
	docker build --ssh default -f cmd/Dockerfile --build-arg TARGET=observation --target production \
		-t gcr.io/dl-flow/observation:latest -t "gcr.io/dl-flow/observation:$(SHORT_COMMIT)" -t "gcr.io/dl-flow/observation:$(IMAGE_TAG)" .

.PHONY: docker-build-observation-debug
docker-build-observation-debug:
	docker build --ssh default -f cmd/Dockerfile --build-arg TARGET=observation --target debug \
		-t gcr.io/dl-flow/observation-debug:latest -t "gcr.io/dl-flow/observation-debug:$(SHORT_COMMIT)" -t "gcr.io/dl-flow/observation-debug:$(IMAGE_TAG)" .

.PHONY: docker-build-flow
docker-build-flow: docker-build-collection docker-build-consensus docker-build-execution docker-build-verification docker-build-observation

.PHONY: docker-push-collection
docker-push-collection:
	docker push gcr.io/dl-flow/collection:latest
	docker push "gcr.io/dl-flow/collection:$(SHORT_COMMIT)"
	docker push "gcr.io/dl-flow/collection:$(IMAGE_TAG)"

.PHONY: docker-push-consensus
docker-push-consensus:
	docker push gcr.io/dl-flow/consensus:latest
	docker push "gcr.io/dl-flow/consensus:$(SHORT_COMMIT)"
	docker push "gcr.io/dl-flow/consensus:$(IMAGE_TAG)"

.PHONY: docker-push-execution
docker-push-execution:
	docker push gcr.io/dl-flow/execution:latest
	docker push "gcr.io/dl-flow/execution:$(SHORT_COMMIT)"
	docker push "gcr.io/dl-flow/execution:$(IMAGE_TAG)"

.PHONY: docker-push-verification
docker-push-verification:
	docker push gcr.io/dl-flow/verification:latest
	docker push "gcr.io/dl-flow/verification:$(SHORT_COMMIT)"
	docker push "gcr.io/dl-flow/verification:$(IMAGE_TAG)"

.PHONY: docker-push-observation
docker-push-observation:
	docker push gcr.io/dl-flow/observation:latest
	docker push "gcr.io/dl-flow/observation:$(SHORT_COMMIT)"
	docker push "gcr.io/dl-flow/observation:$(IMAGE_TAG)"

.PHONY: docker-push-flow
docker-push-flow: docker-push-collection docker-push-consensus docker-push-execution docker-push-verification docker-push-observation

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

.PHONY: docker-run-observation
docker-run-observation:
	docker run -p 9000:9000 -p 3569:3569 gcr.io/dl-flow/observation:latest --nodeid 1234567890123456789012345678901234567890123456789012345678901234 --entries observation-1234567890123456789012345678901234567890123456789012345678901234@localhost:3569=1000

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
