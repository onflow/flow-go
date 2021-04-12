package unittest

import "github.com/onflow/flow-go/model/flow"

var Seal sealFactory

type sealFactory struct{}

func (f *sealFactory) Fixture(opts ...func(*flow.Seal)) *flow.Seal {
	seal := &flow.Seal{
		BlockID:                IdentifierFixture(),
		ResultID:               IdentifierFixture(),
		FinalState:             StateCommitmentFixture(),
		AggregatedApprovalSigs: Seal.AggregatedSignatureFixtures(3), // 3 chunks
	}
	for _, apply := range opts {
		apply(seal)
	}
	return seal
}

func (f *sealFactory) Fixtures(n int) []*flow.Seal {
	seals := make([]*flow.Seal, 0, n)
	for i := 0; i < n; i++ {
		seal := Seal.Fixture()
		seals = append(seals, seal)
	}
	return seals
}

func (f *sealFactory) WithResult(result *flow.ExecutionResult) func(*flow.Seal) {
	return func(seal *flow.Seal) {
		finalState := result.FinalStateCommitment()
		seal.ResultID = result.ID()
		seal.BlockID = result.BlockID
		seal.FinalState = finalState
		seal.AggregatedApprovalSigs = Seal.AggregatedSignatureFixtures(len(result.Chunks))
	}
}

func (f *sealFactory) WithBlockID(blockID flow.Identifier) func(*flow.Seal) {
	return func(seal *flow.Seal) {
		seal.BlockID = blockID
	}
}

func (f *sealFactory) WithBlock(block *flow.Header) func(*flow.Seal) {
	return func(seal *flow.Seal) {
		seal.BlockID = block.ID()
	}
}

func (f *sealFactory) AggregatedSignatureFixtures(number int) []flow.AggregatedSignature {
	sigs := make([]flow.AggregatedSignature, 0, number)
	for ; number > 0; number-- {
		sigs = append(sigs, Seal.AggregatedSignatureFixture())
	}
	return sigs
}

func (f *sealFactory) AggregatedSignatureFixture() flow.AggregatedSignature {
	return flow.AggregatedSignature{
		VerifierSignatures: SignaturesFixture(7),
		SignerIDs:          IdentifierListFixture(7),
	}
}
