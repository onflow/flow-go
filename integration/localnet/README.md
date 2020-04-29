# Local Instrumented Test Environment (LITE)

## Starting the network

Due to the fact that `docker-compose` doesn't fully support BuildKit with the `--ssh` secret, the base layer for the containers must be built before starting the network:

```sh
cd ../../
make docker-build-consensus
```

### Start with Docker Compose

```sh
DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 docker-compose up --build
```
