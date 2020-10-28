// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package matching

import (
	"fmt"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	mempool "github.com/onflow/flow-go/module/mempool/mock"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	realproto "github.com/onflow/flow-go/state/protocol"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storerr "github.com/onflow/flow-go/storage"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// 1. Matching engine should validate the incoming receipt (aka ExecutionReceipt):
//     1. it should stores it to the mempool if valid
//     2. it should ignore it when:
//         1. the origin is invalid
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
	suite.Suite

	// define IDs for consensus, execution, and verification nodes
	conID flow.Identifier
	exeID flow.Identifier
	verID flow.Identifier

	// holds of node identities by ID
	identities map[flow.Identifier]*flow.Identity

	// a list of identities that are assigned the verifier role
	approvers flow.IdentityList

	// state and snapshot are used by the engine to check identities
	state    *protocol.State
	snapshot *protocol.Snapshot

	// results is the backend storage for the mock resultDB
	results   map[flow.Identifier]*flow.ExecutionResult
	resultsDB *storage.ExecutionResults

	// blocks is the backend storage for the mock headersDB and indexDB
	blocks map[flow.Identifier]*flow.Block
	// holds the last sealed block
	lastSealed *flow.Header

	// mock DBs for headers and indexes
	headersDB *storage.Headers
	indexDB   *storage.Index

	// pendingResults is the backend storage for the resultsPL
	pendingResults map[flow.Identifier]*flow.IncorporatedResult
	resultsPL      *mempool.IncorporatedResults
	sealsPL        *mempool.IncorporatedResultSeals

	// mock pools for receipts and approvals
	receiptsPL  *mempool.Receipts
	approvalsPL *mempool.Approvals

	// mock requester used to test requesting missing receipts
	requester *module.Requester

	// mock chunk assigner
	assigner *module.ChunkAssigner

	matching *Engine
}

func (ms *MatchingSuite) SetupTest() {

	unit := engine.NewUnit()
	log := zerolog.New(os.Stderr)
	metrics := metrics.NewNoopCollector()

	// asign node identities
	con := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	exe := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	ver := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

	ms.conID = con.NodeID
	ms.exeID = exe.NodeID
	ms.verID = ver.NodeID

	ms.identities = make(map[flow.Identifier]*flow.Identity)
	ms.identities[ms.conID] = con
	ms.identities[ms.exeID] = exe
	ms.identities[ms.verID] = ver

	// assign 4 nodes to the verification role
	ms.approvers = unittest.IdentityListFixture(4, unittest.WithRole(flow.RoleVerification))

	// instantiate the mock snapshot and state objects
	ms.state = &protocol.State{}
	ms.state.On("AtBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) realproto.Snapshot {
			return ms.snapshot
		},
		nil,
	)

	ms.snapshot = &protocol.Snapshot{}
	ms.snapshot.On("Identity", mock.Anything).Return(
		func(nodeID flow.Identifier) *flow.Identity {
			identity := ms.identities[nodeID]
			return identity
		},
		func(nodeID flow.Identifier) error {
			_, found := ms.identities[nodeID]
			if !found {
				return fmt.Errorf("could not get identity (%x)", nodeID)
			}
			return nil
		},
	)

	// instantiate the results DB and its backend
	ms.results = make(map[flow.Identifier]*flow.ExecutionResult)
	ms.resultsDB = &storage.ExecutionResults{}
	ms.resultsDB.On("ByID", mock.Anything).Return(
		func(resultID flow.Identifier) *flow.ExecutionResult {
			return ms.results[resultID]
		},
		func(resultID flow.Identifier) error {
			_, found := ms.results[resultID]
			if !found {
				return storerr.ErrNotFound
			}
			return nil
		},
	)

	// instantiate the blocks store and the associated mock headersDB and
	// indexDB
	ms.blocks = make(map[flow.Identifier]*flow.Block)
	// Default last sealed block. In most tests, we don't really care what the
	// block contains. Just that its height is 0.
	ms.lastSealed = &flow.Header{}

	ms.headersDB = &storage.Headers{}
	ms.headersDB.On("ByBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) *flow.Header {
			block, found := ms.blocks[blockID]
			if !found {
				return nil
			}
			return block.Header
		},
		func(blockID flow.Identifier) error {
			_, found := ms.blocks[blockID]
			if !found {
				return storerr.ErrNotFound
			}
			return nil
		},
	)
	ms.headersDB.On("ByHeight", mock.Anything).Return(
		func(blockHeight uint64) *flow.Header {
			for _, b := range ms.blocks {
				if b.Header.Height == blockHeight {
					return b.Header
				}
			}
			return nil
		},
		func(blockHeight uint64) error {
			for _, b := range ms.blocks {
				if b.Header.Height == blockHeight {
					return nil
				}
			}
			return storerr.ErrNotFound
		},
	)
	ms.headersDB.On("GetLastSealed").Return(
		func() *flow.Header {
			return ms.lastSealed
		},
		func() error {
			return nil
		},
	)

	ms.indexDB = &storage.Index{}
	ms.indexDB.On("ByBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) *flow.Index {
			block, found := ms.blocks[blockID]
			if !found {
				return nil
			}
			if block.Payload == nil {
				return nil
			}
			return block.Payload.Index()
		},
		func(blockID flow.Identifier) error {
			block, found := ms.blocks[blockID]
			if !found {
				return storerr.ErrNotFound
			}
			if block.Payload == nil {
				return storerr.ErrNotFound
			}
			return nil
		},
	)

	// backend for mock results mempool
	ms.pendingResults = make(map[flow.Identifier]*flow.IncorporatedResult)

	ms.resultsPL = &mempool.IncorporatedResults{}
	ms.resultsPL.On("Size").Return(uint(0)) // only for metrics
	ms.resultsPL.On("All").Return(
		func() []*flow.IncorporatedResult {
			results := make([]*flow.IncorporatedResult, 0, len(ms.pendingResults))
			for _, result := range ms.pendingResults {
				results = append(results, result)
			}
			return results
		},
	)

	ms.sealsPL = &mempool.IncorporatedResultSeals{}
	ms.sealsPL.On("Size").Return(uint(0)) // only for metrics

	ms.receiptsPL = &mempool.Receipts{}
	ms.receiptsPL.On("Size").Return(uint(0)) // only for metrics

	ms.approvalsPL = &mempool.Approvals{}
	ms.approvalsPL.On("Size").Return(uint(0)) // only for metrics

	ms.requester = new(module.Requester)

	ms.assigner = &module.ChunkAssigner{}

	ms.matching = &Engine{
		unit:                    unit,
		log:                     log,
		engineMetrics:           metrics,
		mempool:                 metrics,
		metrics:                 metrics,
		state:                   ms.state,
		requester:               ms.requester,
		resultsDB:               ms.resultsDB,
		headersDB:               ms.headersDB,
		indexDB:                 ms.indexDB,
		incorporatedResults:     ms.resultsPL,
		receipts:                ms.receiptsPL,
		approvals:               ms.approvalsPL,
		seals:                   ms.sealsPL,
		checkingSealing:         atomic.NewBool(false),
		requestReceiptThreshold: 10,
		maxUnsealedResults:      200,
		assigner:                ms.assigner,
		requireApprovals:        true,
	}
}

// Test that we reject receipts that were not sent by their execution node (as
// specified by the ExecutorID field)
func (ms *MatchingSuite) TestOnReceiptInvalidOrigin() {

	// this receipt has a random executor ID
	receipt := unittest.ExecutionReceiptFixture()

	// try to submit it from another random origin
	err := ms.matching.onReceipt(unittest.IdentifierFixture(), receipt)
	ms.Require().Error(err, "should reject receipt with mismatching origin and executor")

	ms.receiptsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// Test that we reject receipts for unknown blocks without generating an error
func (ms *MatchingSuite) TestOnReceiptUnknownBlock() {

	// This receipt has a random block ID, so the matching engine won't find it.
	receipt := unittest.ExecutionReceiptFixture()

	// onReceipt should reject the receipt without throwing an error
	err := ms.matching.onReceipt(receipt.ExecutorID, receipt)
	ms.Require().NoError(err, "should ignore receipt for unknown block")

	ms.receiptsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// Check that we reject receipts for blocks that are already sealed
func (ms *MatchingSuite) TestOnReceiptSealedResult() {

	// save a block and pretend that it is already sealed
	block := unittest.BlockFixture()
	ms.blocks[block.ID()] = &block
	ms.lastSealed = block.Header

	// create a receipt for this block
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithBlock(&block),
	)

	// onReceipt should reject the receipt without throwing and error
	err := ms.matching.onReceipt(receipt.ExecutorID, receipt)
	ms.Require().NoError(err, "should ignore receipt for sealed result")

	ms.receiptsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// Check that we reject receipts that were created by non-executor nodes
func (ms *MatchingSuite) TestOnReceiptInvalidRole() {

	// save a block that is not already sealed
	block := unittest.BlockFixture()
	ms.blocks[block.ID()] = &block

	// create a receipt from a verifier node
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithBlock(&block),
		unittest.WithExecutorID(ms.verID),
	)

	// the receipt should be rejected with an error
	err := ms.matching.onReceipt(receipt.ExecutorID, receipt)
	ms.Require().Error(err, "should reject receipt from wrong node role")

	ms.receiptsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// Check that we reject receipts from non-staked executors
func (ms *MatchingSuite) TestOnReceiptUnstakedExecutor() {

	// save a block that is not already sealed
	block := unittest.BlockFixture()
	ms.blocks[block.ID()] = &block

	// create a receipt from an executor node
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithBlock(&block),
		unittest.WithExecutorID(ms.exeID),
	)

	// assign 0 stake to this executor node
	ms.identities[ms.exeID].Stake = 0

	// the receipt should be rejected with an error
	err := ms.matching.onReceipt(receipt.ExecutorID, receipt)
	ms.Require().Error(err, "should reject receipt from unstaked node")

	ms.receiptsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// Test that we reject receipts that are already pooled
func (ms *MatchingSuite) TestOnReceiptPendingReceipt() {

	// save a block that is not already sealed
	block := unittest.BlockFixture()
	ms.blocks[block.ID()] = &block

	// create a receipt from an executor node that has stake
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithBlock(&block),
		unittest.WithExecutorID(ms.exeID),
	)

	// setup the receipts mempool to check if we attempted to add the receipt to
	// the mempool, and return false as if it was already in the mempool
	ms.receiptsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			added := args.Get(0).(*flow.ExecutionReceipt)
			ms.Assert().Equal(receipt, added)
		},
	).Return(false)

	// onReceipt should return immediately after trying to insert the receipt,
	// but without throwing any errors
	err := ms.matching.onReceipt(receipt.ExecutorID, receipt)
	ms.Require().NoError(err, "should ignore already pending receipt")

	ms.receiptsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// Check that we don't proceed when the corresponding incorporated-results is
// already in the mempool.
//
// NOTE that this is only a temporary step in phase 2 of the verification and
// sealing roadmap (https://www.notion.so/dapperlabs/Engineering-Roadmap-Proposal-e59b4f26168d43b68b865137ca200844)
// In the future, the incorporated-results mempool will be populated by the
// finalizer.
func (ms *MatchingSuite) TestOnReceiptPendingResult() {

	// save a block that is not already sealed
	block := unittest.BlockFixture()
	ms.blocks[block.ID()] = &block

	// create a receipt from an executor node that has stake
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithBlock(&block),
		unittest.WithExecutorID(ms.exeID),
	)

	// setup the receipts mempool to check if we attempted to add the receipt to
	// the mempool
	ms.receiptsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			added := args.Get(0).(*flow.ExecutionReceipt)
			ms.Assert().Equal(receipt, added)
		},
	).Return(true)

	// setup the results mempool to check if we attempted to insert the
	// incorporated result, and return false as if it was already in the mempool
	ms.resultsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			incorporatedResult := args.Get(0).(*flow.IncorporatedResult)
			ms.Assert().Equal(incorporatedResult.Result, &receipt.ExecutionResult)
		},
	).Return(false, nil)

	// onReceipt should return immediately after trying to pool the result, but
	// without throwing any errors
	err := ms.matching.onReceipt(receipt.ExecutorID, receipt)
	ms.Require().NoError(err, "should ignore receipt for already pending result")

	ms.receiptsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
}

// Check that a valid ExecutionReceipt is added to the receipts mempool, and
// that a corresponding IncorporatedResult is added to the results mempool.
func (ms *MatchingSuite) TestOnReceiptValid() {

	// save a block that is not already sealed
	block := unittest.BlockFixture()
	ms.blocks[block.ID()] = &block

	// create a receipt from an executor node that has stake
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithBlock(&block),
		unittest.WithExecutorID(ms.exeID),
	)

	// setup the receipts mempool to check if we attempted to add the receipt to
	// the mempool
	ms.receiptsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			added := args.Get(0).(*flow.ExecutionReceipt)
			ms.Assert().Equal(receipt, added)
		},
	).Return(true)

	// setup the results mempool to check if we attempted to add the
	// incorporated result
	ms.resultsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			incorporatedResult := args.Get(0).(*flow.IncorporatedResult)
			ms.Assert().Equal(incorporatedResult.Result, &receipt.ExecutionResult)
		},
	).Return(true, nil)

	// onReceipt should run to completion without throwing an error
	err := ms.matching.onReceipt(receipt.ExecutorID, receipt)
	ms.Require().NoError(err, "should add receipt and result to mempool if valid")

	ms.receiptsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
}

// Test that we reject approval that were not sent by their approver.
func (ms *MatchingSuite) TestApprovalInvalidOrigin() {

	// this approval has a random approver ID
	approval := unittest.ResultApprovalFixture()

	// try to submit it from another random origin
	err := ms.matching.onApproval(unittest.IdentifierFixture(), approval)
	ms.Require().Error(err, "should reject approval with mismatching origin and approver")

	ms.approvalsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// Test that approvals for unknown blocks are added to the mempool, but without
// doing any other work.
func (ms *MatchingSuite) TestApprovalUnknownBlock() {

	// This approval has a random block ID, so the matching engine won't find
	// it.
	approval := unittest.ResultApprovalFixture()

	// Make sure the approval is added to the cache for future processing
	ms.approvalsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			added := args.Get(0).(*flow.ResultApproval)
			ms.Assert().Equal(approval, added)
		},
	).Return(false, nil)

	// onApproval should not throw an error
	err := ms.matching.onApproval(approval.Body.ApproverID, approval)
	ms.Require().NoError(err, "should cache approvals for unknown blocks")

	ms.approvalsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
}

// Check that we reject approvals for blocks that are already sealed.
func (ms *MatchingSuite) TestOnApprovalSealedResult() {

	// save a block and pretend that it is already sealed
	block := unittest.BlockFixture()
	ms.blocks[block.ID()] = &block
	ms.lastSealed = block.Header

	// create an approval for this block
	approval := unittest.ResultApprovalFixture(
		unittest.WithAttestationBlock(&block),
	)

	// onApproval should reject the approval without throwing an error
	err := ms.matching.onApproval(approval.Body.ApproverID, approval)
	ms.Require().NoError(err, "should ignore approval for sealed result")

	ms.approvalsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// Check that we reject approvals that were created by non-verifier nodes.
func (ms *MatchingSuite) TestOnApprovalInvalidRole() {

	// save a block that is not already sealed
	block := unittest.BlockFixture()
	ms.blocks[block.ID()] = &block

	// create an approval from an execution node
	approval := unittest.ResultApprovalFixture(
		unittest.WithAttestationBlock(&block),
		unittest.WithApproverID(ms.exeID),
	)

	// the approval should be rejected with an error
	err := ms.matching.onApproval(approval.Body.ApproverID, approval)
	ms.Require().Error(err, "should reject approval from wrong node role")

	ms.approvalsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// Check that we reject approvals from non-staked verifiers
func (ms *MatchingSuite) TestOnApprovalInvalidStake() {

	// save a block that is not already sealed
	block := unittest.BlockFixture()
	ms.blocks[block.ID()] = &block

	// create an approval from a verifier node
	approval := unittest.ResultApprovalFixture(
		unittest.WithAttestationBlock(&block),
		unittest.WithApproverID(ms.verID),
	)

	// assign 0 stake to this verifier
	ms.identities[ms.verID].Stake = 0

	// the approval should be rejected with an error
	err := ms.matching.onApproval(approval.Body.ApproverID, approval)
	ms.Require().Error(err, "should reject approval from unstaked approver")

	ms.approvalsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// Test that we reject approvals that are already pooled
func (ms *MatchingSuite) TestOnApprovalPendingApproval() {

	// save a block that is not already sealed
	block := unittest.BlockFixture()
	ms.blocks[block.ID()] = &block

	// create an approval from a verifier node that has stake
	approval := unittest.ResultApprovalFixture(
		unittest.WithAttestationBlock(&block),
		unittest.WithApproverID(ms.verID),
	)

	// setup the approvals mempool to check that we attempted to add the
	// approval, and return false as if it was already in the mempool
	ms.approvalsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			added := args.Get(0).(*flow.ResultApproval)
			ms.Assert().Equal(approval, added)
		},
	).Return(false, nil)

	// onApproval should return immediately after trying to insert the approval,
	// without throwing any errors
	err := ms.matching.onApproval(approval.Body.ApproverID, approval)
	ms.Require().NoError(err, "should ignore approval if already pending")

	ms.approvalsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
}

// Test that valid approvals are added to the mempool
func (ms *MatchingSuite) TestOnApprovalValid() {

	// save a block that is not already sealed
	block := unittest.BlockFixture()
	ms.blocks[block.ID()] = &block

	// create an approval from a verifier node that has stake
	approval := unittest.ResultApprovalFixture(
		unittest.WithAttestationBlock(&block),
		unittest.WithApproverID(ms.verID),
	)

	// check that the approval is correctly added
	ms.approvalsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			added := args.Get(0).(*flow.ResultApproval)
			ms.Assert().Equal(approval, added)
		},
	).Return(true, nil)

	// onApproval should run to completion without throwing any errors
	err := ms.matching.onApproval(approval.Body.ApproverID, approval)
	ms.Require().NoError(err, "should add approval to mempool if valid")

	ms.approvalsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
}

// Test that sealableResults returns an empty list if the results mempool is
// empty.
func (ms *MatchingSuite) TestSealableResultsEmptyMempools() {

	// try to get matched results with nothing in memory pools
	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err, "should not error with empty mempools")
	ms.Assert().Empty(results, "should not have matched results with empty mempools")
}

// Test that we don't seal results for unknown blocks
func (ms *MatchingSuite) TestSealableResultsMissingBlock() {

	// try to seal a result for which we don't have the block
	incorporatedResult := unittest.IncorporatedResultFixture()
	ms.pendingResults[incorporatedResult.ID()] = incorporatedResult

	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)

	ms.Assert().Empty(results, "should not select result with unknown block")
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Rem", 0)
}

// Test that we don't seal results with an unknown previous result
func (ms *MatchingSuite) TestSealableResulstUnknownPrevious() {

	// save a block
	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block

	// add an IncorporatedResult for it in the mempool
	incorporatedResult := unittest.IncorporatedResultForBlockFixture(&block)
	ms.pendingResults[incorporatedResult.ID()] = incorporatedResult

	// check that it is looking for the previous result in the mempool and in
	// storage
	ms.resultsPL.On("ByResultID", mock.Anything).Run(
		func(args mock.Arguments) {
			previousResultID := args.Get(0).(flow.Identifier)
			ms.Assert().Equal(incorporatedResult.Result.PreviousResultID, previousResultID)
		},
	).Return(nil, nil)

	ms.resultsDB.On("ByID", mock.Anything).Run(
		func(args mock.Arguments) {
			previousResultID := args.Get(0).(flow.Identifier)
			ms.Assert().Equal(incorporatedResult.Result.PreviousResultID, previousResultID)
		},
	).Return(nil, nil)

	// sealableResults should skip this result without returning an error
	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)

	ms.Assert().Empty(results, "should not select result with unsealed previous")
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Rem", 0)
}

// let R1 be a result that references block A, and R2 be R1's parent result.
// Then R2 should reference A's parent. Otherwise the result should be removed
// from the mempool.
func (ms *MatchingSuite) TestSealableResultsInvalidSubgraph() {

	// save a block
	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block

	// create a previous result for a random block and add it to the storage
	previous := unittest.ExecutionResultFixture()
	ms.results[previous.ID()] = previous

	// create an incorporated result that references the block and the previous
	// result
	incorporatedResult := unittest.IncorporatedResultFixture(
		unittest.WithIRBlock(&block),
		unittest.WithIRPrevious(previous.ID()),
	)
	ms.pendingResults[incorporatedResult.ID()] = incorporatedResult

	// check that it is looking for the previous result in the mempool before
	// storage
	ms.resultsPL.On("ByResultID", mock.Anything).Run(
		func(args mock.Arguments) {
			previousResultID := args.Get(0).(flow.Identifier)
			ms.Assert().Equal(incorporatedResult.Result.PreviousResultID, previousResultID)
		},
	).Return(nil, nil)

	// check that we are trying to remove the incorporated result from mempool
	ms.resultsPL.On("Rem", mock.Anything).Run(
		func(args mock.Arguments) {
			incResult := args.Get(0).(*flow.IncorporatedResult)
			ms.Assert().Equal(incorporatedResult.ID(), incResult.ID())
		},
	).Return(true)

	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)

	ms.Assert().Empty(results, "should not select result with invalid subgraph")
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Rem", 1)
}

// If an IncorporatedResult contains a result with an inconsistant number of
// chuncks, it should be skipped and removed from the mempool.
func (ms *MatchingSuite) TestSealResultInvalidChunks() {

	// save a block
	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block

	// create a previous result that references the block's parent, and add it
	// to storage
	previous := unittest.ExecutionResultFixture()
	previous.BlockID = block.Header.ParentID
	ms.results[previous.ID()] = previous

	// create an incorporated result that references the block and the previous
	// result
	incorporatedResult := unittest.IncorporatedResultFixture(
		unittest.WithIRBlock(&block),
		unittest.WithIRPrevious(previous.ID()),
	)

	// add an extra chunk to it
	chunk := unittest.ChunkFixture(block.ID())
	chunk.Index = uint64(len(block.Payload.Guarantees))
	incorporatedResult.Result.Chunks = append(incorporatedResult.Result.Chunks, chunk)

	// add the incorporated result to the mempool
	ms.pendingResults[incorporatedResult.ID()] = incorporatedResult

	// check that it is looking for the previous result in the mempool, and
	// return nil (it will be found in storage)
	ms.resultsPL.On("ByResultID", mock.Anything).Run(
		func(args mock.Arguments) {
			previousResultID := args.Get(0).(flow.Identifier)
			ms.Assert().Equal(incorporatedResult.Result.PreviousResultID, previousResultID)
		},
	).Return(nil, nil)

	// check that we are trying to remove the incorporated result from mempool
	ms.resultsPL.On("Rem", mock.Anything).Run(
		func(args mock.Arguments) {
			incResult := args.Get(0).(*flow.IncorporatedResult)
			ms.Assert().Equal(incorporatedResult.ID(), incResult.ID())
		},
	).Return(true)

	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)

	ms.Assert().Empty(results, "should not select result with invalid number of chunks")
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Rem", 1)
}

// If a result corresponds to a block with no payload, and all the above
// conditions are satisfied, it should be sealable.
func (ms *MatchingSuite) TestSealableResultsNoPayload() {

	// save a block
	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block

	// create a previous result that references the block's parent, and add it
	// to storage
	previous := unittest.ExecutionResultFixture()
	previous.BlockID = block.Header.ParentID
	ms.results[previous.ID()] = previous

	// create an incorporated result that references the block and the previous
	// result
	incorporatedResult := unittest.IncorporatedResultFixture(
		unittest.WithIRBlock(&block),
		unittest.WithIRPrevious(previous.ID()),
	)

	// add the incorporated result to the mempool
	ms.pendingResults[incorporatedResult.ID()] = incorporatedResult

	// check that it is looking for the previous result in the mempool, and
	// return nil (it will be found in storage)
	ms.resultsPL.On("ByResultID", mock.Anything).Run(
		func(args mock.Arguments) {
			previousResultID := args.Get(0).(flow.Identifier)
			ms.Assert().Equal(incorporatedResult.Result.PreviousResultID, previousResultID)
		},
	).Return(nil, nil)

	// Setup the assigner to return an emtpy assignment for this result. This
	// happens if the block has not payload.
	assignment := chunks.NewAssignment()
	ms.assigner.On("Assign", incorporatedResult.Result, incorporatedResult.IncorporatedBlockID).Return(assignment, nil)

	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	if ms.Assert().Len(results, 1, "should select result for empty block") {
		sealable := results[0]
		ms.Assert().Equal(incorporatedResult, sealable)
	}
}

// Check that approvals from unassigned verifiers are not counted towards
// accepting a chunk.
func (ms *MatchingSuite) TestSealableResultsUnassignedVerifiers() {

	// save a block
	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block

	// create a previous result that references the block's parent, and add it
	// to storage
	previous := unittest.ExecutionResultFixture()
	previous.BlockID = block.Header.ParentID
	ms.results[previous.ID()] = previous

	// create an incorporated result that references the block and the previous
	// result
	incorporatedResult := unittest.IncorporatedResultFixture(
		unittest.WithIRBlock(&block),
		unittest.WithIRPrevious(previous.ID()),
	)

	// add the incorporated result to the mempool
	ms.pendingResults[incorporatedResult.ID()] = incorporatedResult

	// check that it is looking for the previous result in the mempool, and
	// return nil (it will be found in storage)
	ms.resultsPL.On("ByResultID", mock.Anything).Run(
		func(args mock.Arguments) {
			previousResultID := args.Get(0).(flow.Identifier)
			ms.Assert().Equal(incorporatedResult.Result.PreviousResultID, previousResultID)
		},
	).Return(nil, nil)

	// list of 3 approvers
	assignedApprovers := ms.approvers[:3]

	// create assignment with 3 verification nodes assigned to every chunk
	assignment := chunks.NewAssignment()
	for _, chunk := range incorporatedResult.Result.Chunks {
		assignment.Add(chunk, assignedApprovers.NodeIDs())
	}
	// mock assigner
	ms.assigner.On("Assign", incorporatedResult.Result, incorporatedResult.IncorporatedBlockID).Return(assignment, nil)

	// use a real approval mempool (not mock) because we need its indexing logic
	realApprovalPool, err := stdmap.NewApprovals(1000)
	ms.Require().NoError(err)
	ms.matching.approvals = realApprovalPool

	// approve every chunk by an unassigned verifier.
	unassignedApprover := ms.approvers[3]
	for index := uint64(0); index < uint64(len(incorporatedResult.Result.Chunks)); index++ {
		approval := unittest.ResultApprovalFixture()
		approval.Body.BlockID = block.Header.ID()
		approval.Body.ExecutionResultID = incorporatedResult.Result.ID()
		approval.Body.ApproverID = unassignedApprover.NodeID
		approval.Body.ChunkIndex = index
		_, err := ms.matching.approvals.Add(approval)
		ms.Require().NoError(err)
	}

	// the result should not be matched
	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	ms.Assert().Len(results, 0, "should not count approvals from unassigned verifiers")
}

// Insert an approval from a node that wasn't a staked verifier at that block
// (this may occur when the block wasn't know when the node received the
// approval). Ensure that the approval is removed from the mempool when the
// block becomes known.
func (ms *MatchingSuite) TestRemoveApprovalsFromInvalidVerifiers() {

	// save a block
	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block

	// create a previous result that references the block's parent, and add it
	// to storage
	previous := unittest.ExecutionResultFixture()
	previous.BlockID = block.Header.ParentID
	ms.results[previous.ID()] = previous

	// create an incorporated result that references the block and the previous
	// result
	incorporatedResult := unittest.IncorporatedResultFixture(
		unittest.WithIRBlock(&block),
		unittest.WithIRPrevious(previous.ID()),
	)

	// add the incorporated result to the mempool
	ms.pendingResults[incorporatedResult.ID()] = incorporatedResult

	// check that it is looking for the previous result in the mempool, and
	// return nil (it will be found in storage)
	ms.resultsPL.On("ByResultID", mock.Anything).Run(
		func(args mock.Arguments) {
			previousResultID := args.Get(0).(flow.Identifier)
			ms.Assert().Equal(incorporatedResult.Result.PreviousResultID, previousResultID)
		},
	).Return(nil, nil)

	// assign each chunk to each approver
	assignment := chunks.NewAssignment()
	for _, chunk := range incorporatedResult.Result.Chunks {
		assignment.Add(chunk, ms.approvers.NodeIDs())
	}
	ms.assigner.On("Assign", incorporatedResult.Result, incorporatedResult.IncorporatedBlockID).Return(assignment, nil)

	// not using mock for approvals pool because we need the internal indexing
	// logic
	realApprovalPool, err := stdmap.NewApprovals(1000)
	ms.Require().NoError(err)
	ms.matching.approvals = realApprovalPool

	// add an approval from an unstaked verifier for the first chunk
	approval := unittest.ResultApprovalFixture()
	approval.Body.BlockID = block.Header.ID()
	approval.Body.ExecutionResultID = incorporatedResult.Result.ID()
	approval.Body.ApproverID = unittest.IdentifierFixture() // this is not a staked verifier
	approval.Body.ChunkIndex = 0
	_, err = ms.matching.approvals.Add(approval)
	ms.Require().NoError(err)

	// the result should not be sealable because the approval comes from an
	// unknown verifier
	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	ms.Assert().Empty(results, "should not select result with insufficient approvals")

	// should have deleted the approval of the first chunk
	ms.Assert().Empty(ms.matching.approvals.All(), "should have removed the approval")
}

// Check that when the requireApprovals flag is true, results without approvals
// are not matched. When requireApprovals is false, the results should be
// matched anyway.
func (ms *MatchingSuite) TestSealableResultsInsufficientApprovals() {

	// save a block
	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block

	// create a previous result that references the block's parent, and add it
	// to storage
	previous := unittest.ExecutionResultFixture()
	previous.BlockID = block.Header.ParentID
	ms.results[previous.ID()] = previous

	// create an incorporated result that references the block and the previous
	// result
	incorporatedResult := unittest.IncorporatedResultFixture(
		unittest.WithIRBlock(&block),
		unittest.WithIRPrevious(previous.ID()),
	)

	// add the incorporated result to the mempool
	ms.pendingResults[incorporatedResult.ID()] = incorporatedResult

	// check that it is looking for the previous result in the mempool, and
	// return nil (it will be found in storage)
	ms.resultsPL.On("ByResultID", mock.Anything).Run(
		func(args mock.Arguments) {
			previousResultID := args.Get(0).(flow.Identifier)
			ms.Assert().Equal(incorporatedResult.Result.PreviousResultID, previousResultID)
		},
	).Return(nil, nil)

	// assign each chunk to each approver
	assignment := chunks.NewAssignment()
	for _, chunk := range incorporatedResult.Result.Chunks {
		assignment.Add(chunk, ms.approvers.NodeIDs())
	}
	ms.assigner.On("Assign", incorporatedResult.Result, incorporatedResult.IncorporatedBlockID).Return(assignment, nil)

	// check that we are looking for chunk approvals, but return nil as if not
	// found
	ms.approvalsPL.On("ByChunk", mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			resultID := args.Get(0).(flow.Identifier)
			ms.Assert().Equal(incorporatedResult.Result.ID(), resultID)
		},
	).Return(nil)

	// with requireApprovals = true ( default test case ), it should not collect
	// any results because we haven't added any approvals to the mempool
	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	ms.Assert().Empty(results, "should not select result with insufficient approvals")

	// with requireApprovals = false,  it should collect the result even if
	// there are no corresponding approvals
	ms.matching.requireApprovals = false
	results, err = ms.matching.sealableResults()
	ms.Require().NoError(err)
	if ms.Assert().Len(results, 1, "should select result when requireApprovals flag is false") {
		sealable := results[0]
		ms.Assert().Equal(incorporatedResult, sealable)
	}
}

// Insert a well-formed incorporated result in the mempool, as well as a
// sufficient number of valid result approvals, and check that the seal is
// correctly generated.
func (ms *MatchingSuite) TestSealValid() {

	// save a block
	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block

	// create a previous result that references the block's parent, and add it
	// to storage
	previous := unittest.ExecutionResultFixture()
	previous.BlockID = block.Header.ParentID
	ms.results[previous.ID()] = previous

	// create an incorporated result that references the block and the previous
	// result
	incorporatedResult := unittest.IncorporatedResultFixture(
		unittest.WithIRBlock(&block),
		unittest.WithIRPrevious(previous.ID()),
	)

	// add the incorporated result to the mempool
	ms.pendingResults[incorporatedResult.ID()] = incorporatedResult

	// check that it is looking for the previous result in the mempool, and
	// return nil (it will be found in storage)
	ms.resultsPL.On("ByResultID", mock.Anything).Run(
		func(args mock.Arguments) {
			previousResultID := args.Get(0).(flow.Identifier)
			ms.Assert().Equal(incorporatedResult.Result.PreviousResultID, previousResultID)
		},
	).Return(nil, nil)

	// assign each chunk to each approver
	assignment := chunks.NewAssignment()
	for _, chunk := range incorporatedResult.Result.Chunks {
		assignment.Add(chunk, ms.approvers.NodeIDs())
	}
	ms.assigner.On("Assign", incorporatedResult.Result, incorporatedResult.IncorporatedBlockID).Return(assignment, nil)

	// not using mock for approvals pool because we need the internal indexing
	// logic
	realApprovalPool, err := stdmap.NewApprovals(1000)
	ms.Require().NoError(err)
	ms.matching.approvals = realApprovalPool

	// add enough approvals for each chunk
	for _, approver := range ms.approvers {
		for index := uint64(0); index < uint64(len(incorporatedResult.Result.Chunks)); index++ {
			approval := unittest.ResultApprovalFixture()
			approval.Body.BlockID = block.Header.ID()
			approval.Body.ExecutionResultID = incorporatedResult.Result.ID()
			approval.Body.ApproverID = approver.NodeID
			approval.Body.ChunkIndex = index
			_, err := ms.matching.approvals.Add(approval)
			ms.Require().NoError(err)
		}
	}

	// Get the sealable results. There should be one.
	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	ms.Assert().Len(results, 1, "should select result with sufficient approvals")

	sealable := results[0]
	ms.Assert().Equal(incorporatedResult, sealable)

	// the incorporated result should have collected 1 signature per chunk
	// (happy path)
	ms.Assert().Equal(
		incorporatedResult.Result.Chunks.Len(),
		len(sealable.GetAggregatedSignatures()),
	)

	// check match when we are storing entities
	ms.resultsDB.On("Store", mock.Anything).Run(
		func(args mock.Arguments) {
			stored := args.Get(0).(*flow.ExecutionResult)
			ms.Assert().Equal(incorporatedResult.Result, stored)
		},
	).Return(nil)

	ms.sealsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			seal := args.Get(0).(*flow.IncorporatedResultSeal)
			ms.Assert().Equal(incorporatedResult, seal.IncorporatedResult)
			ms.Assert().Equal(incorporatedResult.Result.BlockID, seal.Seal.BlockID)
			ms.Assert().Equal(incorporatedResult.Result.ID(), seal.Seal.ResultID)
			ms.Assert().Equal(
				incorporatedResult.Result.Chunks.Len(),
				len(seal.Seal.AggregatedApprovalSigs),
			)
		},
	).Return(true)

	err = ms.matching.sealResult(incorporatedResult)
	ms.Require().NoError(err, "should generate seal on correct sealable result")

	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
}

// Test that we request receipts for unsealed finalized blocks when the gap
// exceeds the configured threshold.
func (ms *MatchingSuite) TestRequestReceiptsPendingBlocks() {
	n := 100

	// Create n consecutive blocks
	// the first one is sealed and the last one is final

	headers := []flow.Header{}

	parentHeader := unittest.BlockHeaderFixture()

	for i := 0; i < n; i++ {
		newHeader := unittest.BlockHeaderWithParentFixture(&parentHeader)
		parentHeader = newHeader
		headers = append(headers, newHeader)
	}

	orderedBlocks := []flow.Block{}
	for i := 0; i < n; i++ {
		payload := unittest.PayloadFixture()
		header := headers[i]
		header.PayloadHash = payload.Hash()
		block := flow.Block{
			Header:  &header,
			Payload: payload,
		}
		ms.blocks[block.ID()] = &block
		orderedBlocks = append(orderedBlocks, block)
	}

	ms.state = &protocol.State{}

	ms.state.On("Final").Return(
		func() realproto.Snapshot {
			snapshot := &protocol.Snapshot{}
			snapshot.On("Head").Return(
				func() *flow.Header {
					return orderedBlocks[n-1].Header
				},
				nil,
			)
			return snapshot
		},
		nil,
	)

	ms.state.On("Sealed").Return(
		func() realproto.Snapshot {
			snapshot := &protocol.Snapshot{}
			snapshot.On("Head").Return(
				func() *flow.Header {
					return orderedBlocks[0].Header
				},
				nil,
			)
			return snapshot
		},
		nil,
	)

	ms.matching.state = ms.state

	// the receipts pool is empty, which will trigger request
	ms.receiptsPL.On("All").Return(nil)

	// keep track of requested blocks
	requestedBlocks := []flow.Identifier{}
	ms.requester.On("EntityByID", mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			blockID := args.Get(0).(flow.Identifier)
			requestedBlocks = append(requestedBlocks, blockID)
		},
	).Return()

	err := ms.matching.requestPending()
	ms.Require().NoError(err, "should request results for pending blocks")

	// should request n-1 blocks if n > requestReceiptThreshold
	ms.Assert().Equal(len(requestedBlocks), n-1)
}
