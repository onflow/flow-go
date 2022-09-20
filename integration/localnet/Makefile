COLLECTION = 3
CONSENSUS = 3
VALID_CONSENSUS := $(shell test $(CONSENSUS) -ge 2; echo $$?)
EXECUTION = 2
VALID_EXECUTION := $(shell test $(EXECUTION) -ge 2; echo $$?)
VERIFICATION = 1
ACCESS = 2
OBSERVER = 0
NCLUSTERS=1
EPOCHLEN=10000   # 0 means use default
STAKINGLEN=2000 # 0 means use default
DKGLEN=2000     # 0 means use default
CONSENSUS_DELAY=800ms
COLLECTION_DELAY=950ms

PROFILER=false
PROFILE_UPLOADER=false
TRACING=true
EXTENSIVE_TRACING=false
CADENCE_TRACING=false

# The Git commit hash
BRANCH_NAME=$(shell git rev-parse --abbrev-ref HEAD)
COMMIT=$(shell git rev-parse HEAD)
VERSION=localnetbuild

.PHONY: init
init:
ifeq ($(strip $(VALID_EXECUTION)), 1)
	# multiple execution nodes are required to prevent seals being generated in case of execution forking.
	$(error Number of Execution nodes should be no less than 2)
else ifeq ($(strip $(VALID_CONSENSUS)), 1)
	$(error Number of Consensus nodes should be no less than 2)
else
	go run -tags relic \
		-ldflags="-X 'github.com/onflow/flow-go/cmd/build.commit=${COMMIT}' \
		-X  'github.com/onflow/flow-go/cmd/build.semver=${VERSION}'" \
		bootstrap.go \
		-collection=$(COLLECTION) \
		-consensus=$(CONSENSUS) \
		-execution=$(EXECUTION) \
		-verification=$(VERIFICATION) \
		-access=$(ACCESS) \
		-observer=$(OBSERVER) \
		-nclusters=$(NCLUSTERS) \
		-epoch-length=$(EPOCHLEN) \
		-epoch-staking-phase-length=$(STAKINGLEN) \
		-epoch-dkg-phase-length=$(DKGLEN) \
		-profiler=$(PROFILER) \
		-profile-uploader=$(PROFILE_UPLOADER) \
		-tracing=$(TRACING) \
		-cadence-tracing=$(CADENCE_TRACING) \
		-extensive-tracing=$(EXTENSIVE_TRACING) \
		-consensus-delay=$(CONSENSUS_DELAY) \
		-collection-delay=$(COLLECTION_DELAY)
endif

# Creates a light version of the localnet with just 1 instance for each node type
.PHONY: init-light
init-light:
	$(MAKE) -e COLLECTION=1 CONSENSUS=2 EXECUTION=1 VERIFICATION=1 ACCESS=1 NCLUSTERS=1 init

# Creates a version of localnet configured with short epochs
.PHONY: init-short-epochs
init-short-epochs:
	$(MAKE) -e EPOCHLEN=200 STAKINGLEN=10 DKGLEN=50 init

.PHONY: start
start:
	DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 docker-compose -f docker-compose.metrics.yml up -d --remove-orphans
	DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 docker-compose -f docker-compose.nodes.yml build
	DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 docker-compose -f docker-compose.nodes.yml up -d

.PHONY: stop
stop:
	DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 docker-compose -f docker-compose.metrics.yml -f docker-compose.nodes.yml down -v --remove-orphans

.PHONY: load
load:
	go run --tags relic ../benchmark/cmd/manual -log-level info -tps 1,0,10,0,100 -tps-durations 30s,10s,30s,10s

.PHONY: tps-test
tps-test:
	go run --tags relic ../benchmark/cmd/ci -log-level info -tps 25 -tps-durations 10m

.PHONY: tps-test-ci
tps-test-ci:
	make init
	make start
	go run --tags relic ../benchmark/cmd/ci -ci-run -tps 1
	make stop

gen-bootstrap:
	go run -tags relic ../../cmd/bootstrap genconfig --address-format "%s%d.${NAMESPACE}:3569" --access 2 --collection 6 --consensus 3 --execution 2 --verification 1 --weight 1000 -o ./ --config ./bootstrap/conf/node-config.json
	go run -tags relic ../../cmd/bootstrap keygen --machine-account --config ./bootstrap/conf/node-config.json -o ./bootstrap/keys
	echo {} > ./bootstrap/conf/partner-stakes.json
	mkdir ./bootstrap/partner-nodes
	go run -tags relic ../../cmd/bootstrap rootblock  --root-chain bench --root-height 0 --root-parent 0000000000000000000000000000000000000000000000000000000000000000 --config ./bootstrap/conf/node-config.json -o ./bootstrap/ --fast-kg --partner-dir ./bootstrap/partner-nodes --partner-weights ./bootstrap/conf/partner-stakes.json --internal-priv-dir ./bootstrap/keys/private-root-information
	go run -tags relic ../../cmd/bootstrap finalize --root-commit 0000000000000000000000000000000000000000000000000000000000000000 --service-account-public-key-json "{\"PublicKey\":\"tgiZNE8XebtMbfWoHbc+V4HYld8G3ulR6BPrqZ523TySiMzrWlqa2jkGcfYLcfP9JlPKXE4czG+LWmK+bRclag==\",\"SignAlgo\":2,\"HashAlgo\":1,\"SeqNumber\":0,\"Weight\":1000}" --config ./bootstrap/conf/node-config.json -o ./bootstrap/ --partner-dir ./bootstrap/partner-nodes --partner-weights ./bootstrap/conf/partner-stakes.json --collection-clusters 1 --epoch-counter 0 --epoch-length 1600 --epoch-staking-phase-length 50 --epoch-dkg-phase-length 500 --genesis-token-supply="1000000000.0" --protocol-version=0 --internal-priv-dir ./bootstrap/keys/private-root-information --dkg-data ./bootstrap/private-root-information/root-dkg-data.priv.json --root-block ./bootstrap/public-root-information/root-block.json --root-block-votes-dir ./bootstrap/public-root-information/root-block-votes/

k8s-secrets-create:
	bash ./create-secrets.sh

helm-deploy:
	helm upgrade --install -f ./values.yml ${NAMESPACE} ./flow --debug --namespace ${NAMESPACE}

k8s-delete:
	helm delete ${NAMESPACE} --namespace ${NAMESPACE}
	kubectl delete pvc -l service=flow --namespace ${NAMESPACE}

k8s-delete-secrets:
	kubectl delete secrets -l service=flow  --namespace ${NAMESPACE}

k8s-expose-locally:
	kubectl port-forward service/access1 9000:9000 --namespace ${NAMESPACE}

k8s-pod-health:
	kubectl get pods

k8s-test-network-accessibility:
	flow blocks get latest --host localhost:9000
	flow accounts create --network benchnet --key  e0ef5e52955e6542287db4528b3e8acc84a2c204eee9609f7c3120d1dac5a11b1bcb39677511db14354aa8c1a0ef62151220d97f015d49a8f0b78b653b570bfd --signer benchnet-account -f ./flow.json