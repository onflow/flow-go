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
./flow-ledger-service \
  -wal-dir /path/to/wal \
  -grpc-addr 0.0.0.0:9000 \
  -capacity 100 \
  -checkpoint-distance 100 \
  -checkpoints-to-keep 3
```

## Flags

- `-wal-dir`: Directory for WAL files (required)
- `-grpc-addr`: gRPC server listen address (default: 0.0.0.0:9000)
- `-capacity`: Ledger capacity - number of tries (default: 100)
- `-checkpoint-distance`: Checkpoint distance (default: 100)
- `-checkpoints-to-keep`: Number of checkpoints to keep (default: 3)

## API

The service implements the `LedgerService` gRPC interface defined in `ledger/protobuf/ledger.proto`:

- `InitialState()` - Returns the initial state of the ledger
- `HasState()` - Checks if a state exists
- `GetSingleValue()` - Gets a single value for a key
- `Get()` - Gets multiple values for keys
- `Set()` - Updates keys with new values
- `Prove()` - Generates proofs for keys

