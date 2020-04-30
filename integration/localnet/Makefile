CONSENSUS = 3

.PHONY: init
init:
	go run -tags relic bootstrap.go -consensus=$(CONSENSUS)

.PHONY: start
start:
	DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 docker-compose -f docker-compose.base.yml -f docker-compose.nodes.yml up --build -d

.PHONY: logs
logs:
	docker-compose -f docker-compose.base.yml -f docker-compose.nodes.yml logs -f

.PHONY: stop
stop:
	docker-compose -f docker-compose.base.yml -f docker-compose.nodes.yml down -v

