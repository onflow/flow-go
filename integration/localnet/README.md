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
- Grafana: http://localhost:3000/
  - Username: `admin`
  - Password: `admin`

Here's an example of a Prometheus query that filters by the `consensus` role:

```
avg(rate(consensus_finalized_blocks{role="consensus"}[2m]))
```

## Profiling

Each node exposes a server compatible with the `pprof` profiling tool. This [blog post](https://klotzandrew.com/blog/golang-finding-memory-leaks) walks through a simple example of how to use the tool to diagnose memory leaks.

The profiler endpoints for each node are printed by the `make init` command:

```sh
Profilers:
- collection_1 pprof server will be accessible at localhost:7000
- consensus_1 pprof server will be accessible at localhost:7001
- consensus_2 pprof server will be accessible at localhost:7002
- consensus_3 pprof server will be accessible at localhost:7003
- execution_1 pprof server will be accessible at localhost:7004
- verification_1 pprof server will be accessible at localhost:7005
- access_1 pprof server will be accessible at localhost:7006
```

For example, you can view heap usage by the collection node with this command:

```sh
go tool pprof localhost:7000/debug/pprof/heap
```
