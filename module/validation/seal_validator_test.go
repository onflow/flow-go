package validation

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	mock2 "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestSealValidator(t *testing.T) {
	suite.Run(t, new(SealValidationSuite))
}

type SealValidationSuite struct {
	unittest.BaseChainSuite

	sealValidator *sealValidator
	verifier      *mock2.Verifier
}

func (s *SealValidationSuite) SetupTest() {
	s.SetupChain()
	s.verifier = &mock2.Verifier{}
	s.sealValidator = NewSealValidator(s.State, s.HeadersDB, s.PayloadsDB, s.SealsDB,
		s.Assigner, s.verifier, 1, metrics.NewNoopCollector())
}

// TestSealValid tests submitting of valid seal
func (s *SealValidationSuite) TestSealValid() {
	blockParent := unittest.BlockWithParentFixture(s.LatestFinalizedBlock.Header)
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(s.LatestFinalizedBlock))),
	)
	blockParent.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceipt{receipt},
	})

	s.Extend(&blockParent)

	block := unittest.BlockWithParentFixture(blockParent.Header)
	seal := s.validSealForResult(&receipt.ExecutionResult)
	block.SetPayload(flow.Payload{
		Seals: []*flow.Seal{seal},
	})

	_, err := s.sealValidator.Validate(&block)

	s.Require().NoError(err)
}

// TestSealInvalidBlockID tests that we reject seal with invalid blockID for
// submitted seal
func (s *SealValidationSuite) TestSealInvalidBlockID() {
	blockParent := unittest.BlockWithParentFixture(s.LatestFinalizedBlock.Header)
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(s.LatestFinalizedBlock))),
	)
	blockParent.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceipt{receipt},
	})

	s.Extend(&blockParent)

	block := unittest.BlockWithParentFixture(blockParent.Header)
	seal := s.validSealForResult(&receipt.ExecutionResult)
	seal.BlockID = unittest.IdentifierFixture()
	block.SetPayload(flow.Payload{
		Seals: []*flow.Seal{seal},
	})

	_, err := s.sealValidator.Validate(&block)

	s.Require().Error(err)
	s.Require().True(engine.IsInvalidInputError(err))
}

// TestSealInvalidAggregatedSigCount tests that we reject seal with invalid number of
// approval signatures for submitted seal
func (s *SealValidationSuite) TestSealInvalidAggregatedSigCount() {
	blockParent := unittest.BlockWithParentFixture(s.LatestFinalizedBlock.Header)
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(s.LatestFinalizedBlock))),
	)
	blockParent.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceipt{receipt},
	})

	s.Extend(&blockParent)

	block := unittest.BlockWithParentFixture(blockParent.Header)
	seal := s.validSealForResult(&receipt.ExecutionResult)
	seal.AggregatedApprovalSigs = seal.AggregatedApprovalSigs[1:]
	block.SetPayload(flow.Payload{
		Seals: []*flow.Seal{seal},
	})

	// we want to make sure that the emergency-seal metric is not called because
	// requiredApprovalsForSealing is > 0. We don't mock the EmergencySeal
	// method of the compliance collector, such that the test will fail if the
	// method is called.
	mockMetrics := &mock2.ConsensusMetrics{}
	s.sealValidator.metrics = mockMetrics

	_, err := s.sealValidator.Validate(&block)

	s.Require().Error(err)
	s.Require().True(engine.IsInvalidInputError(err))
}

// TestSealEmergencySeal checks that, when requiredApprovalsForSealVerification
// is 0, a seal which has 0 signatures for at least one chunk will be accepted,
// and that the emergency-seal metric will be incremented.
func (s *SealValidationSuite) TestSealEmergencySeal() {
	blockParent := unittest.BlockWithParentFixture(s.LatestFinalizedBlock.Header)
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(s.LatestFinalizedBlock))),
	)
	blockParent.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceipt{receipt},
	})

	s.Extend(&blockParent)

	block := unittest.BlockWithParentFixture(blockParent.Header)
	seal := s.validSealForResult(&receipt.ExecutionResult)
	seal.AggregatedApprovalSigs = seal.AggregatedApprovalSigs[1:]
	block.SetPayload(flow.Payload{
		Seals: []*flow.Seal{seal},
	})

	s.sealValidator.requiredApprovalsForSealVerification = 0
	mockMetrics := &mock2.ConsensusMetrics{}
	mockMetrics.On("EmergencySeal").Once()
	s.sealValidator.metrics = mockMetrics

	_, err := s.sealValidator.Validate(&block)
	s.Require().NoError(err)

	mockMetrics.AssertExpectations(s.T())
}

// TestSealInvalidChunkSignersCount tests that we reject seal with invalid approval signatures for
// submitted seal
func (s *SealValidationSuite) TestSealInvalidChunkSignersCount() {
	blockParent := unittest.BlockWithParentFixture(s.LatestFinalizedBlock.Header)
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(s.LatestFinalizedBlock))),
	)
	blockParent.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceipt{receipt},
	})

	s.Extend(&blockParent)

	block := unittest.BlockWithParentFixture(blockParent.Header)
	seal := s.validSealForResult(&receipt.ExecutionResult)
	seal.AggregatedApprovalSigs[0].SignerIDs = seal.AggregatedApprovalSigs[0].SignerIDs[1:]
	block.SetPayload(flow.Payload{
		Seals: []*flow.Seal{seal},
	})

	_, err := s.sealValidator.Validate(&block)

	s.Require().Error(err)
	s.Require().True(engine.IsInvalidInputError(err))
}

// TestSealInvalidChunkSignaturesCount tests that we reject seal with invalid approval signatures for
// submitted seal
func (s *SealValidationSuite) TestSealInvalidChunkSignaturesCount() {
	blockParent := unittest.BlockWithParentFixture(s.LatestFinalizedBlock.Header)
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(s.LatestFinalizedBlock))),
	)
	blockParent.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceipt{receipt},
	})

	s.Extend(&blockParent)

	block := unittest.BlockWithParentFixture(blockParent.Header)
	seal := s.validSealForResult(&receipt.ExecutionResult)
	seal.AggregatedApprovalSigs[0].VerifierSignatures = seal.AggregatedApprovalSigs[0].VerifierSignatures[1:]
	block.SetPayload(flow.Payload{
		Seals: []*flow.Seal{seal},
	})

	_, err := s.sealValidator.Validate(&block)

	s.Require().Error(err)
	s.Require().True(engine.IsInvalidInputError(err))
}

// TestSealInvalidChunkAssignment tests that we reject seal with invalid signerID of approval signature for
// submitted seal
func (s *SealValidationSuite) TestSealInvalidChunkAssignment() {
	blockParent := unittest.BlockWithParentFixture(s.LatestFinalizedBlock.Header)
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(s.LatestFinalizedBlock))),
	)
	blockParent.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceipt{receipt},
	})

	s.Extend(&blockParent)

	block := unittest.BlockWithParentFixture(blockParent.Header)
	seal := s.validSealForResult(&receipt.ExecutionResult)
	seal.AggregatedApprovalSigs[0].SignerIDs[0] = unittest.IdentifierFixture()
	block.SetPayload(flow.Payload{
		Seals: []*flow.Seal{seal},
	})

	_, err := s.sealValidator.Validate(&block)

	s.Require().Error(err)
	s.Require().True(engine.IsInvalidInputError(err))
}

// TestHighestSeal tests that Validate will pick the seal corresponding to the highest block when
// the payload contains multiple seals that are not ordered.
func (s *SealValidationSuite) TestHighestSeal() {
	// take finalized block and build a receipt for it
	block3 := unittest.BlockWithParentFixture(s.LatestFinalizedBlock.Header)
	block2Receipt := unittest.ReceiptForBlockFixture(s.LatestFinalizedBlock)
	block3.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceipt{block2Receipt},
	})
	s.Extend(&block3)

	// create and insert block4 containing a receipt for block3
	block3Receipt := unittest.ReceiptForBlockFixture(&block3)
	block4 := unittest.BlockWithParentFixture(block3.Header)
	block4.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceipt{block3Receipt},
	})
	s.Extend(&block4)

	seal2 := s.validSealForResult(&block2Receipt.ExecutionResult)
	seal3 := s.validSealForResult(&block3Receipt.ExecutionResult)

	// include the seals in block5
	block5 := unittest.BlockWithParentFixture(block4.Header)
	block5.SetPayload(flow.Payload{
		// placing seals in the reversed order to test
		// Extend will pick the highest sealed block
		Seals: []*flow.Seal{seal3, seal2},
	})

	last, err := s.sealValidator.Validate(&block5)
	require.NoError(s.T(), err)
	require.Equal(s.T(), last.FinalState, seal3.FinalState)
}

// TestExtendSealNotConnected tests that proposed seals are rejected if they do not form a valid chain on
// top of the last known seal on the branch.
func (s *SealValidationSuite) TestExtendSealNotConnected() {
	// B <- B1 <- B2 <- B3{R(B1), R(B2)} <- B4{S(R(B2))}

	// insert 2 valid blocks
	block1 := unittest.BlockWithParentFixture(s.LatestSealedBlock.Header)
	block1.SetPayload(flow.Payload{})
	s.Extend(&block1)

	block2 := unittest.BlockWithParentFixture(block1.Header)
	block2.SetPayload(flow.Payload{})
	s.Extend(&block2)

	// insert block3 with receipts for block1 and block2
	block1Receipt := unittest.ReceiptForBlockFixture(&block1)
	block2Receipt := unittest.ReceiptForBlockFixture(&block2)

	block3 := unittest.BlockWithParentFixture(block2.Header)
	block3.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceipt{block1Receipt, block2Receipt},
	})
	s.Extend(&block3)

	// Insert block4 with a seal for block 2. Note that there is no seal
	// for block1. The block should be rejected because it contains a seal
	// that breaks the chain.
	block2Seal := s.validSealForResult(&block2Receipt.ExecutionResult)

	block4 := unittest.BlockWithParentFixture(block3.Header)
	block4.SetPayload(flow.Payload{
		Seals: []*flow.Seal{block2Seal},
	})

	_, err := s.sealValidator.Validate(&block4)

	require.Error(s.T(), err)
	require.True(s.T(), engine.IsInvalidInputError(err), err)
}

// TestExtendSealDuplicate tests that payloads containing duplicate seals are rejected.
func (s *SealValidationSuite) TestExtendSealDuplicate() {
	block1 := unittest.BlockWithParentFixture(s.LatestSealedBlock.Header)
	block1.SetPayload(flow.Payload{})
	s.Extend(&block1)

	// create block2 with an execution receipt for block1
	block1Receipt := unittest.ReceiptForBlockFixture(&block1)
	block2 := unittest.BlockWithParentFixture(block1.Header)
	block2.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceipt{block1Receipt},
	})
	s.Extend(&block2)

	// create seal for block1
	block1Seal := unittest.Seal.Fixture(unittest.Seal.WithResult(&block1Receipt.ExecutionResult))

	// B <- B1 <- B2{R(B1)} <- B3{S(R(B1))} <- B4{S(R(B1))}
	s.T().Run("Duplicate seal in separate block", func(t *testing.T) {
		// insert block3 with a seal for block1
		block3 := unittest.BlockWithParentFixture(block2.Header)
		block3.SetPayload(flow.Payload{
			Seals: []*flow.Seal{block1Seal},
		})
		s.Extend(&block3)

		// insert block4 with a duplicate seal
		block4 := unittest.BlockWithParentFixture(block3.Header)
		block4.SetPayload(flow.Payload{
			Seals: []*flow.Seal{block1Seal},
		})
		_, err := s.sealValidator.Validate(&block4)

		// we expect an error because block 4 contains a seal that is
		// already contained in another block on the fork
		require.Error(t, err)
		require.True(t, engine.IsInvalidInputError(err), err)
	})

	// B <- B1 <- B2{R(B1)} <- B3{S(R(B1)), S(R(B1))}
	s.T().Run("Duplicate seal in same payload", func(t *testing.T) {
		// insert block3 with 2 identical seals for block1
		block3 := unittest.BlockWithParentFixture(block2.Header)
		block3.SetPayload(flow.Payload{
			Seals: []*flow.Seal{block1Seal, block1Seal},
		})

		_, err := s.sealValidator.Validate(&block3)

		// we expect an error because block 3 contains duplicate seals
		// within its payload
		require.Error(t, err)
		require.True(t, engine.IsInvalidInputError(err), err)
	})
}

// TestExtendSealNoIncorporatedResult tests that seals are rejected if they correspond to ExecutionResults that are
// not incorporated in blocks on this fork
func (s *SealValidationSuite) TestExtendSealNoIncorporatedResult() {
	block1 := unittest.BlockWithParentFixture(s.LatestSealedBlock.Header)
	block1.SetPayload(flow.Payload{})
	s.Extend(&block1)

	// B-->B1-->B2{Seal(ER1)}
	//
	// Should fail because the fork does not contain an IncorporatedResult for the
	// result (ER1) referenced by the proposed seal.
	s.T().Run("no IncorporatedResult", func(t *testing.T) {
		// create block 2 with a seal for block 1
		block1Result := unittest.ExecutionResultFixture(unittest.WithBlock(&block1))
		block1Seal := unittest.Seal.Fixture(unittest.Seal.WithResult(block1Result))

		block2 := unittest.BlockWithParentFixture(block1.Header)
		block2.SetPayload(flow.Payload{
			Seals: []*flow.Seal{block1Seal},
		})

		_, err := s.sealValidator.Validate(&block2)
		// we expect an error because there is no block on the fork that
		// contains a receipt committing to block1
		require.Error(t, err)
		require.True(t, engine.IsInvalidInputError(err), err)
	})

	// B-->B1-->B2{ER1a}-->B3{Seal(ER1b)}
	//
	// Should fail because ER1a is different than ER1b, although they
	// reference the same block. Technically the fork does not contain an
	// IncorporatedResult for the result referenced by the proposed seal.
	s.T().Run("different IncorporatedResult", func(t *testing.T) {
		// create block2 with an execution receipt for block1
		block1Receipt := unittest.ReceiptForBlockFixture(&block1)
		block2 := unittest.BlockWithParentFixture(block1.Header)
		block2.SetPayload(flow.Payload{
			Receipts: []*flow.ExecutionReceipt{block1Receipt},
		})
		s.Extend(&block2)

		// create block 3 with a seal for block 1, but DIFFERENT execution
		// result than that which was included in block1
		block1Result2 := unittest.ExecutionResultFixture(unittest.WithBlock(&block1))
		block1Seal := unittest.Seal.Fixture(unittest.Seal.WithResult(block1Result2))

		block3 := unittest.BlockWithParentFixture(block2.Header)
		block3.SetPayload(flow.Payload{
			Seals: []*flow.Seal{block1Seal},
		})

		_, err := s.sealValidator.Validate(&block3)
		// we expect an error because there is no block on the fork that
		// contains a receipt committing to the seal's result
		require.Error(t, err)
		require.True(t, engine.IsInvalidInputError(err), err)
	})

	// B-->B1-->B2-->B4{Seal(ER1)}
	//      |
	//      +-->B3{ER1}
	//
	// Should fail because the IncorporatedResult referenced by the seal is
	// on a different fork
	s.T().Run("IncorporatedResult in other fork", func(t *testing.T) {
		// create block2 and block3 as children of block1 (introducing a fork)
		block2 := unittest.BlockWithParentFixture(block1.Header)
		block2.SetPayload(flow.Payload{})
		s.Extend(&block2)

		// only block 3 contains the result
		block1Receipt := unittest.ReceiptForBlockFixture(&block1)
		block3 := unittest.BlockWithParentFixture(block1.Header)
		block3.SetPayload(flow.Payload{
			Receipts: []*flow.ExecutionReceipt{block1Receipt},
		})
		s.Extend(&block3)

		// create block4 on top of block2 containing a seal for the result
		// contained on the other fork
		block1Seal := unittest.Seal.Fixture(unittest.Seal.WithResult(&block1Receipt.ExecutionResult))
		block4 := unittest.BlockWithParentFixture(block2.Header)
		block4.SetPayload(flow.Payload{
			Seals: []*flow.Seal{block1Seal},
		})

		_, err := s.sealValidator.Validate(&block4)
		// we expect an error because there is no block on the fork that
		// contains a receipt committing to the seal's result
		require.Error(t, err)
		require.True(t, engine.IsInvalidInputError(err), err)
	})
}

// validSealForResult generates a valid seal based on ExecutionResult. As part of seal generation it
// configures mocked seal verifier to match approvals based on chunk assignments.
func (s *SealValidationSuite) validSealForResult(result *flow.ExecutionResult) *flow.Seal {
	seal := unittest.Seal.Fixture(unittest.Seal.WithResult(result))

	assignment := s.Assignments[result.ID()]
	for _, chunk := range result.Chunks {
		aggregatedSigs := &seal.AggregatedApprovalSigs[chunk.Index]
		assignedVerifiers := assignment.Verifiers(chunk)
		aggregatedSigs.SignerIDs = assignedVerifiers[:]
		aggregatedSigs.VerifierSignatures = unittest.SignaturesFixture(len(assignedVerifiers))

		for i, aggregatedSig := range aggregatedSigs.VerifierSignatures {
			payload := flow.Attestation{
				BlockID:           result.BlockID,
				ExecutionResultID: result.ID(),
				ChunkIndex:        chunk.Index,
			}.ID()
			s.verifier.On("Verify",
				payload[:],
				aggregatedSig,
				s.Identities[aggregatedSigs.SignerIDs[i]].StakingPubKey).Return(true, nil).Once()
		}
	}
	return seal
}
