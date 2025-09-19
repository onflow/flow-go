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

- **Support deterministic results**: All generators use the same RNG, which allows for reproducible deterministic data using a static seed.
- **Complete Objects**: All generators produce complete and realistic model objects.
- **Provide context awareness**: Generators can create related data that makes sense together
- **Enable easy extension**: Simple and consistent API makes exending or adding new generators straight forward.
- **Improve test reproducibility**: By reusing the same seed from a failed test, you can reproduce exactly the same inputs.

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

### Best Practice

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
├── README.md                                    # This documentation
├── generators_test.go                           # Test suite
├── generator.go                                 # Core GeneratorSuite and options
├── util.go                                      # Utility functions
├── random.go                                    # RandomGenerator implementation
├── *Core Data Types*
├── identifier.go                                # Identifier generator
├── signature.go                                 # Signature generator
├── address.go                                   # Address generator
├── time.go                                      # Time generator
├── *Block Components*
├── block_header.go                              # Block header generator
├── block.go                                     # Block generator
├── payload.go                                   # Block payload generator
├── seal.go                                      # Block seal generator
├── *Consensus Components*
├── quorum_certificate.go                        # Quorum certificate generator
├── timeout_certificate.go                       # Timeout certificate generator
├── signer_indices.go                            # Signer indices generator
├── *Execution Components*
├── execution_result.go                          # Execution result generator
├── execution_receipt.go                         # Execution receipt generator
├── chunk.go                                     # Execution chunk generator
├── chunk_execution_data.go                      # Chunk execution data generator
├── block_execution_data.go                      # Block execution data generator
├── *Transaction Components*
├── transaction.go                               # Transaction generator
├── collection.go                                # Collection generator
├── collection_guarantee.go                      # Collection guarantee generator
├── transaction_result.go                        # Transaction result generator
├── transaction_signature.go                     # Transaction signature generator
├── proposal_key.go                              # Proposal key generator
├── *Events*
├── event.go                                     # Event generator
├── event_type.go                                # Event type generator
├── service_event.go                             # Service event generator
├── service_event_epoch_setup.go                 # Epoch setup service event generator
├── service_event_epoch_commit.go                # Epoch commit service event generator
├── service_event_epoch_recover.go               # Epoch recover service event generator
├── service_event_version_beacon.go              # Version beacon service event generator
├── service_event_protocol_state_version_upgrade.go # Protocol state version upgrade generator
├── service_event_set_epoch_extension_view_count.go # Set epoch extension view count generator
├── service_event_eject_node.go                  # Eject node service event generator
├── *Ledger Components*
├── trie_update.go                               # Trie update generator
├── ledger_path.go                               # Ledger path generator
├── ledger_payload.go                            # Ledger payload generator
├── ledger_value.go                              # Ledger value generator
├── state_commitment.go                          # State commitment generator
├── *Cryptographic Components*
├── crypto.go                                    # Cryptographic generator
├── aggregated_signature.go                      # Aggregated signature generator
└── *Identity Components*
└── identity.go                                  # Identity generator
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
header := suite.Headers().Fixture()

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
header := suite.Headers().Fixture(
    Header.WithHeight(100),
    Header.WithView(200),
)
```

Alternatively, you can use the generator itself as the factory:
```go
// Example: Configure a block header
headerGen := suite.Headers()
header := headerGen.Fixture(
    headerGen.WithHeight(100),
    headerGen.WithView(200),
)
```

Using typed options allows us to namepace the options by type, avoiding name conflicts for similar
options.

### Error Handling
Any generators that can produce an error, check it using `fixtures.NoError(err)` which panics if an
error is encountered.
```go
// No error handling needed - failures will panic
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
random := suite.Random()

// Generate random bytes
bytes := random.RandomBytes(32)

// Generate random string
str := random.RandomString(16)
```

### Available Generators

| Generator | Method | Purpose |
|-----------|--------|---------|
| **Core Data Types** | | |
| Block Headers | `Headers()` | Generate block headers with proper relationships |
| Blocks | `Blocks()` | Generate complete blocks with headers and payloads |
| Identifiers | `Identifiers()` | Generate flow identifiers |
| Signatures | `Signatures()` | Generate cryptographic signatures |
| Addresses | `Addresses()` | Generate flow addresses |
| Time | `Time()` | Generate [time.Time] values |
| **Consensus Components** | | |
| Quorum Certificates | `QuorumCertificates()` | Generate quorum certificates |
| Quorum Certificates with Signer IDs | `QuorumCertificatesWithSignerIDs()` | Generate quorum certificates with signer IDs |
| Timeout Certificates | `TimeoutCertificates()` | Generate timeout certificates |
| Signer Indices | `SignerIndices()` | Generate signer index arrays |
| **Execution Components** | | |
| Execution Results | `ExecutionResults()` | Generate execution results |
| Execution Receipts | `ExecutionReceipts()` | Generate execution receipts |
| Chunks | `Chunks()` | Generate execution chunks |
| Chunk Execution Data | `ChunkExecutionDatas()` | Generate chunk execution data |
| Block Execution Data | `BlockExecutionDatas()` | Generate block execution data |
| Block Execution Data Entities | `BlockExecutionDataEntities()` | Generate block execution data entities |
| **Transaction Components** | | |
| Transactions | `Transactions()` | Generate transaction bodies |
| Collections | `Collections()` | Generate collections of transactions |
| Collection Guarantees | `CollectionGuarantees()` | Generate collection guarantees |
| Transaction Results | `TransactionResults()` | Generate transaction results |
| Light Transaction Results | `LightTransactionResults()` | Generate light transaction results |
| Transaction Signatures | `TransactionSignatures()` | Generate transaction signatures |
| Proposal Keys | `ProposalKeys()` | Generate proposal keys |
| **Block Components** | | |
| Payloads | `Payloads()` | Generate block payloads |
| Seals | `Seals()` | Generate block seals |
| **Events** | | |
| Events | `Events()` | Generate events with encoding support |
| Event Types | `EventTypes()` | Generate event types |
| Service Events | `ServiceEvents()` | Generate service events |
| Epoch Setups | `EpochSetups()` | Generate epoch setup service events |
| Epoch Commits | `EpochCommits()` | Generate epoch commit service events |
| Epoch Recovers | `EpochRecovers()` | Generate epoch recover service events |
| Version Beacons | `VersionBeacons()` | Generate version beacon service events |
| Protocol State Version Upgrades | `ProtocolStateVersionUpgrades()` | Generate protocol state version upgrade service events |
| Set Epoch Extension View Counts | `SetEpochExtensionViewCounts()` | Generate set epoch extension view count service events |
| Eject Nodes | `EjectNodes()` | Generate eject node service events |
| **Ledger Components** | | |
| Trie Updates | `TrieUpdates()` | Generate ledger trie updates |
| Ledger Paths | `LedgerPaths()` | Generate ledger paths |
| Ledger Payloads | `LedgerPayloads()` | Generate ledger payloads |
| Ledger Values | `LedgerValues()` | Generate ledger values |
| State Commitments | `StateCommitments()` | Generate state commitments |
| **Cryptographic Components** | | |
| Crypto | `Crypto()` | Generate cryptographic keys and signatures |
| Aggregated Signatures | `AggregatedSignatures()` | Generate aggregated signatures |
| **Identity Components** | | |
| Identities | `Identities()` | Generate flow identities with cryptographic keys |

## Generator Documentation

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
random := suite.Random()

// Generate random bytes
bytes := random.RandomBytes(32)

// Generate random string
str := random.RandomString(16)

// Generate random unsigned integers less than n
uintVal := random.Uintn(100)
uint32Val := random.Uint32n(100)
uint64Val := random.Uint64n(100)

// Generate random numbers in inclusive ranges (positive ranges only)
intInRange := random.IntInRange(1, 50)
int32InRange := random.Int32InRange(1, 25)
int64InRange := random.Int64InRange(1, 100)
uintInRange := random.UintInRange(10, 90)
uint32InRange := random.Uint32InRange(5, 95)
uint64InRange := random.Uint64InRange(1, 1000)

// Use generic InclusiveRange function
numInRange := InclusiveRange(random, 1, 100)
numInRangeWithType := InclusiveRange[uint32](random, 1, 100)

// Select random element from slice
slice := []string{"apple", "banana", "cherry", "date"}
randomElement := RandomElement(random, slice)

// random also exposes all methods from *rand.Rand
val := random.Uint32()
val64 := random.Uint64()
int32Val := random.Int31()
int64Val := random.Int63()
intVal := random.Intn(100)
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
headerGen := suite.Headers()

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

### Block Generator

Generates complete blocks with headers and payloads using an options pattern for configuration.

**Options:**
- `WithHeight(height uint64)`: Sets the height of the block header
- `WithView(view uint64)`: Sets the view of the block header
- `WithChainID(chainID flow.ChainID)`: Sets the chain ID of the block header
- `WithParent(parentID, parentView, parentHeight)`: Sets parent block information
- `WithParentHeader(parent *flow.Header)`: Sets parent header and derives child values
- `WithProposerID(proposerID flow.Identifier)`: Sets the block proposer ID
- `WithLastViewTC(lastViewTC *flow.TimeoutCertificate)`: Sets the timeout certificate
- `WithTimestamp(timestamp uint64)`: Sets the block timestamp
- `WithPayload(payload flow.Payload)`: Sets the block payload
- `WithHeaderBody(headerBody flow.HeaderBody)`: Sets the header body

**Methods:**
- `Fixture(opts ...BlockOption)`: Generates a block with optional configuration
- `List(n int, opts ...BlockOption)`: Generates a chain of n blocks
- `Genesis(opts ...BlockOption)`: Generates a genesis block

```go
blockGen := suite.Blocks()

// Basic block
block := blockGen.Fixture()

// Block with specific height and view
block := blockGen.Fixture(
    Block.WithHeight(100),
    Block.WithView(200),
)

// Block chain with parent-child relationships
blocks := blockGen.List(5) // Creates chain of 5 blocks

// Genesis block
genesis := blockGen.Genesis(Block.WithChainID(flow.Testnet))
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

#### Execution Result Generator
```go
erGen := suite.ExecutionResults()

// Basic execution result
result := erGen.Fixture()

// With specific block ID
result := erGen.Fixture(
    erGen.WithBlockID(blockID),
)

// With specific chunks
chunks := suite.Chunks().List(3)
result := erGen.Fixture(
    erGen.WithChunks(chunks...),
)

// List of execution results
results := erGen.List(2)
```

#### Execution Receipt Generator
```go
receiptGen := suite.ExecutionReceipts()

// Basic execution receipt
receipt := receiptGen.Fixture()

// With specific executor ID
receipt := receiptGen.Fixture(
    receiptGen.WithExecutorID(executorID),
)

// With specific execution result
result := suite.ExecutionResults().Fixture()
receipt := receiptGen.Fixture(
    receiptGen.WithExecutionResult(result),
)

// List of execution receipts
receipts := receiptGen.List(3)
```

#### Chunk Generator
```go
chunkGen := suite.Chunks()

// Basic chunk
chunk := chunkGen.Fixture()

// With specific collection index
chunk := chunkGen.Fixture(
    chunkGen.WithIndex(2),
)

// List of chunks
chunks := chunkGen.List(4)
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

#### Service Event Generators

Service events represent important protocol-level events in Flow. The suite provides generators for all types of service events.

**Service Event Generator**
```go
serviceEventGen := suite.ServiceEvents()

// Basic service event (random type)
serviceEvent := serviceEventGen.Fixture()

// List of service events
serviceEvents := serviceEventGen.List(3)
```

**Specific Service Event Types**
```go
// Epoch Setup events
epochSetup := suite.EpochSetups().Fixture()

// Epoch Commit events
epochCommit := suite.EpochCommits().Fixture()

// Epoch Recover events
epochRecover := suite.EpochRecovers().Fixture()

// Version Beacon events
versionBeacon := suite.VersionBeacons().Fixture()

// Protocol State Version Upgrade events
protocolUpgrade := suite.ProtocolStateVersionUpgrades().Fixture()

// Set Epoch Extension View Count events
setExtension := suite.SetEpochExtensionViewCounts().Fixture()

// Eject Node events
ejectNode := suite.EjectNodes().Fixture()
```

### Identity Generator

Generates Flow identities with cryptographic keys and metadata.

```go
identityGen := suite.Identities()

// Basic identity
identity := identityGen.Fixture()

// Identity with specific role
identity := identityGen.Fixture(identityGen.WithRole(flow.RoleConsensus))

// List of identities
identities := identityGen.List(4)
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
headerGen := suite.Headers()
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

| Category | File | Purpose |
|----------|------|---------|
| **Core** | `generator.go` | Core `GeneratorSuite` and options |
| | `random.go` | Random value generation utilities |
| | `util.go` | Utility functions |
| | `generators_test.go` | Comprehensive tests |
| **Core Data Types** | `identifier.go` | Identifier generation |
| | `signature.go` | Signature generation |
| | `address.go` | Address generation |
| | `time.go` | Time generation |
| **Block Components** | `block_header.go` | Block header generation |
| | `block.go` | Complete block generation |
| | `payload.go` | Block payload generation |
| | `seal.go` | Block seal generation |
| **Consensus** | `quorum_certificate.go` | Quorum certificate generation |
| | `timeout_certificate.go` | Timeout certificate generation |
| | `signer_indices.go` | Signer indices generation |
| **Execution** | `execution_result.go` | Execution result generation |
| | `execution_receipt.go` | Execution receipt generation |
| | `chunk.go` | Execution chunk generation |
| | `chunk_execution_data.go` | Chunk execution data generation |
| | `block_execution_data.go` | Block execution data generation |
| **Transactions** | `transaction.go` | Transaction generation |
| | `collection.go` | Collection generation |
| | `collection_guarantee.go` | Collection guarantee generation |
| | `transaction_result.go` | Transaction result generation |
| | `transaction_signature.go` | Transaction signature generation |
| | `proposal_key.go` | Proposal key generation |
| **Events** | `event.go` | Event generation |
| | `event_type.go` | Event type generation |
| | `service_event.go` | Service event generation |
| | `service_event_*.go` | Specific service event type generators |
| **Ledger** | `trie_update.go` | Trie update generation |
| | `ledger_path.go` | Ledger path generation |
| | `ledger_payload.go` | Ledger payload generation |
| | `ledger_value.go` | Ledger value generation |
| | `state_commitment.go` | State commitment generation |
| **Cryptographic** | `crypto.go` | Cryptographic key generation |
| | `aggregated_signature.go` | Aggregated signature generation |
| **Identity** | `identity.go` | Identity generation |

### Design Principles

1. **Single Responsibility**: Each generator focuses on one data type
2. **Dependency Injection**: Generators store instances of other generators they need
3. **Options Pattern**: Flexible configuration through options
4. **Error Safety**: All errors returned during fixture construction must be checked using `NoError()`. Important constraints on the inputs must also be checked using `Assert()`
5. **Deterministic**: All generators use the same `rand.Rand`, ensuring consistent and deterministic value when the seed it set.
6. **Consistent**: Easy to add new generators following established patterns.

### Adding New Generators

To add a new generator:

1. **Create a new file** (e.g., `my_type.go`)
2. **Define the options factory and options**
3. **Define the generator struct with dependencies**
4. **Implement the Fixture and List methods**
5. **Add getter method to GeneratorSuite**
6. **Add tests and update documentation**

Example:
```go
// In my_type.go
package fixtures

import "github.com/onflow/flow-go/model/flow"

// MyType is the default options factory for [flow.MyType] generation.
var MyType myTypeFactory

type myTypeFactory struct{}

type MyTypeOption func(*MyTypeGenerator, *flow.MyType)

// WithField is an option that sets the `Field` of the my type.
func (f myTypeFactory) WithField(field string) MyTypeOption {
	return func(g *MyTypeGenerator, myType *flow.MyType) {
		myType.Field = field
	}
}

// MyTypeGenerator generates my types with consistent randomness.
type MyTypeGenerator struct {
	myTypeFactory

	random      *RandomGenerator
	identifiers *IdentifierGenerator
	// Add other generator dependencies as needed
}

func NewMyTypeGenerator(
	random *RandomGenerator,
	identifiers *IdentifierGenerator,
) *MyTypeGenerator {
	return &MyTypeGenerator{
		random:      random,
		identifiers: identifiers,
	}
}

// Fixture generates a [flow.MyType] with random data based on the provided options.
func (g *MyTypeGenerator) Fixture(opts ...MyTypeOption) *flow.MyType {
	myType := &flow.MyType{
		ID:    g.identifiers.Fixture(),
		Field: g.random.RandomString(10),
		// Set other fields with random values
	}

	for _, opt := range opts {
		opt(g, myType)
	}

	return myType
}

// List generates a list of [flow.MyType] with random data.
func (g *MyTypeGenerator) List(n int, opts ...MyTypeOption) []*flow.MyType {
	items := make([]*flow.MyType, n)
	for i := range n {
		items[i] = g.Fixture(opts...)
	}
	return items
}
```

```go
// In generator.go
func (g *GeneratorSuite) MyTypes() *MyTypeGenerator {
	return NewMyTypeGenerator(
		g.Random(),
		g.Identifiers(),
		// Pass other dependencies as needed
	)
}
```

**Key Implementation Patterns:**

1. **Factory Pattern**: Each generator has a global factory variable (e.g., `MyType`) that provides typed options
2. **Options Pattern**: Each option is a function that takes the generator and modifies the object being built
3. **Constructor Injection**: Generators receive their dependencies through `NewXxxGenerator()` constructors
4. **Consistent API**: All generators implement `Fixture(opts...)` and `List(n int, opts...)` methods
5. **Error Handling**: Use `fixtures.NoError(err)` for any operations that might fail
6. **Embedding**: Generator structs embed their factory for direct access to options methods 