1. Init network:

Adjust `./benchmarknet2/node-config.json`

2. Run bootstrap:

```sh
go run -tags relic ./cmd/bootstrap finalize --root-chain test --root-height 0 --root-parent 0000000000000000000000000000000000000000000000000000000000000000 --root-commit 0000000000000000000000000000000000000000000000000000000000000000 --config ./benchmarknet2/node-config.json -o ./benchmarknet2/bootstrap --fast-kg --partner-dir ./benchmarknet2/partner-nodes --partner-stakes ./benchmarknet2/partner-stakes.json --collection-clusters 1
```

3. Generate tar file:

```sh
tar -cvf ./benchmarknet2/genesis-infos-benchmarknet2.tar -C ./benchmarknet2/bootstrap .
```
