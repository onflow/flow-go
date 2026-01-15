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
  -ledger-service-socket /sockets/ledger.sock
```

## Flags

- `-triedir`: Directory for trie files (required)
- `-ledger-service-tcp`: TCP listen address (e.g., 0.0.0.0:9000). If provided, server accepts TCP connections.
- `-ledger-service-socket`: Unix socket path (e.g., /sockets/ledger.sock). If provided, server accepts Unix socket connections. Can specify multiple sockets separated by comma.
- **Note**: At least one of `-ledger-service-tcp` or `-ledger-service-socket` must be provided.
- `-mtrie-cache-size`: MTrie cache size - number of tries (default: 500)
- `-checkpoint-distance`: Checkpoint distance (default: 100)
- `-checkpoints-to-keep`: Number of checkpoints to keep (default: 3)
- `-max-request-size`: Maximum request message size in bytes (default: 1 GiB)
- `-max-response-size`: Maximum response message size in bytes (default: 10 GiB)

## API

The service implements the `LedgerService` gRPC interface defined in `ledger/protobuf/ledger.proto`:

- `InitialState()` - Returns the initial state of the ledger
- `HasState()` - Checks if a state exists
- `GetSingleValue()` - Gets a single value for a key
- `Get()` - Gets multiple values for keys
- `Set()` - Updates keys with new values
- `Prove()` - Generates proofs for keys

