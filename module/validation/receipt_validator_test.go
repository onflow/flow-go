package validation

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	mock2 "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestReceiptValidator(t *testing.T) {
	suite.Run(t, new(ReceiptValidationSuite))
}

type ReceiptValidationSuite struct {
	unittest.BaseChainSuite

	receiptValidator module.ReceiptValidator
	verifier         *mock2.Verifier
}

func (s *ReceiptValidationSuite) SetupTest() {
	s.SetupChain()
	s.verifier = &mock2.Verifier{}
	s.receiptValidator = NewReceiptValidator(s.State, s.HeadersDB, s.IndexDB, s.ResultsDB, s.SealsDB, s.verifier)
}

// TestReceiptValid try submitting valid receipt
func (s *ReceiptValidationSuite) TestReceiptValid() {
	executor := s.Identities[s.ExeID]
	valSubgrph := s.ValidSubgraphFixture()
	receipt := unittest.ExecutionReceiptFixture(unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(valSubgrph.Result))
	s.AddSubgraphFixtureToMempools(valSubgrph)

	receiptID := receipt.ID()
	s.verifier.On("Verify",
		receiptID[:],
		receipt.ExecutorSignature,
		executor.StakingPubKey).Return(true, nil).Once()

	err := s.receiptValidator.Validate(receipt)
	s.Require().NoError(err, "should successfully validate receipt")
	s.verifier.AssertExpectations(s.T())
}

// TestReceiptNoIdentity tests that we reject receipt with invalid `ExecutionResult.ExecutorID`
func (s *ReceiptValidationSuite) TestReceiptNoIdentity() {
	valSubgrph := s.ValidSubgraphFixture()
	receipt := unittest.ExecutionReceiptFixture(unittest.WithExecutorID(unittest.IdentityFixture().NodeID),
		unittest.WithResult(valSubgrph.Result))
	s.AddSubgraphFixtureToMempools(valSubgrph)

	err := s.receiptValidator.Validate(receipt)
	s.Require().Error(err, "should reject invalid identity")
	s.Assert().True(engine.IsInvalidInputError(err))
}

// TestReceiptInvalidStake tests that we reject receipt with invalid stake
func (s *ReceiptValidationSuite) TestReceiptInvalidStake() {
	valSubgrph := s.ValidSubgraphFixture()
	receipt := unittest.ExecutionReceiptFixture(unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(valSubgrph.Result))
	s.AddSubgraphFixtureToMempools(valSubgrph)

	s.verifier.On("Verify",
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(true, nil).Maybe() // call optional, as validator might check stake first

	// replace stake with invalid one
	s.Identities[s.ExeID].Stake = 0

	err := s.receiptValidator.Validate(receipt)
	s.Require().Error(err, "should reject invalid stake")
	s.Assert().True(engine.IsInvalidInputError(err))
}

// TestReceiptInvalidRole tests that we reject receipt with invalid execution node role
func (s *ReceiptValidationSuite) TestReceiptInvalidRole() {
	valSubgrph := s.ValidSubgraphFixture()
	receipt := unittest.ExecutionReceiptFixture(unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(valSubgrph.Result))
	s.AddSubgraphFixtureToMempools(valSubgrph)

	s.verifier.On("Verify",
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(true, nil).Maybe() // call optional, as validator might check stake first

	// replace identity with invalid one
	s.Identities[s.ExeID] = unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))

	err := s.receiptValidator.Validate(receipt)
	s.Require().Error(err, "should reject invalid identity")
	s.Assert().True(engine.IsInvalidInputError(err))
}

// TestReceiptInvalidSignature tests that we reject receipt with invalid signature
func (s *ReceiptValidationSuite) TestReceiptInvalidSignature() {
	executor := s.Identities[s.ExeID]

	valSubgrph := s.ValidSubgraphFixture()
	receipt := unittest.ExecutionReceiptFixture(unittest.WithExecutorID(executor.NodeID),
		unittest.WithResult(valSubgrph.Result))
	s.AddSubgraphFixtureToMempools(valSubgrph)

	s.verifier.On("Verify",
		mock.Anything,
		mock.Anything,
		executor.StakingPubKey).Return(false, nil).Once()

	err := s.receiptValidator.Validate(receipt)
	s.Require().Error(err, "should reject invalid signature")
	s.Assert().True(engine.IsInvalidInputError(err))
	s.verifier.AssertExpectations(s.T())
}

// TestReceiptTooFewChunks tests that we reject receipt with invalid chunk count
func (s *ReceiptValidationSuite) TestReceiptTooFewChunks() {
	valSubgrph := s.ValidSubgraphFixture()
	chunks := valSubgrph.Result.Chunks
	valSubgrph.Result.Chunks = chunks[0 : len(chunks)-2] // drop the last chunk
	receipt := unittest.ExecutionReceiptFixture(unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(valSubgrph.Result))
	s.AddSubgraphFixtureToMempools(valSubgrph)

	s.verifier.On("Verify",
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(true, nil).Maybe()

	err := s.receiptValidator.Validate(receipt)
	s.Require().Error(err, "should reject with invalid chunks")
	s.Assert().True(engine.IsInvalidInputError(err))
}

// TestReceiptTooManyChunks tests that we reject receipt with more chunks than expected
func (s *ReceiptValidationSuite) TestReceiptTooManyChunks() {
	valSubgrph := s.ValidSubgraphFixture()
	chunks := valSubgrph.Result.Chunks
	valSubgrph.Result.Chunks = append(chunks, chunks[len(chunks)-1]) // duplicate the last chunk
	receipt := unittest.ExecutionReceiptFixture(unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(valSubgrph.Result))
	s.AddSubgraphFixtureToMempools(valSubgrph)

	s.verifier.On("Verify",
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(true, nil).Maybe()

	err := s.receiptValidator.Validate(receipt)
	s.Require().Error(err, "should reject with invalid chunks")
	s.Assert().True(engine.IsInvalidInputError(err))
}

// TestReceiptChunkInvalidBlockID tests that we reject receipt with invalid chunk blockID
func (s *ReceiptValidationSuite) TestReceiptChunkInvalidBlockID() {
	valSubgrph := s.ValidSubgraphFixture()
	valSubgrph.Result.Chunks[0].BlockID = unittest.IdentifierFixture()
	receipt := unittest.ExecutionReceiptFixture(unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(valSubgrph.Result))
	s.AddSubgraphFixtureToMempools(valSubgrph)

	s.verifier.On("Verify",
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(true, nil).Maybe()

	err := s.receiptValidator.Validate(receipt)
	s.Require().Error(err, "should reject with invalid chunks")
	s.Assert().True(engine.IsInvalidInputError(err))
}

// TestReceiptInvalidCollectionIndex tests that we reject receipt with invalid chunk collection index
func (s *ReceiptValidationSuite) TestReceiptInvalidCollectionIndex() {
	valSubgrph := s.ValidSubgraphFixture()
	valSubgrph.Result.Chunks[0].CollectionIndex = 42
	receipt := unittest.ExecutionReceiptFixture(unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(valSubgrph.Result))
	s.AddSubgraphFixtureToMempools(valSubgrph)

	s.verifier.On("Verify",
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(true, nil).Maybe()

	err := s.receiptValidator.Validate(receipt)
	s.Require().Error(err, "should reject invalid collection index")
	s.Assert().True(engine.IsInvalidInputError(err))
}

// TestReceiptNoPreviousResult tests that we reject receipt with missing previous result
func (s *ReceiptValidationSuite) TestReceiptNoPreviousResult() {
	valSubgrph := s.ValidSubgraphFixture()
	// invalidate prev execution result, it will result in failing to lookup
	// prev result during sub-graph check
	valSubgrph.PreviousResult = unittest.ExecutionResultFixture()
	receipt := unittest.ExecutionReceiptFixture(unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(valSubgrph.Result))
	s.AddSubgraphFixtureToMempools(valSubgrph)

	s.verifier.On("Verify",
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(true, nil).Maybe()

	err := s.receiptValidator.Validate(receipt)
	s.Require().Error(err, "should reject invalid receipt")
	s.Assert().True(engine.IsUnverifiableInputError(err), err)
}

// TestReceiptInvalidPreviousResult tests that we reject receipt with invalid previous result
func (s *ReceiptValidationSuite) TestReceiptInvalidPreviousResult() {
	valSubgrph := s.ValidSubgraphFixture()
	// invalidate prev execution result blockID, this should fail because
	// prev result points to wrong block
	valSubgrph.PreviousResult.BlockID = unittest.IdentifierFixture()
	receipt := unittest.ExecutionReceiptFixture(unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(valSubgrph.Result))
	s.AddSubgraphFixtureToMempools(valSubgrph)

	s.verifier.On("Verify",
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(true, nil).Maybe()

	err := s.receiptValidator.Validate(receipt)
	s.Require().Error(err, "should reject invalid previous result")
	s.Assert().True(engine.IsInvalidInputError(err), err)
}

// TestReceiptInvalidResultChain tests that we reject receipts,
// where the start state does not match the parent result's end state
func (s *ReceiptValidationSuite) TestReceiptInvalidResultChain() {
	valSubgrph := s.ValidSubgraphFixture()
	// invalidate prev execution result blockID, this should fail because
	// prev result points to wrong block
	valSubgrph.PreviousResult.Chunks[len(valSubgrph.Result.Chunks)-1].EndState = unittest.StateCommitmentFixture()
	receipt := unittest.ExecutionReceiptFixture(unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(valSubgrph.Result))
	s.AddSubgraphFixtureToMempools(valSubgrph)

	s.verifier.On("Verify",
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(true, nil).Maybe()

	err := s.receiptValidator.Validate(receipt)
	s.Require().Error(err, "should reject invalid previous result")
	s.Assert().True(engine.IsInvalidInputError(err), err)
}

// TestMultiReceiptValidResultChain tests that multiple within one block payload
// are accepted, where the receipts are building on top of each other
// (i.e. their results form a chain).
// Say B(A) means block B has receipt for A:
// * we have such chain in storage: G <- A <- B(A) <- C
// * if a child block of C payload contains receipts and results for (B,C)
//   it should be accepted as valid
func (s *ReceiptValidationSuite) TestMultiReceiptValidResultChain() {
	// assuming signatures are all good
	s.verifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	// G <- A <- B <- C
	blocks, result0, seal := unittest.ChainFixture(4)
	s.SealsIndex[blocks[0].ID()] = seal

	receipts := unittest.ReceiptChainFor(blocks, result0)
	blockA, blockB, blockC := blocks[1], blocks[2], blocks[3]
	receiptA, receiptB, receiptC := receipts[1], receipts[2], receipts[3]

	blockA.Payload.Receipts = []*flow.ExecutionReceiptMeta{}
	blockB.Payload.Receipts = []*flow.ExecutionReceiptMeta{receiptA.Meta()}
	blockB.Payload.Results = []*flow.ExecutionResult{&receiptA.ExecutionResult}
	blockC.Payload.Receipts = []*flow.ExecutionReceiptMeta{}
	// update block header so that blocks are chained together
	unittest.ReconnectBlocksAndReceipts(blocks, receipts)
	// assuming all receipts are executed by the correct executor
	for _, r := range receipts {
		r.ExecutorID = s.ExeID
	}

	for _, b := range blocks {
		s.Extend(b)
	}
	s.PersistedResults[result0.ID()] = result0

	candidate := unittest.BlockWithParentFixture(blockC.Header)
	candidate.Payload = &flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{receiptB.Meta(), receiptC.Meta()},
		Results:  []*flow.ExecutionResult{&receiptB.ExecutionResult, &receiptC.ExecutionResult},
	}

	err := s.receiptValidator.ValidatePayload(&candidate)
	s.Require().NoError(err)
}

// we have such chain in storage: G <- A <- B(A) <- C
// if a block payload contains (C,B_bad), they should be invalid
func (s *ReceiptValidationSuite) TestMultiReceiptInvalidParent() {
	// assuming signatures are all good
	s.verifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	// G <- A <- B <- C
	blocks, result0, _ := unittest.ChainFixture(4)

	receipts := unittest.ReceiptChainFor(blocks, result0)
	blockA, blockB, blockC := blocks[1], blocks[2], blocks[3]
	receiptA := receipts[1]
	receiptBInvalid := receipts[2]
	receiptC := receipts[3]
	blockA.Payload.Receipts = []*flow.ExecutionReceiptMeta{}
	blockB.Payload.Receipts = []*flow.ExecutionReceiptMeta{receiptA.Meta()}
	blockB.Payload.Results = []*flow.ExecutionResult{&receiptA.ExecutionResult}
	blockC.Payload.Receipts = []*flow.ExecutionReceiptMeta{}
	// update block header so that blocks are chained together
	unittest.ReconnectBlocksAndReceipts(blocks, receipts)
	// assuming all receipts are executed by the correct executor
	for _, r := range receipts {
		r.ExecutorID = s.ExeID
	}

	for _, b := range blocks {
		s.Extend(b)
	}
	s.PersistedResults[result0.ID()] = result0

	candidate := unittest.BlockWithParentFixture(blockC.Header)
	candidate.Payload = &flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{receiptBInvalid.Meta(), receiptC.Meta()},
		Results:  []*flow.ExecutionResult{&receiptBInvalid.ExecutionResult, &receiptC.ExecutionResult},
	}

	// make receipt B as bad
	receiptBInvalid.ExecutorID = s.VerID
	// receiptB and receiptC
	err := s.receiptValidator.ValidatePayload(&candidate)
	s.Require().Error(err)
	require.True(s.T(), engine.IsInvalidInputError(err), err)
}

// Test that `ValidatePayload` will refuse payloads that contain receipts for blocks that
// are already sealed on the fork, but will accept receipts for blocks that are
// sealed on another fork.
func (s *ReceiptValidationSuite) TestValidationReceiptsForSealedBlock() {
	// assuming signatures are all good
	s.verifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	// create block2
	block2 := unittest.BlockWithParentFixture(s.LatestSealedBlock.Header)
	block2.SetPayload(flow.Payload{})
	s.Extend(&block2)

	block2Receipt := unittest.ExecutionReceiptFixture(unittest.WithResult(
		unittest.ExecutionResultFixture(unittest.WithBlock(&block2),
			unittest.WithPreviousResult(*s.LatestExecutionResult))))

	// B1<--B2<--B3{R{B2)}<--B4{S(R(B2))}<--B5{R'(B2)}

	// create block3 with a receipt for block2
	block3 := unittest.BlockWithParentFixture(block2.Header)
	block3.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{block2Receipt.Meta()},
		Results:  []*flow.ExecutionResult{&block2Receipt.ExecutionResult},
	})
	s.Extend(&block3)

	// create a seal for block2
	seal2 := unittest.Seal.Fixture(unittest.Seal.WithResult(&block2Receipt.ExecutionResult))

	// create block4 containing a seal for block2
	block4 := unittest.BlockWithParentFixture(block3.Header)
	block4.SetPayload(flow.Payload{
		Seals: []*flow.Seal{seal2},
	})
	s.Extend(&block4)

	// insert another receipt for block 2, which is now the highest sealed
	// block, and ensure that the receipt is rejected
	receipt := unittest.ExecutionReceiptFixture(unittest.WithResult(
		unittest.ExecutionResultFixture(unittest.WithBlock(&block2),
			unittest.WithPreviousResult(*s.LatestExecutionResult))),
		unittest.WithExecutorID(s.ExeID))
	block5 := unittest.BlockWithParentFixture(block4.Header)
	block5.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{receipt.Meta()},
		Results:  []*flow.ExecutionResult{&receipt.ExecutionResult},
	})

	err := s.receiptValidator.ValidatePayload(&block5)
	require.Error(s.T(), err)
	require.True(s.T(), engine.IsInvalidInputError(err), err)

	// B1<--B2<--B3{R{B2)}<--B4{S(R(B2))}<--B5{R'(B2)}
	//       |
	//       +---B6{R''(B2)}

	// insert another receipt for B2 but in a separate fork. The fact that
	// B2 is sealed on a separate fork should not cause the receipt to be
	// rejected
	block6 := unittest.BlockWithParentFixture(block2.Header)
	block6.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{receipt.Meta()},
		Results:  []*flow.ExecutionResult{&receipt.ExecutionResult},
	})
	err = s.receiptValidator.ValidatePayload(&block6)
	require.NoError(s.T(), err)
}

// Test that validator will accept payloads with receipts that are referring execution results
// which were incorporated in previous blocks of fork.
func (s *ReceiptValidationSuite) TestValidationReceiptForIncorporatedResult() {
	// assuming signatures are all good
	s.verifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	// create block2
	block2 := unittest.BlockWithParentFixture(s.LatestSealedBlock.Header)
	block2.SetPayload(flow.Payload{})
	s.Extend(&block2)

	executionResult := unittest.ExecutionResultFixture(unittest.WithBlock(&block2),
		unittest.WithPreviousResult(*s.LatestExecutionResult))
	firstReceipt := unittest.ExecutionReceiptFixture(
		unittest.WithResult(executionResult),
		unittest.WithExecutorID(s.ExeID))

	// B1<--B2<--B3{R{B2)}<--B4{(R'(B2))}

	// create block3 with a receipt for block2
	block3 := unittest.BlockWithParentFixture(block2.Header)
	block3.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{firstReceipt.Meta()},
		Results:  []*flow.ExecutionResult{&firstReceipt.ExecutionResult},
	})
	s.Extend(&block3)

	exe := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	s.Identities[exe.NodeID] = exe

	// insert another receipt for block 2, it's a receipt from another execution node
	// for the same result
	secondReceipt := unittest.ExecutionReceiptFixture(
		unittest.WithResult(executionResult),
		unittest.WithExecutorID(exe.NodeID))
	block5 := unittest.BlockWithParentFixture(block3.Header)
	block5.SetPayload(flow.Payload{
		// no results, only receipt
		Receipts: []*flow.ExecutionReceiptMeta{secondReceipt.Meta()},
	})

	err := s.receiptValidator.ValidatePayload(&block5)
	require.NoError(s.T(), err)
}

// TestValidationReceiptWithoutIncorporatedResult verifies that receipts must commit
// to results that are included in the respective fork. Specifically, we test that
// the counter-example is rejected:
//  * we have the chain in storage: G <- A <- B
//                                        ^- C(Result[A], ReceiptMeta[A])
//    here, block C contains the result _and_ the receipt Meta-data for block A
//  * now receive the new block X: G <- A <- B <- X(ReceiptMeta[A])
//    Note that X only contains the receipt for A, but _not_ the result.
// Block X must be considered invalid, because confirming validity of
// ReceiptMeta[A] requires information _not_ included in the fork.
func (s *ReceiptValidationSuite) TestValidationReceiptWithoutIncorporatedResult() {
	// assuming signatures are all good
	s.verifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	// create block A
	blockA := unittest.BlockWithParentFixture(s.LatestSealedBlock.Header) // for block G, we use the LatestSealedBlock
	s.Extend(&blockA)

	// result for A; and receipt for A
	resultA := unittest.ExecutionResultFixture(unittest.WithBlock(&blockA), unittest.WithPreviousResult(*s.LatestExecutionResult))
	receiptA := unittest.ExecutionReceiptFixture(unittest.WithResult(resultA), unittest.WithExecutorID(s.ExeID))

	// create block B and block C
	blockB := unittest.BlockWithParentFixture(blockA.Header)
	blockC := unittest.BlockWithParentFixture(blockA.Header)
	blockC.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{receiptA.Meta()},
		Results:  []*flow.ExecutionResult{resultA},
	})
	s.Extend(&blockB)
	s.Extend(&blockC)

	// create block X:
	blockX := unittest.BlockWithParentFixture(blockB.Header)
	blockX.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{receiptA.Meta()},
	})

	err := s.receiptValidator.ValidatePayload(&blockX)
	require.Error(s.T(), err)
	require.True(s.T(), engine.IsInvalidInputError(err), err)
}

// Test that validator will reject payloads that contain receipts for blocks that
// are not on the fork
//
// B1<--B2<--B3
//      |
//      +----B4{R(B3)}
func (s *ReceiptValidationSuite) TestValidationReceiptsBlockNotOnFork() {
	// create block2
	block2 := unittest.BlockWithParentFixture(s.LatestFinalizedBlock.Header)
	block2.Payload.Guarantees = nil
	block2.Header.PayloadHash = block2.Payload.Hash()
	s.Extend(&block2)

	// create block3
	block3 := unittest.BlockWithParentFixture(block2.Header)
	block3.SetPayload(flow.Payload{})
	s.Extend(&block3)

	block3Receipt := unittest.ReceiptForBlockFixture(&block3)

	block4 := unittest.BlockWithParentFixture(block2.Header)
	block4.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{block3Receipt.Meta()},
		Results:  []*flow.ExecutionResult{&block3Receipt.ExecutionResult},
	})
	err := s.receiptValidator.ValidatePayload(&block4)
	require.Error(s.T(), err)
	require.True(s.T(), engine.IsInvalidInputError(err), err)
}

// Test that Extend will refuse payloads that contain duplicate receipts, where
// duplicates can be in another block on the fork, or within the payload.
func (s *ReceiptValidationSuite) TestExtendReceiptsDuplicate() {

	block2 := unittest.BlockWithParentFixture(s.LatestFinalizedBlock.Header)
	block2.SetPayload(flow.Payload{})
	s.Extend(&block2)

	receipt := unittest.ReceiptForBlockFixture(&block2)

	// B1 <- B2 <- B3{R(B2)} <- B4{R(B2)}
	s.T().Run("duplicate receipt in different block", func(t *testing.T) {
		block3 := unittest.BlockWithParentFixture(block2.Header)
		block3.SetPayload(flow.Payload{
			Receipts: []*flow.ExecutionReceiptMeta{receipt.Meta()},
			Results:  []*flow.ExecutionResult{&receipt.ExecutionResult},
		})
		s.Extend(&block3)

		block4 := unittest.BlockWithParentFixture(block3.Header)
		block4.SetPayload(flow.Payload{
			Receipts: []*flow.ExecutionReceiptMeta{receipt.Meta()},
			Results:  []*flow.ExecutionResult{&receipt.ExecutionResult},
		})
		err := s.receiptValidator.ValidatePayload(&block4)
		require.Error(t, err)
		require.True(t, engine.IsInvalidInputError(err), err)
	})

	// B1 <- B2 <- B3{R(B2), R(B2)}
	s.T().Run("duplicate receipt in same block", func(t *testing.T) {
		block3 := unittest.BlockWithParentFixture(block2.Header)
		block3.SetPayload(flow.Payload{
			Receipts: []*flow.ExecutionReceiptMeta{
				receipt.Meta(),
				receipt.Meta(),
			},
			Results: []*flow.ExecutionResult{
				&receipt.ExecutionResult,
			},
		})
		err := s.receiptValidator.ValidatePayload(&block3)
		require.Error(t, err)
		require.True(t, engine.IsInvalidInputError(err), err)
	})
}
