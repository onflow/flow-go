# Flow Local Instrumented Test Environment (FLITE)

FLITE is a tool for running a full version of the Flow blockchain

## Bootstrapping

Before running the Flow network it is necessary to run a bootstrapping process. 
This generates keys for each of the nodes and a genesis block to build on.

Bootstrap a new network:

```sh
make init
```

You can optionally specify the number of nodes for each role:

```sh
make -e COLLECTION=2 CONSENSUS=5 EXECUTION=3 VERIFICATION=2 ACCESS=2 init
```

### Profiling

You can turn on automatic profiling for all nodes. Profiles are written every 2
minutes to `./profiler`.

```sh
make -e PROFILER=true init
```

## Start the network

This command will automatically build new Docker images from your latest code changes
and then start the test network:

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
