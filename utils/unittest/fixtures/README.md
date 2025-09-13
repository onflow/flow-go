# Flow Go Fixtures Module

A context-aware test fixture generation system for Flow Go that provides deterministic, reproducible
test data with shared randomness across all generators. This module replaces the standalone fixture
functions with a comprehensive suite of generator objects.

## Table of Contents

1. [Overview](#overview)
2. [Reproducibility](#reproducibility)
3. [Concurrency](#concurrency)
4. [Module Structure](#module-structure)
5. [Quick Start](#quick-start)
6. [Core Concepts](#core-concepts)
7. [Generator Suite](#generator-suite)
8. [Available Generators](#available-generators)
9. [Migration Guide](#migration-guide)
10. [Testing](#testing)
11. [Architecture](#architecture)

## Overview

The fixtures module replaces standalone fixture functions with a suite of context-aware generator objects that:

- **Share randomness**: All generators use the same RNG for consistency
- **Support deterministic results**: Explicit seed control for reproducible tests
- **Provide context awareness**: Generators can create related data that makes sense together
- **Enable easy extension**: Simple to add new generator types
- **Improve test reproducibility**: Consistent results across test runs

## Reproducibility

This suite is designed to allow producing complete data types with reproducible random data. This is
critical for certain test scenarios like testing hash functions or data serializers. In most cases,
deterministic data is not strictly require.

However, it is very useful to be able to replay a failed test that used random data. Imagine the 
scenario where a test failed in CI due to some corner case bug with data handling. Since the data
was randomly generated, it's extrememly difficult to reverse engineer the inputs that caused the failure.
With deterministic test fixtures, we could grab the random seed used by the test from the logs,
then rerun the test locally with the exact same test data.

This does require that some extra care is taken while designing the tests, especially any tests that
use multiple goroutines, or any tests that run subtests with `Parallel()`. See [Concurrency](#concurrancy).

## Concurrency

The generator suite does support concurrent usage, but it is discouraged to ensure that tests remain
reproducible. Any time that the main suite or any generator are used within different goroutines or
parallel subtests, the specific order that fixtures are produced with vary depending on the go
scheduler. The order a fixture is generated determines the random data provided by the PRNG, thus if
fixtures are produced concurrently, their output will not be deterministic!

### Best Pracice

To support using concurrency within your tests AND get deterministic random fixtures, you need to
follow these best practices:

1. Always use a single `GeneratorSuite` per test. This ensures you can replay the test individually,
and allows for tests to be run in parallel without losing support for deterministic fixtures.
2. Always generate all fixture data before executing any concurrent logic.
3. **Never** generate any fixtures outside of the test's main goroutine.

It is fine to share a `GeneratorSuite` between subtests so long as they are **not** marked `Parallel()`.
It is also fine to use a `GeneratorSuite` within a test suite since suite tests do not support parallelism.

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
suite := fixtures.NewGeneratorSuite(fixtures.WithSeed(12345))
```

### Basic Usage Examples

```go
// Generate a block header
header := suite.BlockHeaders().Fixture()

// Generate an identifier
id := suite.Identifiers().Fixture()

// Generate a collection with 3 transactions
collection := suite.Collections().Fixture(3)

// Generate events for a transaction
txID := suite.Identifiers().Fixture()
events := suite.Events().ForTransaction(txID, 0, 3)
```

## Core Concepts

### Generator Suite
The `GeneratorSuite` is the central object that:
- Manages a shared random number generator
- Provides access to specialized generators
- Ensures consistent randomness across all generators

### Options Pattern
Most generators use an options pattern for configuration. This is an example using the default
`Header` options factory:
```go
// Example: Configure a block header
header := suite.BlockHeaders().Fixture(
    Header.WithHeight(100),
    Header.WithView(200),
)
```

Alternatively, you can use the generator itself as the factory:
```go
// Example: Configure a block header
headerGen := suite.BlockHeaders()
header := headerGen.Fixture(
    headerGen.WithHeight(100),
    headerGen.WithView(200),
)
```

Using typed options allows us to namepace the options by type, avoiding name conflicts for similar
options.

### Error Handling
All generators that can produce an error require a `testing.TB` parameter and use `require.NoError` internally to assert no errors are encountered:
```go
// No error handling needed - failures will fail the test
result := suite.TransactionResults().Fixture()
```

## Generator Suite

### Constructor Options

```go
// No options: Use random seed
suite := fixtures.NewGeneratorSuite(t)

// WithSeed(seed int64): Set specific seed for deterministic results
suite := fixtures.NewGeneratorSuite(fixtures.WithSeed(42))
```

### Core Methods

```go
// Access the random generator
randomGen := suite.Random()

// Generate random bytes
bytes := randomGen.RandomBytes(32)

// Generate random string
str := randomGen.RandomString(16)
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
| Time | `Time()` | Generate [time.Time] values |
| Ledger Paths | `LedgerPaths()` | Generate ledger paths |
| Ledger Payloads | `LedgerPayloads()` | Generate ledger payloads |
| Time | `Time()` | Generate [time.Time] values |

## Available Generators

### Random Generator

The RandomGenerator provides consistent random value generation for all other generators. It exposes all methods from `*rand.Rand` and provides additional convenience methods for common use cases.

**Core Methods:**
- `RandomBytes(n int)`: Generates n random bytes
- `RandomString(length uint)`: Generates a random string of specified length
- `Uint32()`: Generates a random uint32
- `Uint64()`: Generates a random uint64
- `Int31()`: Generates a random int32
- `Int63()`: Generates a random int64
- `Intn(n int)`: Generates a random int in the range [0, n)

**Unsigned Integer Methods (n > 0 required):**
- `Uintn(n uint)`: Generates a random uint strictly less than n
- `Uint32n(n uint32)`: Generates a random uint32 strictly less than n
- `Uint64n(n uint64)`: Generates a random uint64 strictly less than n

**Range Methods (positive ranges only):**
- `IntInRange(min, max int)`: Generates a random int in the inclusive range [min, max]
- `Int32InRange(min, max int32)`: Generates a random int32 in the inclusive range [min, max]
- `Int64InRange(min, max int64)`: Generates a random int64 in the inclusive range [min, max]
- `UintInRange(min, max uint)`: Generates a random uint in the inclusive range [min, max]
- `Uint32InRange(min, max uint32)`: Generates a random uint32 in the inclusive range [min, max]
- `Uint64InRange(min, max uint64)`: Generates a random uint64 in the inclusive range [min, max]

**Generic Functions:**
- `InclusiveRange[T](g *RandomGenerator, min, max T)`: Generic function for generating random numbers in inclusive ranges
- `RandomElement[T](g *RandomGenerator, slice []T)`: Selects a random element from a slice

**Constraints:**
- All `*n` methods require n > 0 (will panic if n = 0)
- All range methods only support positive ranges (will panic with negative ranges)
- The RandomGenerator exposes all methods from `*rand.Rand` for additional functionality

```go
randomGen := suite.Random()

// Generate random bytes
bytes := randomGen.RandomBytes(32)

// Generate random string
str := randomGen.RandomString(16)

// Generate random unsigned integers less than n
uintVal := randomGen.Uintn(100)
uint32Val := randomGen.Uint32n(100)
uint64Val := randomGen.Uint64n(100)

// Generate random numbers in inclusive ranges (positive ranges only)
intInRange := randomGen.IntInRange(1, 50)
int32InRange := randomGen.Int32InRange(1, 25)
int64InRange := randomGen.Int64InRange(1, 100)
uintInRange := randomGen.UintInRange(10, 90)
uint32InRange := randomGen.Uint32InRange(5, 95)
uint64InRange := randomGen.Uint64InRange(1, 1000)

// Use generic InclusiveRange function
numInRange := InclusiveRange(randomGen, 1, 100)
numInRangeWithType := InclusiveRange[uint32](randomGen, 1, 100)

// Select random element from slice
slice := []string{"apple", "banana", "cherry", "date"}
randomElement := RandomElement(randomGen, slice)

// randomGen also exposes all methods from *rand.Rand
val := randomGen.Uint32()
val64 := randomGen.Uint64()
int32Val := randomGen.Int31()
int64Val := randomGen.Int63()
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
header := headerGen.Fixture()

// Header with specific height
header := headerGen.Fixture(
    headerGen.WithHeight(100),
)

// Header with specific view
header := headerGen.Fixture(
    headerGen.WithView(200),
)

// Header for specific chain
header := headerGen.Fixture(
    headerGen.WithChainID(flow.Testnet),
)

// Header with parent
parent := headerGen.Fixture()
child := headerGen.Fixture(
    headerGen.WithParent(parent),
)

// Header with parent and source of randomness
parent := headerGen.Fixture()
source := suite.Random().RandomBytes(32)
child := headerGen.Fixture(
    headerGen.WithParentAndSoR(parent, source),
)
```

### Primitive Generators

#### Identifier Generator
```go
idGen := suite.Identifiers()

// Single identifier
id := idGen.Fixture()

// List of identifiers
ids := idGen.List(5)
```

#### Signature Generator
```go
sigGen := suite.Signatures()

// Single signature
sig := sigGen.Fixture()

// List of signatures
sigs := sigGen.List(3)
```

#### Address Generator
```go
addrGen := suite.Addresses()

// Default random address on default chain (Testnet)
addr := addrGen.Fixture()

// Address for specific chain
addr := addrGen.Fixture(addrGen.WithChainID(flow.Testnet))

// Service account address
addr := addrGen.Fixture(addrGen.ServiceAddress())

// Invalid address
invalidAddr := CorruptAddress(addrGen.Fixture(), flow.Testnet)
```

#### Signer Indices Generator
```go
indicesGen := suite.SignerIndices()

// Generate indices with total validators and signer count
indices := indicesGen.Fixture(indicesGen.WithSignerCount(10, 4))

// Generate indices at specific positions
indices := indicesGen.Fixture(indicesGen.WithIndices([]int{0, 2, 4}))

// Generate list of indices
indicesList := indicesGen.List(3, indicesGen.WithSignerCount(10, 2))
```

### Consensus Generators

#### Quorum Certificate Generator
```go
qcGen := suite.QuorumCertificates()

// Basic quorum certificate
qc := qcGen.Fixture()

// With specific view and block ID
qc := qcGen.Fixture(
    qcGen.WithView(100),
    qcGen.WithBlockID(blockID),
)

// With root block ID (sets view to 0)
qc := qcGen.Fixture(
    qcGen.WithRootBlockID(blockID),
)

// Certifies specific block
qc := qcGen.Fixture(
    qcGen.CertifiesBlock(header),
)

// With signer indices and source
qc := qcGen.Fixture(
    qcGen.WithSignerIndices(signerIndices),
    qcGen.WithRandomnessSource(source),
)

// List of certificates
qcList := qcGen.List(3)
```

### Execution Data Generators

#### Chunk Execution Data Generator
```go
cedGen := suite.ChunkExecutionDatas()

// Basic chunk execution data
ced := cedGen.Fixture()

// With minimum size
ced := cedGen.Fixture(
    cedGen.WithMinSize(100),
)

// List of chunk execution data
cedList := cedGen.List(3)
```

#### Block Execution Data Generator
```go
bedGen := suite.BlockExecutionDatas()

// Basic block execution data
bed := bedGen.Fixture()

// With specific block ID
bed := bedGen.Fixture(
    bedGen.WithBlockID(blockID),
)

// With specific chunk execution datas
chunks := suite.ChunkExecutionDatas().List(3)
bed := bedGen.Fixture(
    bedGen.WithChunkExecutionDatas(chunks...),
)

// List of block execution data
bedList := bedGen.List(2)
```

#### Block Execution Data Entity Generator
```go
bedEntityGen := suite.BlockExecutionDataEntities()

// Basic block execution data entity
entity := bedEntityGen.Fixture()

// With specific block ID
entity := bedEntityGen.Fixture(
    bedEntityGen.WithBlockID(blockID),
)

// List of entities
entityList := bedEntityGen.List(2)
```

### Transaction Generators

#### Transaction Generator
```go
txGen := suite.Transactions()

// Basic transaction body
tx := txGen.Fixture()

// With custom gas limit
tx := txGen.Fixture(
    txGen.WithGasLimit(100),
)

// List of transactions
txList := txGen.List(3)
```

#### Full Transaction Generator
```go
fullTxGen := suite.FullTransactions()

// Complete transaction (with TransactionBody embedded)
tx := fullTxGen.Fixture()

// List of complete transactions
txList := fullTxGen.List(2)
```

#### Collection Generator
```go
colGen := suite.Collections()

// Collection with 1 transaction (default)
col := colGen.Fixture()

// Collection with specific transaction count
col := colGen.Fixture(colGen.WithTxCount(3))

// Collection with specific transactions
transactions := suite.Transactions().List(2)
col := colGen.Fixture(colGen.WithTransactions(transactions))

// List of collections each with 3 transactions
colList := colGen.List(2, colGen.WithTxCount(3))
```

### Ledger Generators

#### Trie Update Generator
```go
trieGen := suite.TrieUpdates()

// Basic trie update
trie := trieGen.Fixture()

// With specific number of paths
trie := trieGen.Fixture(
    trieGen.WithNumPaths(5),
)

// List of trie updates
trieList := trieGen.List(2)
```

### Transaction Result Generators

#### Transaction Result Generator
```go
trGen := suite.TransactionResults()

// Basic transaction result
tr := trGen.Fixture()

// With custom error message
tr := trGen.Fixture(
    trGen.WithErrorMessage("custom error"),
)

// List of results
trList := trGen.List(2)
```

#### Light Transaction Result Generator
```go
ltrGen := suite.LightTransactionResults()

// Basic light transaction result
ltr := ltrGen.Fixture()

// With failed status
ltr := ltrGen.Fixture(
    ltrGen.WithFailed(true),
)

// List of light results
ltrList := ltrGen.List(2)
```

### Transaction Component Generators

#### Transaction Signature Generator
```go
tsGen := suite.TransactionSignatures()

// Basic transaction signature
ts := tsGen.Fixture()

// With custom signer index
ts := tsGen.Fixture(
    tsGen.WithSignerIndex(5),
)

// List of signatures
tsList := tsGen.List(2)
```

#### Proposal Key Generator
```go
pkGen := suite.ProposalKeys()

// Basic proposal key
pk := pkGen.Fixture()

// With custom sequence number
pk := pkGen.Fixture(
    pkGen.WithSequenceNumber(100),
)

// List of proposal keys
pkList := pkGen.List(2)
```

### Event Generators

#### Event Type Generator
```go
eventTypeGen := suite.EventTypes()

// Basic event type
eventType := eventTypeGen.Fixture()

// With custom event name
eventType := eventTypeGen.Fixture(
    eventTypeGen.WithEventName("CustomEvent"),
)

// With custom contract name
eventType := eventTypeGen.Fixture(
    eventTypeGen.WithContractName("CustomContract"),
)

// With specific address
address := suite.Addresses().Fixture()
eventType := eventTypeGen.Fixture(
    eventTypeGen.WithAddress(address),
)
```

#### Event Generator
```go
eventGen := suite.Events()

// Basic event with default CCF encoding
event := eventGen.Fixture()

// With specific encoding
eventWithCCF := eventGen.Fixture(
    eventGen.WithEncoding(entities.EventEncodingVersion_CCF_V0),
)
eventWithJSON := eventGen.Fixture(
    eventGen.WithEncoding(entities.EventEncodingVersion_JSON_CDC_V0),
)

// With custom type and encoding
event := eventGen.Fixture(
    eventGen.WithEventType("A.0x1.Test.Event"),
    eventGen.WithEncoding(entities.EventEncodingVersion_JSON_CDC_V0),
)

// Events for specific transaction
txID := suite.Identifiers().Fixture()
txEvents := eventGen.ForTransaction(txID, 0, 3)

// Events for multiple transactions
txIDs := suite.Identifiers().List(2)
allEvents := eventGen.ForTransactions(txIDs, 2)
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
time1 := timeGen.Fixture()

// Time with specific base time
now := time.Now()
time2 := timeGen.Fixture(timeGen.WithBaseTime(now))

// Time with specific base time
baseTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
time3 := timeGen.Fixture(timeGen.WithBaseTime(baseTime))

// Time with specific offset
time4 := timeGen.Fixture(timeGen.WithOffset(time.Hour))

// Time with random offset
time5 := timeGen.Fixture(timeGen.WithOffsetRandom(24*time.Hour))

// Time in a specific timezone
time6 := timeGen.Fixture(timeGen.WithTimezone(time.FixedZone("EST", -5*3600)))

// Time with multiple options
time7 := timeGen.Fixture(
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
path1 := pathGen.Fixture()

// List of paths
paths := pathGen.List(3)
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
payload1 := payloadGen.Fixture()

// Payload with specific size
payload2 := payloadGen.Fixture(payloadGen.WithSize(4, 16))

// Payload with specific value
value := suite.LedgerValues().Fixture()
payload3 := payloadGen.Fixture(payloadGen.WithValue(value))

// List of payloads
payloads := payloadGen.List(3)
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
value1 := valueGen.Fixture()

// Value with specific size
value2 := valueGen.Fixture(valueGen.WithSize(4, 16))

// List of values
values := valueGen.List(3)

// List of values with specific size
largeValues := valueGen.List(2, valueGen.WithSize(8, 32))
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
header := headerGen.Fixture()
header := headerGen.Fixture(headerGen.WithChainID(flow.Testnet))
header := headerGen.Fixture(headerGen.WithParent(parent))
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