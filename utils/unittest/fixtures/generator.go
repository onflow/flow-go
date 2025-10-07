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

// Headers returns a generator for [flow.Header].
func (g *GeneratorSuite) Headers() *HeaderGenerator {
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

// Blocks returns a generator for [flow.Block].
func (g *GeneratorSuite) Blocks() *BlockGenerator {
	return NewBlockGenerator(
		g.Random(),
		g.Identifiers(),
		g.Headers(),
		g.Payloads(),
		g.chainID,
	)
}

// Payloads returns a generator for [flow.Payload].
func (g *GeneratorSuite) Payloads() *PayloadGenerator {
	return NewPayloadGenerator(
		g.Random(),
		g.Identifiers(),
		g.Guarantees(),
		g.Seals(),
		g.ExecutionReceiptStubs(),
		g.ExecutionResults(),
	)
}

// Seals returns a generator for [flow.Seal].
func (g *GeneratorSuite) Seals() *SealGenerator {
	return NewSealGenerator(
		g.Random(),
		g.Identifiers(),
		g.StateCommitments(),
		g.AggregatedSignatures(),
	)
}

// Guarantees returns a generator for [flow.CollectionGuarantee].
func (g *GeneratorSuite) Guarantees() *CollectionGuaranteeGenerator {
	return NewCollectionGuaranteeGenerator(
		g.Random(),
		g.Identifiers(),
		g.Signatures(),
		g.SignerIndices(),
		g.chainID,
	)
}

// ExecutionReceiptStubs returns a generator for [flow.ExecutionReceiptStub].
func (g *GeneratorSuite) ExecutionReceiptStubs() *ExecutionReceiptStubGenerator {
	return NewExecutionReceiptStubGenerator(
		g.Random(),
		g.Identifiers(),
		g.Signatures(),
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

// QuorumCertificatesWithSignerIDs returns a generator for [flow.QuorumCertificateWithSignerIDs].
func (g *GeneratorSuite) QuorumCertificatesWithSignerIDs() *QuorumCertificateWithSignerIDsGenerator {
	return NewQuorumCertificateWithSignerIDsGenerator(
		g.Random(),
		g.Identifiers(),
		g.QuorumCertificates(),
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
		g.Random(),
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

func (g *GeneratorSuite) Identities() *IdentityGenerator {
	return NewIdentityGenerator(g.Random(), g.Crypto(), g.Identifiers(), g.Addresses())
}

func (g *GeneratorSuite) Crypto() *CryptoGenerator {
	return NewCryptoGenerator(g.Random())
}

// StateCommitments returns a generator for [flow.StateCommitment].
func (g *GeneratorSuite) StateCommitments() *StateCommitmentGenerator {
	return NewStateCommitmentGenerator(g.Random())
}

// AggregatedSignatures returns a generator for [flow.AggregatedSignature].
func (g *GeneratorSuite) AggregatedSignatures() *AggregatedSignatureGenerator {
	return NewAggregatedSignatureGenerator(g.Random(), g.Identifiers(), g.Signatures())
}

// TimeoutCertificates returns a generator for [flow.TimeoutCertificate].
func (g *GeneratorSuite) TimeoutCertificates() *TimeoutCertificateGenerator {
	return NewTimeoutCertificateGenerator(g.Random(), g.QuorumCertificates(), g.Signatures(), g.SignerIndices())
}

// ExecutionResults returns a generator for [flow.ExecutionResult].
func (g *GeneratorSuite) ExecutionResults() *ExecutionResultGenerator {
	return NewExecutionResultGenerator(
		g.Random(),
		g.Identifiers(),
		g.Chunks(),
		g.ServiceEvents(),
		g.StateCommitments(),
	)
}

// ExecutionReceipts returns a generator for [flow.ExecutionReceipt].
func (g *GeneratorSuite) ExecutionReceipts() *ExecutionReceiptGenerator {
	return NewExecutionReceiptGenerator(
		g.Random(),
		g.Identifiers(),
		g.ExecutionResults(),
		g.Signatures(),
	)
}

// Chunks returns a generator for [flow.Chunk].
func (g *GeneratorSuite) Chunks() *ChunkGenerator {
	return NewChunkGenerator(
		g.Random(),
		g.Identifiers(),
		g.StateCommitments(),
	)
}

// ServiceEvents returns a generator for [flow.ServiceEvent].
func (g *GeneratorSuite) ServiceEvents() *ServiceEventGenerator {
	return NewServiceEventGenerator(
		g.Random(),
		g.EpochSetups(),
		g.EpochCommits(),
		g.EpochRecovers(),
		g.VersionBeacons(),
		g.ProtocolStateVersionUpgrades(),
		g.SetEpochExtensionViewCounts(),
		g.EjectNodes(),
	)
}

// VersionBeacons returns a generator for [flow.VersionBeacon].
func (g *GeneratorSuite) VersionBeacons() *VersionBeaconGenerator {
	return NewVersionBeaconGenerator(g.Random())
}

// ProtocolStateVersionUpgrades returns a generator for [flow.ProtocolStateVersionUpgrade].
func (g *GeneratorSuite) ProtocolStateVersionUpgrades() *ProtocolStateVersionUpgradeGenerator {
	return NewProtocolStateVersionUpgradeGenerator(g.Random())
}

// SetEpochExtensionViewCounts returns a generator for [flow.SetEpochExtensionViewCount].
func (g *GeneratorSuite) SetEpochExtensionViewCounts() *SetEpochExtensionViewCountGenerator {
	return NewSetEpochExtensionViewCountGenerator(g.Random())
}

// EjectNodes returns a generator for [flow.EjectNode].
func (g *GeneratorSuite) EjectNodes() *EjectNodeGenerator {
	return NewEjectNodeGenerator(g.Identifiers())
}

// EpochSetups returns a generator for [flow.EpochSetup].
func (g *GeneratorSuite) EpochSetups() *EpochSetupGenerator {
	return NewEpochSetupGenerator(g.Random(), g.Time(), g.Identities())
}

// EpochCommits returns a generator for [flow.EpochCommit].
func (g *GeneratorSuite) EpochCommits() *EpochCommitGenerator {
	return NewEpochCommitGenerator(
		g.Random(),
		g.Crypto(),
		g.Identifiers(),
		g.QuorumCertificatesWithSignerIDs(),
	)
}

// EpochRecovers returns a generator for [flow.EpochRecover].
func (g *GeneratorSuite) EpochRecovers() *EpochRecoverGenerator {
	return NewEpochRecoverGenerator(
		g.Random(),
		g.EpochSetups(),
		g.EpochCommits(),
	)
}

// PendingExecutionEvents returns a generator for [flow.PendingExecutionEvent].
func (g *GeneratorSuite) PendingExecutionEvents() *PendingExecutionEventGenerator {
	return NewPendingExecutionEventGenerator(
		g.Random(),
		g.Addresses(),
		g.Events(),
		g.chainID,
	)
}
