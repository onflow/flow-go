# Analysis: Refactoring Remote Ledger Service with shmipc-go

## Executive Summary

The remote ledger service is a **strong candidate** for refactoring with [shmipc-go](https://github.com/cloudwego/shmipc-go) due to its IO-intensive nature and large message sizes. However, the refactoring would require significant architectural changes and careful consideration of trade-offs.

## Current Implementation

### Architecture
- **Location**: `ledger/remote/`
- **Protocol**: gRPC over Unix domain sockets or TCP
- **Client**: `ledger/remote/client.go` - implements `ledger.Ledger` interface
- **Server**: `ledger/remote/service.go` - gRPC service implementation
- **Standalone Service**: `cmd/ledger/main.go` - runs as separate process

### Key Characteristics

1. **Large Message Sizes**
   - Maximum message size: **1 GiB** (default)
   - Proofs can be very large (explicitly mentioned in code comments)
   - Code comment: *"This was increased to fix 'grpc: received message larger than max' errors when generating proofs for blocks with many state changes"*

2. **Operations**
   - `InitialState()` - Returns initial ledger state
   - `HasState()` - Checks if state exists
   - `GetSingleValue()` - Gets single value
   - `Get()` - Gets multiple values (batch reads)
   - `Set()` - Updates ledger (batch writes)
   - `Prove()` - Generates proofs (large responses)

3. **Usage Pattern**
   - Inter-process communication between execution node and ledger service
   - IO-intensive operations
   - Currently uses Unix domain sockets (with data copying overhead)

## shmipc-go Overview

### Key Features
- **Zero-copy communication** using Linux shared memory (`memfd_create`)
- Uses Unix domain sockets or TCP **only for synchronization**, not data transfer
- **Batch IO support** - can queue multiple requests before synchronization
- Better performance for IO-intensive or large-package scenarios

### Performance Benefits (from library benchmarks)
- Small packets (64B-1KB): Comparable to Unix domain sockets
- Large packets (4KB-4MB): **2-3x performance improvement**
- Zero-copy eliminates kernel-user space data copying

## Feasibility Assessment

### ✅ Advantages

1. **Perfect Use Case Match**
   - Large message sizes (proofs up to 1 GiB)
   - IO-intensive operations
   - Inter-process communication
   - Currently uses Unix domain sockets

2. **Performance Gains**
   - Zero-copy would eliminate data copying overhead
   - Batch IO could reduce system calls
   - Significant improvement expected for large proofs

3. **Architecture Fit**
   - Standalone service model fits shmipc-go's design
   - Client-server pattern aligns with library usage

### ⚠️ Challenges

1. **Platform Limitations** ✅ **NOT A CONCERN**
   - shmipc-go is **Linux-specific** (uses `memfd_create`)
   - **Deployment is Linux-only in Docker** - perfect match!
   - No need for fallback mechanism

2. **Protocol Replacement**
   - Current: gRPC with protobuf (type-safe, well-defined)
   - shmipc-go: Custom protocol (would need to implement)
   - Loss of gRPC features:
     - Automatic code generation from `.proto` files
     - Built-in error handling and status codes
     - Streaming support (not currently used, but available)
     - Standardized RPC semantics

3. **Implementation Complexity**
   - Need to implement custom serialization/deserialization
   - Manual protocol definition and versioning
   - Error handling and retry logic
   - Connection management and lifecycle

4. **Testing and Maintenance**
   - More complex testing (shared memory lifecycle)
   - Debugging shared memory issues is harder
   - Less tooling support compared to gRPC

5. **Migration Path**
   - Would need to maintain both implementations during transition
   - Feature parity verification
   - Performance benchmarking

## Recommended Approach

### Option 1: Hybrid Approach (Recommended)

Create a **transport abstraction layer** that allows switching between gRPC and shmipc-go:

```
ledger/remote/
├── transport/
│   ├── interface.go          # Transport interface
│   ├── grpc/
│   │   ├── client.go
│   │   └── server.go
│   └── shmipc/
│       ├── client.go
│       └── server.go
├── client.go                 # Uses transport interface
└── service.go                # Uses transport interface
```

**Benefits:**
- Can benchmark both implementations
- Gradual migration path
- Fallback to gRPC on non-Linux platforms
- A/B testing capability

### Option 2: Full Migration

Replace gRPC entirely with shmipc-go:

**Pros:**
- Maximum performance
- Simpler architecture (single transport)

**Cons:**
- Platform-specific (Linux only)
- Loss of gRPC ecosystem benefits
- Higher risk

### Option 3: Keep Current Implementation

**Reasons:**
- gRPC is well-established and maintainable
- Current performance may be sufficient
- Cross-platform support
- Better tooling and debugging

## Implementation Considerations

### Docker Configuration

Since deployment is Docker-only, ensure proper shared memory configuration:

```yaml
# docker-compose.yml example
services:
  ledger_service:
    # ... other config ...
    shm_size: 2g  # Ensure sufficient shared memory
    ipc: host      # Or use shared IPC namespace
    
  execution_node:
    # ... other config ...
    shm_size: 2g
    ipc: host      # Share IPC namespace with ledger_service
```

Or use Docker's `--ipc=container:ledger_service` to share IPC namespace.

### Protocol Design

1. **Message Format**
   - **Keep protobuf for serialization** - just change transport layer
   - Reuse existing `ledger/protobuf/ledger.proto` definitions
   - Serialize protobuf messages into shared memory buffers
   - Use shmipc-go's `Stream.BufferWriter()` and `Stream.BufferReader()` for zero-copy

2. **Protocol Structure**
   ```
   [Message Type (1 byte)] [Protobuf Length (4 bytes)] [Protobuf Data (variable)]
   ```
   - Message types: InitialState, HasState, GetSingleValue, Get, Set, Prove
   - Error responses: [Error Code (1 byte)] [Error Message (variable)]

3. **Version Negotiation**
   - Add version handshake during connection establishment
   - Support protocol versioning for future changes

### Shared Memory Management

1. **Lifecycle**
   - Create shared memory regions on connection
   - Use shmipc-go's built-in management
   - Cleanup on connection close

2. **Size Configuration**
   - Default: 1 GiB (matching current max message size)
   - Configurable via flag: `--shmipc-buffer-size`
   - Consider multiple buffers for concurrent requests

3. **Memory Pool**
   - Reuse shared memory buffers
   - Pre-allocate buffers for common operations
   - Monitor memory usage

### Batch Operations

Leverage shmipc-go's batch IO capabilities:

1. **Request Batching**
   - Queue multiple `Get()` requests before synchronization
   - Batch `Set()` operations when possible
   - Reduce system calls significantly

2. **Implementation Pattern**
   ```go
   // Client side
   stream := session.GetStream()
   writer := stream.BufferWriter()
   
   // Queue multiple requests
   for _, req := range requests {
       writer.Reserve(len(serializedReq))
       writer.Write(serializedReq)
   }
   
   // Single synchronization
   writer.Flush()
   ```

### Error Handling

1. **Connection Failures**
   - Retry with exponential backoff (similar to current gRPC client)
   - Handle shared memory creation failures
   - Fallback to gRPC if shmipc-go unavailable

2. **Shared Memory Errors**
   - Handle out-of-memory scenarios
   - Buffer overflow detection
   - Connection reset handling

3. **Error Codes**
   - Map gRPC status codes to custom error codes
   - Maintain compatibility with existing error handling

### Testing Strategy

1. **Unit Tests**
   - Protocol serialization/deserialization
   - Error handling paths
   - Message type encoding

2. **Integration Tests**
   - Real shared memory communication
   - Docker-based tests with IPC namespace sharing
   - Large message handling (1 GiB proofs)

3. **Performance Benchmarks**
   - Compare shmipc-go vs gRPC for each operation
   - Measure latency and throughput
   - Test with various message sizes
   - Profile memory usage

4. **Stress Testing**
   - Concurrent requests
   - Large proof generation
   - Memory pressure scenarios

## Performance Expectations

Based on shmipc-go benchmarks and current usage:

- **Small operations** (InitialState, HasState): Minimal improvement (~10-20%)
- **Medium operations** (GetSingleValue, small Get): Moderate improvement (~30-50%)
- **Large operations** (large Get, Set with many keys): Significant improvement (~2-3x)
- **Proof generation** (Prove with large proofs): **Major improvement** (~2-4x expected)

## Recommendation

**✅ STRONGLY RECOMMEND: Proceed with shmipc-go refactoring**

Given that:
- ✅ **Linux-only deployment** (no platform concerns)
- ✅ **Docker-only environment** (perfect for shared memory IPC)
- ✅ **Large message sizes** (proofs up to 1 GiB)
- ✅ **IO-intensive operations** (frequent ledger reads/writes)

**Recommended Approach: Option 1 (Hybrid with Transport Abstraction)**

This allows:
- Benchmarking both implementations side-by-side
- Gradual migration with fallback capability
- A/B testing to validate performance improvements
- Maintaining gRPC as backup during transition

**Alternative: Option 2 (Direct Migration)** if:
- Performance is critical and immediate improvement is needed
- Team is confident in shmipc-go implementation
- Simpler architecture is preferred (single transport)

## Docker Configuration for shmipc-go

### Current Setup
The ledger service and execution nodes currently communicate via Unix domain sockets mounted in a shared volume (`/sockets`).

### Required Changes for shmipc-go

**Option 1: Shared IPC Namespace (Recommended)**

Modify `integration/localnet/builder/bootstrap.go`:

```go
// In prepareLedgerService()
service.IpcMode = "shareable"  // Make ledger service IPC shareable

// In prepareExecutionService() when using remote ledger
if i < ledgerExecutionCount {
    service.IpcMode = fmt.Sprintf("container:%s", "ledger_service_1")
    // ... existing socket mount code ...
}
```

**Option 2: Host IPC Namespace**

```go
// Both services
service.IpcMode = "host"
```

**Option 3: Shared Memory Size**

Ensure sufficient shared memory:

```go
service.ShmSize = "2g"  // 2 GiB shared memory
```

### Updated Docker Compose Structure

The `Service` struct in `bootstrap.go` would need:

```go
type Service struct {
    // ... existing fields ...
    IpcMode string  // IPC namespace mode
    ShmSize string  // Shared memory size
}
```

### Migration Path

1. **Phase 1: Add shmipc-go support alongside gRPC**
   - Add transport abstraction
   - Implement shmipc-go client/server
   - Add feature flag: `--ledger-transport=grpc|shmipc`

2. **Phase 2: Docker configuration**
   - Add IPC namespace sharing
   - Configure shared memory size
   - Update docker-compose generation

3. **Phase 3: Testing and benchmarking**
   - Performance comparison
   - Stress testing
   - Validation

4. **Phase 4: Gradual rollout**
   - Enable shmipc-go for test environments
   - Monitor and validate
   - Switch default transport

## Next Steps (if proceeding)

1. **Proof of Concept**
   - Implement basic shmipc-go transport for one operation (e.g., `GetSingleValue`)
   - Add IPC namespace configuration to Docker setup
   - Benchmark against gRPC implementation
   - Validate performance improvements

2. **Design Review**
   - Protocol design (protobuf over shmipc)
   - Error handling strategy
   - Docker IPC configuration
   - Migration plan

3. **Implementation Plan**
   - Transport abstraction layer (`ledger/remote/transport/`)
   - shmipc-go client/server implementation
   - Docker configuration updates
   - Testing strategy (Docker-based integration tests)
   - Documentation

## References

- [shmipc-go GitHub](https://github.com/cloudwego/shmipc-go)
- Current implementation: `ledger/remote/`
- gRPC service definition: `ledger/protobuf/ledger.proto`
