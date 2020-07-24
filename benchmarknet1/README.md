1. Init network:

Adjust `./benchmarknet1/node-config.json`

2. Run bootstrap:

```sh
go run -tags relic ./cmd/bootstrap finalize --root-chain test --root-height 0 --root-parent 0000000000000000000000000000000000000000000000000000000000000000 --root-commit 0000000000000000000000000000000000000000000000000000000000000000 --config ./benchmarknet1/node-config.json -o ./benchmarknet1/bootstrap --fast-kg --partner-dir ./benchmarknet1/partner-nodes --partner-stakes ./benchmarknet1/partner-stakes.json --collection-clusters 1
```

3. Generate tar file:

```sh
tar -cvf ./benchmarknet1/genesis-infos-benchmarknet1.tar -C ./benchmarknet1/bootstrap .
```
