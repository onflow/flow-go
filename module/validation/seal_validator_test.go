package validation

import (
	"testing"

	"github.com/stretchr/testify/mock"
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
	s.sealValidator = NewSealValidator(s.State, s.HeadersDB, s.IndexDB, s.ResultsDB, s.SealsDB,
		s.Assigner, s.verifier, 1, metrics.NewNoopCollector())
}

// TestSealValid tests submitting of valid seal
func (s *SealValidationSuite) TestSealValid() {
	// the BaseChainSuite creates the following fork
	//   RootBlock <- LatestSealedBlock <- LatestFinalizedBlock
	// with `LatestExecutionResult` as ExecutionResult for LatestSealedBlock
	blockParent := unittest.BlockWithParentFixture(s.LatestFinalizedBlock.Header)
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(unittest.ExecutionResultFixture(
			unittest.WithBlock(s.LatestFinalizedBlock),
			unittest.WithPreviousResult(*s.LatestExecutionResult),
		)),
	)

	blockParent.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{receipt.Meta()},
		Results:  []*flow.ExecutionResult{&receipt.ExecutionResult},
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
		Receipts: []*flow.ExecutionReceiptMeta{receipt.Meta()},
		Results:  []*flow.ExecutionResult{&receipt.ExecutionResult},
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
		Receipts: []*flow.ExecutionReceiptMeta{receipt.Meta()},
		Results:  []*flow.ExecutionResult{&receipt.ExecutionResult},
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
	// the BaseChainSuite creates the following fork
	//   RootBlock <- LatestSealedBlock <- LatestFinalizedBlock
	// with `LatestExecutionResult` as ExecutionResult for LatestSealedBlock
	blockParent := unittest.BlockWithParentFixture(s.LatestFinalizedBlock.Header)
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(unittest.ExecutionResultFixture(
			unittest.WithBlock(s.LatestFinalizedBlock),
			unittest.WithPreviousResult(*s.LatestExecutionResult),
		)),
	)
	blockParent.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{receipt.Meta()},
		Results:  []*flow.ExecutionResult{&receipt.ExecutionResult},
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
		Receipts: []*flow.ExecutionReceiptMeta{receipt.Meta()},
		Results:  []*flow.ExecutionResult{&receipt.ExecutionResult},
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
		Receipts: []*flow.ExecutionReceiptMeta{receipt.Meta()},
		Results:  []*flow.ExecutionResult{&receipt.ExecutionResult},
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
		Receipts: []*flow.ExecutionReceiptMeta{receipt.Meta()},
		Results:  []*flow.ExecutionResult{&receipt.ExecutionResult},
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

// TestHighestSeal tests that Validate will pick the seal corresponding to the
// highest block when the payload contains multiple seals that are not ordered.
// We test with the following known fork:
//    ... <- B1 <- B2 <- B3{Receipt(B2), Result(B2)} <- B4{Receipt(B3), Result(B3)}
// with
//  * B1 is the latest sealed block: we use s.LatestSealedBlock,
//    which has the result s.LatestExecutionResult
//  * B2 is the latest finalized block: we use s.LatestFinalizedBlock
// Now we consider the new candidate block B5:
//    ... <- B4 <-B5{ SealResult(B3), SealResult(B2) }
// Note that the order of the seals is specifically reversed. We expect that
// the validator handles this without error.
func (s *SealValidationSuite) TestHighestSeal() {
	// take finalized block and build a receipt for it
	block3 := unittest.BlockWithParentFixture(s.LatestFinalizedBlock.Header)
	block2Receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(unittest.ExecutionResultFixture(
			unittest.WithBlock(s.LatestFinalizedBlock),
			unittest.WithPreviousResult(*s.LatestExecutionResult),
		)),
	)
	block3.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{block2Receipt.Meta()},
		Results:  []*flow.ExecutionResult{&block2Receipt.ExecutionResult},
	})
	s.Extend(&block3)

	// create and insert block4 containing a receipt for block3
	block3Receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(unittest.ExecutionResultFixture(
			unittest.WithBlock(&block3),
			unittest.WithPreviousResult(block2Receipt.ExecutionResult),
		)),
	)
	block4 := unittest.BlockWithParentFixture(block3.Header)
	block4.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{block3Receipt.Meta()},
		Results:  []*flow.ExecutionResult{&block3Receipt.ExecutionResult},
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

// TestValidatePayload_SealsSkipBlock verifies that proposed seals
// are rejected if the chain of proposed seals skips a block.
// We test with the following known fork:
//    S  <- B0 <- B1 <- B2 <- B3{R(B0), R(B1), R(B2)}
// where S is the latest sealed block.
// Now we consider the new candidate block X:
//    S  <- B0 <- B1 <- B2 <- B3{R(B0), R(B1), R(B2)} <-X
// It would be valid for X to seal the chain of execution results R(B0), R(B1), R(B2)
// We test the two distinct failure cases:
//  (i) X has no seal for the immediately next unsealed block B0
// (ii) X has a seal for the immediately next unsealed block B0 but skips
//      the seal for one of the following blocks (here B1)
// In addition, we also run a valid test case to confirm the proper construction of the test
func (s *SealValidationSuite) TestValidatePayload_SealsSkipBlock() {
	// assuming signatures are all good
	s.verifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil).Maybe()

	blocks := unittest.ChainFixtureFrom(4, s.LatestSealedBlock.Header)

	// B3's payload contains results and receipts for B0, B1, B2
	resultB0 := unittest.ExecutionResultFixture(unittest.WithBlock(blocks[0]), unittest.WithPreviousResult(*s.LatestExecutionResult))
	receipts := unittest.ReceiptChainFor(blocks, resultB0)
	blocks[3].SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{receipts[0].Meta(), receipts[1].Meta(), receipts[2].Meta()},
		Results:  []*flow.ExecutionResult{&receipts[0].ExecutionResult, &receipts[1].ExecutionResult, &receipts[2].ExecutionResult},
	})

	for _, b := range blocks {
		s.Extend(b)
	}

	block0Seal := s.validSealForResult(&receipts[0].ExecutionResult)
	block1Seal := s.validSealForResult(&receipts[1].ExecutionResult)
	block2Seal := s.validSealForResult(&receipts[2].ExecutionResult)

	// S  <- B0 <- B1 <- B2 <- B3{R(B0), R(B1), R(B2)} <- X{Seal(R(B1))}
	s.T().Run("no seal for the immediately next unsealed block", func(t *testing.T) {
		X := unittest.BlockWithParentFixture(blocks[3].Header)
		X.SetPayload(flow.Payload{
			Seals: []*flow.Seal{block1Seal},
		})

		_, err := s.sealValidator.Validate(&X)
		require.Error(s.T(), err)
		require.True(s.T(), engine.IsInvalidInputError(err), err)
	})

	// S  <- B0 <- B1 <- B2 <- B3{R(B0), R(B1), R(B2)} <- X{Seal(R(B0)), Seal(R(B2))}
	s.T().Run("seals skip one of the following blocks", func(t *testing.T) {
		X := unittest.BlockWithParentFixture(blocks[3].Header)
		X.SetPayload(flow.Payload{
			Seals: []*flow.Seal{block0Seal, block2Seal},
		})

		_, err := s.sealValidator.Validate(&X)
		require.Error(s.T(), err)
		require.True(s.T(), engine.IsInvalidInputError(err), err)
	})

	// S  <- B0 <- B1 <- B2 <- B3{R(B0), R(B1), R(B2)} <- X{Seal(R(B0)), Seal(R(B1)), Seal(R(B2))}
	s.T().Run("valid test case", func(t *testing.T) {
		X := unittest.BlockWithParentFixture(blocks[3].Header)
		X.SetPayload(flow.Payload{
			Seals: []*flow.Seal{block0Seal, block1Seal, block2Seal},
		})

		_, err := s.sealValidator.Validate(&X)
		require.NoError(s.T(), err)
	})
}

// TestExtendSeal_ExecutionDisconnected verifies that the Seal Validator only
// accepts a seals, if their execution results connect.
// Specifically, we test that the counter-example is rejected:
//  * we have the chain in storage:
//     S <- A{Result[S]_1, Result[S]_2, ReceiptMeta[S]_1, ReceiptMeta[S]_2}
//           <- B{Result[A]_1, Result[A]_2, ReceiptMeta[A]_1, ReceiptMeta[A]_2}
//             <- C{Result[B]_1, Result[B]_2, ReceiptMeta[B]_1, ReceiptMeta[B]_2}
//                 <- D{Seal for Result[S]_1}
//  * Note that we are explicitly testing the handling of an execution fork that
//    was incorporated _before_ the seal
//       Blocks:      S  <-----------   A    <-----------   B
//      Results:   Result[S]_1  <-  Result[A]_1  <-  Result[B]_1 :: the root of this execution tree is sealed
//                 Result[S]_2  <-  Result[A]_2  <-  Result[B]_2 :: the root of this execution tree conflicts with sealed result
//  * Now we consider the new candidate block X:
//     S <- A{..} <- B{..} <- C{..} <- D{..} <- X
// We test the two distinct failure cases:
//   (i) illegal to seal Result[A]_2, because it is _not_ derived from the sealed result
//       (we verify checking of the payload seals with respect to the existing seals)
//  (ii) illegal to seal Result[A]_1 followed by Result[B]_2, as Result[B]_2 not _not_ derived
//       from the sealed result (we verify checking of the payload seals with respect to each other)
// In addition, we also run a valid test case to confirm the proper construction of the test
func (s *SealValidationSuite) TestValidatePayload_ExecutionDisconnected() {
	// assuming signatures are all good
	s.verifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil).Maybe()

	blocks := []*flow.Block{&s.LatestSealedBlock} // slice with elements  [S, A, B, C, D]
	blocks = append(blocks, unittest.ChainFixtureFrom(4, s.LatestSealedBlock.Header)...)
	receiptChain1 := unittest.ReceiptChainFor(blocks, unittest.ExecutionResultFixture()) // elements  [Result[S]_1, Result[A]_1, Result[B]_1, ...]
	receiptChain2 := unittest.ReceiptChainFor(blocks, unittest.ExecutionResultFixture()) // elements  [Result[S]_2, Result[A]_2, Result[B]_2, ...]

	for i := 1; i <= 3; i++ { // set payload for blocks A, B, C
		blocks[i].SetPayload(flow.Payload{
			Results:  []*flow.ExecutionResult{&receiptChain1[i-1].ExecutionResult, &receiptChain2[i-1].ExecutionResult},
			Receipts: []*flow.ExecutionReceiptMeta{receiptChain1[i-1].Meta(), receiptChain2[i-1].Meta()},
		})
	}
	blocks[4].SetPayload(flow.Payload{
		Seals: []*flow.Seal{unittest.Seal.Fixture(unittest.Seal.WithResult(&receiptChain1[0].ExecutionResult))},
	})
	for i := 0; i <= 4; i++ {
		// we need to run this several times, as in each iteration as we have _multiple_ execution chains.
		// In each iteration, we only mange to reconnect one additional height
		unittest.ReconnectBlocksAndReceipts(blocks, receiptChain1)
		unittest.ReconnectBlocksAndReceipts(blocks, receiptChain2)
	}

	for _, b := range blocks {
		s.Extend(b)
	}

	// seals for inclusion in X
	sealA1 := s.validSealForResult(&receiptChain1[1].ExecutionResult)
	sealB1 := s.validSealForResult(&receiptChain1[2].ExecutionResult)
	sealA2 := s.validSealForResult(&receiptChain2[1].ExecutionResult)
	sealB2 := s.validSealForResult(&receiptChain2[2].ExecutionResult)

	// S <- A{..} <- B{..} <- C{..} <- D{..} <- X{Seal for Result[A]_2}
	s.T().Run("seals in candidate block does connect to latest sealed result of parent", func(t *testing.T) {
		X := unittest.BlockWithParentFixture(blocks[4].Header)
		X.SetPayload(flow.Payload{
			Seals: []*flow.Seal{sealA2},
		})

		_, err := s.sealValidator.Validate(&X)
		require.Error(s.T(), err)
		require.True(s.T(), engine.IsInvalidInputError(err), err)
	})

	// S <- A{..} <- B{..} <- C{..} <- D{..} <- X{Seal for Result[A]_1; Seal for Result[B]_2}
	s.T().Run("sealed execution results within candidate block do not form a chain", func(t *testing.T) {
		X := unittest.BlockWithParentFixture(blocks[4].Header)
		X.SetPayload(flow.Payload{
			Seals: []*flow.Seal{sealA1, sealB2},
		})

		_, err := s.sealValidator.Validate(&X)
		require.Error(s.T(), err)
		require.True(s.T(), engine.IsInvalidInputError(err), err)
	})

	// S <- A{..} <- B{..} <- C{..} <- D{..} <- X{Seal for Result[A]_1; Seal for Result[B]_1}
	s.T().Run("valid test case", func(t *testing.T) {
		X := unittest.BlockWithParentFixture(blocks[4].Header)
		X.SetPayload(flow.Payload{
			Seals: []*flow.Seal{sealA1, sealB1},
		})

		_, err := s.sealValidator.Validate(&X)
		require.NoError(s.T(), err)
	})
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
		Receipts: []*flow.ExecutionReceiptMeta{block1Receipt.Meta()},
		Results:  []*flow.ExecutionResult{&block1Receipt.ExecutionResult},
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
			Receipts: []*flow.ExecutionReceiptMeta{block1Receipt.Meta()},
			Results:  []*flow.ExecutionResult{&block1Receipt.ExecutionResult},
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
			Receipts: []*flow.ExecutionReceiptMeta{block1Receipt.Meta()},
			Results:  []*flow.ExecutionResult{&block1Receipt.ExecutionResult},
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
