package validation

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	fmock "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestReceiptValidator(t *testing.T) {
	suite.Run(t, new(ReceiptValidationSuite))
}

type ReceiptValidationSuite struct {
	unittest.BaseChainSuite

	receiptValidator module.ReceiptValidator
	publicKey        *fmock.PublicKey
}

func (s *ReceiptValidationSuite) SetupTest() {
	s.SetupChain()
	s.publicKey = &fmock.PublicKey{}
	s.Identities[s.ExeID].StakingPubKey = s.publicKey
	s.receiptValidator = NewReceiptValidator(s.State, s.HeadersDB, s.IndexDB, s.ResultsDB, s.SealsDB)
}

// TestReceiptValid try submitting valid receipt
func (s *ReceiptValidationSuite) TestReceiptValid() {
	valSubgrph := s.ValidSubgraphFixture()
	receipt := unittest.ExecutionReceiptFixture(unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(valSubgrph.Result))
	s.AddSubgraphFixtureToMempools(valSubgrph)

	receiptID := receipt.ID()
	s.publicKey.On("Verify",
		receipt.ExecutorSignature,
		receiptID[:],
		mock.Anything,
	).Return(true, nil).Once()

	err := s.receiptValidator.Validate(receipt)
	s.Require().NoError(err, "should successfully validate receipt")
	s.publicKey.AssertExpectations(s.T())
}

// TestReceiptNoIdentity tests that we reject receipt with invalid `ExecutionResult.ExecutorID`
func (s *ReceiptValidationSuite) TestReceiptNoIdentity() {
	valSubgrph := s.ValidSubgraphFixture()
	node := unittest.IdentityFixture()
	mockPk := &fmock.PublicKey{}
	node.StakingPubKey = mockPk

	receipt := unittest.ExecutionReceiptFixture(unittest.WithExecutorID(node.NodeID),
		unittest.WithResult(valSubgrph.Result))
	s.AddSubgraphFixtureToMempools(valSubgrph)
	receiptID := receipt.ID()

	mockPk.On("Verify",
		receiptID[:],
		receipt.ExecutorSignature,
		mock.Anything,
	).Return(true, nil).Once()
	err := s.receiptValidator.Validate(receipt)
	s.Require().Error(err, "should reject invalid identity")
	s.Assert().True(engine.IsInvalidInputError(err))
}

// TestReceiptFromZeroWeightNode tests that we reject receipt from node with zero weight
func (s *ReceiptValidationSuite) TestReceiptFromZeroWeightNode() {
	valSubgrph := s.ValidSubgraphFixture()
	receipt := unittest.ExecutionReceiptFixture(unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(valSubgrph.Result))
	s.AddSubgraphFixtureToMempools(valSubgrph)

	s.publicKey.On("Verify",
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(true, nil).Maybe() // call optional, as validator might check weight first

	// replace weight with invalid one
	s.Identities[s.ExeID].Weight = 0

	err := s.receiptValidator.Validate(receipt)
	s.Require().Error(err, "should reject invalid weight")
	s.Assert().True(engine.IsInvalidInputError(err))
}

// TestReceiptInvalidRole tests that we reject receipt with invalid execution node role
func (s *ReceiptValidationSuite) TestReceiptInvalidRole() {
	valSubgrph := s.ValidSubgraphFixture()
	receipt := unittest.ExecutionReceiptFixture(unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(valSubgrph.Result))
	s.AddSubgraphFixtureToMempools(valSubgrph)

	s.publicKey.On("Verify",
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(true, nil).Maybe() // call optional, as validator might check weight first

	// replace identity with invalid one
	s.Identities[s.ExeID] = unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))

	err := s.receiptValidator.Validate(receipt)
	s.Require().Error(err, "should reject invalid identity")
	s.Assert().True(engine.IsInvalidInputError(err))
}

// TestReceiptInvalidSignature tests that we reject receipt with invalid signature
func (s *ReceiptValidationSuite) TestReceiptInvalidSignature() {

	valSubgrph := s.ValidSubgraphFixture()
	receipt := unittest.ExecutionReceiptFixture(unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(valSubgrph.Result))
	s.AddSubgraphFixtureToMempools(valSubgrph)

	s.publicKey.On("Verify",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(false, nil).Once()

	err := s.receiptValidator.Validate(receipt)
	s.Require().Error(err, "should reject invalid signature")
	s.Assert().True(engine.IsInvalidInputError(err))
	s.publicKey.AssertExpectations(s.T())
}

// TestReceiptTooFewChunks tests that we reject receipt with invalid chunk count
func (s *ReceiptValidationSuite) TestReceiptTooFewChunks() {
	valSubgrph := s.ValidSubgraphFixture()
	chunks := valSubgrph.Result.Chunks
	valSubgrph.Result.Chunks = chunks[0 : len(chunks)-2] // drop the last chunk
	receipt := unittest.ExecutionReceiptFixture(unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(valSubgrph.Result))
	s.AddSubgraphFixtureToMempools(valSubgrph)

	s.publicKey.On("Verify",
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

	s.publicKey.On("Verify",
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

	s.publicKey.On("Verify",
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

	s.publicKey.On("Verify",
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

	s.publicKey.On("Verify",
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
	receipt := unittest.ExecutionReceiptFixture(unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(valSubgrph.Result))
	s.AddSubgraphFixtureToMempools(valSubgrph)

	// invalidate prev execution result blockID, this should fail because
	// prev result points to wrong block
	valSubgrph.PreviousResult.BlockID = unittest.IdentifierFixture()

	s.publicKey.On("Verify",
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
	receipt := unittest.ExecutionReceiptFixture(unittest.WithExecutorID(s.ExeID),
		unittest.WithResult(valSubgrph.Result))
	s.AddSubgraphFixtureToMempools(valSubgrph)

	// invalidate prev execution result blockID, this should fail because
	// prev result points to wrong block
	valSubgrph.PreviousResult.Chunks[len(valSubgrph.Result.Chunks)-1].EndState = unittest.StateCommitmentFixture()

	s.publicKey.On("Verify",
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(true, nil).Maybe()

	err := s.receiptValidator.Validate(receipt)
	s.Require().Error(err, "should reject invalid previous result")
	s.Assert().True(engine.IsInvalidInputError(err), err)
}

// TestMultiReceiptValidResultChain tests that multiple receipts and results
// within one block payload are accepted, where the receipts are building on
// top of each other (i.e. their results form a chain).
// Say B(A) means block B has receipt for A:
// * we have such chain in storage: G <- A <- B(A) <- C
// * if a child block of C payload contains receipts and results for (B,C)
//   it should be accepted as valid
func (s *ReceiptValidationSuite) TestMultiReceiptValidResultChain() {
	// assuming signatures are all good
	s.publicKey.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

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

	err := s.receiptValidator.ValidatePayload(candidate)
	s.Require().NoError(err)
}

// we have such chain in storage: G <- A <- B(A) <- C
// if a block payload contains (C,B_bad), they should be invalid
func (s *ReceiptValidationSuite) TestMultiReceiptInvalidParent() {
	// assuming signatures are all good
	s.publicKey.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	// G <- A <- B <- C
	blocks, result0, seal := unittest.ChainFixture(4)
	s.SealsIndex[blocks[0].ID()] = seal

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

	// make receipt B as bad
	receiptBInvalid.ExecutorID = unittest.IdentifierFixture()

	candidate := unittest.BlockWithParentFixture(blockC.Header)
	candidate.Payload = &flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{receiptBInvalid.Meta(), receiptC.Meta()},
		Results:  []*flow.ExecutionResult{&receiptBInvalid.ExecutionResult, &receiptC.ExecutionResult},
	}

	// receiptB and receiptC
	err := s.receiptValidator.ValidatePayload(candidate)
	s.Require().Error(err)
	require.True(s.T(), engine.IsInvalidInputError(err), err)
}

// Test that `ValidatePayload` will refuse payloads that contain receipts for blocks that
// are already sealed on the fork, but will accept receipts for blocks that are
// sealed on another fork.
func (s *ReceiptValidationSuite) TestValidationReceiptsForSealedBlock() {
	// assuming signatures are all good
	s.publicKey.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	// create block2
	block2 := unittest.BlockWithParentFixture(s.LatestSealedBlock.Header)
	block2.SetPayload(flow.Payload{})
	s.Extend(block2)

	block2Receipt := unittest.ExecutionReceiptFixture(unittest.WithResult(
		unittest.ExecutionResultFixture(unittest.WithBlock(block2),
			unittest.WithPreviousResult(*s.LatestExecutionResult))))

	// B1<--B2<--B3{R{B2)}<--B4{S(R(B2))}<--B5{R'(B2)}

	// create block3 with a receipt for block2
	block3 := unittest.BlockWithParentFixture(block2.Header)
	block3.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{block2Receipt.Meta()},
		Results:  []*flow.ExecutionResult{&block2Receipt.ExecutionResult},
	})
	s.Extend(block3)

	// create a seal for block2
	seal2 := unittest.Seal.Fixture(unittest.Seal.WithResult(&block2Receipt.ExecutionResult))

	// create block4 containing a seal for block2
	block4 := unittest.BlockWithParentFixture(block3.Header)
	block4.SetPayload(flow.Payload{
		Seals: []*flow.Seal{seal2},
	})
	s.Extend(block4)

	// insert another receipt for block 2, which is now the highest sealed
	// block, and ensure that the receipt is rejected
	receipt := unittest.ExecutionReceiptFixture(unittest.WithResult(
		unittest.ExecutionResultFixture(unittest.WithBlock(block2),
			unittest.WithPreviousResult(*s.LatestExecutionResult))),
		unittest.WithExecutorID(s.ExeID))
	block5 := unittest.BlockWithParentFixture(block4.Header)
	block5.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{receipt.Meta()},
		Results:  []*flow.ExecutionResult{&receipt.ExecutionResult},
	})

	err := s.receiptValidator.ValidatePayload(block5)
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
	err = s.receiptValidator.ValidatePayload(block6)
	require.NoError(s.T(), err)
}

// Test that validator will accept payloads with receipts that are referring execution results
// which were incorporated in previous blocks of fork.
func (s *ReceiptValidationSuite) TestValidationReceiptForIncorporatedResult() {
	// assuming signatures are all good
	s.publicKey.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	// create block2
	block2 := unittest.BlockWithParentFixture(s.LatestSealedBlock.Header)
	block2.SetPayload(flow.Payload{})
	s.Extend(block2)

	executionResult := unittest.ExecutionResultFixture(unittest.WithBlock(block2),
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
	s.Extend(block3)

	exe := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	s.Identities[exe.NodeID] = exe
	exe.StakingPubKey = s.publicKey // make sure the other exection node's signatures are valid

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

	err := s.receiptValidator.ValidatePayload(block5)
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
	s.publicKey.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	// create block A
	blockA := unittest.BlockWithParentFixture(s.LatestSealedBlock.Header) // for block G, we use the LatestSealedBlock
	s.Extend(blockA)

	// result for A; and receipt for A
	resultA := unittest.ExecutionResultFixture(unittest.WithBlock(blockA), unittest.WithPreviousResult(*s.LatestExecutionResult))
	receiptA := unittest.ExecutionReceiptFixture(unittest.WithResult(resultA), unittest.WithExecutorID(s.ExeID))

	// create block B and block C
	blockB := unittest.BlockWithParentFixture(blockA.Header)
	blockC := unittest.BlockWithParentFixture(blockA.Header)
	blockC.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{receiptA.Meta()},
		Results:  []*flow.ExecutionResult{resultA},
	})
	s.Extend(blockB)
	s.Extend(blockC)

	// create block X:
	blockX := unittest.BlockWithParentFixture(blockB.Header)
	blockX.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{receiptA.Meta()},
	})

	err := s.receiptValidator.ValidatePayload(blockX)
	require.Error(s.T(), err)
	require.True(s.T(), engine.IsInvalidInputError(err), err)
}

// TestPayloadWithExecutionFork checks that the Receipt Validator only
// accepts results that decent from the sealed result. Specifically, we test that
// the counter-example is rejected:
//  * we have the chain in storage:
//     S <- A(Result[S]_1, Result[S]_2, ReceiptMeta[S]_1, ReceiptMeta[S]_2)
//            <- B(Seal for Result[S]_2)
//               <- X(Result[A]_1, Result[A]_2, Result[A]_3,
//                    ReceiptMeta[A]_1, ReceiptMeta[A]_2, ReceiptMeta[A]_3)
//  * Note that we are explicitly testing the handling of an execution fork _before_
//    and _after_ the sealed result
//       Blocks:      S  <-----------   A
//      Results:   Result[S]_1  <-  Result[A]_1  :: the root of this execution tree conflicts with sealed result
//                 Result[S]_2  <-  Result[A]_2  :: the root of this execution tree is sealed
//                              ^-  Result[A]_3
// Expected Behaviour:
// In the fork which X extends, Result[S]_2 has been sealed. Hence, it should be
//   (i) illegal to include Result[A]_1, because it is _not_ derived from the sealed result.
//  (ii) legal to include only results Result[A]_2 and Result[A]_3, as they are derived from the sealed result.
func (s *ReceiptValidationSuite) TestPayloadWithExecutionFork() {
	// assuming signatures are all good
	s.publicKey.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	// block S: we use s.LatestSealedBlock; its result is s.LatestExecutionResult
	blockS := s.LatestSealedBlock
	resultS1 := s.LatestExecutionResult
	receiptS1 := unittest.ExecutionReceiptFixture(unittest.WithResult(resultS1), unittest.WithExecutorID(s.ExeID))
	resultS2 := unittest.ExecutionResultFixture(unittest.WithBlock(&blockS))
	receiptS2 := unittest.ExecutionReceiptFixture(unittest.WithResult(resultS2), unittest.WithExecutorID(s.ExeID))

	// create block A, including results and receipts for it
	blockA := unittest.BlockWithParentFixture(blockS.Header)
	blockA.SetPayload(flow.Payload{
		Results:  []*flow.ExecutionResult{resultS1, resultS2},
		Receipts: []*flow.ExecutionReceiptMeta{receiptS1.Meta(), receiptS2.Meta()},
	})
	s.Extend(blockA)

	// create block B
	blockB := unittest.BlockWithParentFixture(blockA.Header)
	sealResultS2 := unittest.Seal.Fixture(unittest.Seal.WithBlock(blockS.Header), unittest.Seal.WithResult(resultS2))
	blockB.SetPayload(flow.Payload{
		Seals: []*flow.Seal{sealResultS2},
	})
	s.Extend(blockB)

	// create Result[A]_1, Result[A]_2, Result[A]_3 and their receipts
	resultA1 := unittest.ExecutionResultFixture(unittest.WithBlock(blockA), unittest.WithPreviousResult(*resultS1))
	receiptA1 := unittest.ExecutionReceiptFixture(unittest.WithResult(resultA1), unittest.WithExecutorID(s.ExeID))
	resultA2 := unittest.ExecutionResultFixture(unittest.WithBlock(blockA), unittest.WithPreviousResult(*resultS2))
	receiptA2 := unittest.ExecutionReceiptFixture(unittest.WithResult(resultA2), unittest.WithExecutorID(s.ExeID))
	resultA3 := unittest.ExecutionResultFixture(unittest.WithBlock(blockA), unittest.WithPreviousResult(*resultS2))
	receiptA3 := unittest.ExecutionReceiptFixture(unittest.WithResult(resultA3), unittest.WithExecutorID(s.ExeID))

	// SCENARIO (i): a block containing Result[A]_1 should fail validation
	blockX := unittest.BlockWithParentFixture(blockB.Header)
	blockX.SetPayload(flow.Payload{
		Results:  []*flow.ExecutionResult{resultA1, resultA2, resultA3},
		Receipts: []*flow.ExecutionReceiptMeta{receiptA1.Meta(), receiptA2.Meta(), receiptA3.Meta()},
	})
	err := s.receiptValidator.ValidatePayload(blockX)
	require.Error(s.T(), err)
	require.True(s.T(), engine.IsInvalidInputError(err), err)

	// SCENARIO (ii): a block containing only results Result[A]_2 and Result[A]_3 should pass validation
	blockX = unittest.BlockWithParentFixture(blockB.Header)
	blockX.SetPayload(flow.Payload{
		Results:  []*flow.ExecutionResult{resultA2, resultA3},
		Receipts: []*flow.ExecutionReceiptMeta{receiptA2.Meta(), receiptA3.Meta()},
	})
	err = s.receiptValidator.ValidatePayload(blockX)
	require.NoError(s.T(), err)
}

// TestMultiLevelExecutionTree verifies that a result is accepted that
// extends a multi-level execution tree :
//  * Let S be the latest sealed block
//  * we have the chain in storage:
//     S <- A <- B(Result[A], ReceiptMeta[A]) <- C(Result[B], ReceiptMeta[B])
//  * now receive the new block X:
//     S <- A <- B(Result[A], ReceiptMeta[A]) <- C(Result[B], ReceiptMeta[B]) <- X(Result[C], ReceiptMeta[C])
// Block X should be considered valid, as it extends the
// Execution Tree with root latest sealed Result (i.e. result sealed for S)
func (s *ReceiptValidationSuite) TestMultiLevelExecutionTree() {
	// assuming signatures are all good
	s.publicKey.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	// create block A, including result and receipt for it
	blockA := unittest.BlockWithParentFixture(s.LatestSealedBlock.Header)
	resultA := unittest.ExecutionResultFixture(unittest.WithBlock(blockA), unittest.WithPreviousResult(*s.LatestExecutionResult))
	receiptA := unittest.ExecutionReceiptFixture(unittest.WithResult(resultA), unittest.WithExecutorID(s.ExeID))
	s.Extend(blockA)

	// create block B, including result and receipt for it
	blockB := unittest.BlockWithParentFixture(blockA.Header)
	blockB.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{receiptA.Meta()},
		Results:  []*flow.ExecutionResult{resultA},
	})
	resultB := unittest.ExecutionResultFixture(unittest.WithBlock(blockB), unittest.WithPreviousResult(*resultA))
	receiptB := unittest.ExecutionReceiptFixture(unittest.WithResult(resultB), unittest.WithExecutorID(s.ExeID))
	s.Extend(blockB)

	// create block C, including result and receipt for it
	blockC := unittest.BlockWithParentFixture(blockB.Header)
	blockC.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{receiptB.Meta()},
		Results:  []*flow.ExecutionResult{resultB},
	})
	resultC := unittest.ExecutionResultFixture(unittest.WithBlock(blockC), unittest.WithPreviousResult(*resultB))
	receiptC := unittest.ExecutionReceiptFixture(unittest.WithResult(resultC), unittest.WithExecutorID(s.ExeID))
	s.Extend(blockC)

	// create block X:
	blockX := unittest.BlockWithParentFixture(blockC.Header)
	blockX.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{receiptC.Meta()},
		Results:  []*flow.ExecutionResult{resultC},
	})

	err := s.receiptValidator.ValidatePayload(blockX)
	require.NoError(s.T(), err)
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
	s.Extend(block2)

	// create block3
	block3 := unittest.BlockWithParentFixture(block2.Header)
	block3.SetPayload(flow.Payload{})
	s.Extend(block3)

	block3Receipt := unittest.ReceiptForBlockFixture(block3)

	block4 := unittest.BlockWithParentFixture(block2.Header)
	block4.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{block3Receipt.Meta()},
		Results:  []*flow.ExecutionResult{&block3Receipt.ExecutionResult},
	})
	err := s.receiptValidator.ValidatePayload(block4)
	require.Error(s.T(), err)
	require.True(s.T(), engine.IsInvalidInputError(err), err)
}

// Test that Extend will refuse payloads that contain duplicate receipts, where
// duplicates can be in another block on the fork, or within the payload.
func (s *ReceiptValidationSuite) TestExtendReceiptsDuplicate() {

	block2 := unittest.BlockWithParentFixture(s.LatestFinalizedBlock.Header)
	block2.SetPayload(flow.Payload{})
	s.Extend(block2)

	receipt := unittest.ReceiptForBlockFixture(block2)

	// B1 <- B2 <- B3{R(B2)} <- B4{R(B2)}
	s.T().Run("duplicate receipt in different block", func(t *testing.T) {
		block3 := unittest.BlockWithParentFixture(block2.Header)
		block3.SetPayload(flow.Payload{
			Receipts: []*flow.ExecutionReceiptMeta{receipt.Meta()},
			Results:  []*flow.ExecutionResult{&receipt.ExecutionResult},
		})
		s.Extend(block3)

		block4 := unittest.BlockWithParentFixture(block3.Header)
		block4.SetPayload(flow.Payload{
			Receipts: []*flow.ExecutionReceiptMeta{receipt.Meta()},
			Results:  []*flow.ExecutionResult{&receipt.ExecutionResult},
		})
		err := s.receiptValidator.ValidatePayload(block4)
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
		err := s.receiptValidator.ValidatePayload(block3)
		require.Error(t, err)
		require.True(t, engine.IsInvalidInputError(err), err)
	})
}

// `TestValidateReceiptAfterBootstrap` tests a special case when we try to produce a new block
// after genesis with empty payload.
func (s *ReceiptValidationSuite) TestValidateReceiptAfterBootstrap() {
	// assuming signatures are all good
	s.publicKey.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	// G
	blocks, result0, seal := unittest.ChainFixture(0)
	s.SealsIndex[blocks[0].ID()] = seal

	for _, b := range blocks {
		s.Extend(b)
	}
	s.PersistedResults[result0.ID()] = result0

	candidate := unittest.BlockWithParentFixture(blocks[0].Header)
	err := s.receiptValidator.ValidatePayload(candidate)
	s.Require().NoError(err)
}
