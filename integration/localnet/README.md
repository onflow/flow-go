# Flow Local Instrumented Test Environment (FLITE)

FLITE is a tool for running a full version of the Flow blockchain.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
## Table of Contents

- [Bootstrapping](#bootstrapping)
  - [Configuration](#configuration)
  - [Profiling](#profiling)
- [Start the network](#start-the-network)
- [Stop the network](#stop-the-network)
- [Logs](#logs)
- [Metrics](#metrics)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Bootstrapping

Before running the Flow network it is necessary to run a bootstrapping process. 
This generates keys for each of the nodes and a genesis block to build on.

Bootstrap a new network:

```sh
make init
```

### Configuration

Various properties of the local network can be configured when it is initialized.
All configuration is optional.

Specify the number of nodes for each role:

```sh
make -e COLLECTION=2 CONSENSUS=5 EXECUTION=3 VERIFICATION=2 ACCESS=2 init
```

Specify the number of collector clusters:

```sh
make -e NCLUSTERS=3 init
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

## Loader

Localnet can be loaded easily as well

```
make load
```

The command by default will load your localnet with 1 tps for 30s, then 10 tps for 30s, and finally 100 tps indefinitely.

More about the loader can be found in the loader module.
