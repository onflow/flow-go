1. Adjust/generate config:

Adjust `./integration/benchmarknet/benchmarknet5/node-config.json` or

```sh
go run -tags relic ./cmd/bootstrap genconfig --address-format "%s-%03d.benchmarknet5.nodes.onflow.org:3569" --access 5 --collection 50 --consensus 90 --execution 2 --verification 9 -o ./integration/benchmarknet/benchmarknet5/
```

2. Run bootstrap:

```sh
go run -tags relic ./cmd/bootstrap finalize --root-chain test --root-height 0 --root-parent 0000000000000000000000000000000000000000000000000000000000000000 --root-commit 0000000000000000000000000000000000000000000000000000000000000000 --config ./integration/benchmarknet/benchmarknet5/node-config.json -o ./integration/benchmarknet/benchmarknet5/bootstrap --fast-kg --partner-dir ./integration/benchmarknet/benchmarknet5/partner-nodes --partner-stakes ./integration/benchmarknet/benchmarknet5/partner-stakes.json --collection-clusters 2
```

3. Generate tar file:

```sh
tar -cvf ./integration/benchmarknet/benchmarknet5/genesis-infos-benchmarknet5.tar -C ./integration/benchmarknet/benchmarknet5/bootstrap .
```

4. Build the containers:

```sh
make docker-build-flow
make docker-build-loader
```

5. Tag the containers:

```sh
docker tag gcr.io/dl-flow/collection:latest gcr.io/dl-flow/benchmark/collection:benchmarknet5; \
docker tag gcr.io/dl-flow/consensus:latest gcr.io/dl-flow/benchmark/consensus:benchmarknet5; \
docker tag gcr.io/dl-flow/execution:latest gcr.io/dl-flow/benchmark/execution:benchmarknet5; \
docker tag gcr.io/dl-flow/verification:latest gcr.io/dl-flow/benchmark/verification:benchmarknet5; \
docker tag gcr.io/dl-flow/access:latest gcr.io/dl-flow/benchmark/access:benchmarknet5; \
docker tag gcr.io/dl-flow/loader:latest gcr.io/dl-flow/benchmark/loader:benchmarknet5
```

6. Push all containers:

```sh
docker push gcr.io/dl-flow/benchmark/collection:benchmarknet5; \
docker push gcr.io/dl-flow/benchmark/consensus:benchmarknet5; \
docker push gcr.io/dl-flow/benchmark/execution:benchmarknet5; \
docker push gcr.io/dl-flow/benchmark/verification:benchmarknet5; \
docker push gcr.io/dl-flow/benchmark/access:benchmarknet5; \
docker push gcr.io/dl-flow/benchmark/loader:benchmarknet5
```
