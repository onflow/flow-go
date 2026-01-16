# Remote Ledger Transport Abstraction - Proof of Concept

This is a proof-of-concept implementation for refactoring the remote ledger service to support multiple transport mechanisms, including shmipc-go for zero-copy communication.

## Architecture

The transport abstraction allows switching between different transport mechanisms (gRPC, shmipc-go) without changing the client/server implementation.

```
ledger/remote/
├── transport/
│   ├── interface.go          # Transport interface definitions
│   ├── factory.go            # Factory placeholders (to avoid import cycles)
│   ├── handler_adapter.go    # Adapter from ledger.Ledger to ServerHandler
│   ├── grpc/
│   │   ├── client.go         # gRPC client transport
│   │   └── server.go         # gRPC server transport
│   └── shmipc/
│       ├── client.go         # shmipc-go client transport (POC)
│       └── server.go         # shmipc-go server transport (POC)
├── transport_factory.go      # Factory functions (avoids import cycles)
└── client_transport.go       # Transport-based client implementation
```

## Current Status

### ✅ Completed

1. **Transport Interface** (`transport/interface.go`)
   - `ClientTransport` interface for client-side communication
   - `ServerTransport` interface for server-side communication
   - `ServerHandler` interface for request handling
   - Transport type definitions

2. **gRPC Transport** (`transport/grpc/`)
   - Client implementation wrapping existing gRPC client
   - Server implementation adapting to transport interface
   - Full compatibility with existing gRPC code

3. **Transport Factory** (`transport_factory.go`)
   - Factory functions to create clients/servers
   - Avoids import cycles by being in the remote package

4. **Handler Adapter** (`transport/handler_adapter.go`)
   - Adapts `ledger.Ledger` to `ServerHandler` interface
   - Handles protobuf conversions

5. **Transport Client** (`client_transport.go`)
   - New client implementation using transport abstraction
   - Implements `ledger.Ledger` interface
   - Can switch between gRPC and shmipc-go

### 🚧 In Progress / Needs Fixing

1. **shmipc-go Transport** (`transport/shmipc/`)
   - Client and server implementations created
   - **Issue**: shmipc-go API usage needs to be corrected
   - **Issue**: Need to verify correct usage of `Stream.BufferWriter()` and `Stream.BufferReader()`
   - **Issue**: Config fields may have different names

2. **Protobuf Compatibility**
   - Using old `github.com/golang/protobuf/proto` package
   - Need to ensure all protobuf messages work correctly

## Usage

### Creating a Client

```go
import (
    "github.com/onflow/flow-go/ledger/remote"
    "github.com/onflow/flow-go/ledger/remote/transport"
)

// Using gRPC (existing)
client, err := remote.NewTransportClient(
    transport.TransportTypeGRPC,
    "unix:///tmp/ledger.sock",
    logger,
    1<<30, // maxRequestSize
    1<<30, // maxResponseSize
    0,     // bufferSize (not used for gRPC)
)

// Using shmipc-go (new)
client, err := remote.NewTransportClient(
    transport.TransportTypeShmipc,
    "unix:///tmp/ledger.sock",
    logger,
    0,     // maxRequestSize (not used for shmipc)
    0,     // maxResponseSize (not used for shmipc)
    1<<30, // bufferSize (shared memory size)
)
```

### Creating a Server

```go
import (
    "net"
    "github.com/onflow/flow-go/ledger/remote"
    "github.com/onflow/flow-go/ledger/remote/transport"
)

listener, _ := net.Listen("unix", "/tmp/ledger.sock")

// Create handler adapter
handler := transport.NewLedgerHandlerAdapter(ledger)

// Create transport server
server, err := remote.NewTransportServer(
    transport.TransportTypeShmipc,
    listener,
    logger,
    0, 0, 1<<30, // sizes
)

// Start server
go server.Serve(handler)
```

## Next Steps

1. **Fix shmipc-go API Usage**
   - Review shmipc-go documentation/examples
   - Correct `Stream.BufferWriter()` and `BufferReader()` usage
   - Fix config field names
   - Verify `Reserve()` and `Flush()` methods

2. **Testing**
   - Create integration tests
   - Test with real shared memory
   - Benchmark against gRPC

3. **Docker Configuration**
   - Update Docker setup to share IPC namespace
   - Configure shared memory size
   - Test in Docker environment

4. **Error Handling**
   - Improve error handling and retry logic
   - Handle shared memory errors gracefully

5. **Documentation**
   - Add usage examples
   - Document performance characteristics
   - Add migration guide

## Dependencies

- `github.com/cloudwego/shmipc-go v0.2.0` - Added to go.mod
- Existing protobuf and gRPC dependencies

## Notes

- The transport abstraction avoids import cycles by placing factory functions in the `remote` package
- Protobuf messages use the older `github.com/golang/protobuf/proto` package
- shmipc-go requires Linux and proper shared memory configuration
