## Intro
Admin tool allows us to dynamically change settings of the running node without a restart. It can be used to change log level, and turn on profiler etc.

## Usage

### List all commands
```
curl localhost:9002/admin/run_command -H 'Content-Type: application/json' -d '{"commandName": "list-commands"}'
```

### To change log level
Flow, and other zerolog-based libraries:

```
curl localhost:9002/admin/run_command -H 'Content-Type: application/json' -d '{"commandName": "set-log-level", "data": "debug"}'
```

libp2p, badger, and other golog-based libraries:

```
curl localhost:9002/admin/run_command -H 'Content-Type: application/json' -d '{"commandName": "set-golog-level", "data": "debug"}'
```

### To get the latest finalized block
```
curl localhost:9002/admin/run_command -H 'Content-Type: application/json' -d '{"commandName": "read-blocks", "data": { "block": "final" }}'
```

### To get the latest sealed block
```
curl localhost:9002/admin/run_command -H 'Content-Type: application/json' -d '{"commandName": "read-blocks", "data": { "block": "sealed" }}'
```

### To get block by height
```
curl localhost:9002/admin/run_command -H 'Content-Type: application/json' -d '{"commandName": "read-blocks", "data": { "block": 24998900 }}'
```

### To get identity by peer id
```
curl localhost:9002/admin/run_command -H 'Content-Type: application/json' -d '{"commandName": "get-latest-identity", "data": { "peer_id": "QmNqszdfyEZmMCXcnoUdBDWboFvVLF5reyKPuiqFQT77Vw" }}'
```

### To get transactions for ranges (only available to staked access and execution nodes)
```
curl localhost:9002/admin/run_command -H 'Content-Type: application/json' -d '{"commandName": "get-transactions", "data": { "start-height": 340, "end-height": 343 }}'
```

### To get blocks for ranges (works for any node type, for block payload, only prints the collection ids)
```
curl localhost:9002/admin/run_command -H 'Content-Type: application/json' -d '{"commandName": "read-range-blocks", "data": { "start-height": 105172044, "end-height": 105172047 }}'
```

### To get cluster block for ranges (only available to collection nodes, only prints the transaction ids)

```
curl localhost:9002/admin/run_command -H 'Content-Type: application/json' -d '{"commandName": "read-range-cluster-blocks", "data": { "chain-id": "cluster-576-e8af4702d837acb77868a95f61eb212f90b14c6b7d61c89f48949fd27d1a269b", "start-height": 25077, "end-height": 25080 }}'
```

### To get execution data for a block by execution_data_id (only available execution nodes and access nodes with execution sync enabled)
```
curl localhost:9002/admin/run_command -H 'Content-Type: application/json' -d '{"commandName": "read-execution-data", "data": { "execution_data_id": "2fff2b05e7226c58e3c14b3549ab44a354754761c5baa721ea0d1ea26d069dc4" }}'
```

#### To get a list of all updatable configs
```
curl localhost:9002/admin/run_command -H 'Content-Type: application/json' -d '{"commandName": "list-configs"}'
```

### To get a config value
```
curl localhost:9002/admin/run_command -H 'Content-Type: application/json' -d '{"commandName": "get-config", "data": "consensus-required-approvals-for-sealing"}'
```

### To set a config value
#### Example: require 1 approval for consensus sealing
```
curl localhost:9002/admin/run_command -H 'Content-Type: application/json' -d '{"commandName": "set-config", "data": {"consensus-required-approvals-for-sealing": 1}}'
```
TODO remove
#### Example: set block rate delay to 750ms
```
curl localhost:9002/admin/run_command -H 'Content-Type: application/json' -d '{"commandName": "set-config", "data": {"hotstuff-block-rate-delay": "750ms"}}'
```
#### Example: enable the auto-profiler
```
curl localhost:9002/admin/run_command -H 'Content-Type: application/json' -d '{"commandName": "set-config", "data": {"profiler-enabled": true}}'
```
#### Example: manually trigger the auto-profiler for 1 minute
```
curl localhost:9002/admin/run_command -H 'Content-Type: application/json' -d '{"commandName": "set-config", "data": {"profiler-trigger": "1m"}}'
```

### Set a stop height
```
curl localhost:9002/admin/run_command -H 'Content-Type: application/json' -d '{"commandName": "stop-at-height", "data": { "height": 1111, "crash": false }}'
```

### Add/Remove/Get address to rate limit a payer from adding transactions to collection nodes' mempool
```
curl localhost:9002/admin/run_command -H 'Content-Type: application/json' -d '{"commandName": "ingest-tx-rate-limit", "data": { "command": "add", "addresses": "a08d349e8037d6e5,e6765c6113547fb7" }}'
curl localhost:9002/admin/run_command -H 'Content-Type: application/json' -d '{"commandName": "ingest-tx-rate-limit", "data": { "command": "remove", "addresses": "a08d349e8037d6e5,e6765c6113547fb7" }}'
curl localhost:9002/admin/run_command -H 'Content-Type: application/json' -d '{"commandName": "ingest-tx-rate-limit", "data": { "command": "get" }'
curl localhost:9002/admin/run_command -H 'Content-Type: application/json' -d '{"commandName": "ingest-tx-rate-limit", "data": { "command": "get_config" }'
// to limit 1 per second
curl localhost:9002/admin/run_command -H 'Content-Type: application/json' -d '{"commandName": "ingest-tx-rate-limit", "data": { "command": "set_config", "limit": 1, "burst": 1 }'
```
