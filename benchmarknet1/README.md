1. Init network:

Adjust `./benchmarknet1/node-config.json`

2. Run bootstrap:

```sh
go run -tags relic ../cmd/bootstrap finalize --root-chain test --root-height 0 --root-parent 0000000000000000000000000000000000000000000000000000000000000000 --root-commit 8d75917eea88ea4243c2dab32b1024ebf00da33fa41988e55bc44462cb3a4fef --config ./benchmarknet1/node-config.json -o ./benchmarknet1/bootstrap/root-infos --fast-kg --partner-dir ./benchmarknet1/partner-nodes --partner-stakes ./benchmarknet1/partner-stakes.json
```

3. Generate tar file:

```sh
tar -cvf ./benchmarknet1/genesis-infos-benchmarknet1.tar ./benchmarknet1/root-infos/*
```
