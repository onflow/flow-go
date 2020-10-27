// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package matching

import (
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
	suite.Suite

	// IDENTITIES
	conID flow.Identifier
	exeID flow.Identifier
	verID flow.Identifier

	identities map[flow.Identifier]*flow.Identity
	approvers  flow.IdentityList

	// BLOCKS
	rootBlock            flow.Block
	latestSealedBlock    flow.Block
	latestFinalizedBlock flow.Block
	unfinalizedBlock     flow.Block
	blocks               map[flow.Identifier]*flow.Block

	// PROTOCOL STATE
	state          *protocol.State
	sealedSnapshot *protocol.Snapshot
	finalSnapshot  *protocol.Snapshot

	// MEMPOOLS and STORAGE which are injected into Matching Engine
	// mock storage.ExecutionResults: backed by in-memory map persistedResults
	resultsDB        *storage.ExecutionResults
	persistedResults map[flow.Identifier]*flow.ExecutionResult

	// mock mempool.IncorporatedResults: backed by in-memory map pendingResults
	resultsPL      *mempool.IncorporatedResults
	pendingResults map[flow.Identifier]*flow.IncorporatedResult

	// mock mempool.IncorporatedResultSeals: backed by in-memory map pendingSeals
	sealsPL      *mempool.IncorporatedResultSeals
	pendingSeals map[flow.Identifier]*flow.IncorporatedResultSeal

	// mock BLOCK STORAGE: backed by in-memory map blocks
	headersDB *storage.Headers // backed by map blocks
	indexDB   *storage.Index   // backed by map blocks

	// mock mempool.Approvals: used to test whether or not Matching Engine stores approvals
	approvalsPL *mempool.Approvals

	// misc SERVICE COMPONENTS which are injected into Matching Engine
	requester *module.Requester
	assigner  *module.ChunkAssigner

	// MATCHING ENGINE
	matching *Engine
}

func (ms *MatchingSuite) SetupTest() {

	// ~~~~~~~~~~~~~~~~~~~~~~~~~~ SETUP IDENTITIES ~~~~~~~~~~~~~~~~~~~~~~~~~~ //
	unit := engine.NewUnit()
	log := zerolog.New(os.Stderr)
	metrics := metrics.NewNoopCollector()

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

	ms.approvers = unittest.IdentityListFixture(4, unittest.WithRole(flow.RoleVerification))
	for _, verifier := range ms.approvers {
		ms.identities[verifier.ID()] = verifier
	}

	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~ SETUP BLOCKS ~~~~~~~~~~~~~~~~~~~~~~~~~~~~ //
	// rootBlock <- latestSealedBlock <- latestFinalizedBlock <- unfinalizedBlock
	ms.rootBlock = unittest.BlockFixture()
	ms.latestSealedBlock = unittest.BlockWithParentFixture(ms.rootBlock.Header)
	ms.latestFinalizedBlock = unittest.BlockWithParentFixture(ms.latestSealedBlock.Header)
	ms.unfinalizedBlock = unittest.BlockWithParentFixture(ms.latestFinalizedBlock.Header)

	ms.blocks = make(map[flow.Identifier]*flow.Block)
	ms.blocks[ms.rootBlock.ID()] = &ms.rootBlock
	ms.blocks[ms.latestSealedBlock.ID()] = &ms.latestSealedBlock
	ms.blocks[ms.latestFinalizedBlock.ID()] = &ms.latestFinalizedBlock
	ms.blocks[ms.unfinalizedBlock.ID()] = &ms.unfinalizedBlock

	// ~~~~~~~~~~~~~~~~~~~~~~~~ SETUP PROTOCOL STATE ~~~~~~~~~~~~~~~~~~~~~~~~ //
	ms.state = &protocol.State{}

	// define the protocol state snapshot of the latest finalized block
	ms.state.On("Final").Return(
		func() realproto.Snapshot {
			return ms.finalSnapshot
		},
		nil,
	)
	ms.finalSnapshot = &protocol.Snapshot{}
	ms.finalSnapshot.On("Head").Return(
		func() *flow.Header {
			return ms.latestFinalizedBlock.Header
		},
		nil,
	)

	// define the protocol state snapshot of the latest finalized and sealed block
	ms.state.On("Sealed").Return(
		func() realproto.Snapshot {
			return ms.sealedSnapshot
		},
		nil,
	)
	ms.sealedSnapshot = &protocol.Snapshot{}
	ms.sealedSnapshot.On("Head").Return(
		func() *flow.Header {
			return ms.latestSealedBlock.Header
		},
		nil,
	)

	// define the protocol state snapshot for any block in `ms.blocks`
	ms.state.On("AtBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) realproto.Snapshot {
			block, found := ms.blocks[blockID]
			if !found {
				return stateSnapshotForUnknownBlock()
			}
			return stateSnapshotForKnownBlock(block.Header, ms.identities)
		},
	)

	// ~~~~~~~~~~~~~~~~~~~~~~~ SETUP RESULTS STORAGE ~~~~~~~~~~~~~~~~~~~~~~~~ //
	ms.persistedResults = make(map[flow.Identifier]*flow.ExecutionResult)
	ms.resultsDB = &storage.ExecutionResults{}
	ms.resultsDB.On("ByID", mock.Anything).Return(
		func(resultID flow.Identifier) *flow.ExecutionResult {
			return ms.persistedResults[resultID]
		},
		func(resultID flow.Identifier) error {
			_, found := ms.persistedResults[resultID]
			if !found {
				return storerr.ErrNotFound
			}
			return nil
		},
	).Maybe()
	ms.resultsDB.On("Store", mock.Anything).Return(
		func(result *flow.ExecutionResult) error {
			_, found := ms.persistedResults[result.BlockID]
			if found {
				return storerr.ErrAlreadyExists
			}
			return nil
		},
	).Maybe() // this call is optional

	// ~~~~~~~~~~~~~~~~~~~~ SETUP BLOCK HEADER STORAGE ~~~~~~~~~~~~~~~~~~~~~ //
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

	// ~~~~~~~~~~~~~~~~~~~~ SETUP BLOCK PAYLOAD STORAGE ~~~~~~~~~~~~~~~~~~~~~ //
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

	// ~~~~~~~~~~~~~~~~ SETUP INCORPORATED RESULTS MEMPOOL ~~~~~~~~~~~~~~~~~ //
	ms.pendingResults = make(map[flow.Identifier]*flow.IncorporatedResult)
	ms.resultsPL = &mempool.IncorporatedResults{}
	ms.resultsPL.On("Size").Return(uint(0)).Maybe() // only for metrics
	ms.resultsPL.On("All").Return(
		func() []*flow.IncorporatedResult {
			results := make([]*flow.IncorporatedResult, 0, len(ms.pendingResults))
			for _, result := range ms.pendingResults {
				results = append(results, result)
			}
			return results
		},
	).Maybe()

	// ~~~~~~~~~~~~~~~~~~~~~~ SETUP APPROVALS MEMPOOL ~~~~~~~~~~~~~~~~~~~~~~ //
	ms.approvalsPL = &mempool.Approvals{}
	ms.approvalsPL.On("Size").Return(uint(0)).Maybe() // only for metrics

	// ~~~~~~~~~~~~~~~~~~~~~~~~ SETUP SEALS MEMPOOL ~~~~~~~~~~~~~~~~~~~~~~~~ //
	ms.pendingSeals = make(map[flow.Identifier]*flow.IncorporatedResultSeal)
	ms.sealsPL = &mempool.IncorporatedResultSeals{}
	ms.sealsPL.On("Size").Return(uint(0)).Maybe() // only for metrics
	ms.sealsPL.On("ByID", mock.Anything).Return(
		func(sealID flow.Identifier) *flow.IncorporatedResultSeal {
			return ms.pendingSeals[sealID]
		},
		func(sealID flow.Identifier) bool {
			_, found := ms.pendingSeals[sealID]
			return found
		},
	)

	// ~~~~~~~~~~~~~~~~~~~~~~~ SETUP MATCHING ENGINE ~~~~~~~~~~~~~~~~~~~~~~~ //
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
		approvals:               ms.approvalsPL,
		seals:                   ms.sealsPL,
		checkingSealing:         atomic.NewBool(false),
		requestReceiptThreshold: 10,
		maxUnsealedResults:      200,
		assigner:                ms.assigner,
		requireApprovals:        true,
	}
}

func (ms *MatchingSuite) TestOnReceiptInvalidOrigin() {
	// we don't validate the origin of an execution receipt anymore, as Execution Nodes
	// might forward us Execution Receipts from others for blocks they haven't computed themselves
	ms.T().Skip()

	// try to submit a receipt with a random origin ID
	originID := ms.exeID
	receipt := unittest.ExecutionReceiptFixture()

	err := ms.matching.onReceipt(originID, receipt)
	ms.Require().Error(err, "should reject receipt with mismatching origin and executor")

	ms.resultsDB.AssertNumberOfCalls(ms.T(), "Store", 0)
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// try to submit a receipt from a NON-ExecutionNode  (here: consensus node)
func (ms *MatchingSuite) TestOnReceiptUnknownBlock() {
	originID := ms.conID
	receipt := unittest.ExecutionReceiptFixture(unittest.WithExecutorID(originID))

	// onReceipt should not throw an error
	err := ms.matching.onReceipt(originID, receipt)
	ms.Require().NoError(err, "should ignore receipt for unknown block")

	ms.resultsDB.AssertNumberOfCalls(ms.T(), "Store", 0)
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// try to submit a receipt for a known block from a consensus node
func (ms *MatchingSuite) TestOnReceiptInvalidRole() {
	originID := ms.conID
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(originID),
		unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(&ms.unfinalizedBlock))),
	)

	err := ms.matching.onReceipt(originID, receipt)
	ms.Require().Error(err, "should reject receipt from wrong node role")
	ms.Require().True(engine.IsInvalidInputError(err))

	ms.resultsDB.AssertNumberOfCalls(ms.T(), "Store", 0)
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// try ot submit a receipt from an Execution node, with zero unstaked node
func (ms *MatchingSuite) TestOnReceiptUnstakedExecutor() {
	originID := ms.exeID
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(originID),
		unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(&ms.unfinalizedBlock))),
	)
	ms.identities[originID].Stake = 0

	err := ms.matching.onReceipt(originID, receipt)
	ms.Require().Error(err, "should reject receipt from unstaked node")
	ms.Require().True(engine.IsInvalidInputError(err))

	ms.resultsDB.AssertNumberOfCalls(ms.T(), "Store", 0)
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// matching engine should drop Result for known block that is already sealed
// without trying to store anything
func (ms *MatchingSuite) TestOnReceiptSealedResult() {
	originID := ms.exeID
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(originID),
		unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(&ms.latestSealedBlock))),
	)

	err := ms.matching.onReceipt(originID, receipt)
	ms.Require().NoError(err, "should ignore receipt for sealed result")

	ms.resultsDB.AssertNumberOfCalls(ms.T(), "Store", 0)
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// try to submit a receipt for an already received result
func (ms *MatchingSuite) TestOnReceiptPendingResult() {
	originID := ms.exeID
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(originID),
		unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(&ms.unfinalizedBlock))),
	)

	ms.resultsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			incorporatedResult := args.Get(0).(*flow.IncorporatedResult)
			ms.Assert().Equal(incorporatedResult.Result, &receipt.ExecutionResult)
		},
	).Return(false, nil)

	err := ms.matching.onReceipt(originID, receipt)
	ms.Require().NoError(err, "should ignore receipt for already pending result")
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
	ms.resultsDB.AssertNumberOfCalls(ms.T(), "Store", 1)

	// resubmit receipt
	err = ms.matching.onReceipt(originID, receipt)
	ms.Require().NoError(err, "should ignore receipt for already pending result")
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 2)
	ms.resultsDB.AssertNumberOfCalls(ms.T(), "Store", 2)

	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// try to submit a receipt that should be valid
func (ms *MatchingSuite) TestOnReceiptValid() {
	originID := ms.exeID
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(originID),
		unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(&ms.unfinalizedBlock))),
	)

	ms.resultsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			incorporatedResult := args.Get(0).(*flow.IncorporatedResult)
			ms.Assert().Equal(incorporatedResult.Result, &receipt.ExecutionResult)
		},
	).Return(true, nil)

	err := ms.matching.onReceipt(originID, receipt)
	ms.Require().NoError(err, "should add receipt and result to mempool if valid")

	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// try to submit an approval where the message origin is inconsistent with the message creator
func (ms *MatchingSuite) TestApprovalInvalidOrigin() {
	// approval from valid origin (i.e. a verification node) but with random ApproverID
	originID := ms.verID
	approval := unittest.ResultApprovalFixture() // with random ApproverID

	err := ms.matching.onApproval(originID, approval)
	ms.Require().Error(err, "should reject approval with mismatching origin and executor")
	ms.Require().True(engine.IsInvalidInputError(err))

	// approval from random origin but with valid ApproverID (i.e. a verification node)
	originID = unittest.IdentifierFixture() // random origin
	approval = unittest.ResultApprovalFixture(unittest.WithApproverID(ms.verID))

	err = ms.matching.onApproval(originID, approval)
	ms.Require().Error(err, "should reject approval with mismatching origin and executor")
	ms.Require().True(engine.IsInvalidInputError(err))

	// In both cases, we expect the approval to be rejected without hitting the mempools
	ms.approvalsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// Try to submit an approval for an unknown block.
// As the block is unknown, the ID of teh sender should
//not matter as there is no block to verify it against
func (ms *MatchingSuite) TestApprovalUnknownBlock() {
	originID := ms.conID
	approval := unittest.ResultApprovalFixture(unittest.WithApproverID(originID)) // generates approval for random block ID

	// make sure the approval is added to the cache for future processing
	// check calls have the correct parameters
	ms.approvalsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			added := args.Get(0).(*flow.ResultApproval)
			ms.Assert().Equal(approval, added)
		},
	).Return(true, nil)

	// onApproval should not throw an error
	err := ms.matching.onApproval(originID, approval)
	ms.Require().NoError(err, "should cache approvals for unknown blocks")

	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.approvalsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// try to submit an approval from a consensus node
func (ms *MatchingSuite) TestOnApprovalInvalidRole() {
	originID := ms.conID
	approval := unittest.ResultApprovalFixture(
		unittest.WithBlockID(ms.unfinalizedBlock.ID()),
		unittest.WithApproverID(originID),
	)

	err := ms.matching.onApproval(originID, approval)
	ms.Require().Error(err, "should reject approval from wrong approver role")
	ms.Require().True(engine.IsInvalidInputError(err))

	ms.approvalsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// try to submit an approval from an unstaked approver
func (ms *MatchingSuite) TestOnApprovalInvalidStake() {
	originID := ms.verID
	approval := unittest.ResultApprovalFixture(
		unittest.WithBlockID(ms.unfinalizedBlock.ID()),
		unittest.WithApproverID(originID),
	)
	ms.identities[originID].Stake = 0

	err := ms.matching.onApproval(originID, approval)
	ms.Require().Error(err, "should reject approval from unstaked approver")
	ms.Require().True(engine.IsInvalidInputError(err))

	ms.approvalsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// try to submit an approval for a sealed result
func (ms *MatchingSuite) TestOnApprovalSealedResult() {
	originID := ms.verID
	approval := unittest.ResultApprovalFixture(
		unittest.WithBlockID(ms.latestSealedBlock.ID()),
		unittest.WithApproverID(originID),
	)

	err := ms.matching.onApproval(originID, approval)
	ms.Require().NoError(err, "should ignore approval for sealed result")

	ms.approvalsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// try to submit an approval that is already in the mempool
func (ms *MatchingSuite) TestOnApprovalPendingApproval() {
	originID := ms.verID
	approval := unittest.ResultApprovalFixture(unittest.WithApproverID(originID))

	// check calls have the correct parameters
	ms.approvalsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			added := args.Get(0).(*flow.ResultApproval)
			ms.Assert().Equal(approval, added)
		},
	).Return(false, nil)

	err := ms.matching.onApproval(originID, approval)
	ms.Require().NoError(err, "should ignore approval if already pending")

	ms.approvalsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// try to submit an approval for a known block
func (ms *MatchingSuite) TestOnApprovalValid() {
	originID := ms.verID
	approval := unittest.ResultApprovalFixture(
		unittest.WithBlockID(ms.unfinalizedBlock.ID()),
		unittest.WithApproverID(originID),
	)

	// check calls have the correct parameters
	ms.approvalsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			added := args.Get(0).(*flow.ResultApproval)
			ms.Assert().Equal(approval, added)
		},
	).Return(true, nil)

	err := ms.matching.onApproval(originID, approval)
	ms.Require().NoError(err, "should add approval to mempool if valid")

	ms.approvalsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// try to get matched results with nothing in memory pools
func (ms *MatchingSuite) TestSealableResultsEmptyMempools() {
	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err, "should not error with empty mempools")
	ms.Assert().Empty(results, "should not have matched results with empty mempools")
}

// TestSealableResultsValid tests matching.Engine.sealableResults():
//  * a well-formed incorporated result R is in the mempool
//  * sufficient number of valid result approvals for result R
//  * R.PreviousResultID references a known result (i.e. stored in resultsDB)
//  * R forms a valid sub-graph with its previous result (aka parent result)
// Method Engine.sealableResults() should return R as an element of the sealable results
func (ms *MatchingSuite) TestSealableResultsValid() {
	valSubgrph := ms.validSubgraphFixture()
	ms.addSubgraphFixtureToMempools(valSubgrph)

	// test output of Matching Engine's sealableResults()
	results, err := ms.matching.sealableResults()
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
	valSubgrph := ms.validSubgraphFixture()
	ms.addSubgraphFixtureToMempools(valSubgrph)
	delete(ms.blocks, valSubgrph.Block.ID()) // remove block the execution receipt pertains to

	_, err := ms.matching.sealableResults()
	ms.Require().Error(err)
}

// Given an incorporated result in the mempool, whose previous result
// (aka parent result) is not known:
//   * skip this result
//   * this result should not be removed from the mempool
func (ms *MatchingSuite) TestSealableResultUnknownPrevious() {
	subgrph := ms.validSubgraphFixture()
	ms.addSubgraphFixtureToMempools(subgrph)
	delete(ms.persistedResults, subgrph.PreviousResult.ID()) // remove previous execution result from storage layer

	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	ms.Assert().Empty(results, "should not select result with unsealed previous")

	ms.resultsDB.AssertNumberOfCalls(ms.T(), "ByID", 1)
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Rem", 0)
}

// TestSealableResultsInvalidSubgraph tests matching.Engine.sealableResults():
// let R1 be a result that references block A, and R2 be R1's parent result.
//  * the execution results form a valid subgraph if and only if
//    R2 should reference A's parent.
// Method sealableResults() should
//   * neither consider R1 nor R2 sealable incorporated results and
//   * remove R1 from IncorporatedResults mempool, i.e. `resultsPL`
func (ms *MatchingSuite) TestSealableResultsInvalidSubgraph() {
	subgrph := ms.validSubgraphFixture()
	subgrph.PreviousResult.BlockID = unittest.IdentifierFixture() // invalidate subgraph
	subgrph.Result.PreviousResultID = subgrph.PreviousResult.ID()
	ms.addSubgraphFixtureToMempools(subgrph)

	// we expect business logic to remove the incorporated result with failed sub-graph check from mempool
	ms.resultsPL.On("Rem", entityWithID(subgrph.IncorporatedResult.ID())).Return(true).Once()

	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	ms.Assert().Empty(results, "should not select result with invalid subgraph")
	ms.resultsPL.AssertExpectations(ms.T()) // asserts that resultsPL.Rem(incorporatedResult.ID()) was called
}

// TestSealableResultsInvalidChunks tests that matching.Engine.sealableResults()
// performs the following chunk checks on the result:
//   * the number k of chunks in the execution result equals to
//     the number of collections in the corresponding block _plus_ 1 (for system chunk)
//   * for each index idx := 0, 1, ..., k
//     there exists once chunk
// Here we test that an IncorporatedResult with too _few_ chunks is not sealed and removed from the mempool
func (ms *MatchingSuite) TestSealableResults_TooFewChunks() {
	subgrph := ms.validSubgraphFixture()
	chunks := subgrph.Result.Chunks
	subgrph.Result.Chunks = chunks[0 : len(chunks)-2] // drop the last chunk
	ms.addSubgraphFixtureToMempools(subgrph)

	// we expect business logic to remove the incorporated result with failed sub-graph check from mempool
	ms.resultsPL.On("Rem", entityWithID(subgrph.IncorporatedResult.ID())).Return(true).Once()

	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	ms.Assert().Empty(results, "should not select result with too many chunks")
	ms.resultsPL.AssertExpectations(ms.T()) // asserts that resultsPL.Rem(incorporatedResult.ID()) was called
}

// TestSealableResults_TooManyChunks tests that matching.Engine.sealableResults()
// performs the following chunk checks on the result:
//   * the number k of chunks in the execution result equals to
//     the number of collections in the corresponding block _plus_ 1 (for system chunk)
//   * for each index idx := 0, 1, ..., k
//     there exists once chunk
// Here we test that an IncorporatedResult with too _many_ chunks is not sealed and removed from the mempool
func (ms *MatchingSuite) TestSealableResults_TooManyChunks() {
	subgrph := ms.validSubgraphFixture()
	chunks := subgrph.Result.Chunks
	subgrph.Result.Chunks = append(chunks, chunks[len(chunks)-1]) // duplicate the last entry
	ms.addSubgraphFixtureToMempools(subgrph)

	// we expect business logic to remove the incorporated result with failed sub-graph check from mempool
	ms.resultsPL.On("Rem", entityWithID(subgrph.IncorporatedResult.ID())).Return(true).Once()

	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	ms.Assert().Empty(results, "should not select result with too few chunks")
	ms.resultsPL.AssertExpectations(ms.T()) // asserts that resultsPL.Rem(incorporatedResult.ID()) was called
}

// TestSealableResults_InvalidChunks tests that matching.Engine.sealableResults()
// performs the following chunk checks on the result:
//   * the number k of chunks in the execution result equals to
//     the number of collections in the corresponding block _plus_ 1 (for system chunk)
//   * for each index idx := 0, 1, ..., k
//     there exists once chunk
// Here we test that an IncorporatedResult with
//   * correct number of chunks
//   * but one missing chunk and one duplicated chunk
// is not sealed and removed from the mempool
func (ms *MatchingSuite) TestSealableResults_InvalidChunks() {
	subgrph := ms.validSubgraphFixture()
	chunks := subgrph.Result.Chunks
	chunks[len(chunks)-2] = chunks[len(chunks)-1] // overwrite second-last with last entry, which is now duplicated
	// yet we have the correct number of elements in the chunk list
	ms.addSubgraphFixtureToMempools(subgrph)

	// we expect business logic to remove the incorporated result with failed sub-graph check from mempool
	ms.resultsPL.On("Rem", entityWithID(subgrph.IncorporatedResult.ID())).Return(true).Once()

	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	ms.Assert().Empty(results, "should not select result with invalid chunk list")
	ms.resultsPL.AssertExpectations(ms.T()) // asserts that resultsPL.Rem(incorporatedResult.ID()) was called
}

// TestSealableResults_NoPayload_MissingChunk tests that matching.Engine.sealableResults()
// enforces the correct number of chunks for empty blocks, i.e. blocks with no payload:
//  * execution receipt with missing system chunk should be rejected
func (ms *MatchingSuite) TestSealableResults_NoPayload_MissingChunk() {
	subgrph := ms.validSubgraphFixture()
	subgrph.Block.Payload = nil                                                              // override block's payload to nil
	subgrph.IncorporatedResult.IncorporatedBlockID = subgrph.Block.ID()                      // update block's ID
	subgrph.IncorporatedResult.Result.BlockID = subgrph.Block.ID()                           // update block's ID
	subgrph.IncorporatedResult.Result.Chunks = subgrph.IncorporatedResult.Result.Chunks[0:0] // empty chunk list
	ms.addSubgraphFixtureToMempools(subgrph)

	// we expect business logic to remove the incorporated result with failed sub-graph check from mempool
	ms.resultsPL.On("Rem", entityWithID(subgrph.IncorporatedResult.ID())).Return(true).Once()

	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	ms.Assert().Empty(results, "should not select result with invalid chunk list")
	ms.resultsPL.AssertExpectations(ms.T()) // asserts that resultsPL.Rem(incorporatedResult.ID()) was called
}

// TestSealableResults_NoPayload_TooManyChunk tests that matching.Engine.sealableResults()
// enforces the correct number of chunks for empty blocks, i.e. blocks with no payload:
//  * execution receipt with more than one chunk should be rejected
func (ms *MatchingSuite) TestSealableResults_NoPayload_TooManyChunk() {
	subgrph := ms.validSubgraphFixture()
	subgrph.Block.Payload = nil                                                              // override block's payload to nil
	subgrph.IncorporatedResult.IncorporatedBlockID = subgrph.Block.ID()                      // update block's ID
	subgrph.IncorporatedResult.Result.BlockID = subgrph.Block.ID()                           // update block's ID
	subgrph.IncorporatedResult.Result.Chunks = subgrph.IncorporatedResult.Result.Chunks[0:2] // two chunks
	ms.addSubgraphFixtureToMempools(subgrph)

	// we expect business logic to remove the incorporated result with failed sub-graph check from mempool
	ms.resultsPL.On("Rem", entityWithID(subgrph.IncorporatedResult.ID())).Return(true).Once()

	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	ms.Assert().Empty(results, "should not select result with invalid chunk list")
	ms.resultsPL.AssertExpectations(ms.T()) // asserts that resultsPL.Rem(incorporatedResult.ID()) was called
}

// TestSealableResults_NoPayload_WrongIndexChunk tests that matching.Engine.sealableResults()
// enforces the correct number of chunks for empty blocks, i.e. blocks with no payload:
//  * execution receipt with a single chunk, but wrong chunk index, should be rejected
func (ms *MatchingSuite) TestSealableResults_NoPayload_WrongIndexChunk() {
	subgrph := ms.validSubgraphFixture()
	subgrph.Block.Payload = nil                                                              // override block's payload to nil
	subgrph.IncorporatedResult.IncorporatedBlockID = subgrph.Block.ID()                      // update block's ID
	subgrph.IncorporatedResult.Result.BlockID = subgrph.Block.ID()                           // update block's ID
	subgrph.IncorporatedResult.Result.Chunks = subgrph.IncorporatedResult.Result.Chunks[2:2] // chunk with chunkIndex == 2
	ms.addSubgraphFixtureToMempools(subgrph)

	// we expect business logic to remove the incorporated result with failed sub-graph check from mempool
	ms.resultsPL.On("Rem", entityWithID(subgrph.IncorporatedResult.ID())).Return(true).Once()

	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	ms.Assert().Empty(results, "should not select result with invalid chunk list")
	ms.resultsPL.AssertExpectations(ms.T()) // asserts that resultsPL.Rem(incorporatedResult.ID()) was called
}

// TestSealableResultsUnassignedVerifiers tests that matching.Engine.sealableResults():
// only considers approvals from assigned verifiers
func (ms *MatchingSuite) TestSealableResultsUnassignedVerifiers() {
	subgrph := ms.validSubgraphFixture()

	assignedVerifiersPerChunk := uint(len(ms.approvers) / 2)
	assignment := chunks.NewAssignment()
	approvals := make(map[uint64]map[flow.Identifier]*flow.ResultApproval)
	for _, chunk := range subgrph.IncorporatedResult.Result.Chunks {
		assignment.Add(chunk, ms.approvers[0:assignedVerifiersPerChunk].NodeIDs()) // assign leading half verifiers

		// generate approvals by _tailing_ half verifiers
		chunkApprovals := make(map[flow.Identifier]*flow.ResultApproval)
		for _, approver := range ms.approvers[assignedVerifiersPerChunk:len(ms.approvers)] {
			chunkApprovals[approver.NodeID] = approvalFor(subgrph.IncorporatedResult.Result, chunk.Index, approver.NodeID)
		}
		approvals[chunk.Index] = chunkApprovals
	}
	subgrph.Assignment = assignment
	subgrph.Approvals = approvals

	ms.addSubgraphFixtureToMempools(subgrph)

	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	ms.Assert().Empty(results, "should not select result with ")
	ms.approvalsPL.AssertExpectations(ms.T()) // asserts that resultsPL.Rem(incorporatedResult.ID()) was called
}

// TestSealableResults_UnknownVerifiers tests that matching.Engine.sealableResults():
//   * removes approvals from unknown verification nodes from mempool
func (ms *MatchingSuite) TestSealableResults_ApprovalsForUnknownBlockRemain() {
	// make child block for unfinalizedBlock, i.e.:
	//   <- unfinalizedBlock <- block
	// and create Execution result ands approval for this block
	block := unittest.BlockWithParentFixture(ms.unfinalizedBlock.Header)
	er := unittest.ExecutionResultFixture(unittest.WithBlock(&block))
	app1 := approvalFor(er, 0, unittest.IdentifierFixture()) // from unknown node

	ms.approvalsPL.On("All").Return([]*flow.ResultApproval{app1})
	chunkApprovals := make(map[flow.Identifier]*flow.ResultApproval)
	chunkApprovals[app1.Body.ApproverID] = app1
	ms.approvalsPL.On("ByChunk", er.ID(), 0).Return(chunkApprovals)

	_, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	ms.approvalsPL.AssertNumberOfCalls(ms.T(), "RemApproval", 0)
	ms.approvalsPL.AssertNumberOfCalls(ms.T(), "RemChunk", 0)
}

// TestRemoveApprovalsFromInvalidVerifiers tests that matching.Engine.sealableResults():
//   * removes approvals from invalid verification nodes from mempool
// This may occur when the block wasn't know when the node received the approval.
// Note: we test a scenario here, were result is sealable; it just has additional
//      approvals from invalid nodes
func (ms *MatchingSuite) TestRemoveApprovalsFromInvalidVerifiers() {
	subgrph := ms.validSubgraphFixture()

	// add invalid approvals to leading chunk:
	app1 := approvalFor(subgrph.IncorporatedResult.Result, 0, unittest.IdentifierFixture()) // from unknown node
	app2 := approvalFor(subgrph.IncorporatedResult.Result, 0, ms.exeID)                     // from known but non-VerificationNode
	ms.identities[ms.verID].Stake = 0
	app3 := approvalFor(subgrph.IncorporatedResult.Result, 0, ms.verID) // from zero-weight VerificationNode
	subgrph.Approvals[0][app1.Body.ApproverID] = app1
	subgrph.Approvals[0][app2.Body.ApproverID] = app2
	subgrph.Approvals[0][app3.Body.ApproverID] = app3

	ms.addSubgraphFixtureToMempools(subgrph)

	// we expect business logic to remove the approval from the unknown node
	ms.approvalsPL.On("RemApproval", entityWithID(app1.ID())).Return(true, nil).Once()
	ms.approvalsPL.On("RemApproval", entityWithID(app2.ID())).Return(true, nil).Once()
	ms.approvalsPL.On("RemApproval", entityWithID(app3.ID())).Return(true, nil).Once()

	_, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	ms.approvalsPL.AssertExpectations(ms.T()) // asserts that resultsPL.Rem(incorporatedResult.ID()) was called
}

// TestSealableResultsInsufficientApprovals tests matching.Engine.sealableResults():
//  * a result where at least one chunk has not enough approvals (require
//    currently at least one) should not be sealable
func (ms *MatchingSuite) TestSealableResultsInsufficientApprovals() {
	subgrph := ms.validSubgraphFixture()
	delete(subgrph.Approvals, uint64(len(subgrph.Result.Chunks)-1))
	ms.addSubgraphFixtureToMempools(subgrph)

	// test output of Matching Engine's sealableResults()
	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	ms.Assert().Empty(results, "expecting no sealable result")
}

// TestRequestReceiptsPendingBlocks tests matching.Engine.requestPending():
//   * generate n=100 consecutive blocks, where the first one is sealed and the last one is final
func (ms *MatchingSuite) TestRequestReceiptsPendingBlocks() {
	// create blocks
	n := 100
	orderedBlocks := make([]flow.Block, 0, n)
	parentBlock := ms.unfinalizedBlock
	for i := 0; i < n; i++ {
		block := unittest.BlockWithParentFixture(parentBlock.Header)
		ms.blocks[block.ID()] = &block
		orderedBlocks = append(orderedBlocks, block)
		parentBlock = block
	}

	// progress latest sealed and latest finalized:
	ms.latestSealedBlock = orderedBlocks[0]
	ms.latestFinalizedBlock = orderedBlocks[n-1]

	// Expecting all blocks to be requested: from sealed height + 1 up to (incl.) latest finalized
	for i := 1; i < n; i++ {
		id := orderedBlocks[i].ID()
		ms.requester.On("EntityByID", id, mock.Anything).Return().Once()
	}
	ms.sealsPL.On("All").Return([]*flow.IncorporatedResultSeal{}).Maybe()

	err := ms.matching.requestPending()
	ms.Require().NoError(err, "should request results for pending blocks")
	ms.requester.AssertExpectations(ms.T()) // asserts that requester.EntityByID(<blockID>, filter.Any) was called
}

func stateSnapshotForUnknownBlock() *protocol.Snapshot {
	snapshot := &protocol.Snapshot{}
	snapshot.On("Identity", mock.Anything).Return(
		nil, storerr.ErrNotFound,
	)
	snapshot.On("Head", mock.Anything).Return(
		nil, storerr.ErrNotFound,
	)
	return snapshot
}

func stateSnapshotForKnownBlock(block *flow.Header, identities map[flow.Identifier]*flow.Identity) *protocol.Snapshot {
	snapshot := &protocol.Snapshot{}
	snapshot.On("Identity", mock.Anything).Return(
		func(nodeID flow.Identifier) *flow.Identity {
			return identities[nodeID]
		},
		func(nodeID flow.Identifier) error {
			_, found := identities[nodeID]
			if !found {
				return realproto.IdentityNotFoundErr{NodeID: nodeID}
			}
			return nil
		},
	)
	snapshot.On("Head").Return(block, nil)
	return snapshot
}

func approvalFor(result *flow.ExecutionResult, chunkIdx uint64, approverID flow.Identifier) *flow.ResultApproval {
	return unittest.ResultApprovalFixture(
		unittest.WithBlockID(result.BlockID),
		unittest.WithExecutionResultID(result.ID()),
		unittest.WithApproverID(approverID),
		unittest.WithChunk(chunkIdx),
	)
}

func entityWithID(expectedID flow.Identifier) interface{} {
	return mock.MatchedBy(
		func(entity flow.Entity) bool {
			return expectedID == entity.ID()
		})
}

// subgraphFixture represents a subgraph of the blockchain:
//  Result   -----------------------------------> Block
//    |                                             |
//    |                                             v
//    |                                           ParentBlock
//    v
//  PreviousResult  ---> PreviousResult.BlockID
//
// Depending on validity of the subgraph:
//   *  valid:   PreviousResult.BlockID == ParentBlock.ID()
//   *  invalid: PreviousResult.BlockID != ParentBlock.ID()
type subgraphFixture struct {
	Block              *flow.Block
	ParentBlock        *flow.Block
	Result             *flow.ExecutionResult
	PreviousResult     *flow.ExecutionResult
	IncorporatedResult *flow.IncorporatedResult
	Assignment         *chunks.Assignment
	Approvals          map[uint64]map[flow.Identifier]*flow.ResultApproval // chunkIndex -> Verifier Node ID -> Approval
}

// Generates a valid subgraph:
// let
//  * R1 be a result which pertains to blockA
//  * R2 be R1's previous result,
//    where R2 pertains to blockB
// The execution results form a valid subgraph if and only if:
//    blockA.ParentID == blockB.ID
func (ms *MatchingSuite) validSubgraphFixture() subgraphFixture {
	// BLOCKS: <- previousBlock <- block
	parentBlock := unittest.BlockFixture()
	block := unittest.BlockWithParentFixture(parentBlock.Header)

	// RESULTS for blocks:
	previousResult := unittest.ExecutionResultFixture(unittest.WithBlock(&parentBlock))
	result := unittest.ExecutionResultFixture(
		unittest.WithBlock(&block),
		unittest.WithPreviousResult(*previousResult),
	)

	// Exec Receipt for block with valid subgraph
	incorporatedResult := unittest.IncorporatedResult.Fixture(unittest.IncorporatedResult.WithResult(result))

	// assign each chunk to 50% of validation Nodes and generate respective approvals
	assignment := chunks.NewAssignment()
	assignedVerifiersPerChunk := uint(len(ms.approvers) / 2)
	approvals := make(map[uint64]map[flow.Identifier]*flow.ResultApproval)
	for _, chunk := range incorporatedResult.Result.Chunks {
		assignedVerifiers := ms.approvers.Sample(assignedVerifiersPerChunk)
		assignment.Add(chunk, assignedVerifiers.NodeIDs())

		// generate approvals
		chunkApprovals := make(map[flow.Identifier]*flow.ResultApproval)
		for _, approver := range assignedVerifiers {
			chunkApprovals[approver.NodeID] = approvalFor(incorporatedResult.Result, chunk.Index, approver.NodeID)
		}
		approvals[chunk.Index] = chunkApprovals
	}

	return subgraphFixture{
		Block:              &block,
		ParentBlock:        &parentBlock,
		Result:             result,
		PreviousResult:     previousResult,
		IncorporatedResult: incorporatedResult,
		Assignment:         assignment,
		Approvals:          approvals,
	}
}

// addSubgraphFixtureToMempools adds add entities in subgraph to mempools and persistent storage mocks
func (ms *MatchingSuite) addSubgraphFixtureToMempools(subgraph subgraphFixture) {
	ms.blocks[subgraph.ParentBlock.ID()] = subgraph.ParentBlock
	ms.blocks[subgraph.Block.ID()] = subgraph.Block
	ms.persistedResults[subgraph.PreviousResult.ID()] = subgraph.PreviousResult
	ms.persistedResults[subgraph.Result.ID()] = subgraph.Result
	ms.pendingResults[subgraph.IncorporatedResult.ID()] = subgraph.IncorporatedResult

	ms.assigner.On("Assign", subgraph.IncorporatedResult.Result, subgraph.IncorporatedResult.IncorporatedBlockID).Return(subgraph.Assignment, nil).Maybe()
	for index := uint64(0); index < uint64(len(subgraph.IncorporatedResult.Result.Chunks)); index++ {
		ms.approvalsPL.On("ByChunk", subgraph.IncorporatedResult.Result.ID(), index).Return(subgraph.Approvals[index]).Maybe()
	}
}
