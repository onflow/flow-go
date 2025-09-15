package fixtures

import (
	"github.com/onflow/flow-go/model/flow"
)

// Seal is the default options factory for [flow.Seal] generation.
var Seal sealFactory

type sealFactory struct{}

type SealOption func(*SealGenerator, *flow.Seal)

// WithBlockID is an option that sets the `BlockID` of the seal.
func (f sealFactory) WithBlockID(blockID flow.Identifier) SealOption {
	return func(g *SealGenerator, seal *flow.Seal) {
		seal.BlockID = blockID
	}
}

// WithResultID is an option that sets the `ResultID` of the seal.
func (f sealFactory) WithResultID(resultID flow.Identifier) SealOption {
	return func(g *SealGenerator, seal *flow.Seal) {
		seal.ResultID = resultID
	}
}

// WithFinalState is an option that sets the `FinalState` of the seal.
func (f sealFactory) WithFinalState(finalState flow.StateCommitment) SealOption {
	return func(g *SealGenerator, seal *flow.Seal) {
		seal.FinalState = finalState
	}
}

// WithAggregatedApprovalSigs is an option that sets the `AggregatedApprovalSigs` of the seal.
func (f sealFactory) WithAggregatedApprovalSigs(sigs ...flow.AggregatedSignature) SealOption {
	return func(g *SealGenerator, seal *flow.Seal) {
		seal.AggregatedApprovalSigs = sigs
	}
}

// SealGenerator generates seals with consistent randomness.
type SealGenerator struct {
	sealFactory

	random               *RandomGenerator
	identifiers          *IdentifierGenerator
	stateCommitments     *StateCommitmentGenerator
	aggregatedSignatures *AggregatedSignatureGenerator
}

func NewSealGenerator(
	random *RandomGenerator,
	identifiers *IdentifierGenerator,
	stateCommitments *StateCommitmentGenerator,
	aggregatedSignatures *AggregatedSignatureGenerator,
) *SealGenerator {
	return &SealGenerator{
		random:               random,
		identifiers:          identifiers,
		stateCommitments:     stateCommitments,
		aggregatedSignatures: aggregatedSignatures,
	}
}

// Fixture generates a [flow.Seal] with random data based on the provided options.
func (g *SealGenerator) Fixture(opts ...SealOption) *flow.Seal {
	seal := &flow.Seal{
		BlockID:                g.identifiers.Fixture(),
		ResultID:               g.identifiers.Fixture(),
		FinalState:             g.stateCommitments.Fixture(),
		AggregatedApprovalSigs: g.aggregatedSignatures.List(g.random.IntInRange(1, 4)),
	}

	for _, opt := range opts {
		opt(g, seal)
	}

	return seal
}

// List generates a list of [flow.Seal] with random data.
func (g *SealGenerator) List(n int, opts ...SealOption) []*flow.Seal {
	seals := make([]*flow.Seal, 0, n)
	for range n {
		seals = append(seals, g.Fixture(opts...))
	}
	return seals
}
