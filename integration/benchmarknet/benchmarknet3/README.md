1. Adjust/generate config:

Adjust `./integration/benchmarknet/benchmarknet3/node-config.json` or

```sh
go run -tags relic ./cmd/bootstrap genconfig --address-format "%s-%03d.benchmarknet3.nodes.onflow.org:3569" --access 5 --collection 50 --consensus 50 --execution 2 --verification 9 -o ./integration/benchmarknet/benchmarknet3/
```

2. Run bootstrap:

```sh
go run -tags relic ./cmd/bootstrap finalize --root-chain test --root-height 0 --root-parent 0000000000000000000000000000000000000000000000000000000000000000 --root-commit 0000000000000000000000000000000000000000000000000000000000000000 --config ./integration/benchmarknet/benchmarknet3/node-config.json -o ./integration/benchmarknet/benchmarknet3/bootstrap --fast-kg --partner-dir ./integration/benchmarknet/benchmarknet3/partner-nodes --partner-stakes ./integration/benchmarknet/benchmarknet3/partner-stakes.json --collection-clusters 2
```

3. Generate tar file:

```sh
tar -cvf ./integration/benchmarknet/benchmarknet3/genesis-infos-benchmarknet3.tar -C ./integration/benchmarknet/benchmarknet3/bootstrap .
```

4. Build the containers:

```sh
make docker-build-flow
make docker-build-loader
```

5. Tag the containers:

```sh
docker tag gcr.io/dl-flow/collection:latest gcr.io/dl-flow/benchmark/collection:benchmarknet3
docker tag gcr.io/dl-flow/consensus:latest gcr.io/dl-flow/benchmark/consensus:benchmarknet3
docker tag gcr.io/dl-flow/execution:latest gcr.io/dl-flow/benchmark/execution:benchmarknet3
docker tag gcr.io/dl-flow/verification:latest gcr.io/dl-flow/benchmark/verification:benchmarknet3
docker tag gcr.io/dl-flow/access:latest gcr.io/dl-flow/benchmark/access:benchmarknet3
docker tag gcr.io/dl-flow/loader:latest gcr.io/dl-flow/benchmark/loader:benchmarknet3
```

6. Push all containers:

```sh
docker push gcr.io/dl-flow/benchmark/collection:benchmarknet3
docker push gcr.io/dl-flow/benchmark/consensus:benchmarknet3
docker push gcr.io/dl-flow/benchmark/execution:benchmarknet3
docker push gcr.io/dl-flow/benchmark/verification:benchmarknet3
docker push gcr.io/dl-flow/benchmark/access:benchmarknet3
docker push gcr.io/dl-flow/benchmark/loader:benchmarknet3
```
