## Intro
Admin tool allows us to dynamically change settings of the running node without a restart. It can be used to change log level, and turn on profiler etc.

## Usage

### List all commands
```
curl localhost:9002/admin/run_command -H 'Content-Type: application/json' -d '{"commandName": "list-commands"}'
```

### To change log level
```
curl localhost:9002/admin/run_command -H 'Content-Type: application/json' -d '{"commandName": "set-log-level", "data": "debug"}'
```

### To turn on profiler
```
curl localhost:9002/admin/run_command -H 'Content-Type: application/json' -d '{"commandName": "set-profiler-enabled", "data": true}'
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

