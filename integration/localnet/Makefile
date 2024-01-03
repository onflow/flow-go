COLLECTION = 3
CONSENSUS = 2
VALID_CONSENSUS := $(shell test $(CONSENSUS) -ge 2; echo $$?)
EXECUTION = 2
VALID_EXECUTION := $(shell test $(EXECUTION) -ge 2; echo $$?)
VERIFICATION = 1
ACCESS = 1
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
LOGLEVEL=DEBUG

# The Git commit hash
COMMIT=$(shell git rev-parse HEAD)

# The version to include in container builds. Must be semver compliant
ifeq ($(VERSION),)
	VERSION := $(shell git describe --tags --abbrev=2 --match "v*" 2>/dev/null)-localnetbuild
endif

CURRENT_DIRECTORY=$(shell pwd)

# These vars set the default TPS values for tps-ci
# These can be overwritten at the time make is called on tps-ci
# by setting these as arguments at the time the make target is called
# e.g. "make tps-ci TPS_INIT=5 TPS_MIN=1 TPS_MAX=30"
TPS_MIN=100
TPS_MAX=1000
TPS_INIT=$(shell echo $$(( $(TPS_MAX) * 9 / 10 )) )
DURATION=30m

.PHONY: bootstrap
bootstrap:
ifeq ($(strip $(VALID_EXECUTION)), 1)
	# multiple execution nodes are required to prevent seals being generated in case of execution forking.
	$(error Number of Execution nodes should be no less than 2)
else ifeq ($(strip $(VALID_CONSENSUS)), 1)
	$(error Number of Consensus nodes should be no less than 2)
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
# (Except for consensus and execution nodes, which are set to 2)
.PHONY: bootstrap-light
bootstrap-light:
	$(MAKE) -e COLLECTION=1 CONSENSUS=2 EXECUTION=2 VERIFICATION=1 ACCESS=1 NCLUSTERS=1 bootstrap

# CI tests have a larger number of nodes
.PHONY: bootstrap-ci
bootstrap-ci:
	$(MAKE) -e COLLECTION=10 VERIFICATION=10 NCLUSTERS=10 LOGLEVEL=INFO bootstrap

# Creates a version of localnet configured with short epochs
.PHONY: bootstrap-short-epochs
bootstrap-short-epochs:
	$(MAKE) -e EPOCHLEN=200 STAKINGLEN=10 DKGLEN=50 bootstrap

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

.PHONY: load
load:
	go run ../benchmark/cmd/manual -log-level info -tps 1,10,100 -tps-durations 30s,30s

.PHONY: tps-ci-smoke
tps-ci-smoke:
	go run ../benchmark/cmd/ci -log-level info -tps-initial 1 -tps-min 1 -tps-max 10 -duration 20s -tps-adjust-interval 1s -stat-interval 1s -bigquery-upload=false

.PHONY: tps-ci
tps-ci: bootstrap-ci build-flow start-flow
	go run ../benchmark/cmd/ci -log-level info -tps-initial $(TPS_INIT) -tps-min $(TPS_MIN) -tps-max $(TPS_MAX) -duration $(DURATION)

.PHONY: clean-data
clean-data:
	DOCKER_BUILDKIT=1 docker build -t environment-clean ../../cmd
	docker run --mount=type=bind,source="$(CURRENT_DIRECTORY)"/data,target=/data environment-clean chmod -R 777 /data

	# deletes all generated files and folders from bootstrap and test running
	rm -rf ./data
	rm -rf ./bootstrap
	rm -rf ./trie
	rm -rf ./profiler
	rm -f ./targets.nodes.json
	rm -f ./docker-compose.nodes.yml

# deletes the stopped environment-clean container(s) - running this command inside another target doesn't delete the containers so it's isolated to run in a separate target
# Note: running this target shows an error on the command line "make: *** [clean-data2] Error 1" but the container is still deleted
.PHONY: clean-data2
clean-data2:
	docker rm $(shell docker ps -aq --filter ancestor=environment-clean)
	#$(shell docker rm $$(docker ps -aq --filter ancestor=environment-clean))

