package fixtures

import (
	"math/rand"
	"time"

	"github.com/onflow/flow-go/model/flow"
)

// GeneratorSuite provides a context-aware generator system for creating test fixtures.
// It manages a shared random number generator and provides access to specialized generators.
type GeneratorSuite struct {
	rng *rand.Rand

	chainID flow.ChainID
}

// GeneratorSuiteOption defines an option for configuring a GeneratorSuite.
type GeneratorSuiteOption func(*generatorSuiteConfig)

type generatorSuiteConfig struct {
	seed    int64
	chainID flow.ChainID
}

// WithSeed sets the random seed used for the random number generator used by all generators.
// Specifying a seed makes the fixture data deterministic.
func WithSeed(seed int64) GeneratorSuiteOption {
	return func(config *generatorSuiteConfig) {
		config.seed = seed
	}
}

// WithChainID sets the chain ID that's used as the default for all generators.
func WithChainID(chainID flow.ChainID) GeneratorSuiteOption {
	return func(config *generatorSuiteConfig) {
		config.chainID = chainID
	}
}

// NewGeneratorSuite creates a new generator suite with optional configuration.
// If no seed is specified, a random seed is generated.
// If no chain ID is specified, the default chain ID is [flow.Emulator].
func NewGeneratorSuite(opts ...GeneratorSuiteOption) *GeneratorSuite {
	config := &generatorSuiteConfig{
		chainID: flow.Emulator,
		seed:    time.Now().UnixNano(), // default random seed
	}

	for _, opt := range opts {
		opt(config)
	}

	return &GeneratorSuite{
		chainID: config.chainID,
		rng:     rand.New(rand.NewSource(config.seed)),
	}
}

// ChainID returns the default chain ID used by all generators.
func (g *GeneratorSuite) ChainID() flow.ChainID {
	return g.chainID
}

// Random returns the shared random generator.
func (g *GeneratorSuite) Random() *RandomGenerator {
	return NewRandomGenerator(g.rng)
}

// BlockHeaders returns a generator for [flow.Header].
func (g *GeneratorSuite) BlockHeaders() *BlockHeaderGenerator {
	return NewBlockHeaderGenerator(
		g.Random(),
		g.Identifiers(),
		g.Signatures(),
		g.SignerIndices(),
		g.QuorumCertificates(),
		g.Time(),
		g.chainID,
	)
}

// Identifiers returns a shared generator for [flow.Identifier].
func (g *GeneratorSuite) Identifiers() *IdentifierGenerator {
	return NewIdentifierGenerator(g.Random())
}

// Signatures returns a shared generator for [crypto.Signature].
func (g *GeneratorSuite) Signatures() *SignatureGenerator {
	return NewSignatureGenerator(g.Random())
}

// Addresses returns a shared generator for [flow.Address].
func (g *GeneratorSuite) Addresses() *AddressGenerator {
	return NewAddressGenerator(g.Random(), g.chainID)
}

// SignerIndices returns a generator for [flow.SignerIndices].
func (g *GeneratorSuite) SignerIndices() *SignerIndicesGenerator {
	return NewSignerIndicesGenerator(g.Random())
}

// QuorumCertificates returns a generator for [flow.QuorumCertificate].
func (g *GeneratorSuite) QuorumCertificates() *QuorumCertificateGenerator {
	return NewQuorumCertificateGenerator(
		g.Random(),
		g.Identifiers(),
		g.SignerIndices(),
		g.Signatures(),
	)
}

// Transactions returns a generator for [flow.TransactionBody].
func (g *GeneratorSuite) Transactions() *TransactionGenerator {
	return NewTransactionGenerator(
		g.Identifiers(),
		g.ProposalKeys(),
		g.Addresses(),
		g.TransactionSignatures(),
	)
}

// Collections returns a generator for [flow.Collection].
func (g *GeneratorSuite) Collections() *CollectionGenerator {
	return NewCollectionGenerator(g.Transactions())
}

// TransactionResults returns a generator for [flow.TransactionResult].
func (g *GeneratorSuite) TransactionResults() *TransactionResultGenerator {
	return NewTransactionResultGenerator(g.Random(), g.Identifiers())
}

// LightTransactionResults returns a generator for [flow.LightTransactionResult].
func (g *GeneratorSuite) LightTransactionResults() *LightTransactionResultGenerator {
	return NewLightTransactionResultGenerator(g.TransactionResults())
}

// TransactionSignatures returns a generator for [flow.TransactionSignature].
func (g *GeneratorSuite) TransactionSignatures() *TransactionSignatureGenerator {
	return NewTransactionSignatureGenerator(g.Random(), g.Addresses())
}

// ProposalKeys returns a generator for [flow.ProposalKey].
func (g *GeneratorSuite) ProposalKeys() *ProposalKeyGenerator {
	return NewProposalKeyGenerator(g.Addresses())
}

// Events returns a generator for [flow.Event].
func (g *GeneratorSuite) Events() *EventGenerator {
	return NewEventGenerator(
		g.Random(),
		g.Identifiers(),
		g.EventTypes(),
		g.Addresses(),
	)
}

// EventTypes returns a generator for [flow.EventType].
func (g *GeneratorSuite) EventTypes() *EventTypeGenerator {
	return NewEventTypeGenerator(g.Random(), g.Addresses())
}

// ChunkExecutionDatas returns a generator for [flow.ChunkExecutionData].
func (g *GeneratorSuite) ChunkExecutionDatas() *ChunkExecutionDataGenerator {
	return NewChunkExecutionDataGenerator(
		g.Random(),
		g.Collections(),
		g.LightTransactionResults(),
		g.Events(),
		g.TrieUpdates(),
	)
}

// BlockExecutionDatas returns a generator for [flow.BlockExecutionData].
func (g *GeneratorSuite) BlockExecutionDatas() *BlockExecutionDataGenerator {
	return NewBlockExecutionDataGenerator(
		g.Identifiers(),
		g.ChunkExecutionDatas(),
	)
}

// BlockExecutionDataEntities returns a generator for [flow.BlockExecutionDataEntity].
func (g *GeneratorSuite) BlockExecutionDataEntities() *BlockExecutionDataEntityGenerator {
	return NewBlockExecutionDataEntityGenerator(g.BlockExecutionDatas())
}

// TrieUpdates returns a generator for [ledger.TrieUpdate].
func (g *GeneratorSuite) TrieUpdates() *TrieUpdateGenerator {
	return NewTrieUpdateGenerator(
		g.Random(),
		g.LedgerPaths(),
		g.LedgerPayloads(),
	)
}

// LedgerPaths returns a generator for [ledger.Path].
func (g *GeneratorSuite) LedgerPaths() *LedgerPathGenerator {
	return NewLedgerPathGenerator(g.Random())
}

// LedgerPayloads returns a generator for [ledger.Payload].
func (g *GeneratorSuite) LedgerPayloads() *LedgerPayloadGenerator {
	return NewLedgerPayloadGenerator(g.Random(), g.LedgerValues())
}

// LedgerValues returns a generator for [ledger.Value].
func (g *GeneratorSuite) LedgerValues() *LedgerValueGenerator {
	return NewLedgerValueGenerator(g.Random())
}

// Time returns a generator for [time.Time].
func (g *GeneratorSuite) Time() *TimeGenerator {
	return NewTimeGenerator(g.Random())
}
