# Local Instrumented Test Environment (LITE)

## Start the network

Due to the fact that `docker-compose` doesn't fully support BuildKit with the `--ssh` secret, the base layer for the containers must be built before starting the network:

```sh
cd ../../
make docker-build-consensus
```

### Start with Docker Compose

```sh
DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 docker-compose up --build
```

### Stop the network

The network needs to be stopped between each consecutive run to clear the chain state:

```sh
docker-compose down -v
```

## Metrics

You can view realtime metrics while the network is running:

- Prometheus: http://localhost:9090/
- Grafana: http://localhost:3000/
  - Username: `admin`
  - Password: `admin`
