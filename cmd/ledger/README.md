# Ledger Service

A standalone gRPC service that provides remote access to ledger operations.

## Building

The protobuf code must be generated first:

```bash
cd ledger/protobuf
buf generate
```

Then build the service:

```bash
go build -o flow-ledger-service ./cmd/ledger
```

## Running

```bash
# Listen on TCP only
./flow-ledger-service \
  -triedir /path/to/trie \
  -ledger-service-tcp 0.0.0.0:9000

# Listen on Unix socket only
./flow-ledger-service \
  -triedir /path/to/trie \
  -ledger-service-socket /sockets/ledger.sock

# Listen on both TCP and Unix socket
./flow-ledger-service \
  -triedir /path/to/trie \
  -ledger-service-tcp 0.0.0.0:9000 \
  -ledger-service-socket /sockets/ledger.sock \
  -mtrie-cache-size 500 \
  -checkpoint-distance 100 \
  -checkpoints-to-keep 3

# With admin server enabled (use port 9003 to avoid conflict with execution node's 9002)
./flow-ledger-service \
  -triedir /path/to/trie \
  -ledger-service-tcp 0.0.0.0:9000 \
  -admin-addr 0.0.0.0:9003
```

## Flags

- `-triedir`: Directory for trie files (required)
- `-ledger-service-tcp`: TCP listen address (e.g., 0.0.0.0:9000). If provided, server accepts TCP connections.
- `-ledger-service-socket`: Unix socket path (e.g., /sockets/ledger.sock). If provided, server accepts Unix socket connections. Can specify multiple sockets separated by comma.
- **Note**: At least one of `-ledger-service-tcp` or `-ledger-service-socket` must be provided.
- `-admin-addr`: Address to bind on for admin HTTP server (e.g., 0.0.0.0:9003). If provided, enables admin commands. Use a different port than the execution node's admin server (default 9002). Optional.
- `-mtrie-cache-size`: MTrie cache size - number of tries (default: 500)
- `-checkpoint-distance`: Checkpoint distance (default: 100)
- `-checkpoints-to-keep`: Number of checkpoints to keep (default: 3)
- `-max-request-size`: Maximum request message size in bytes (default: 1 GiB)
- `-max-response-size`: Maximum response message size in bytes (default: 1 GiB)
- `-loglevel`: Log level (panic, fatal, error, warn, info, debug) (default: info)

## Admin Commands

When `-admin-addr` is provided, the service exposes an HTTP admin API for managing the ledger service.

### Available Commands

- `trigger-checkpoint`: Triggers a checkpoint to be created as soon as the current WAL segment file is finished writing. This is useful for manually creating checkpoints without waiting for the automatic checkpoint distance.
- `ping`: Simple health check command to verify the admin server is responsive.
- `list-commands`: Lists all available admin commands.

**Examples:**
```bash
# Trigger a checkpoint
curl -X POST http://localhost:9003/admin/run_command \
  -H "Content-Type: application/json" \
  -d '{"commandName": "trigger-checkpoint", "data": {}}'

# Ping the admin server
curl -X POST http://localhost:9003/admin/run_command \
  -H "Content-Type: application/json" \
  -d '{"commandName": "ping", "data": {}}'

# List all available commands
curl -X POST http://localhost:9003/admin/run_command \
  -H "Content-Type: application/json" \
  -d '{"commandName": "list-commands", "data": {}}'
```

**Note:** When running an execution node with a remote ledger service (using `--ledger-service-addr`), the `trigger-checkpoint` command on the execution node is disabled. You must use the ledger service's admin endpoint to trigger checkpoints.

## API

The service implements the `LedgerService` gRPC interface defined in `ledger/protobuf/ledger.proto`:

- `InitialState()` - Returns the initial state of the ledger
- `HasState()` - Checks if a state exists
- `GetSingleValue()` - Gets a single value for a key
- `Get()` - Gets multiple values for keys
- `Set()` - Updates keys with new values
- `Prove()` - Generates proofs for keys

