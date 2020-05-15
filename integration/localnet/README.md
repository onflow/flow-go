# Flow Local Instrumented Test Environment (FLITE)

## Bootstrap the network

Bootstrap a new network:

```sh
make init
```

You can optionally specify the number of nodes for each role:

```sh
make -e COLLECTION=2 CONSENSUS=5 EXECUTION=3 VERIFICATION=2 ACCESS=2 init
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
- Traces (Jaeger): http://localhost:16686/
- Grafana: http://localhost:3000/
  - Username: `admin`
  - Password: `admin`

Here's an example of a Prometheus query that filters by the `consensus` role:

```
avg(rate(consensus_finalized_blocks{role="consensus"}[2m]))
```
