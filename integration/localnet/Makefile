.PHONY: start
start:
	DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 docker-compose up --build -d

.PHONY: logs
logs:
	docker-compose logs -f consensus_1 consensus_2 consensus_3

.PHONY: stop
stop:
	docker-compose down
	docker-compose rm -v consensus_1 consensus_2 consensus_3
