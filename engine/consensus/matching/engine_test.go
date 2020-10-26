// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package matching

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

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
	//ms.finalSnapshot.On("Identity", mock.Anything).Return(
	//	func(nodeID flow.Identifier) *flow.Identity {
	//		identity := ms.identities[nodeID]
	//		return identity
	//	},
	//	func(nodeID flow.Identifier) error {
	//		_, found := ms.identities[nodeID]
	//		if !found {
	//			return fmt.Errorf("could not get identity (%x)", nodeID)
	//		}
	//		return nil
	//	},
	//)
	//ms.finalSnapshot.On("Identities", mock.Anything).Return(
	//	func(selector flow.IdentityFilter) flow.IdentityList {
	//		return ms.approvers
	//	},
	//	func(selector flow.IdentityFilter) error {
	//		return nil
	//	},
	//)
	//
	//ms.state.On("AtBlockID", mock.Anything).Return(
	//	func(blockID flow.Identifier) realproto.Snapshot {
	//		return ms.refBlockSnapshot
	//	},
	//	nil,
	//)

	//ms.refBlockHeader = &flow.Header{Height: 20} // only need height
	//ms.refBlockSnapshot = &protocol.Snapshot{}
	//ms.refBlockSnapshot.On("Identity", mock.Anything).Return(
	//	func(nodeID flow.Identifier) *flow.Identity {
	//		identity := ms.identities[nodeID]
	//		return identity
	//	},
	//	func(nodeID flow.Identifier) error {
	//		_, found := ms.identities[nodeID]
	//		if !found {
	//			return fmt.Errorf("could not get identity (%x)", nodeID)
	//		}
	//		return nil
	//	},
	//)
	//ms.refBlockSnapshot.On("Identities", mock.Anything).Return(
	//	func(selector flow.IdentityFilter) flow.IdentityList {
	//		return ms.approvers
	//	},
	//	func(selector flow.IdentityFilter) error {
	//		return nil
	//	},
	//)
	//ms.refBlockSnapshot.On("Head").Return(
	//	func() *flow.Header {
	//		return ms.refBlockHeader
	//	},
	//	nil,
	//)

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
		metrics:                 metrics,
		mempool:                 metrics,
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

// TestSealableResultsValid tests matching.Engine.sealableResults():
//  * a well-formed incorporated result R is in the mempool
//  * sufficient number of valid result approvals for result R
//  * R.PreviousResultID references a known result (i.e. stored in resultsDB)
//  * R forms a valid sub-graph with its previous result (aka parent result)
// Method Engine.sealableResults() should return R as an element of the sealable results
func (ms *MatchingSuite) TestSealableResultsValid() {
	// BLOCKS: <- previousBlock <- block
	//previousBlock := unittest.BlockWithParentFixture(ms.unfinalizedBlock.Header)
	previousBlock := unittest.BlockFixture()
	block := unittest.BlockWithParentFixture(previousBlock.Header)

	// RESULTS for blocks:
	previousResult := unittest.ExecutionResultFixture(unittest.WithBlock(&previousBlock))
	result := unittest.ExecutionResultFixture(
		unittest.WithBlock(&block),
		unittest.WithPreviousResult(previousResult.ID()),
	)

	// Exec Receipt for block with valid subgraph
	incorporatedResult := unittest.IncorporatedResult.Fixture(unittest.IncorporatedResult.WithResult(result))

	// add entities to mempools and persistent storage mocks:
	ms.blocks[block.Header.ID()] = &block
	ms.persistedResults[previousResult.ID()] = previousResult
	ms.persistedResults[result.ID()] = result
	ms.pendingResults[incorporatedResult.ID()] = incorporatedResult

	// assign each chunk to each approver
	assignment := chunks.NewAssignment()
	for _, chunk := range incorporatedResult.Result.Chunks {
		assignment.Add(chunk, ms.approvers.NodeIDs())
	}
	ms.assigner.On("Assign", incorporatedResult.Result, incorporatedResult.IncorporatedBlockID).Return(assignment, nil).Once()

	// add enough approvals for each chunk
	print(fmt.Sprintf("%d\n", len(incorporatedResult.Result.Chunks)))
	for index := 0; index < len(incorporatedResult.Result.Chunks); index++ {
		chunkApprovals := make(map[flow.Identifier]*flow.ResultApproval)
		for _, approver := range ms.approvers {
			chunkApprovals[approver.NodeID] = approvalFor(incorporatedResult.Result, uint64(index), approver.NodeID)
		}
		ms.approvalsPL.On("ByChunk", incorporatedResult.Result.ID(), uint64(index)).Return(chunkApprovals).Once()
	}

	// test output of Matching Engine's sealableResults()
	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	ms.Assert().Equal(1, len(results), "expecting a single return value")
	ms.Assert().Equal(incorporatedResult.ID(), results[0].ID(), "expecting a single return value")

	ms.resultsDB.AssertExpectations(ms.T())
	ms.assigner.AssertExpectations(ms.T())
	ms.approvalsPL.AssertExpectations(ms.T())
}

func approvalFor(result *flow.ExecutionResult, chunkIdx uint64, approverID flow.Identifier) *flow.ResultApproval {
	return unittest.ResultApprovalFixture(
		unittest.WithBlockID(result.BlockID),
		unittest.WithExecutionResultID(result.ID()),
		unittest.WithApproverID(approverID),
		unittest.WithChunk(chunkIdx),
	)
}

func expectedID(expectedID flow.Identifier) interface{} {
	return mock.MatchedBy(
		func(actualID flow.Identifier) bool {
			return expectedID == actualID
		})
}

func entityWithID(expectedID flow.Identifier) interface{} {
	return mock.MatchedBy(
		func(entity flow.Entity) bool {
			return expectedID == entity.ID()
		})
}

// try to get matched results with nothing in memory pools
func (ms *MatchingSuite) TestSealableResultsEmptyMempools() {
	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err, "should not error with empty mempools")
	ms.Assert().Empty(results, "should not have matched results with empty mempools")
}

// Try to seal a result for which we don't have the block.
// This tests verifies that Matching engine is performing self-consistency checking:
// Not finding the block for an incorporated result is a fatal
// implementation bug, as we only add results to the IncorporatedResults
// mempool, where _both_ the block that incorporates the result as well
// as the block the result pertains to are known
func (ms *MatchingSuite) TestSealableResultsMissingBlock() {
	incorporatedResult := unittest.IncorporatedResultFixture()
	ms.pendingResults[incorporatedResult.ID()] = incorporatedResult

	_, err := ms.matching.sealableResults()
	ms.Require().Error(err)
}

// Given an incorporated result in the mempool, whose previous result
// (aka parent result) is not known:
//   * skip this result
//   * this result should not be removed from the mempool
func (ms *MatchingSuite) TestSealableResultUnknownPrevious() {
	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block
	incorporatedResult := unittest.IncorporatedResultForBlockFixture(&block)

	ms.pendingResults[incorporatedResult.ID()] = incorporatedResult

	// check that it is looking for the previous result, but return nil as if
	// not found
	ms.resultsDB.On("ByID", mock.Anything).Run(
		func(args mock.Arguments) {
			previousResultID := args.Get(0).(flow.Identifier)
			ms.Assert().Equal(incorporatedResult.Result.PreviousResultID, previousResultID)
		},
	).Return(nil, storerr.ErrNotFound)

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
	blockA := unittest.BlockFixture() // the parent block's ID is randomly generated here

	// RESULTS for blocks:
	resultR2 := unittest.ExecutionResultFixture() // the result pertains to a block whose ID is random generated here
	resultR1 := unittest.ExecutionResultFixture(
		unittest.WithBlock(&blockA),
		unittest.WithPreviousResult(resultR2.ID()),
	)

	// Exec Receipt for block with valid subgraph
	incorporatedResult := unittest.IncorporatedResult.Fixture(unittest.IncorporatedResult.WithResult(resultR1))

	// add entities to mempools and persistent storage mocks:
	ms.blocks[blockA.Header.ID()] = &blockA
	ms.persistedResults[resultR2.ID()] = resultR2
	ms.persistedResults[resultR1.ID()] = resultR1
	ms.pendingResults[incorporatedResult.ID()] = incorporatedResult

	// we expect business logic to remove the incorporated result with failed sub-graph check from mempool
	ms.resultsPL.On("Rem", entityWithID(incorporatedResult.ID())).Return(true).Once()

	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	ms.Assert().Empty(results, "should not select result with invalid subgraph")

	ms.resultsPL.AssertExpectations(ms.T()) // asserts that resultsPL.Rem(incorporatedResult.ID()) was called
}

func (ms *MatchingSuite) TestSealResultInvalidChunks() {

	// try to seal a result with a mismatching chunk count (one too many)
	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block
	incorporatedResult := unittest.IncorporatedResultForBlockFixture(&block)
	previous := unittest.ExecutionResultFixture()
	previous.BlockID = block.Header.ParentID
	incorporatedResult.Result.PreviousResultID = previous.ID()

	// add an extra chunk
	chunk := unittest.ChunkFixture(block.ID())
	chunk.Index = uint64(len(block.Payload.Guarantees))
	incorporatedResult.Result.Chunks = append(incorporatedResult.Result.Chunks, chunk)

	// add incorporated result to mempool
	ms.pendingResults[incorporatedResult.Result.ID()] = incorporatedResult

	// check that it is looking for the previous result, and return previous
	ms.resultsPL.On("ByResultID", mock.Anything).Run(
		func(args mock.Arguments) {
			previousResultID := args.Get(0).(flow.Identifier)
			ms.Assert().Equal(incorporatedResult.Result.PreviousResultID, previousResultID)
		},
	).Return(previous, nil)

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

func (ms *MatchingSuite) TestSealableResultsNoPayload() {

	block := unittest.BlockFixture()
	block.Payload = nil // empty payload
	ms.blocks[block.Header.ID()] = &block
	incorporatedResult := unittest.IncorporatedResultForBlockFixture(&block)
	previous := unittest.ExecutionResultFixture()
	previous.BlockID = block.Header.ParentID
	incorporatedResult.Result.PreviousResultID = previous.ID()

	// add incorporated result to mempool
	ms.pendingResults[incorporatedResult.Result.ID()] = incorporatedResult

	// check that it is looking for the previous result, and return previous
	ms.resultsPL.On("ByResultID", mock.Anything).Run(
		func(args mock.Arguments) {
			previousResultID := args.Get(0).(flow.Identifier)
			ms.Assert().Equal(incorporatedResult.Result.PreviousResultID, previousResultID)
		},
	).Return(previous, nil)

	// check that we are trying to remove the incorporated result from mempool
	ms.resultsPL.On("Rem", mock.Anything).Run(
		func(args mock.Arguments) {
			incResult := args.Get(0).(*flow.IncorporatedResult)
			ms.Assert().Equal(incorporatedResult.ID(), incResult.ID())
		},
	).Return(true)

	assignment := chunks.NewAssignment()
	ms.assigner.On("Assign", incorporatedResult.Result, incorporatedResult.IncorporatedBlockID).Return(assignment, nil)

	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	if ms.Assert().Len(results, 1, "should select result for empty block") {
		sealable := results[0]
		ms.Assert().Equal(incorporatedResult, sealable)
	}
}

func (ms *MatchingSuite) TestSealableResultsUnassignedVerifiers() {

	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block
	incorporatedResult := unittest.IncorporatedResultForBlockFixture(&block)
	previous := unittest.ExecutionResultFixture()
	previous.BlockID = block.Header.ParentID
	incorporatedResult.Result.PreviousResultID = previous.ID()

	// add incorporated result to mempool
	ms.pendingResults[incorporatedResult.Result.ID()] = incorporatedResult

	// check that it is looking for the previous result, and return previous
	ms.resultsPL.On("ByResultID", mock.Anything).Run(
		func(args mock.Arguments) {
			previousResultID := args.Get(0).(flow.Identifier)
			ms.Assert().Equal(incorporatedResult.Result.PreviousResultID, previousResultID)
		},
	).Return(previous, nil)

	// list of 3 approvers
	assignedApprovers := ms.approvers[:3]

	// create assignment with 3 verification node assigned to every chunk
	assignment := chunks.NewAssignment()
	for _, chunk := range incorporatedResult.Result.Chunks {
		assignment.Add(chunk, assignedApprovers.NodeIDs())
	}
	// mock assigner
	ms.assigner.On("Assign", incorporatedResult.Result, incorporatedResult.IncorporatedBlockID).Return(assignment, nil)

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

	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	ms.Assert().Len(results, 0, "should not count approvals from unassigned verifiers")
}

// Insert an approval from a node that wasn't a staked verifier at that block
// (this may occur when the block wasn't know when the node received the
// approval). Ensure that the approval is removed from the mempool when the
// block becomes known.
func (ms *MatchingSuite) TestRemoveApprovalsFromInvalidVerifiers() {
	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block
	incorporatedResult := unittest.IncorporatedResultForBlockFixture(&block)
	previous := unittest.ExecutionResultFixture()
	previous.BlockID = block.Header.ParentID
	incorporatedResult.Result.PreviousResultID = previous.ID()

	// add incorporated result to mempool
	ms.pendingResults[incorporatedResult.Result.ID()] = incorporatedResult

	// check that it is looking for the previous result, and return previous
	ms.resultsPL.On("ByResultID", mock.Anything).Run(
		func(args mock.Arguments) {
			previousResultID := args.Get(0).(flow.Identifier)
			ms.Assert().Equal(incorporatedResult.Result.PreviousResultID, previousResultID)
		},
	).Return(previous, nil)

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

	// with requireApprovals = true ( default test case ), it should not collect
	// any results because we haven't added any approvals to the mempool
	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	ms.Assert().Empty(results, "should not select result with insufficient approvals")

	// should have deleted the approval of the first chunk
	ms.Assert().Empty(ms.matching.approvals.All(), "should have removed the approval")
}

func (ms *MatchingSuite) TestSealableResultsInsufficientApprovals() {

	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block
	incorporatedResult := unittest.IncorporatedResultForBlockFixture(&block)
	previous := unittest.ExecutionResultFixture()
	previous.BlockID = block.Header.ParentID
	incorporatedResult.Result.PreviousResultID = previous.ID()

	// add incorporated result to mempool
	ms.pendingResults[incorporatedResult.Result.ID()] = incorporatedResult

	// check that it is looking for the previous result, and return previous
	ms.resultsPL.On("ByResultID", mock.Anything).Run(
		func(args mock.Arguments) {
			previousResultID := args.Get(0).(flow.Identifier)
			ms.Assert().Equal(incorporatedResult.Result.PreviousResultID, previousResultID)
		},
	).Return(previous, nil)

	// check that we are trying to remove the incorporated result from mempool
	ms.resultsPL.On("Rem", mock.Anything).Run(
		func(args mock.Arguments) {
			incResult := args.Get(0).(*flow.IncorporatedResult)
			ms.Assert().Equal(incorporatedResult.ID(), incResult.ID())
		},
	).Return(true)

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

// insert a well-formed incorporated result in the mempool, as well as a
// sufficient number of valid result approvals, and check that the seal is
// correctly generated.
func (ms *MatchingSuite) TestSealValid() {

	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block
	incorporatedResult := unittest.IncorporatedResultForBlockFixture(&block)
	previous := unittest.ExecutionResultFixture()
	previous.BlockID = block.Header.ParentID
	incorporatedResult.Result.PreviousResultID = previous.ID()

	// add incorporated result to mempool
	ms.pendingResults[incorporatedResult.Result.ID()] = incorporatedResult

	// check that it is looking for the previous result, and return previous
	ms.resultsPL.On("ByResultID", mock.Anything).Run(
		func(args mock.Arguments) {
			previousResultID := args.Get(0).(flow.Identifier)
			ms.Assert().Equal(incorporatedResult.Result.PreviousResultID, previousResultID)
		},
	).Return(previous, nil)

	// check that we are trying to remove the incorporated result from mempool
	ms.resultsPL.On("Rem", mock.Anything).Run(
		func(args mock.Arguments) {
			incResult := args.Get(0).(*flow.IncorporatedResult)
			ms.Assert().Equal(incorporatedResult.ID(), incResult.ID())
		},
	).Return(true)

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

	ms.resultsDB.AssertNumberOfCalls(ms.T(), "Store", 1)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
}

func (ms *MatchingSuite) TestRequestReceiptsPendingBlocks() {
	n := 100

	// Create n consecutive blocks
	// the first one is sealed and the last one is final

	headers := []flow.Header{}

	parentHeader := flow.Header{
		ChainID:        flow.Emulator,
		ParentID:       unittest.IdentifierFixture(),
		Height:         0,
		PayloadHash:    unittest.IdentifierFixture(),
		Timestamp:      time.Now().UTC(),
		View:           uint64(rand.Intn(1000)),
		ParentVoterIDs: unittest.IdentifierListFixture(4),
		ParentVoterSig: unittest.SignatureFixture(),
		ProposerID:     unittest.IdentifierFixture(),
		ProposerSig:    unittest.SignatureFixture(),
	}

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

	// the results are not in the DB, which will trigger request
	ms.resultsDB.On("ByBlockID", mock.Anything).Return(nil, storerr.ErrNotFound)

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
