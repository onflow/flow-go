// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package matching

import (
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/storage"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
)

// 1. Matching engine should validate the incoming receipt (aka ExecutionReceipt):
//     1. it should stores it to the mempool if valid
//     2. it should ignore it when:
//         1. the origin is invalid [Condition removed for now -> will be replaced by valid EN signature in future]
//         2. the role is invalid
//         3. the result (a receipt has one result, multiple receipts might have the same result) has been sealed already
//         4. the receipt has been received before
//         5. the result has been received before
// 2. Matching engine should validate the incoming approval (aka ResultApproval):
//     1. it should store it to the mempool if valid
//     2. it should ignore it when:
//         1. the origin is invalid
//         2. the role is invalid
//         3. the result has been sealed already
// 3. Matching engine should be able to find matched results:
//     1. It should find no matched result if there is no result and no approval
//     2. it should find 1 matched result if we received a receipt, and the block has no payload (impossible now, system every block will have at least one chunk to verify)
//     3. It should find no matched result if there is only result, but no approval (skip for now, because we seal results without approvals)
// 4. Matching engine should be able to seal a matched result:
//     1. It should not seal a matched result if:
//         1. the block is missing (consensus hasn’t received this executed block yet)
//         2. the approvals for a certain chunk are insufficient (skip for now, because we seal results without approvals)
//         3. there is some chunk didn’t receive enough approvals
//         4. the previous result is not known
//         5. the previous result references the wrong block
//     2. It should seal a matched result if the approvals are sufficient
// 5. Matching engine should request results from execution nodes:
//     1. If there are unsealed and finalized blocks, it should request the execution receipts from the execution nodes.
func TestMatchingEngine(t *testing.T) {
	suite.Run(t, new(MatchingSuite))
}

type MatchingSuite struct {
	unittest.BaseChainSuite
	// misc SERVICE COMPONENTS which are injected into Matching Engine
	requester        *mockmodule.Requester
	receiptValidator *mockmodule.ReceiptValidator

	// MATCHING ENGINE
	matching *Engine
}

func (ms *MatchingSuite) SetupTest() {
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~ SETUP SUITE ~~~~~~~~~~~~~~~~~~~~~~~~~~ //
	ms.SetupChain()

	unit := engine.NewUnit()
	log := zerolog.New(os.Stderr)
	metrics := metrics.NewNoopCollector()
	tracer := trace.NewNoopTracer()

	// ~~~~~~~~~~~~~~~~~~~~~~~ SETUP MATCHING ENGINE ~~~~~~~~~~~~~~~~~~~~~~~ //
	ms.requester = new(mockmodule.Requester)
	ms.receiptValidator = &mockmodule.ReceiptValidator{}

	ms.matching = &Engine{
		unit:                                 unit,
		log:                                  log,
		tracer:                               tracer,
		engineMetrics:                        metrics,
		mempool:                              metrics,
		metrics:                              metrics,
		state:                                ms.State,
		receiptRequester:                     ms.requester,
		receiptsDB:                           ms.ReceiptsDB,
		headersDB:                            ms.HeadersDB,
		indexDB:                              ms.IndexDB,
		incorporatedResults:                  ms.ResultsPL,
		receipts:                             ms.ReceiptsPL,
		approvals:                            ms.ApprovalsPL,
		seals:                                ms.SealsPL,
		pendingReceipts:                      stdmap.NewPendingReceipts(100),
		isCheckingSealing:                    atomic.NewBool(false),
		sealingThreshold:                     10,
		maxResultsToRequest:                  200,
		assigner:                             ms.Assigner,
		receiptValidator:                     ms.receiptValidator,
		requestTracker:                       NewRequestTracker(1, 3),
		approvalRequestsThreshold:            10,
		requiredApprovalsForSealConstruction: DefaultRequiredApprovalsForSealConstruction,
		emergencySealingActive:               false,
	}
}

// Test that we reject receipts for unknown blocks without generating an error
func (ms *MatchingSuite) TestOnReceiptUnknownBlock() {
	// This receipt has a random block ID, so the matching engine won't find it.
	receipt := unittest.ExecutionReceiptFixture()

	// onReceipt should reject the receipt without throwing an error
	_, err := ms.matching.onCurrentReceipt(receipt)
	ms.Require().NoError(err, "should drop receipt for unknown block without error")

	ms.ReceiptsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.ResultsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// matching engine should drop Result for known block that is already sealed
// without trying to store anything
func (ms *MatchingSuite) TestOnReceiptSealedResult() {
	originID := ms.ExeID
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(originID),
		unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(&ms.LatestSealedBlock))),
	)

	_, err := ms.matching.onCurrentReceipt(receipt)
	ms.Require().NoError(err, "should ignore receipt for sealed result")

	ms.ReceiptsDB.AssertNumberOfCalls(ms.T(), "Store", 0)
	ms.ResultsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// Test that we drop receipts that are already pooled
func (ms *MatchingSuite) TestOnReceiptPendingReceipt() {
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(ms.ExeID),
		unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(&ms.UnfinalizedBlock))),
	)

	ms.receiptValidator.On("Validate", []*flow.ExecutionReceipt{receipt}).Return(nil)

	// setup the receipts mempool to check if we attempted to add the receipt to
	// the mempool, and return false as we are testing the case where it was already in the mempool
	ms.ReceiptsPL.On("AddReceipt", receipt, ms.UnfinalizedBlock.Header).Return(false, nil).Once()

	// onReceipt should return immediately after realizing the receipt is already in the mempool
	// but without throwing any errors
	_, err := ms.matching.onCurrentReceipt(receipt)
	ms.Require().NoError(err, "should ignore already pending receipt")

	ms.ReceiptsPL.AssertExpectations(ms.T())
	ms.ReceiptsDB.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// Test that we store different receipts for the same result
func (ms *MatchingSuite) TestOnReceiptPendingResult() {
	originID := ms.ExeID
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(originID),
		unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(&ms.UnfinalizedBlock))),
	)
	ms.receiptValidator.On("Validate", []*flow.ExecutionReceipt{receipt}).Return(nil)

	// setup the results mempool to check if we attempted to insert the
	// incorporated result, and return false as if it was already in the mempool
	// TODO: remove for later sealing phases
	ms.ResultsPL.
		On("Add", incorporatedResult(receipt.ExecutionResult.BlockID, &receipt.ExecutionResult)).
		Return(false, nil).Once()

	// Expect the receipt to be added to mempool
	ms.ReceiptsPL.On("AddReceipt", receipt, ms.UnfinalizedBlock.Header).Return(true, nil).Once()

	_, err := ms.matching.onCurrentReceipt(receipt)
	ms.Require().NoError(err, "should not error for different receipt for already pending result")
	ms.ReceiptsPL.AssertExpectations(ms.T())
	ms.ReceiptsDB.AssertNumberOfCalls(ms.T(), "Store", 1)
}

// TestOnReceipt_ReceiptInPersistentStorage verifies that Matching Engine adds
// a receipt to the mempool, even if it is already in persistent storage. This
// can happen after a crash, where the mempools got wiped
func (ms *MatchingSuite) TestOnReceipt_ReceiptInPersistentStorage() {
	originID := ms.ExeID
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(originID),
		unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(&ms.UnfinalizedBlock))),
	)
	ms.receiptValidator.On("Validate", []*flow.ExecutionReceipt{receipt}).Return(nil)

	// Persistent storage layer for Receipts has the receipt already stored
	ms.ReceiptsDB.On("Store", receipt).Return(storage.ErrAlreadyExists).Once()

	// The receipt should be added to the receipts mempool
	ms.ReceiptsPL.On("AddReceipt", receipt, ms.UnfinalizedBlock.Header).Return(true, nil).Once()

	_, err := ms.matching.onCurrentReceipt(receipt)
	ms.Require().NoError(err, "should not error for different receipt for already pending result")
	ms.ReceiptsPL.AssertExpectations(ms.T())
	ms.ReceiptsDB.AssertNumberOfCalls(ms.T(), "Store", 1)
}

// try to submit a receipt that should be valid
func (ms *MatchingSuite) TestOnReceiptValid() {
	originID := ms.ExeID
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(originID),
		unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(&ms.UnfinalizedBlock))),
	)

	ms.receiptValidator.On("Validate", []*flow.ExecutionReceipt{receipt}).Return(nil).Once()

	// we expect that receipt is added to mempool
	ms.ReceiptsPL.On("AddReceipt", receipt, ms.UnfinalizedBlock.Header).Return(true, nil).Once()

	// onReceipt should run to completion without throwing an error
	_, err := ms.matching.onCurrentReceipt(receipt)
	ms.Require().NoError(err, "should add receipt and result to mempool if valid")

	ms.receiptValidator.AssertExpectations(ms.T())
	ms.ReceiptsPL.AssertExpectations(ms.T())
}

// TestOnReceiptInvalid tests that we reject receipts that don't pass the ReceiptValidator
func (ms *MatchingSuite) TestOnReceiptInvalid() {
	// we use the same Receipt as in TestOnReceiptValid to ensure that the matching engine is not
	// rejecting the receipt for any other reason
	originID := ms.ExeID
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(originID),
		unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(&ms.UnfinalizedBlock))),
	)
	ms.receiptValidator.On("Validate", []*flow.ExecutionReceipt{receipt}).Return(engine.NewInvalidInputError("")).Once()

	_, err := ms.matching.onCurrentReceipt(receipt)
	ms.Require().Error(err, "should reject receipt that does not pass ReceiptValidator")
	ms.Assert().True(engine.IsInvalidInputError(err))

	ms.receiptValidator.AssertExpectations(ms.T())
	ms.ReceiptsDB.AssertNumberOfCalls(ms.T(), "Store", 0)
}

// try to submit an approval where the message origin is inconsistent with the message creator
func (ms *MatchingSuite) TestApprovalInvalidOrigin() {
	// approval from valid origin (i.e. a verification node) but with random ApproverID
	originID := ms.VerID
	approval := unittest.ResultApprovalFixture() // with random ApproverID

	err := ms.matching.onApproval(originID, approval)
	ms.Require().Error(err, "should reject approval with mismatching origin and executor")
	ms.Require().True(engine.IsInvalidInputError(err))

	// approval from random origin but with valid ApproverID (i.e. a verification node)
	originID = unittest.IdentifierFixture() // random origin
	approval = unittest.ResultApprovalFixture(unittest.WithApproverID(ms.VerID))

	err = ms.matching.onApproval(originID, approval)
	ms.Require().Error(err, "should reject approval with mismatching origin and executor")
	ms.Require().True(engine.IsInvalidInputError(err))

	// In both cases, we expect the approval to be rejected without hitting the mempools
	ms.ApprovalsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// Try to submit an approval for an unknown block.
// As the block is unknown, the ID of the sender should
// not matter as there is no block to verify it against
func (ms *MatchingSuite) TestApprovalUnknownBlock() {
	originID := ms.ConID
	approval := unittest.ResultApprovalFixture(unittest.WithApproverID(originID)) // generates approval for random block ID

	// Make sure the approval is added to the cache for future processing
	ms.ApprovalsPL.On("Add", approval).Return(true, nil).Once()

	// onApproval should not throw an error
	err := ms.matching.onApproval(approval.Body.ApproverID, approval)
	ms.Require().NoError(err, "should cache approvals for unknown blocks")

	ms.ApprovalsPL.AssertExpectations(ms.T())
}

// try to submit an approval from a consensus node
func (ms *MatchingSuite) TestOnApprovalInvalidRole() {
	originID := ms.ConID
	approval := unittest.ResultApprovalFixture(
		unittest.WithBlockID(ms.UnfinalizedBlock.ID()),
		unittest.WithApproverID(originID),
	)

	err := ms.matching.onApproval(originID, approval)
	ms.Require().Error(err, "should reject approval from wrong approver role")
	ms.Require().True(engine.IsInvalidInputError(err))

	ms.ApprovalsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// try to submit an approval from an unstaked approver
func (ms *MatchingSuite) TestOnApprovalInvalidStake() {
	originID := ms.VerID
	approval := unittest.ResultApprovalFixture(
		unittest.WithBlockID(ms.UnfinalizedBlock.ID()),
		unittest.WithApproverID(originID),
	)
	ms.Identities[originID].Stake = 0

	err := ms.matching.onApproval(originID, approval)
	ms.Require().Error(err, "should reject approval from unstaked approver")
	ms.Require().True(engine.IsInvalidInputError(err))

	ms.ApprovalsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// try to submit an approval for a sealed result
func (ms *MatchingSuite) TestOnApprovalSealedResult() {
	originID := ms.VerID
	approval := unittest.ResultApprovalFixture(
		unittest.WithBlockID(ms.LatestSealedBlock.ID()),
		unittest.WithApproverID(originID),
	)

	err := ms.matching.onApproval(originID, approval)
	ms.Require().NoError(err, "should ignore approval for sealed result")

	ms.ApprovalsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// try to submit an approval that is already in the mempool
func (ms *MatchingSuite) TestOnApprovalPendingApproval() {
	originID := ms.VerID
	approval := unittest.ResultApprovalFixture(unittest.WithApproverID(originID))

	// setup the approvals mempool to check that we attempted to add the
	// approval, and return false as if it was already in the mempool
	ms.ApprovalsPL.On("Add", approval).Return(false, nil).Once()

	// onApproval should return immediately after trying to insert the approval,
	// without throwing any errors
	err := ms.matching.onApproval(approval.Body.ApproverID, approval)
	ms.Require().NoError(err, "should ignore approval if already pending")

	ms.ApprovalsPL.AssertExpectations(ms.T())
}

// try to submit an approval for a known block
func (ms *MatchingSuite) TestOnApprovalValid() {
	originID := ms.VerID
	approval := unittest.ResultApprovalFixture(
		unittest.WithBlockID(ms.UnfinalizedBlock.ID()),
		unittest.WithApproverID(originID),
	)

	// check that the approval is correctly added
	ms.ApprovalsPL.On("Add", approval).Return(true, nil).Once()

	// onApproval should run to completion without throwing any errors
	err := ms.matching.onApproval(approval.Body.ApproverID, approval)
	ms.Require().NoError(err, "should add approval to mempool if valid")

	ms.ApprovalsPL.AssertExpectations(ms.T())
}

// try to get matched results with nothing in memory pools
func (ms *MatchingSuite) TestSealableResultsEmptyMempools() {
	results, _, err := ms.matching.sealableResults()
	ms.Require().NoError(err, "should not error with empty mempools")
	ms.Assert().Empty(results, "should not have matched results with empty mempools")
}

// TestSealableResultsValid tests matching.Engine.sealableResults():
//  * a well-formed incorporated result R is in the mempool
//  * sufficient number of valid result approvals for result R
//  * R.PreviousResultID references a known result (i.e. stored in ResultsDB)
//  * R forms a valid sub-graph with its previous result (aka parent result)
// Method Engine.sealableResults() should return R as an element of the sealable results
func (ms *MatchingSuite) TestSealableResultsValid() {
	valSubgrph := ms.ValidSubgraphFixture()
	ms.AddSubgraphFixtureToMempools(valSubgrph)

	// test output of Matching Engine's sealableResults()
	results, _, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	ms.Assert().Equal(1, len(results), "expecting a single return value")
	ms.Assert().Equal(valSubgrph.IncorporatedResult.ID(), results[0].ID(), "expecting a single return value")
}

// Try to seal a result for which we don't have the block.
// This tests verifies that Matching engine is performing self-consistency checking:
// Not finding the block for an incorporated result is a fatal
// implementation bug, as we only add results to the IncorporatedResults
// mempool, where _both_ the block that incorporates the result as well
// as the block the result pertains to are known
func (ms *MatchingSuite) TestSealableResultsMissingBlock() {
	valSubgrph := ms.ValidSubgraphFixture()
	ms.AddSubgraphFixtureToMempools(valSubgrph)
	delete(ms.Blocks, valSubgrph.Block.ID()) // remove block the execution receipt pertains to

	_, _, err := ms.matching.sealableResults()
	ms.Require().Error(err)
}

// TestSealableResultsUnassignedVerifiers tests that matching.Engine.sealableResults():
// only considers approvals from assigned verifiers
func (ms *MatchingSuite) TestSealableResultsUnassignedVerifiers() {
	subgrph := ms.ValidSubgraphFixture()

	assignedVerifiersPerChunk := uint(len(ms.Approvers) / 2)
	assignment := chunks.NewAssignment()
	approvals := make(map[uint64]map[flow.Identifier]*flow.ResultApproval)
	for _, chunk := range subgrph.IncorporatedResult.Result.Chunks {
		assignment.Add(chunk, ms.Approvers[0:assignedVerifiersPerChunk].NodeIDs()) // assign leading half verifiers

		// generate approvals by _tailing_ half verifiers
		chunkApprovals := make(map[flow.Identifier]*flow.ResultApproval)
		for _, approver := range ms.Approvers[assignedVerifiersPerChunk:len(ms.Approvers)] {
			chunkApprovals[approver.NodeID] = unittest.ApprovalFor(subgrph.IncorporatedResult.Result, chunk.Index, approver.NodeID)
		}
		approvals[chunk.Index] = chunkApprovals
	}
	subgrph.Assignment = assignment
	subgrph.Approvals = approvals

	ms.AddSubgraphFixtureToMempools(subgrph)

	results, _, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	ms.Assert().Empty(results, "should not select result with ")
	ms.ApprovalsPL.AssertExpectations(ms.T()) // asserts that ResultsPL.Rem(incorporatedResult.ID()) was called
}

// TestSealableResults_UnknownVerifiers tests that matching.Engine.sealableResults():
//   * removes approvals from unknown verification nodes from mempool
func (ms *MatchingSuite) TestSealableResults_ApprovalsForUnknownBlockRemain() {
	// make child block for UnfinalizedBlock, i.e.:
	//   <- UnfinalizedBlock <- block
	// and create Execution result ands approval for this block
	block := unittest.BlockWithParentFixture(ms.UnfinalizedBlock.Header)
	er := unittest.ExecutionResultFixture(unittest.WithBlock(&block))
	app1 := unittest.ApprovalFor(er, 0, unittest.IdentifierFixture()) // from unknown node

	ms.ApprovalsPL.On("All").Return([]*flow.ResultApproval{app1})
	chunkApprovals := make(map[flow.Identifier]*flow.ResultApproval)
	chunkApprovals[app1.Body.ApproverID] = app1
	ms.ApprovalsPL.On("ByChunk", er.ID(), 0).Return(chunkApprovals)

	_, _, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	ms.ApprovalsPL.AssertNumberOfCalls(ms.T(), "RemApproval", 0)
	ms.ApprovalsPL.AssertNumberOfCalls(ms.T(), "RemChunk", 0)
}

// TestRemoveApprovalsFromInvalidVerifiers tests that matching.Engine.sealableResults():
//   * removes approvals from invalid verification nodes from mempool
// This may occur when the block wasn't know when the node received the approval.
// Note: we test a scenario here, were result is sealable; it just has additional
//      approvals from invalid nodes
func (ms *MatchingSuite) TestRemoveApprovalsFromInvalidVerifiers() {
	subgrph := ms.ValidSubgraphFixture()

	// add invalid approvals to leading chunk:
	app1 := unittest.ApprovalFor(subgrph.IncorporatedResult.Result, 0, unittest.IdentifierFixture()) // from unknown node
	app2 := unittest.ApprovalFor(subgrph.IncorporatedResult.Result, 0, ms.ExeID)                     // from known but non-VerificationNode
	ms.Identities[ms.VerID].Stake = 0
	app3 := unittest.ApprovalFor(subgrph.IncorporatedResult.Result, 0, ms.VerID) // from zero-weight VerificationNode
	subgrph.Approvals[0][app1.Body.ApproverID] = app1
	subgrph.Approvals[0][app2.Body.ApproverID] = app2
	subgrph.Approvals[0][app3.Body.ApproverID] = app3

	ms.AddSubgraphFixtureToMempools(subgrph)

	// we expect business logic to remove the approval from the unknown node
	ms.ApprovalsPL.On("RemApproval", unittest.EntityWithID(app1.ID())).Return(true, nil).Once()
	ms.ApprovalsPL.On("RemApproval", unittest.EntityWithID(app2.ID())).Return(true, nil).Once()
	ms.ApprovalsPL.On("RemApproval", unittest.EntityWithID(app3.ID())).Return(true, nil).Once()

	_, _, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	ms.ApprovalsPL.AssertExpectations(ms.T()) // asserts that ResultsPL.Rem(incorporatedResult.ID()) was called
}

// TestSealableResultsInsufficientApprovals tests matching.Engine.sealableResults():
//  * a result where at least one chunk has not enough approvals (require
//    currently at least one) should not be sealable
func (ms *MatchingSuite) TestSealableResultsInsufficientApprovals() {
	subgrph := ms.ValidSubgraphFixture()
	delete(subgrph.Approvals, uint64(len(subgrph.Result.Chunks)-1))
	ms.AddSubgraphFixtureToMempools(subgrph)

	// test output of Matching Engine's sealableResults()
	results, _, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	ms.Assert().Empty(results, "expecting no sealable result")
}

// TestSealableResultsEmergencySealingMultipleCandidates tests matching.Engine.sealableResults():
// When emergency sealing is active we should be able to identify and pick as candidates incorporated results
// that are deep enough but still without verifications.
func (ms *MatchingSuite) TestSealableResultsEmergencySealingMultipleCandidates() {
	// make sure that emergency sealing is enabled
	ms.matching.emergencySealingActive = true
	emergencySealingCandidates := make([]flow.Identifier, 10)

	for i := range emergencySealingCandidates {
		block := unittest.BlockWithParentFixture(ms.LatestFinalizedBlock.Header)
		receipt := unittest.ExecutionReceiptFixture(
			unittest.WithExecutorID(ms.ExeID),
			unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(ms.LatestFinalizedBlock))))
		block.SetPayload(flow.Payload{
			Receipts: []*flow.ExecutionReceipt{receipt},
		})
		// TODO: replace this with block.ID(), for now IncoroporatedBlockID == ExecutionResult.BlockID
		emergencySealingCandidates[i] = receipt.ExecutionResult.BlockID
		ms.Extend(&block)
		delete(ms.PendingApprovals[receipt.ExecutionResult.ID()], uint64(len(receipt.ExecutionResult.Chunks)-1))
		ms.LatestFinalizedBlock = &block
	}

	// at this point we have results without enough approvals
	// no sealable results expected
	results, _, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	ms.Assert().Empty(results, "expecting no sealable result")

	// setup a new finalized block which is new enough that satisfies emergency sealing condition
	for i := 0; i < DefaultEmergencySealingThreshold; i++ {
		block := unittest.BlockWithParentFixture(ms.LatestFinalizedBlock.Header)
		ms.Extend(&block)
		ms.LatestFinalizedBlock = &block
	}

	// once emergency sealing is active and ERs are deep enough in chain
	// we are expecting all stalled seals to be selected as candidates
	results, _, err = ms.matching.sealableResults()
	ms.Require().NoError(err)
	ms.Require().Equal(len(emergencySealingCandidates), len(results), "expecting valid number of sealable results")
	for _, id := range emergencySealingCandidates {
		matched := false
		for _, ir := range results {
			if ir.IncorporatedBlockID == id {
				matched = true
				break
			}
		}
		ms.Assert().True(matched, "expect to find IR with valid ID")
	}
}

// TestRequestPendingReceipts tests matching.Engine.requestPendingReceipts():
//   * generate n=100 consecutive blocks, where the first one is sealed and the last one is final
func (ms *MatchingSuite) TestRequestPendingReceipts() {
	// create blocks
	n := 100
	orderedBlocks := make([]flow.Block, 0, n)
	parentBlock := ms.UnfinalizedBlock
	for i := 0; i < n; i++ {
		block := unittest.BlockWithParentFixture(parentBlock.Header)
		ms.Blocks[block.ID()] = &block
		orderedBlocks = append(orderedBlocks, block)
		parentBlock = block
	}

	// progress latest sealed and latest finalized:
	ms.LatestSealedBlock = orderedBlocks[0]
	ms.LatestFinalizedBlock = &orderedBlocks[n-1]

	// Expecting all blocks to be requested: from sealed height + 1 up to (incl.) latest finalized
	for i := 1; i < n; i++ {
		id := orderedBlocks[i].ID()
		ms.requester.On("EntityByID", id, mock.Anything).Return().Once()
	}
	ms.SealsPL.On("All").Return([]*flow.IncorporatedResultSeal{}).Maybe()

	_, _, err := ms.matching.requestPendingReceipts()
	ms.Require().NoError(err, "should request results for pending blocks")
	ms.requester.AssertExpectations(ms.T()) // asserts that requester.EntityByID(<blockID>, filter.Any) was called
}

// TestRequestPendingApprovals checks that requests are sent only for chunks
// that have not collected enough approvals yet, and are sent only to the
// verifiers assigned to those chunks. It also checks that the threshold and
// rate limiting is respected.
func (ms *MatchingSuite) TestRequestPendingApprovals() {

	// n is the total number of blocks and incorporated-results we add to the
	// chain and mempool
	n := 100

	// s is the number of incorporated results that have already collected
	// enough approval for every chunk, so they should not require any approval
	// requests
	s := 50

	// create blocks
	unsealedFinalizedBlocks := make([]flow.Block, 0, n)
	parentBlock := ms.UnfinalizedBlock
	for i := 0; i < n; i++ {
		block := unittest.BlockWithParentFixture(parentBlock.Header)
		ms.Blocks[block.ID()] = &block
		unsealedFinalizedBlocks = append(unsealedFinalizedBlocks, block)
		parentBlock = block
	}

	// progress latest sealed and latest finalized:
	ms.LatestSealedBlock = unsealedFinalizedBlocks[0]
	ms.LatestFinalizedBlock = &unsealedFinalizedBlocks[n-1]

	// add an unfinalized block; it shouldn't require an approval request
	unfinalizedBlock := unittest.BlockWithParentFixture(parentBlock.Header)
	ms.Blocks[unfinalizedBlock.ID()] = &unfinalizedBlock

	// we will assume that all chunks are assigned to the same two verifiers.
	verifiers := unittest.IdentifierListFixture(2)

	// the matching engine requires approvals from both verifiers for each chunk
	ms.matching.requiredApprovalsForSealConstruction = 2

	// expectedRequests collects the set of ApprovalRequests that should be sent
	expectedRequests := []*messages.ApprovalRequest{}

	// populate the incorporated-results mempool with:
	// - 50 that have collected two signatures per chunk
	// - 25 that have collected only one signature
	// - 25 that have collected no signatures
	//
	// each chunk is assigned to both verifiers we defined above
	//
	// we populate expectedRequests with requests for chunks that are missing
	// signatures, and that are below the approval request threshold.
	//
	//     sealed          unsealed/finalized
	// |              ||                        |
	// 1 <- 2 <- .. <- s <- s+1 <- .. <- n-t <- n
	//                 |                  |
	//                    expected reqs
	for i := 0; i < n; i++ {

		// Create an incorporated result for unsealedFinalizedBlocks[i].
		// By default the result will contain 17 chunks.
		ir := unittest.IncorporatedResult.Fixture(
			unittest.IncorporatedResult.WithResult(
				unittest.ExecutionResultFixture(
					unittest.WithBlock(&unsealedFinalizedBlocks[i]),
				),
			),
			unittest.IncorporatedResult.WithIncorporatedBlockID(
				unsealedFinalizedBlocks[i].ID(),
			),
		)

		assignment := chunks.NewAssignment()

		for _, chunk := range ir.Result.Chunks {

			// assign the verifier to this chunk
			assignment.Add(chunk, verifiers)
			ms.Assigner.On("Assign", ir.Result, ir.IncorporatedBlockID).Return(assignment, nil)

			if i < s {
				// the first s results receive 2 signatures per chunk
				ir.AddSignature(chunk.Index, verifiers[0], unittest.SignatureFixture())
				ir.AddSignature(chunk.Index, verifiers[1], unittest.SignatureFixture())
			} else {
				if i < s+25 {
					// the next 25 have only 1 signature
					ir.AddSignature(chunk.Index, verifiers[0], unittest.SignatureFixture())
				}
				// all these chunks are missing at least one signature so we
				// expect requests to be sent out if the result's block is below
				// the threshold
				if i < n-int(ms.matching.approvalRequestsThreshold) {
					expectedRequests = append(expectedRequests,
						&messages.ApprovalRequest{
							ResultID:   ir.Result.ID(),
							ChunkIndex: chunk.Index,
						})
				}
			}
		}

		ms.PendingResults[ir.ID()] = ir
	}

	// exp is the number of requests that we expect
	exp := n - s - int(ms.matching.approvalRequestsThreshold)

	// add an incorporated-result for a block that was already sealed. We
	// expect that no approval requests will be sent for this result, even if it
	// hasn't collected any approvals yet.
	sealedBlockIR := unittest.IncorporatedResult.Fixture(
		unittest.IncorporatedResult.WithResult(
			unittest.ExecutionResultFixture(
				unittest.WithBlock(&ms.LatestSealedBlock),
			),
		),
		unittest.IncorporatedResult.WithIncorporatedBlockID(
			ms.LatestSealedBlock.ID(),
		),
	)
	ms.PendingResults[sealedBlockIR.ID()] = sealedBlockIR

	// add an incorporated-result for an unfinalized block. It should not
	// generate any requests either.
	unfinalizedBlockIR := unittest.IncorporatedResult.Fixture(
		unittest.IncorporatedResult.WithResult(
			unittest.ExecutionResultFixture(
				unittest.WithBlock(&unfinalizedBlock),
			),
		),
		unittest.IncorporatedResult.WithIncorporatedBlockID(
			unfinalizedBlock.ID(),
		),
	)
	ms.PendingResults[unfinalizedBlock.ID()] = unfinalizedBlockIR

	// wire-up the approval requests conduit to keep track of all sent requests
	// and check that the targets match with the verifiers who haven't signed
	requests := []*messages.ApprovalRequest{}
	conduit := &mocknetwork.Conduit{}
	// mock the Publish method when requests are sent to 2 verifiers
	conduit.On("Publish", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			// collect the request
			ar, ok := args[0].(*messages.ApprovalRequest)
			ms.Assert().True(ok)
			requests = append(requests, ar)
		})
	// mock the Publish method when requests are sent to only 1 verifier (which
	// should be verifiers[1] by design, because we only included a signature
	// from verifiers[0])
	conduit.On("Publish", mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			// collect the request
			ar, ok := args[0].(*messages.ApprovalRequest)
			ms.Assert().True(ok)
			requests = append(requests, ar)

			// check that the target is the verifier for which the approval is
			// missing
			target, ok := args[1].(flow.Identifier)
			ms.Assert().True(ok)
			ms.Assert().Equal(verifiers[1], target)
		})
	ms.matching.approvalConduit = conduit

	_, err := ms.matching.requestPendingApprovals()
	ms.Require().NoError(err)

	// first time it goes through, no requests should be made because of the
	// blackout period
	ms.Assert().Len(requests, 0)

	// Check the request tracker
	ms.Assert().Equal(exp, len(ms.matching.requestTracker.index))
	for _, expectedRequest := range expectedRequests {
		requestItem := ms.matching.requestTracker.Get(
			expectedRequest.ResultID,
			expectedRequest.ChunkIndex,
		)
		ms.Assert().Equal(uint(0), requestItem.Requests)
	}

	// wait for the max blackout period to elapse and retry
	time.Sleep(3 * time.Second)
	_, err = ms.matching.requestPendingApprovals()
	ms.Require().NoError(err)

	// now we expect that requests have been sent for the chunks that haven't
	// collected enough approvals
	ms.Assert().Len(requests, len(expectedRequests))

	// Check the request tracker
	ms.Assert().Equal(exp, len(ms.matching.requestTracker.index))
	for _, expectedRequest := range expectedRequests {
		requestItem := ms.matching.requestTracker.Get(
			expectedRequest.ResultID,
			expectedRequest.ChunkIndex,
		)
		ms.Assert().Equal(uint(1), requestItem.Requests)
	}
}

// incorporatedResult returns a testify `argumentMatcher` that only accepts an
// IncorporatedResult with the given parameters
func incorporatedResult(blockID flow.Identifier, result *flow.ExecutionResult) interface{} {
	return mock.MatchedBy(func(ir *flow.IncorporatedResult) bool {
		return ir.IncorporatedBlockID == blockID && ir.Result.ID() == result.ID()
	})
}
