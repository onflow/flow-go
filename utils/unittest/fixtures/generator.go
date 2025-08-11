package fixtures

import (
	"math/rand"
	"testing"
	"time"
)

// GeneratorSuite provides a context-aware generator system for creating test fixtures.
// It manages a shared random number generator and provides access to specialized generators.
type GeneratorSuite struct {
	rng *rand.Rand
}

// GeneratorSuiteOption defines an option for configuring a GeneratorSuite.
type GeneratorSuiteOption func(*generatorSuiteConfig)

type generatorSuiteConfig struct {
	seed int64
}

// WithSeed sets the seed for the generator suite.
func WithSeed(seed int64) GeneratorSuiteOption {
	return func(config *generatorSuiteConfig) {
		config.seed = seed
	}
}

// NewGeneratorSuite creates a new generator suite with optional configuration.
// If no seed is specified, a random seed is generated.
func NewGeneratorSuite(t testing.TB, opts ...GeneratorSuiteOption) *GeneratorSuite {
	config := &generatorSuiteConfig{
		seed: time.Now().UnixNano(), // default random seed
	}

	// Apply options
	for _, opt := range opts {
		opt(config)
	}

	t.Logf("generator suite seed: %d", config.seed)

	return &GeneratorSuite{
		rng: rand.New(rand.NewSource(config.seed)),
	}
}

// RNG returns the random number generator used by this suite.
func (g *GeneratorSuite) RNG() *rand.Rand {
	return g.rng
}

// Random returns a random generator for this suite.
func (g *GeneratorSuite) Random() *RandomGenerator {
	return &RandomGenerator{
		rng: g.rng,
	}
}

// BlockHeaders returns a block header generator for this suite.
func (g *GeneratorSuite) BlockHeaders() *BlockHeaderGenerator {
	return &BlockHeaderGenerator{
		randomGen:        g.Random(),
		identifierGen:    g.Identifiers(),
		signatureGen:     g.Signatures(),
		signerIndicesGen: g.SignerIndices(),
		quorumCertGen:    g.QuorumCertificates(),
		timeGen:          g.Time(),
	}
}

// Identifiers returns an identifier generator for this suite.
func (g *GeneratorSuite) Identifiers() *IdentifierGenerator {
	return &IdentifierGenerator{
		randomGen: g.Random(),
	}
}

// Signatures returns a signature generator for this suite.
func (g *GeneratorSuite) Signatures() *SignatureGenerator {
	return &SignatureGenerator{
		randomGen: g.Random(),
	}
}

// Addresses returns an address generator for this suite.
func (g *GeneratorSuite) Addresses() *AddressGenerator {
	return &AddressGenerator{
		randomGen: g.Random(),
	}
}

// SignerIndices returns a signer indices generator for this suite.
func (g *GeneratorSuite) SignerIndices() *SignerIndicesGenerator {
	return &SignerIndicesGenerator{}
}

// QuorumCertificates returns a quorum certificate generator for this suite.
func (g *GeneratorSuite) QuorumCertificates() *QuorumCertificateGenerator {
	return &QuorumCertificateGenerator{
		randomGen:        g.Random(),
		identifierGen:    g.Identifiers(),
		signerIndicesGen: g.SignerIndices(),
		signatureGen:     g.Signatures(),
	}
}

// ChunkExecutionDatas returns a chunk execution data generator for this suite.
func (g *GeneratorSuite) ChunkExecutionDatas() *ChunkExecutionDataGenerator {
	return &ChunkExecutionDataGenerator{
		randomGen:                 g.Random(),
		collectionGen:             g.Collections(),
		lightTransactionResultGen: g.LightTransactionResults(),
		eventGen:                  g.Events(),
		trieUpdateGen:             g.TrieUpdates(),
	}
}

// BlockExecutionDatas returns a block execution data generator for this suite.
func (g *GeneratorSuite) BlockExecutionDatas() *BlockExecutionDataGenerator {
	return &BlockExecutionDataGenerator{
		identifierGen:         g.Identifiers(),
		chunkExecutionDataGen: g.ChunkExecutionDatas(),
	}
}

// BlockExecutionDataEntities returns a block execution data entity generator for this suite.
func (g *GeneratorSuite) BlockExecutionDataEntities() *BlockExecutionDataEntityGenerator {
	return &BlockExecutionDataEntityGenerator{
		BlockExecutionDataGenerator: g.BlockExecutionDatas(),
	}
}

// Transactions returns a transaction generator for this suite.
func (g *GeneratorSuite) Transactions() *TransactionGenerator {
	return &TransactionGenerator{
		identifierGen:     g.Identifiers(),
		proposalKeyGen:    g.ProposalKeys(),
		addressGen:        g.Addresses(),
		transactionSigGen: g.TransactionSignatures(),
	}
}

// FullTransactions returns a full transaction generator for this suite.
func (g *GeneratorSuite) FullTransactions() *FullTransactionGenerator {
	return &FullTransactionGenerator{
		TransactionGenerator: g.Transactions(),
	}
}

// Collections returns a collection generator for this suite.
func (g *GeneratorSuite) Collections() *CollectionGenerator {
	return &CollectionGenerator{
		transactionGen: g.Transactions(),
	}
}

// TrieUpdates returns a trie update generator for this suite.
func (g *GeneratorSuite) TrieUpdates() *TrieUpdateGenerator {
	return &TrieUpdateGenerator{
		randomGen: g.Random(),
	}
}

// TransactionResults returns a transaction result generator for this suite.
func (g *GeneratorSuite) TransactionResults() *TransactionResultGenerator {
	return &TransactionResultGenerator{
		randomGen:     g.Random(),
		identifierGen: g.Identifiers(),
	}
}

// LightTransactionResults returns a light transaction result generator for this suite.
func (g *GeneratorSuite) LightTransactionResults() *LightTransactionResultGenerator {
	return &LightTransactionResultGenerator{
		TransactionResultGenerator: g.TransactionResults(),
	}
}

// TransactionSignatures returns a transaction signature generator for this suite.
func (g *GeneratorSuite) TransactionSignatures() *TransactionSignatureGenerator {
	return &TransactionSignatureGenerator{
		randomGen:  g.Random(),
		addressGen: g.Addresses(),
	}
}

// ProposalKeys returns a proposal key generator for this suite.
func (g *GeneratorSuite) ProposalKeys() *ProposalKeyGenerator {
	return &ProposalKeyGenerator{
		addressGen: g.Addresses(),
	}
}

// Events returns an event generator for this suite.
func (g *GeneratorSuite) Events() *EventGenerator {
	return &EventGenerator{
		randomGen:     g.Random(),
		identifierGen: g.Identifiers(),
		eventTypeGen:  g.EventTypes(),
		addressGen:    g.Addresses(),
	}
}

// EventTypes returns an event type generator for this suite.
func (g *GeneratorSuite) EventTypes() *EventTypeGenerator {
	return &EventTypeGenerator{
		randomGen:  g.Random(),
		addressGen: g.Addresses(),
	}
}

// Time returns a time generator for this suite.
func (g *GeneratorSuite) Time() *TimeGenerator {
	return &TimeGenerator{
		randomGen: g.Random(),
	}
}
