package validation

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestSealValidator(t *testing.T) {
	suite.Run(t, new(SealValidationSuite))
}

type SealValidationSuite struct {
	unittest.BaseChainSuite

	sealValidator *sealValidator
	metrics       *module.ConsensusMetrics
	verifier      *module.Verifier
}

func (s *SealValidationSuite) SetupTest() {
	s.SetupChain()
	s.verifier = &module.Verifier{}
	s.metrics = &module.ConsensusMetrics{}

	var err error
	s.sealValidator, err = NewSealValidator(s.State, s.HeadersDB, s.IndexDB, s.ResultsDB, s.SealsDB,
		s.Assigner, s.verifier, 2, 2, s.metrics)
	s.Require().NoError(err)
}

// TestConsistencyCheckOnApprovals verifies that SealValidator instantiation fails if
// required number of approvals for seal construction is smaller than for seal verification
func (s *SealValidationSuite) TestConsistencyCheckOnApprovals() {
	_, err := NewSealValidator(s.State, s.HeadersDB, s.IndexDB, s.ResultsDB, s.SealsDB,
		s.Assigner, s.verifier, 2, 3, s.metrics)
	s.Require().Error(err)
}

// TestSealValid tests that a candidate block with a valid seal passes validation.
// We test with the following fork:
//   ... <- LatestSealedBlock <- B0 <- B1{ Result[B0], Receipt[B0] } <- B2 <- ░newBlock{ Seal[B0]}░
// The gap of 1 block, i.e. B2, is required to avoid a sealing edge-case
// (see test `TestSeal_EnforceGap` for more details)
func (s *SealValidationSuite) TestSealValid() {
	// Notes:
	//  * the result for `LatestSealedBlock` is `LatestExecutionResult` (already initialized in test setup)
	//  * as block B0, we use `LatestFinalizedBlock` (already initialized in test setup)
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(unittest.ExecutionResultFixture(
			unittest.WithBlock(s.LatestFinalizedBlock),
			unittest.WithPreviousResult(*s.LatestExecutionResult),
		)),
	)

	// construct block B1 and B2
	b1 := unittest.BlockWithParentFixture(s.LatestFinalizedBlock.Header)
	b1.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{receipt.Meta()},
		Results:  []*flow.ExecutionResult{&receipt.ExecutionResult},
	})
	b2 := unittest.BlockWithParentFixture(b1.Header)
	s.Extend(&b1)
	s.Extend(&b2)

	// construct `newBlock`
	seal := s.validSealForResult(&receipt.ExecutionResult)
	newBlock := unittest.BlockWithParentFixture(b2.Header)
	newBlock.SetPayload(flow.Payload{
		Seals: []*flow.Seal{seal},
	})

	_, err := s.sealValidator.Validate(&newBlock)
	s.Require().NoError(err)
}

// TestSeal_EnforceGap checks the seal-validation does _not_ allow to seal a result that was
// incorporated in the direct parent. In other words, there must be at least a 1-block gap
// between the block incorporating the result and the block sealing the result. Enforcing
// such gap is important for the following reason:
//  * We need the Source of Randomness for the block that _incorporates_ the sealed result,
//    to compute the verifier assignment. Therefore, we require that the block _incorporating_
//    the result has at least one child in the fork, _before_ we include the seal.
//  * Thereby, we guarantee that a verifier assignment can be computed without needing
//    unverified information (the unverified qc) from the block that we are just constructing.
func (s *SealValidationSuite) TestSeal_EnforceGap() {
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

	// block includes seal for direct parent:
	block := unittest.BlockWithParentFixture(blockParent.Header)
	seal := s.validSealForResult(&receipt.ExecutionResult)
	block.SetPayload(flow.Payload{
		Seals: []*flow.Seal{seal},
	})

	_, err := s.sealValidator.Validate(&block)
	s.Require().True(engine.IsInvalidInputError(err))
}

// TestSealInvalidBlockID tests that we reject seal with invalid blockID for
// submitted seal. We test with the following fork:
//   ... <- LatestSealedBlock <- B0 <- B1{ Result[B0], Receipt[B0] } <- B2 <- ░newBlock{ Seal[B0]}░
// The gap of 1 block, i.e. B2, is required to avoid a sealing edge-case
// (see test `TestSeal_EnforceGap` for more details)
func (s *SealValidationSuite) TestSealInvalidBlockID() {
	// Notes:
	//  * the result for `LatestSealedBlock` is `LatestExecutionResult` (already initialized in test setup)
	//  * as block B0, we use `LatestFinalizedBlock` (already initialized in test setup)
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(unittest.ExecutionResultFixture(
			unittest.WithBlock(s.LatestFinalizedBlock),
			unittest.WithPreviousResult(*s.LatestExecutionResult),
		)),
	)

	// construct block B1 and B2
	b1 := unittest.BlockWithParentFixture(s.LatestFinalizedBlock.Header)
	b1.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{receipt.Meta()},
		Results:  []*flow.ExecutionResult{&receipt.ExecutionResult},
	})
	b2 := unittest.BlockWithParentFixture(b1.Header)
	s.Extend(&b1)
	s.Extend(&b2)

	newBlock := unittest.BlockWithParentFixture(b2.Header)
	seal := s.validSealForResult(&receipt.ExecutionResult)
	seal.BlockID = unittest.IdentifierFixture()
	newBlock.SetPayload(flow.Payload{
		Seals: []*flow.Seal{seal},
	})

	_, err := s.sealValidator.Validate(&newBlock)
	s.Require().Error(err)
	s.Require().True(engine.IsInvalidInputError(err))
}

// TestSealInvalidAggregatedSigCount tests that we reject seal with invalid number of
// approval signatures for submitted seal. We test with the following fork:
//   ... <- LatestSealedBlock <- B0 <- B1{ Result[B0], Receipt[B0] } <- B2 <- ░newBlock{ Seal[B0]}░
// The gap of 1 block, i.e. B2, is required to avoid a sealing edge-case
// (see test `TestSeal_EnforceGap` for more details)
func (s *SealValidationSuite) TestSealInvalidAggregatedSigCount() {
	// Notes:
	//  * the result for `LatestSealedBlock` is `LatestExecutionResult` (already initialized in test setup)
	//  * as block B0, we use `LatestFinalizedBlock` (already initialized in test setup)
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(unittest.ExecutionResultFixture(
			unittest.WithBlock(s.LatestFinalizedBlock),
			unittest.WithPreviousResult(*s.LatestExecutionResult),
		)),
	)

	// construct block B1 and B2
	b1 := unittest.BlockWithParentFixture(s.LatestFinalizedBlock.Header)
	b1.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{receipt.Meta()},
		Results:  []*flow.ExecutionResult{&receipt.ExecutionResult},
	})
	b2 := unittest.BlockWithParentFixture(b1.Header)
	s.Extend(&b1)
	s.Extend(&b2)

	// construct `newBlock`
	newBlock := unittest.BlockWithParentFixture(b2.Header)
	seal := s.validSealForResult(&receipt.ExecutionResult)
	seal.AggregatedApprovalSigs = seal.AggregatedApprovalSigs[1:]
	newBlock.SetPayload(flow.Payload{
		Seals: []*flow.Seal{seal},
	})

	// we want to make sure that the emergency-seal metric is not called because
	// requiredApprovalsForSealVerification is > 0.
	s.metrics.On("EmergencySeal").Run(func(args mock.Arguments) {
		s.T().Errorf("should not count as emmergency sealed, because seal has fewer approvals than required for Seal Verification")
	}).Return()
	_, err := s.sealValidator.Validate(&newBlock)
	s.Require().Error(err)
	s.Require().True(engine.IsInvalidInputError(err))
}

// TestSealEmergencySeal checks that, when requiredApprovalsForSealVerification
// is 0, a seal which has 0 signatures for at least one chunk will be accepted,
// and that the emergency-seal metric will be incremented.
// We test with the following fork:
//   ... <- LatestSealedBlock <- B0 <- B1{ Result[B0], Receipt[B0] } <- B2 <- ░newBlock{ Seal[B0]}░
// The gap of 1 block, i.e. B2, is required to avoid a sealing edge-case
// (see test `TestSeal_EnforceGap` for more details)
func (s *SealValidationSuite) TestSealEmergencySeal() {
	// Notes:
	//  * the result for `LatestSealedBlock` is `LatestExecutionResult` (already initialized in test setup)
	//  * as block B0, we use `LatestFinalizedBlock` (already initialized in test setup)
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(unittest.ExecutionResultFixture(
			unittest.WithBlock(s.LatestFinalizedBlock),
			unittest.WithPreviousResult(*s.LatestExecutionResult),
		)),
	)

	// construct block B1 and B2
	b1 := unittest.BlockWithParentFixture(s.LatestFinalizedBlock.Header)
	b1.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{receipt.Meta()},
		Results:  []*flow.ExecutionResult{&receipt.ExecutionResult},
	})
	b2 := unittest.BlockWithParentFixture(b1.Header)
	s.Extend(&b1)
	s.Extend(&b2)

	// requiredApprovalsForSealConstruction = 2
	// receive seal with 2 approvals => _not_ emergency sealed
	newBlock := s.makeBlockSealingResult(b2.Header, &receipt.ExecutionResult, 2)
	metrics := &module.ConsensusMetrics{}
	metrics.On("EmergencySeal").Run(func(args mock.Arguments) {
		s.T().Errorf("happy path sealing should not be counted as emmergency sealed")
	}).Return()
	s.sealValidator.metrics = metrics
	//
	_, err := s.sealValidator.Validate(newBlock)
	s.Require().NoError(err)

	// requiredApprovalsForSealConstruction = 2
	// requiredApprovalsForSealVerification = 1
	// receive seal with 1 approval => emergency sealed
	s.sealValidator.requiredApprovalsForSealVerification = 1
	newBlock = s.makeBlockSealingResult(b2.Header, &receipt.ExecutionResult, 1)
	metrics = &module.ConsensusMetrics{}
	metrics.On("EmergencySeal").Once()
	s.sealValidator.metrics = metrics
	//
	_, err = s.sealValidator.Validate(newBlock)
	s.Require().NoError(err)
	metrics.AssertExpectations(s.T())

	// requiredApprovalsForSealConstruction = 2
	// requiredApprovalsForSealVerification = 1
	// receive seal with 0 approval => invalid
	s.sealValidator.requiredApprovalsForSealVerification = 1
	newBlock = s.makeBlockSealingResult(b2.Header, &receipt.ExecutionResult, 0)
	metrics = &module.ConsensusMetrics{}
	metrics.On("EmergencySeal").Run(func(args mock.Arguments) {
		s.T().Errorf("invaid seal should not be counted as emmergency sealed")
	}).Return()
	s.sealValidator.metrics = metrics
	//
	_, err = s.sealValidator.Validate(newBlock)
	s.Require().Error(err)
	s.Require().True(engine.IsInvalidInputError(err))

	// requiredApprovalsForSealConstruction = 2
	// requiredApprovalsForSealVerification = 0
	// receive seal with 1 approval => emergency sealed
	s.sealValidator.requiredApprovalsForSealVerification = 0
	newBlock = s.makeBlockSealingResult(b2.Header, &receipt.ExecutionResult, 1)
	metrics = &module.ConsensusMetrics{}
	metrics.On("EmergencySeal").Once()
	s.sealValidator.metrics = metrics
	//
	_, err = s.sealValidator.Validate(newBlock)
	s.Require().NoError(err)
	metrics.AssertExpectations(s.T())

	// requiredApprovalsForSealConstruction = 2
	// requiredApprovalsForSealVerification = 0
	// receive seal with 0 approval => emergency sealed
	s.sealValidator.requiredApprovalsForSealVerification = 0
	newBlock = s.makeBlockSealingResult(b2.Header, &receipt.ExecutionResult, 0)
	metrics = &module.ConsensusMetrics{}
	metrics.On("EmergencySeal").Once()
	s.sealValidator.metrics = metrics
	//
	_, err = s.sealValidator.Validate(newBlock)
	s.Require().NoError(err)
	metrics.AssertExpectations(s.T())
}

// TestSealInvalidChunkSignersCount tests that we reject seal with invalid approval signatures for
// submitted seal
func (s *SealValidationSuite) TestSealInvalidChunkSignersCount() {
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(unittest.ExecutionResultFixture(
			unittest.WithBlock(s.LatestFinalizedBlock),
			unittest.WithPreviousResult(*s.LatestExecutionResult),
		)),
	)

	// construct block B1 and B2
	b1 := unittest.BlockWithParentFixture(s.LatestFinalizedBlock.Header)
	b1.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{receipt.Meta()},
		Results:  []*flow.ExecutionResult{&receipt.ExecutionResult},
	})
	b2 := unittest.BlockWithParentFixture(b1.Header)
	s.Extend(&b1)
	s.Extend(&b2)

	newBlock := unittest.BlockWithParentFixture(b2.Header)
	seal := s.validSealForResult(&receipt.ExecutionResult)
	seal.AggregatedApprovalSigs[0].SignerIDs = seal.AggregatedApprovalSigs[0].SignerIDs[1:]
	newBlock.SetPayload(flow.Payload{
		Seals: []*flow.Seal{seal},
	})

	_, err := s.sealValidator.Validate(&newBlock)
	s.Require().Error(err)
	s.Require().True(engine.IsInvalidInputError(err))
}

// TestSealInvalidChunkSignaturesCount tests that we reject seal with invalid approval signatures for
// submitted seal
func (s *SealValidationSuite) TestSealInvalidChunkSignaturesCount() {
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(unittest.ExecutionResultFixture(
			unittest.WithBlock(s.LatestFinalizedBlock),
			unittest.WithPreviousResult(*s.LatestExecutionResult),
		)),
	)

	// construct block B1 and B2
	b1 := unittest.BlockWithParentFixture(s.LatestFinalizedBlock.Header)
	b1.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{receipt.Meta()},
		Results:  []*flow.ExecutionResult{&receipt.ExecutionResult},
	})
	b2 := unittest.BlockWithParentFixture(b1.Header)
	s.Extend(&b1)
	s.Extend(&b2)

	newBlock := unittest.BlockWithParentFixture(b2.Header)
	seal := s.validSealForResult(&receipt.ExecutionResult)
	seal.AggregatedApprovalSigs[0].VerifierSignatures = seal.AggregatedApprovalSigs[0].VerifierSignatures[1:]
	newBlock.SetPayload(flow.Payload{
		Seals: []*flow.Seal{seal},
	})

	_, err := s.sealValidator.Validate(&newBlock)
	s.Require().Error(err)
	s.Require().True(engine.IsInvalidInputError(err))
}

// TestSealDuplicatedApproval verifies that the seal validator rejects an invalid
// seal where approvals are repeated. Otherwise, this would open up an attack vector,
// where the seal contains insufficient approvals when duplicating.
func (s *SealValidationSuite) TestSealDuplicatedApproval() {
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(unittest.ExecutionResultFixture(
			unittest.WithBlock(s.LatestFinalizedBlock),
			unittest.WithPreviousResult(*s.LatestExecutionResult),
		)),
	)

	// construct block B1 and B2
	b1 := unittest.BlockWithParentFixture(s.LatestFinalizedBlock.Header)
	b1.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{receipt.Meta()},
		Results:  []*flow.ExecutionResult{&receipt.ExecutionResult},
	})
	b2 := unittest.BlockWithParentFixture(b1.Header)
	s.Extend(&b1)
	s.Extend(&b2)

	newBlock := unittest.BlockWithParentFixture(b2.Header)
	seal := s.validSealForResult(&receipt.ExecutionResult)
	seal.AggregatedApprovalSigs[0].VerifierSignatures[1] = seal.AggregatedApprovalSigs[0].VerifierSignatures[0]
	seal.AggregatedApprovalSigs[0].SignerIDs[1] = seal.AggregatedApprovalSigs[0].SignerIDs[0]
	newBlock.SetPayload(flow.Payload{
		Seals: []*flow.Seal{seal},
	})

	_, err := s.sealValidator.Validate(&newBlock)
	s.Require().Error(err)
	s.Require().True(engine.IsInvalidInputError(err))
}

// TestSealInvalidChunkAssignment tests that we reject seal with invalid signerID of approval signature for
// submitted seal. We test with the following fork:
//   ... <- LatestSealedBlock <- B0 <- B1{ Result[B0], Receipt[B0] } <- B2 <- ░newBlock{ Seal[B0]}░
// The gap of 1 block, i.e. B2, is required to avoid a sealing edge-case
// (see test `TestSeal_EnforceGap` for more details)
func (s *SealValidationSuite) TestSealInvalidChunkAssignment() {
	// Notes:
	//  * the result for `LatestSealedBlock` is `LatestExecutionResult` (already initialized in test setup)
	//  * as block B0, we use `LatestFinalizedBlock` (already initialized in test setup)
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(unittest.ExecutionResultFixture(
			unittest.WithBlock(s.LatestFinalizedBlock),
			unittest.WithPreviousResult(*s.LatestExecutionResult),
		)),
	)

	// construct block B1 and B2
	b1 := unittest.BlockWithParentFixture(s.LatestFinalizedBlock.Header)
	b1.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{receipt.Meta()},
		Results:  []*flow.ExecutionResult{&receipt.ExecutionResult},
	})
	b2 := unittest.BlockWithParentFixture(b1.Header)
	s.Extend(&b1)
	s.Extend(&b2)

	newBlock := unittest.BlockWithParentFixture(b2.Header)
	seal := s.validSealForResult(&receipt.ExecutionResult)
	seal.AggregatedApprovalSigs[0].SignerIDs[0] = unittest.IdentifierFixture()
	newBlock.SetPayload(flow.Payload{
		Seals: []*flow.Seal{seal},
	})

	_, err := s.sealValidator.Validate(&newBlock)
	s.Require().Error(err)
	s.Require().True(engine.IsInvalidInputError(err))
}

// TestHighestSeal tests that Validate will pick the seal corresponding to the
// highest block when the payload contains multiple seals that are not ordered.
// We test with the following known fork:
//    ... <- B1 <- B2 <- B3{Receipt(B2), Result(B2)} <- B4{Receipt(B3), Result(B3)} <- B5
// with
//  * B1 is the latest sealed block: we use s.LatestSealedBlock,
//    which has the result s.LatestExecutionResult
//  * B2 is the latest finalized block: we use s.LatestFinalizedBlock
// Now we consider the new candidate block B6:
//    ... <- B5 <-B6{ SealResult(B3), SealResult(B2) }
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

	// block B5
	block5 := unittest.BlockWithParentFixture(block4.Header)
	s.Extend(&block5)

	// construct block B5 and its payload components, i.e. seals for block B2 and B3
	seal2 := s.validSealForResult(&block2Receipt.ExecutionResult)
	seal3 := s.validSealForResult(&block3Receipt.ExecutionResult)
	block6 := unittest.BlockWithParentFixture(block5.Header)
	block6.SetPayload(flow.Payload{
		// placing seals in the reversed order to test
		// Extend will pick the highest sealed block
		Seals: []*flow.Seal{seal3, seal2},
	})

	last, err := s.sealValidator.Validate(&block6)
	require.NoError(s.T(), err)
	require.Equal(s.T(), last.FinalState, seal3.FinalState)
}

// TestValidatePayload_SealsSkipBlock verifies that proposed seals
// are rejected if the chain of proposed seals skips a block.
// We test with the following known fork:
//    S  <- B0 <- B1 <- B2 <- B3{R(B0), R(B1), R(B2)} <- B4
// where S is the latest sealed block.
// The gap of 1 block, i.e. B2, is required to avoid a sealing edge-case
// (see test `TestSeal_EnforceGap` for more details)
//
// Now we consider the new candidate block X:
//    S  <- B0 <- B1 <- B2 <- B3{R(B0), R(B1), R(B2)} <- B4 <-X
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
	b4 := unittest.BlockWithParentFixture(blocks[3].Header)
	blocks = append(blocks, &b4)

	for _, b := range blocks {
		s.Extend(b)
	}

	block0Seal := s.validSealForResult(&receipts[0].ExecutionResult)
	block1Seal := s.validSealForResult(&receipts[1].ExecutionResult)
	block2Seal := s.validSealForResult(&receipts[2].ExecutionResult)

	// S  <- B0 <- B1 <- B2 <- B3{R(B0), R(B1), R(B2)} <- B4 <- X{Seal(R(B1))}
	s.T().Run("no seal for the immediately next unsealed block", func(t *testing.T) {
		X := unittest.BlockWithParentFixture(b4.Header)
		X.SetPayload(flow.Payload{
			Seals: []*flow.Seal{block1Seal},
		})

		_, err := s.sealValidator.Validate(&X)
		require.Error(s.T(), err)
		require.True(s.T(), engine.IsInvalidInputError(err), err)
	})

	// S  <- B0 <- B1 <- B2 <- B3{R(B0), R(B1), R(B2)} <- B4 <- X{Seal(R(B0)), Seal(R(B2))}
	s.T().Run("seals skip one of the following blocks", func(t *testing.T) {
		X := unittest.BlockWithParentFixture(b4.Header)
		X.SetPayload(flow.Payload{
			Seals: []*flow.Seal{block0Seal, block2Seal},
		})

		_, err := s.sealValidator.Validate(&X)
		require.Error(s.T(), err)
		require.True(s.T(), engine.IsInvalidInputError(err), err)
	})

	// S  <- B0 <- B1 <- B2 <- B3{R(B0), R(B1), R(B2)} <- B4 <- X{Seal(R(B0)), Seal(R(B1)), Seal(R(B2))}
	s.T().Run("valid test case", func(t *testing.T) {
		X := unittest.BlockWithParentFixture(b4.Header)
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

// TestSealValid tests that a candidate block with a valid seal passes validation.
// We test with the following fork:
//   ... <- LatestSealedBlock <- B0 <- B1{ Result[B0], Receipt[B0] } <- B2 <- ...
// The gap of 1 block, i.e. B2, is required to avoid a sealing edge-case
// (see test `TestSeal_EnforceGap` for more details)
func (s *SealValidationSuite) TestExtendSealDuplicate() {
	// Notes:
	//  * the result for `LatestSealedBlock` is `LatestExecutionResult` (already initialized in test setup)
	//  * as block B0, we use `LatestFinalizedBlock` (already initialized in test setup)
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(unittest.ExecutionResultFixture(
			unittest.WithBlock(s.LatestFinalizedBlock),
			unittest.WithPreviousResult(*s.LatestExecutionResult),
		)),
	)

	// construct block B1 and B2
	b1 := unittest.BlockWithParentFixture(s.LatestFinalizedBlock.Header)
	b1.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{receipt.Meta()},
		Results:  []*flow.ExecutionResult{&receipt.ExecutionResult},
	})
	b2 := unittest.BlockWithParentFixture(b1.Header)
	s.Extend(&b1)
	s.Extend(&b2)

	sealB1 := s.validSealForResult(&receipt.ExecutionResult) // seal for block B1

	// <- LatestSealedBlock <- B0 <- B1{ Result[B0], Receipt[B0] } <- B2 <- B3{S(R(B1))} <- B4{S(R(B1))}
	s.T().Run("Duplicate seal in separate block", func(t *testing.T) {
		// insert B3 with a seal for block1
		b3 := unittest.BlockWithParentFixture(b2.Header)
		b3.SetPayload(flow.Payload{
			Seals: []*flow.Seal{sealB1},
		})
		s.Extend(&b3)

		// insert B4 with a duplicate seal
		b4 := unittest.BlockWithParentFixture(b3.Header)
		b4.SetPayload(flow.Payload{
			Seals: []*flow.Seal{sealB1},
		})

		// we expect an error because block 4 contains a seal that is
		// already contained in another block on the fork
		_, err := s.sealValidator.Validate(&b4)
		require.Error(t, err)
		require.True(t, engine.IsInvalidInputError(err), err)
	})

	// <- LatestSealedBlock <- B0 <- B1{ Result[B0], Receipt[B0] } <- B2 <- B3{S(R(B1)), S(R(B1))}
	s.T().Run("Duplicate seal in same payload", func(t *testing.T) {
		// insert block3 with 2 identical seals for block1
		block3 := unittest.BlockWithParentFixture(b2.Header)
		block3.SetPayload(flow.Payload{
			Seals: []*flow.Seal{sealB1, sealB1},
		})

		_, err := s.sealValidator.Validate(&block3)

		// we expect an error because block 3 contains duplicate seals within its payload
		require.Error(t, err)
		require.True(t, engine.IsInvalidInputError(err), err)
	})
}

// TestExtendSeal_NoIncorporatedResult tests that seals are rejected if _no_ for the sealed block
// was incorporated in blocks. We test with the following fork:
//   ... <- LatestSealedBlock <- B0 <- B1 <- B2 <- ░newBlock{ Seal[B0]}░
// The gap of 1 block, i.e. B2, is required to avoid a sealing edge-case
// (see test `TestSeal_EnforceGap` for more details)
func (s *SealValidationSuite) TestExtendSeal_NoIncorporatedResult() {
	// Notes:
	//  * the result for `LatestSealedBlock` is `LatestExecutionResult` (already initialized in test setup)
	//  * as block B0, we use `LatestFinalizedBlock` (already initialized in test setup)
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(unittest.ExecutionResultFixture(
			unittest.WithBlock(s.LatestFinalizedBlock),
			unittest.WithPreviousResult(*s.LatestExecutionResult),
		)),
	)

	// construct block B1 and B2
	b1 := unittest.BlockWithParentFixture(s.LatestFinalizedBlock.Header)
	b2 := unittest.BlockWithParentFixture(b1.Header)
	s.Extend(&b1)
	s.Extend(&b2)

	// construct `newBlock`
	newBlock := unittest.BlockWithParentFixture(b2.Header)
	seal := unittest.Seal.Fixture(unittest.Seal.WithResult(&receipt.ExecutionResult))
	newBlock.SetPayload(flow.Payload{
		Seals: []*flow.Seal{seal},
	})

	// we expect an error because there is no block on the fork that
	// contains a receipt committing to block1
	_, err := s.sealValidator.Validate(&newBlock)
	s.Require().Error(err)
	s.Require().True(engine.IsInvalidInputError(err), err)
}

// TestExtendSeal_DifferentIncorporatedResult tests that seals are rejected if a result is sealed that is
// different than the one incorporated in this fork. We test with the following fork:
//   ... <- LatestSealedBlock <- B0 <- B1 {ER0a} <- B2 <- ░newBlock{ Seal[ER0b]}░
// The gap of 1 block, i.e. B2, is required to avoid a sealing edge-case
// (see test `TestSeal_EnforceGap` for more details)
func (s *SealValidationSuite) TestExtendSeal_DifferentIncorporatedResult() {
	// Notes:
	//  * the result for `LatestSealedBlock` is `LatestExecutionResult` (already initialized in test setup)
	//  * as block B0, we use `LatestFinalizedBlock` (already initialized in test setup)
	incorporatedReceipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(unittest.ExecutionResultFixture(
			unittest.WithBlock(s.LatestFinalizedBlock),
			unittest.WithPreviousResult(*s.LatestExecutionResult),
		)),
	)
	differentResult := unittest.ExecutionResultFixture(
		unittest.WithBlock(s.LatestFinalizedBlock),
		unittest.WithPreviousResult(*s.LatestExecutionResult),
	)

	// construct block B1 and B2
	b1 := unittest.BlockWithParentFixture(s.LatestFinalizedBlock.Header)
	b1.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{incorporatedReceipt.Meta()},
		Results:  []*flow.ExecutionResult{&incorporatedReceipt.ExecutionResult},
	})
	b2 := unittest.BlockWithParentFixture(b1.Header)
	s.Extend(&b1)
	s.Extend(&b2)

	// construct `newBlock`
	seal := unittest.Seal.Fixture(unittest.Seal.WithResult(differentResult))
	newBlock := unittest.BlockWithParentFixture(b2.Header)
	newBlock.SetPayload(flow.Payload{
		Seals: []*flow.Seal{seal},
	})

	// Should fail because ER0a is different than ER0b, although they
	// reference the same block. Technically the fork does not contain an
	// IncorporatedResult for the result referenced by the proposed seal.
	_, err := s.sealValidator.Validate(&newBlock)
	s.Require().Error(err)
	s.Require().True(engine.IsInvalidInputError(err), err)
}

// TestExtendSeal_ResultIncorporatedOnDifferentFork tests that seals are rejected if they correspond
// to ExecutionResults that are incorporated in a different fork
// We test with the following fork:
//   ... <- LatestSealedBlock <- B0 <- B1 <- B2 <- ░newBlock{ Seal[ER0]}░
//                                 └-- A1{ ER0 }
// The gap of 1 block, i.e. B2, is required to avoid a sealing edge-case
// (see test `TestSeal_EnforceGap` for more details)
func (s *SealValidationSuite) TestExtendSeal_ResultIncorporatedOnDifferentFork() {
	// Notes:
	//  * the result for `LatestSealedBlock` is `LatestExecutionResult` (already initialized in test setup)
	//  * as block B0, we use `LatestFinalizedBlock` (already initialized in test setup)
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(unittest.ExecutionResultFixture(
			unittest.WithBlock(s.LatestFinalizedBlock),
			unittest.WithPreviousResult(*s.LatestExecutionResult),
		)),
	)

	// construct block A1
	a1 := unittest.BlockWithParentFixture(s.LatestFinalizedBlock.Header)
	a1.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{receipt.Meta()},
		Results:  []*flow.ExecutionResult{&receipt.ExecutionResult},
	})
	s.Extend(&a1)

	// construct block B1 and B2
	b1 := unittest.BlockWithParentFixture(s.LatestFinalizedBlock.Header)
	b2 := unittest.BlockWithParentFixture(b1.Header)
	s.Extend(&b1)
	s.Extend(&b2)

	// construct `newBlock`
	seal := s.validSealForResult(&receipt.ExecutionResult)
	newBlock := unittest.BlockWithParentFixture(b2.Header)
	newBlock.SetPayload(flow.Payload{
		Seals: []*flow.Seal{seal},
	})

	// we expect an error because there is no block on the fork that
	// contains a receipt committing to the seal's result
	_, err := s.sealValidator.Validate(&newBlock)
	s.Require().Error(err)
	s.Require().True(engine.IsInvalidInputError(err), err)
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
				s.Identities[aggregatedSigs.SignerIDs[i]].StakingPubKey).Return(true, nil).Maybe()
		}
	}
	return seal
}

// makeBlockSealingResult constructs a child block of `parentBlock`, whose payload includes
// a seal for `sealedResult`. For each chunk, the seal has aggregated approval signatures from
// `numberApprovals` assigned verification Nodes.
// Note: numberApprovals cannot be larger than the number of assigned verification nodes.
func (s *SealValidationSuite) makeBlockSealingResult(parentBlock *flow.Header, sealedResult *flow.ExecutionResult, numberApprovals int) *flow.Block {
	seal := s.validSealForResult(sealedResult)
	for chunkIndex := 0; chunkIndex < len(seal.AggregatedApprovalSigs); chunkIndex++ {
		seal.AggregatedApprovalSigs[chunkIndex].SignerIDs = seal.AggregatedApprovalSigs[chunkIndex].SignerIDs[:numberApprovals]
		seal.AggregatedApprovalSigs[chunkIndex].VerifierSignatures = seal.AggregatedApprovalSigs[chunkIndex].VerifierSignatures[:numberApprovals]
	}

	sealingBlock := unittest.BlockWithParentFixture(parentBlock)
	sealingBlock.SetPayload(flow.Payload{
		Seals: []*flow.Seal{seal},
	})
	return &sealingBlock
}
