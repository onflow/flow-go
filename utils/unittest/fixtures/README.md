# Flow Go Fixtures Module

A context-aware test fixture generation system for Flow Go that provides deterministic, reproducible test data with shared randomness across all generators. This module replaces the standalone fixture functions with a comprehensive suite of generator objects.

## Table of Contents

1. [Overview](#overview)
2. [Quick Start](#quick-start)
3. [Core Concepts](#core-concepts)
4. [Generator Suite](#generator-suite)
5. [Available Generators](#available-generators)
6. [Migration Guide](#migration-guide)
7. [Testing](#testing)
8. [Architecture](#architecture)

## Overview

The fixtures module replaces standalone fixture functions with a suite of context-aware generator objects that:

- **Share randomness**: All generators use the same RNG for consistency
- **Support deterministic results**: Explicit seed control for reproducible tests
- **Provide context awareness**: Generators can create related data that makes sense together
- **Enable easy extension**: Simple to add new generator types
- **Improve test reproducibility**: Consistent results across test runs

## Module Structure

The fixtures module is organized as follows:

```
utils/unittest/fixtures/
├── README.md              # This documentation
├── examples.go            # Usage examples
├── generators_test.go     # Test suite
├── generator.go           # Core GeneratorSuite and options
├── random.go              # RandomGenerator implementation
├── block_header.go        # Block header generator
├── identifier.go          # Identifier generator
├── signature.go           # Signature generator
├── address.go             # Address generator
├── signer_indices.go      # Signer indices generator
├── quorum_certificate.go  # Quorum certificate generator
├── chunk_execution_data.go # Chunk execution data generator
├── block_execution_data.go # Block execution data generator
├── transaction.go         # Transaction generator
├── collection.go          # Collection generator
├── trie_update.go         # Trie update generator
├── transaction_result.go  # Transaction result generator
├── transaction_signature.go # Transaction signature generator
├── proposal_key.go        # Proposal key generator
├── event.go               # Event generator
└── event_type.go          # Event type generator
```

## Quick Start

### Import and Create Suite

```go
import (
    "github.com/onflow/flow-go/utils/unittest/fixtures"
)

// Create suite with random seed
suite := fixtures.NewGeneratorSuite(t)

// Create suite with specific seed for deterministic reproducible results
suite := fixtures.NewGeneratorSuite(t, fixtures.WithSeed(12345))
```

### Basic Usage Examples

```go
// Generate a block header
header := suite.BlockHeaders().Fixture(t)

// Generate an identifier
id := suite.Identifiers().Fixture(t)

// Generate a collection with 3 transactions
collection := suite.Collections().Fixture(t, 3)

// Generate events for a transaction
txID := suite.Identifiers().Fixture(t)
events := suite.Events().ForTransaction(t, txID, 0, 3)
```

## Core Concepts

### Generator Suite
The `GeneratorSuite` is the central object that:
- Manages a shared random number generator
- Provides access to specialized generators
- Ensures consistent randomness across all generators

### Options Pattern
Most generators use an options pattern for configuration:
```go
// Example: Configure a block header
headerGen := suite.BlockHeaders()
header := headerGen.Fixture(t,
    headerGen.WithHeight(100),
    headerGen.WithView(200),
)
```

### Error Handling
All generators that can produce an error require a `testing.TB` parameter and use `require.NoError` internally to assert no errors are encountered:
```go
// No error handling needed - failures will fail the test
result := suite.TransactionResults().Fixture(t)
```

## Generator Suite

### Constructor Options

```go
// No options: Use random seed
suite := fixtures.NewGeneratorSuite(t)

// WithSeed(seed int64): Set specific seed for deterministic results
suite := fixtures.NewGeneratorSuite(t, fixtures.WithSeed(42))
```

### Core Methods

```go
// Access the shared RNG
rng := suite.RNG()

// Access the random generator
randomGen := suite.Random()

// Generate random bytes
bytes := randomGen.RandomBytes(t, 32)

// Generate random string
str := randomGen.RandomString(16)

// Generate random uint64 in range
num := randomGen.Uint64InRange(1, 100)
```

### Available Generators

| Generator | Method | Purpose |
|-----------|--------|---------|
| Block Headers | `BlockHeaders()` | Generate block headers with proper relationships |
| Identifiers | `Identifiers()` | Generate flow identifiers |
| Signatures | `Signatures()` | Generate cryptographic signatures |
| Addresses | `Addresses()` | Generate flow addresses |
| Signer Indices | `SignerIndices()` | Generate signer index arrays |
| Quorum Certificates | `QuorumCertificates()` | Generate quorum certificates |
| Chunk Execution Data | `ChunkExecutionDatas()` | Generate chunk execution data |
| Block Execution Data | `BlockExecutionDatas()` | Generate block execution data |
| Block Execution Data Entities | `BlockExecutionDataEntities()` | Generate block execution data entities |
| Transactions | `Transactions()` | Generate transaction bodies |
| Full Transactions | `FullTransactions()` | Generate complete transactions |
| Collections | `Collections()` | Generate collections of transactions |
| Trie Updates | `TrieUpdates()` | Generate ledger trie updates |
| Transaction Results | `TransactionResults()` | Generate transaction results |
| Light Transaction Results | `LightTransactionResults()` | Generate light transaction results |
| Transaction Signatures | `TransactionSignatures()` | Generate transaction signatures |
| Proposal Keys | `ProposalKeys()` | Generate proposal keys |
| Events | `Events()` | Generate events with encoding support |
| Event Types | `EventTypes()` | Generate event types |
| Time | `Time()` | Generate time.Time values |
| Ledger Paths | `LedgerPaths()` | Generate ledger paths |
| Ledger Payloads | `LedgerPayloads()` | Generate ledger payloads |
| Ledger Values | `LedgerValues()` | Generate ledger values |

## Available Generators

### Random Generator

The RandomGenerator provides consistent random value generation for all other generators.

**Methods:**
- `RandomBytes(t testing.TB, n int)`: Generates n random bytes
- `RandomString(length uint)`: Generates a random string of specified length
- `Uint64InRange(min, max uint64)`: Generates a random uint64 in the specified range [min, max]
- `Uint32()`: Generates a random uint32
- `Uint64()`: Generates a random uint64
- `Int31()`: Generates a random int32
- `Int63()`: Generates a random int64
- `Intn(n int)`: Generates a random int in the range [0, n)

```go
randomGen := suite.Random()

// Generate random bytes
bytes := randomGen.RandomBytes(t, 32)

// Generate random string
str := randomGen.RandomString(16)

// Generate random number in range
num := randomGen.Uint64InRange(1, 100)

// randomGen also exposes all methods from *rand.Rand

// Generate random uint32
val := randomGen.Uint32()

// Generate random uint64
val64 := randomGen.Uint64()

// Generate random int32
int32Val := randomGen.Int31()

// Generate random int64
int64Val := randomGen.Int63()

// Generate random int in range [0, n)
intVal := randomGen.Intn(100)
```

### Block Header Generator

Generates block headers with proper field relationships and chain-specific defaults. Uses an options pattern for configuration.

**Options:**
- `WithHeight(height uint64)`: Sets the height of the block header
- `WithView(view uint64)`: Sets the view of the block header
- `WithChainID(chainID flow.ChainID)`: Sets the chain ID of the block header
- `WithParent(parent *flow.Header)`: Sets the parent header (ignores height, view, and chainID if set)
- `WithParentAndSoR(parent *flow.Header, source []byte)`: Sets the parent and source of randomness

**Methods:**
- `Fixture(t testing.TB, opts ...func(*blockHeaderConfig))`: Generates a block header with optional configuration

```go
headerGen := suite.BlockHeaders()

// Basic header
header := headerGen.Fixture(t)

// Header with specific height
header := headerGen.Fixture(t,
    headerGen.WithHeight(100),
)

// Header with specific view
header := headerGen.Fixture(t,
    headerGen.WithView(200),
)

// Header for specific chain
header := headerGen.Fixture(t,
    headerGen.WithChainID(flow.Testnet),
)

// Header with parent
parent := headerGen.Fixture(t)
child := headerGen.Fixture(t,
    headerGen.WithParent(parent),
)

// Header with parent and source of randomness
parent := headerGen.Fixture(t)
source := suite.RandomBytes(t, 32)
child := headerGen.Fixture(t,
    headerGen.WithParentAndSoR(parent, source),
)
```

### Primitive Generators

#### Identifier Generator
```go
idGen := suite.Identifiers()

// Single identifier
id := idGen.Fixture(t)

// List of identifiers
ids := idGen.List(t, 5)
```

#### Signature Generator
```go
sigGen := suite.Signatures()

// Single signature
sig := sigGen.Fixture(t)

// List of signatures
sigs := sigGen.List(t, 3)
```

#### Address Generator
```go
addrGen := suite.Addresses()

// Default random address on default chain (Testnet)
addr := addrGen.Fixture(t)

// Address for specific chain
addr := addrGen.Fixture(t, addrGen.WithChainID(flow.Testnet))

// Service account address
addr := addrGen.Fixture(t, addrGen.ServiceAddress())

// Invalid address
invalidAddr := CorruptAddress(t, addrGen.Fixture(t), flow.Testnet)
```

#### Signer Indices Generator
```go
indicesGen := suite.SignerIndices()

// Generate indices with total validators and signer count
indices := indicesGen.Fixture(t, indicesGen.WithSignerCount(10, 4))

// Generate indices at specific positions
indices := indicesGen.Fixture(t, indicesGen.WithIndices([]int{0, 2, 4}))

// Generate list of indices
indicesList := indicesGen.List(t, 3, indicesGen.WithSignerCount(10, 2))
```

### Consensus Generators

#### Quorum Certificate Generator
```go
qcGen := suite.QuorumCertificates()

// Basic quorum certificate
qc := qcGen.Fixture(t)

// With specific view and block ID
qc := qcGen.Fixture(t,
    qcGen.WithView(100),
    qcGen.WithBlockID(blockID),
)

// With root block ID (sets view to 0)
qc := qcGen.Fixture(t,
    qcGen.WithRootBlockID(blockID),
)

// Certifies specific block
qc := qcGen.Fixture(t,
    qcGen.CertifiesBlock(header),
)

// With signer indices and source
qc := qcGen.Fixture(t,
    qcGen.WithSignerIndices(signerIndices),
    qcGen.WithRandomnessSource(source),
)

// List of certificates
qcList := qcGen.List(t, 3)
```

### Execution Data Generators

#### Chunk Execution Data Generator
```go
cedGen := suite.ChunkExecutionDatas()

// Basic chunk execution data
ced := cedGen.Fixture(t)

// With minimum size
ced := cedGen.Fixture(t,
    cedGen.WithMinSize(100),
)

// List of chunk execution data
cedList := cedGen.List(t, 3)
```

#### Block Execution Data Generator
```go
bedGen := suite.BlockExecutionDatas()

// Basic block execution data
bed := bedGen.Fixture(t)

// With specific block ID
bed := bedGen.Fixture(t,
    bedGen.WithBlockID(blockID),
)

// With specific chunk execution datas
chunks := suite.ChunkExecutionDatas().List(t, 3)
bed := bedGen.Fixture(t,
    bedGen.WithChunkExecutionDatas(chunks...),
)

// List of block execution data
bedList := bedGen.List(t, 2)
```

#### Block Execution Data Entity Generator
```go
bedEntityGen := suite.BlockExecutionDataEntities()

// Basic block execution data entity
entity := bedEntityGen.Fixture(t)

// With specific block ID
entity := bedEntityGen.Fixture(t,
    bedEntityGen.WithBlockID(blockID),
)

// List of entities
entityList := bedEntityGen.List(t, 2)
```

### Transaction Generators

#### Transaction Generator
```go
txGen := suite.Transactions()

// Basic transaction body
tx := txGen.Fixture(t)

// With custom gas limit
tx := txGen.Fixture(t,
    txGen.WithGasLimit(100),
)

// List of transactions
txList := txGen.List(t, 3)
```

#### Full Transaction Generator
```go
fullTxGen := suite.FullTransactions()

// Complete transaction (with TransactionBody embedded)
tx := fullTxGen.Fixture(t)

// List of complete transactions
txList := fullTxGen.List(t, 2)
```

#### Collection Generator
```go
colGen := suite.Collections()

// Collection with 1 transaction (default)
col := colGen.Fixture(t)

// Collection with specific transaction count
col := colGen.Fixture(t, colGen.WithTxCount(3))

// Collection with specific transactions
transactions := suite.Transactions().List(t, 2)
col := colGen.Fixture(t, colGen.WithTransactions(transactions))

// List of collections each with 3 transactions
colList := colGen.List(t, 2, colGen.WithTxCount(3))
```

### Ledger Generators

#### Trie Update Generator
```go
trieGen := suite.TrieUpdates()

// Basic trie update
trie := trieGen.Fixture(t)

// With specific number of paths
trie := trieGen.Fixture(t,
    trieGen.WithNumPaths(5),
)

// List of trie updates
trieList := trieGen.List(t, 2)
```

### Transaction Result Generators

#### Transaction Result Generator
```go
trGen := suite.TransactionResults()

// Basic transaction result
tr := trGen.Fixture(t)

// With custom error message
tr := trGen.Fixture(t,
    trGen.WithErrorMessage("custom error"),
)

// List of results
trList := trGen.List(t, 2)
```

#### Light Transaction Result Generator
```go
ltrGen := suite.LightTransactionResults()

// Basic light transaction result
ltr := ltrGen.Fixture(t)

// With failed status
ltr := ltrGen.Fixture(t,
    ltrGen.WithFailed(true),
)

// List of light results
ltrList := ltrGen.List(t, 2)
```

### Transaction Component Generators

#### Transaction Signature Generator
```go
tsGen := suite.TransactionSignatures()

// Basic transaction signature
ts := tsGen.Fixture(t)

// With custom signer index
ts := tsGen.Fixture(t,
    tsGen.WithSignerIndex(5),
)

// List of signatures
tsList := tsGen.List(t, 2)
```

#### Proposal Key Generator
```go
pkGen := suite.ProposalKeys()

// Basic proposal key
pk := pkGen.Fixture(t)

// With custom sequence number
pk := pkGen.Fixture(t,
    pkGen.WithSequenceNumber(100),
)

// List of proposal keys
pkList := pkGen.List(t, 2)
```

### Event Generators

#### Event Type Generator
```go
eventTypeGen := suite.EventTypes()

// Basic event type
eventType := eventTypeGen.Fixture(t)

// With custom event name
eventType := eventTypeGen.Fixture(t,
    eventTypeGen.WithEventName("CustomEvent"),
)

// With custom contract name
eventType := eventTypeGen.Fixture(t,
    eventTypeGen.WithContractName("CustomContract"),
)

// With specific address
address := suite.Addresses().Fixture(t)
eventType := eventTypeGen.Fixture(t,
    eventTypeGen.WithAddress(address),
)
```

#### Event Generator
```go
eventGen := suite.Events()

// Basic event with default CCF encoding
event := eventGen.Fixture(t)

// With specific encoding
eventWithCCF := eventGen.Fixture(t,
    eventGen.WithEncoding(entities.EventEncodingVersion_CCF_V0),
)
eventWithJSON := eventGen.Fixture(t,
    eventGen.WithEncoding(entities.EventEncodingVersion_JSON_CDC_V0),
)

// With custom type and encoding
event := eventGen.Fixture(t,
    eventGen.WithEventType("A.0x1.Test.Event"),
    eventGen.WithEncoding(entities.EventEncodingVersion_JSON_CDC_V0),
)

// Events for specific transaction
txID := suite.Identifiers().Fixture(t)
txEvents := eventGen.ForTransaction(t, txID, 0, 3)

// Events for multiple transactions
txIDs := suite.Identifiers().List(t, 2)
allEvents := eventGen.ForTransactions(t, txIDs, 2)
```

### Time Generator

Generates `time.Time` values with various options for time-based testing scenarios.

**Options:**
- `WithBaseTime(baseTime time.Time)`: Sets the base time for generation
- `WithOffset(offset time.Duration)`: Sets a specific offset from the base time
- `WithOffsetRandom(max time.Duration)`: Sets a random offset from the base time (0 to max)
- `WithTimezone(tz *time.Location)`: Sets the timezone for time generation

**Methods:**
- `Fixture(t testing.TB, opts ...func(*timeConfig))`: Generates a time.Time value with optional configuration

```go
timeGen := suite.Time()

// Basic time fixture
time1 := timeGen.Fixture(t)

// Time with specific base time
now := time.Now()
time2 := timeGen.Fixture(t, timeGen.WithBaseTime(now))

// Time with specific base time
baseTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
time3 := timeGen.Fixture(t, timeGen.WithBaseTime(baseTime))

// Time with specific offset
time4 := timeGen.Fixture(t, timeGen.WithOffset(time.Hour))

// Time with random offset
time5 := timeGen.Fixture(t, timeGen.WithOffsetRandom(24*time.Hour))

// Time in a specific timezone
time6 := timeGen.Fixture(t, timeGen.WithTimezone(time.FixedZone("EST", -5*3600)))

// Time with multiple options
time7 := timeGen.Fixture(t,
	timeGen.WithBaseTime(baseTime),
	timeGen.WithOffset(time.Hour),
	timeGen.WithTimezone(time.FixedZone("EST", -5*3600)),
)
```

### Ledger Path Generator

Generates ledger paths with consistent randomness and deduplication.

**Options:**
- `WithCount(count int)`: Sets the number of paths to generate

**Methods:**
- `Fixture(t testing.TB, opts ...func(*pathConfig))`: Generates a single ledger path
- `List(t testing.TB, n int, opts ...func(*pathConfig))`: Generates a list of ledger paths

```go
pathGen := suite.LedgerPaths()

// Basic path fixture
path1 := pathGen.Fixture(t)

// List of paths
paths := pathGen.List(t, 3)
```

### Ledger Payload Generator

Generates ledger payloads with consistent randomness and configurable sizes.

**Options:**
- `WithSize(minSize, maxSize int)`: Sets the payload size range
- `WithValue(value ledger.Value)`: Sets the value for the payload

**Methods:**
- `Fixture(t testing.TB, opts ...func(*payloadConfig))`: Generates a single ledger payload
- `List(t testing.TB, n int, opts ...func(*payloadConfig))`: Generates a list of ledger payloads

```go
payloadGen := suite.LedgerPayloads()

// Basic payload fixture
payload1 := payloadGen.Fixture(t)

// Payload with specific size
payload2 := payloadGen.Fixture(t, payloadGen.WithSize(4, 16))

// Payload with specific value
value := suite.LedgerValues().Fixture(t)
payload3 := payloadGen.Fixture(t, payloadGen.WithValue(value))

// List of payloads
payloads := payloadGen.List(t, 3)
```

### Ledger Value Generator

Generates ledger values with consistent randomness and configurable sizes.

**Options:**
- `WithSize(minSize, maxSize int)`: Sets the value size range [minSize, maxSize)

**Methods:**
- `Fixture(t testing.TB, opts ...func(*valueConfig))`: Generates a single ledger value
- `List(t testing.TB, n int, opts ...func(*valueConfig))`: Generates a list of ledger values

```go
valueGen := suite.LedgerValues()

// Basic value fixture
value1 := valueGen.Fixture(t)

// Value with specific size
value2 := valueGen.Fixture(t, valueGen.WithSize(4, 16))

// List of values
values := valueGen.List(t, 3)

// List of values with specific size
largeValues := valueGen.List(t, 2, valueGen.WithSize(8, 32))
```

## Migration Guide

### From Standalone Fixtures

The new fixtures module replaces standalone fixture functions. Here's how to migrate:

**Old Way:**
```go
import "github.com/onflow/flow-go/utils/unittest"

// Standalone functions
header := unittest.BlockHeaderFixture()
header := unittest.BlockHeaderFixtureOnChain(flow.Testnet)
header := unittest.BlockHeaderWithParentFixture(parent)
```

**New Way:**
```go
import "github.com/onflow/flow-go/utils/unittest/fixtures"

// Create suite once per test
suite := fixtures.NewGeneratorSuite(t) // or fixtures.WithSeed(12345)

// Use suite generators
headerGen := suite.BlockHeaders()
header := headerGen.Fixture(t)
header := headerGen.Fixture(t, headerGen.WithChainID(flow.Testnet))
header := headerGen.Fixture(t, headerGen.WithParent(parent))
```

### Benefits of Migration

1. **Deterministic Results**: Explicit seed control for reproducible tests
2. **Shared Randomness**: All generators use the same RNG for consistency
3. **Context Awareness**: Generators create related data that makes sense together
4. **Better Error Handling**: Proper error handling with `testing.TB`
5. **Easier Extension**: Simple to add new generator types

## Testing

The fixtures module includes comprehensive tests:

```bash
# Run all fixture tests
go test -v ./utils/unittest/fixtures

# Run specific test
go test -v ./utils/unittest/fixtures -run TestGeneratorSuite

# Run deterministic tests
go test -v ./utils/unittest/fixtures -run TestGeneratorSuiteDeterministic
```

### Test Coverage

- Deterministic results with the same seed
- Proper relationships between generated objects
- Correct field values and constraints
- Random seed generation when seed is not specified
- All generator options and methods

## Architecture

### File Organization

The fixtures module is organized into focused files:

| File | Purpose |
|------|---------|
| `generator.go` | Core `GeneratorSuite` and options |
| `random.go` | Random value generation utilities |
| `block_header.go` | Block header generation |
| `identifier.go` | Identifier generation |
| `signature.go` | Signature generation |
| `address.go` | Address generation |
| `signer_indices.go` | Signer indices generation |
| `quorum_certificate.go` | Quorum certificate generation |
| `chunk_execution_data.go` | Chunk execution data generation |
| `block_execution_data.go` | Block execution data generation |
| `transaction.go` | Transaction generation |
| `collection.go` | Collection generation |
| `trie_update.go` | Trie update generation |
| `transaction_result.go` | Transaction result generation |
| `transaction_signature.go` | Transaction signature generation |
| `proposal_key.go` | Proposal key generation |
| `event.go` | Event generation |
| `event_type.go` | Event type generation |
| `time.go` | Time generation |
| `ledger_path.go` | Ledger path generation |
| `ledger_payload.go` | Ledger payload generation |
| `ledger_value.go` | Ledger value generation |
| `examples.go` | Usage examples |
| `generators_test.go` | Comprehensive tests |

### Design Principles

1. **Single Responsibility**: Each generator focuses on one data type
2. **Dependency Injection**: Generators store instances of other generators they need
3. **Options Pattern**: Flexible configuration through options
4. **Error Safety**: All methods require `testing.TB` and handle errors properly
5. **Deterministic**: Same seed produces same results
6. **Extensible**: Easy to add new generators following established patterns

### Adding New Generators

To add a new generator:

1. Create a new file (e.g., `my_type.go`)
2. Define the generator struct with stored dependencies
3. Implement options pattern for configuration
4. Add getter method to `GeneratorSuite`
5. Add tests and examples
6. Update documentation

Example:
```go
// In my_type.go
type MyTypeGenerator struct {
    suite *GeneratorSuite
    // Store other generators as needed
}

func (g *MyTypeGenerator) Fixture(t testing.TB, opts ...func(*config)) *MyType {
    // Implementation
}

// In generator.go
func (g *GeneratorSuite) MyTypes() *MyTypeGenerator {
    return &MyTypeGenerator{suite: g}
}
``` 