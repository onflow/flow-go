COLLECTION = 3
CONSENSUS = 3
EXECUTION = 2
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
TRACING=true
EXTENSIVE_TRACING=false
CADENCE_TRACING=false

# The Git commit hash
COMMIT=$(shell git rev-parse HEAD)
VERSION=localnetbuild

.PHONY: init
init:
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
		-tracing=$(TRACING) \
		-cadence-tracing=$(CADENCE_TRACING) \
		-extensive-tracing=$(EXTENSIVE_TRACING) \
		-consensus-delay=$(CONSENSUS_DELAY) \
		-collection-delay=$(COLLECTION_DELAY)

# Creates a light version of the localnet with just 1 instance for each node type
.PHONY: init-light
init-light:
	$(MAKE) -e COLLECTION=1 CONSENSUS=2 EXECUTION=1 VERIFICATION=1 ACCESS=1 NCLUSTERS=1 init

# Creates a version of localnet configured with short epochs
.PHONY: init-short-epochs
init-short-epochs:
	$(MAKE) -e EPOCHLEN=200 STAKINGLEN=10 DKGLEN=50 init

# Installs logging plugin
.PHONY: install-docker-plugin
install-docker-plugin:
	docker plugin inspect loki || \
		docker plugin install grafana/loki-docker-driver:latest --alias loki --grant-all-permissions

.PHONY: start
start: install-docker-plugin
	DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 docker-compose -f docker-compose.metrics.yml up -d --remove-orphans
	DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 docker-compose -f docker-compose.nodes.yml build
	DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 docker-compose -f docker-compose.nodes.yml up -d

.PHONY: stop
stop:
	DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 docker-compose -f docker-compose.metrics.yml -f docker-compose.nodes.yml down -v

.PHONY: load
load:
	go run --tags relic ../loader -log-level info -tps 1,0,10,0,100 -tps-durations 30s,10s,30s,10s
