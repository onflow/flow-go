# Local Instrumented Test Environment (LITE)

## Bootstrap the network

Bootstrap a new network:

```sh
make init
```

You can optionally specify the number of consensus nodes:

```sh
make -e CONSENSUS=5 init
```

## Start the network

This command will automatically build new Docker images from your latest code changes and then start the test network:

```sh
make start
```

## Stop the network

The network needs to be stopped between each consecutive run to clear the chain state:

```sh
make stop
```

## Logs

You can view log output from all nodes:

```sh
make logs
```

## Metrics

You can view realtime metrics while the network is running:

- Prometheus: http://localhost:9090/
- Grafana: http://localhost:3000/
  - Username: `admin`
  - Password: `admin`
