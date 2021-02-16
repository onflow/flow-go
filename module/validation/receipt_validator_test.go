package validation

import (
	"testing"

	"github.com/stretchr/testify/mock"
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

	err := s.receiptValidator.Validate(one(receipt))
	s.Require().NoError(err, "should successfully validate receipt")
	s.verifier.AssertExpectations(s.T())
}

// TestReceiptNoIdentity tests that we reject receipt with invalid `ExecutionResult.ExecutorID`
func (s *ReceiptValidationSuite) TestReceiptNoIdentity() {
	valSubgrph := s.ValidSubgraphFixture()
	receipt := unittest.ExecutionReceiptFixture(unittest.WithExecutorID(unittest.IdentityFixture().NodeID),
		unittest.WithResult(valSubgrph.Result))
	s.AddSubgraphFixtureToMempools(valSubgrph)

	err := s.receiptValidator.Validate(one(receipt))
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

	err := s.receiptValidator.Validate(one(receipt))
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

	err := s.receiptValidator.Validate(one(receipt))
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

	err := s.receiptValidator.Validate(one(receipt))
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

	err := s.receiptValidator.Validate(one(receipt))
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

	err := s.receiptValidator.Validate(one(receipt))
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

	err := s.receiptValidator.Validate(one(receipt))
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

	err := s.receiptValidator.Validate(one(receipt))
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

	err := s.receiptValidator.Validate(one(receipt))
	s.Require().Error(err, "should reject invalid receipt")
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

	err := s.receiptValidator.Validate(one(receipt))
	s.Require().Error(err, "should reject invalid previous result")
}

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

	err := s.receiptValidator.Validate(one(receipt))
	s.Require().Error(err, "should reject invalid previous result")
}

// say B(A) means block B has receipt for A:
// we have such chain in storage: G <- A <- B(A) <- C
// if a block payload contains (B,C), they should be valid.
func (s *ReceiptValidationSuite) TestMultiReceiptValidResultChain() {
	// assuming signatures are all good
	s.verifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	prepareReceipts := func() []*flow.ExecutionReceipt {
		// G <- A <- B <- C
		blocks, result0, _ := unittest.ChainFixture(4)

		receipts := unittest.ReceiptChainFor(blocks, result0)
		blockA, blockB, blockC := blocks[1], blocks[2], blocks[3]
		receiptA := receipts[1]
		blockA.Payload.Receipts = []*flow.ExecutionReceipt{}
		blockB.Payload.Receipts = []*flow.ExecutionReceipt{receiptA}
		blockC.Payload.Receipts = []*flow.ExecutionReceipt{}
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
		return receipts
	}

	receipts := prepareReceipts()
	// recieptB and receiptC
	receiptsInNewBlock := []*flow.ExecutionReceipt{receipts[2], receipts[3]}
	err := s.receiptValidator.Validate(receiptsInNewBlock)
	s.Require().NoError(err)
}

// we have such chain in storage: G <- A <- B(A) <- C
// if a block payload contains (C,B_bad), they should be invalid
func (s *ReceiptValidationSuite) TestMultiReceiptInvalidParent() {
	// assuming signatures are all good
	s.verifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	prepareReceipts := func() []*flow.ExecutionReceipt {
		// G <- A <- B <- C
		blocks, result0, _ := unittest.ChainFixture(4)

		receipts := unittest.ReceiptChainFor(blocks, result0)
		blockA, blockB, blockC := blocks[1], blocks[2], blocks[3]
		receiptA := receipts[1]
		blockA.Payload.Receipts = []*flow.ExecutionReceipt{}
		blockB.Payload.Receipts = []*flow.ExecutionReceipt{receiptA}
		blockC.Payload.Receipts = []*flow.ExecutionReceipt{}
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
		return receipts
	}

	receipts := prepareReceipts()
	// make receipt B as bad
	receipts[2].ExecutorID = s.VerID
	// recieptB and receiptC
	receiptsInNewBlock := []*flow.ExecutionReceipt{receipts[2], receipts[3]}
	err := s.receiptValidator.Validate(receiptsInNewBlock)
	s.Require().Error(err)
}

func (s *ReceiptValidationSuite) TestValidatePayloadWithInvalidIncorporatedReceipt() {
	s.verifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	// insert 2 valid blocks
	blockA := unittest.BlockWithParentFixture(s.LatestSealedBlock.Header)
	blockA.SetPayload(flow.Payload{})
	s.Extend(&blockA)

	blockAresult := unittest.ExecutionResultFixture(
		unittest.WithBlock(&blockA),
	)
	blockAreceipt := unittest.ExecutionReceiptFixture(unittest.WithResult(blockAresult),
		unittest.WithExecutorID(s.ExeID))

	blockB := unittest.BlockWithParentFixture(blockA.Header)
	blockB.SetPayload(flow.Payload{Receipts: one(blockAreceipt)})
	s.Extend(&blockB)

	blockC := unittest.BlockWithParentFixture(blockB.Header)
	blockC.SetPayload(flow.Payload{})
	s.Extend(&blockC)

	blockD := unittest.BlockWithParentFixture(blockA.Header)
	blockD.SetPayload(flow.Payload{Receipts: one(blockAreceipt)})
	s.Extend(&blockD)

	blockBresult := unittest.ExecutionResultFixture(
		unittest.WithBlock(&blockB),
		unittest.WithPreviousResult(*blockAresult),
	)
	blockBreceipt := unittest.ExecutionReceiptFixture(unittest.WithResult(blockBresult),
		unittest.WithExecutorID(s.ExeID))

	blockE := unittest.BlockWithParentFixture(blockD.Header)
	blockE.SetPayload(flow.Payload{Receipts: one(blockBreceipt)})
	s.Extend(&blockE)

	blockCresult := unittest.ExecutionResultFixture(
		unittest.WithBlock(&blockC),
		unittest.WithPreviousResult(*blockBresult),
	)
	blockCreceipt := unittest.ExecutionReceiptFixture(unittest.WithResult(blockCresult),
		unittest.WithExecutorID(s.ExeID))

	blockX := unittest.BlockWithParentFixture(blockC.Header)
	blockX.SetPayload(flow.Payload{Receipts: one(blockCreceipt)})
	s.Extend(&blockX)

	err := s.receiptValidator.ValidatePayload(&blockX)
	s.Require().Errorf(err, "invalid payload because of wrong receipt fork")

}

func one(receipt *flow.ExecutionReceipt) []*flow.ExecutionReceipt {
	return []*flow.ExecutionReceipt{receipt}
}
