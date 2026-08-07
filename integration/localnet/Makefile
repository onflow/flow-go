COLLECTION = 3
CONSENSUS = 2
VALID_CONSENSUS := $(shell test $(CONSENSUS) -ge 2; echo $$?)
EXECUTION = 2
VALID_EXECUTION := $(shell test $(EXECUTION) -ge 2; echo $$?)
LEDGER_EXECUTION = 0
VALID_LEDGER_EXECUTION := $(shell test $(LEDGER_EXECUTION) -le $(EXECUTION); echo $$?)
TEST_EXECUTION = 0
VERIFICATION = 1
ACCESS = 1
OBSERVER = 0
NCLUSTERS=1
EPOCHLEN=10000                  # 0 means use default
STAKINGLEN=2000                 # 0 means use default
DKGLEN=2000                     # 0 means use default
KVSTORE_VERSION=default
EPOCH_EXTENSION_LEN=600
FINALIZATION_SAFETY_THRESHOLD=100 # 0 means use default
CONSENSUS_DELAY=800ms
COLLECTION_DELAY=950ms

PROFILER=false
PROFILE_UPLOADER=false
TRACING=true
EXTENSIVE_TRACING=false
LOGLEVEL=DEBUG

# The Git commit hash
COMMIT=$(shell git rev-parse HEAD)

# The version to include in container builds. Must be semver compliant
ifeq ($(VERSION),)
	VERSION := $(shell git describe --tags --abbrev=2 --match "v*" 2>/dev/null)-localnetbuild
endif

CURRENT_DIRECTORY=$(shell pwd)

.PHONY: bootstrap
bootstrap:
ifeq ($(strip $(VALID_EXECUTION)), 1)
	# multiple execution nodes are required to prevent seals being generated in case of execution forking.
	$(error Number of Execution nodes should be no less than 2)
else ifeq ($(strip $(VALID_CONSENSUS)), 1)
	$(error Number of Consensus nodes should be no less than 2)
else ifeq ($(strip $(VALID_LEDGER_EXECUTION)), 1)
	$(error LEDGER_EXECUTION ($(LEDGER_EXECUTION)) should not be greater than EXECUTION ($(EXECUTION)))
else
	go run \
		-ldflags="-X 'github.com/onflow/flow-go/cmd/build.commit=${COMMIT}' \
		-X  'github.com/onflow/flow-go/cmd/build.semver=${VERSION}'" \
		builder/*.go \
		-loglevel=$(LOGLEVEL) \
		-collection=$(COLLECTION) \
		-consensus=$(CONSENSUS) \
		-execution=$(EXECUTION) \
		-verification=$(VERIFICATION) \
		-access=$(ACCESS) \
		-observer=$(OBSERVER) \
		-test-execution=$(TEST_EXECUTION) \
		-nclusters=$(NCLUSTERS) \
		-epoch-length=$(EPOCHLEN) \
		-epoch-staking-phase-length=$(STAKINGLEN) \
		-epoch-dkg-phase-length=$(DKGLEN) \
		-kvstore-version=$(KVSTORE_VERSION) \
		-kvstore-epoch-extension-view-count=$(EPOCH_EXTENSION_LEN) \
		-kvstore-finalization-safety-threshold=$(FINALIZATION_SAFETY_THRESHOLD) \
		-profiler=$(PROFILER) \
		-profile-uploader=$(PROFILE_UPLOADER) \
		-tracing=$(TRACING) \
		-extensive-tracing=$(EXTENSIVE_TRACING) \
		-consensus-delay=$(CONSENSUS_DELAY) \
		-collection-delay=$(COLLECTION_DELAY) \
		-ledger-execution=$(LEDGER_EXECUTION)
endif

# Creates a light version of the localnet with just 1 instance for each node type
# (Except for consensus and execution nodes, which are set to 2)
.PHONY: bootstrap-light
bootstrap-light:
	$(MAKE) -e COLLECTION=1 CONSENSUS=2 EXECUTION=2 VERIFICATION=1 ACCESS=1 NCLUSTERS=1 bootstrap

bootstrap-test-en:
	$(MAKE) -e COLLECTION=1 CONSENSUS=2 EXECUTION=2 VERIFICATION=1 ACCESS=1 NCLUSTERS=1 TEST_EXECUTION=1 bootstrap

# Creates a version of localnet configured with short epochs
.PHONY: bootstrap-short-epochs
bootstrap-short-epochs:
	$(MAKE) -e EPOCHLEN=200 STAKINGLEN=5 DKGLEN=50 FINALIZATION_SAFETY_THRESHOLD=20 bootstrap

# Starts the network - must have been bootstrapped first. Builds fresh images.
.PHONY: start
start: start-metrics start-flow

# Starts the network, using the most recently built images.
# Useful for rapid iteration, when no code has changed or only one images needs to be re-built.
.PHONY: start-cached
start-cached: start-metrics start-flow-cached

# Starts metrics services
.PHONY: start-metrics
start-metrics:
	DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 docker compose -f docker-compose.metrics.yml up -d

# Starts a version of localnet with just flow nodes and without metrics services.
# This prevents port collision and consumption when these services are not needed.
# All images are re-built prior to being started.
.PHONY: start-flow
start-flow:
	DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 docker compose -f docker-compose.nodes.yml up -d --build

# Same as start-flow, but most recently built images are used.
.PHONY: start-flow-cached
start-flow-cached:
	DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 docker compose -f docker-compose.nodes.yml up -d

.PHONY: build-flow
build-flow:
	DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 docker compose -f docker-compose.nodes.yml build

.PHONY: stop
stop:
	DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 docker compose -f docker-compose.metrics.yml -f docker-compose.nodes.yml down -v --remove-orphans

.PHONY: stop-flow
stop-flow:
	DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 docker compose -f docker-compose.nodes.yml down -v

# Smoke-checks that the network is running and sealing blocks, by polling the
# access_1 REST API twice and confirming the sealed height increased.
.PHONY: verify
verify:
	@get_height() { curl --connect-timeout 2 --max-time 10 -sf "http://localhost:4004/v1/blocks?height=sealed" | sed -n 's/.*"height": "\([0-9]*\)".*/\1/p' | head -1; }; \
	h1=$$(get_height); \
	if [ -z "$$h1" ]; then \
		echo "FAIL: could not query sealed block height from http://localhost:4004 - is the network running? (make start)"; \
		exit 1; \
	fi; \
	sleep 5; \
	h2=$$(get_height); \
	if [ -z "$$h2" ] || [ "$$h2" -le "$$h1" ]; then \
		echo "FAIL: sealed height did not increase ($$h1 -> $$h2)"; \
		exit 1; \
	fi; \
	echo "OK: localnet is sealing blocks (height $$h1 -> $$h2)"

.PHONY: clean-data
clean-data:
	DOCKER_BUILDKIT=1 docker build -t environment-clean ../../cmd
	docker run --rm --mount=type=bind,source="$(CURRENT_DIRECTORY)"/data,target=/data environment-clean chmod -R 777 /data

	# deletes all generated files and folders from bootstrap and test running
	rm -rf ./data
	rm -rf ./bootstrap
	rm -rf ./trie
	rm -rf ./profiler
	rm -rf ./sockets
	rm -f ./targets.nodes.json
	rm -f ./docker-compose.nodes.yml
	rm -f ./ports.nodes.json

# deletes the stopped environment-clean container(s) - running this command inside another target doesn't delete the containers so it's isolated to run in a separate target
# Note: running this target shows an error on the command line "make: *** [clean-data2] Error 1" but the container is still deleted
.PHONY: clean-data2
clean-data2:
	docker rm $(shell docker ps -aq --filter ancestor=environment-clean)
	#$(shell docker rm $$(docker ps -aq --filter ancestor=environment-clean))

