.PHONY: start
start:
	DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 docker-compose up --build -d

.PHONY: logs
logs:
	docker-compose logs -f consensus_1 consensus_2 consensus_3 consensus_4 consensus_5 consensus_6 consensus_7 consensus_8 consensus_9 consensus_10

.PHONY: stop
stop:
	docker-compose down
	docker-compose rm -v consensus_1 consensus_2 consensus_3 consensus_4 consensus_5 consensus_6 consensus_7 consensus_8 consensus_9 consensus_10
