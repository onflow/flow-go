COLLECTION = 1
CONSENSUS = 3
EXECUTION = 1
VERIFICATION = 1
ACCESS = 1
NCLUSTERS=1

.PHONY: init
init:
	go run -tags relic bootstrap.go \
		-collection=$(COLLECTION) \
		-consensus=$(CONSENSUS) \
		-execution=$(EXECUTION) \
		-verification=$(VERIFICATION) \
		-access=$(ACCESS) \
		-nclusters=$(NCLUSTERS)

.PHONY: start
start:
	DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 docker-compose -f docker-compose.base.yml -f docker-compose.nodes.yml up --build -d

.PHONY: nodes
nodes:
	DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 docker-compose -f docker-compose.nodes.yml up --build -d

.PHONY: logs
logs:
	docker-compose -f docker-compose.base.yml -f docker-compose.nodes.yml logs -f

.PHONY: stop
stop:
	docker-compose -f docker-compose.base.yml -f docker-compose.nodes.yml down -v
