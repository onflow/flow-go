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

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/chunks"
	"github.com/dapperlabs/flow-go/model/flow"
	mempool "github.com/dapperlabs/flow-go/module/mempool/mock"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	"github.com/dapperlabs/flow-go/module/metrics"
	module "github.com/dapperlabs/flow-go/module/mock"
	realproto "github.com/dapperlabs/flow-go/state/protocol"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
	storerr "github.com/dapperlabs/flow-go/storage"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// 1. Matching engine should validate the incoming receipt (aka ExecutionReceipt):
//     1. it should stores it to the mempool if valid
//     2. it should ignore it when:
//         1. the origin is invalid
//         2. the role is invalid
//         3. the receipt has been received before
// 		   4. the corresponding block has been sealed
// 2. Matching engine should validate the incoming approval (aka ResultApproval):
//     1. it should store it to the mempool if valid
//     2. it should ignore it when:
//         1. the origin is invalid
//         2. the role is invalid
//         3. the corresponding block has been sealed
// 3. Matching engine should be able to find matched results:
//     1. It should find no matched result if there is no result and no approval
//     2. it should find 1 matched result if we received a receipt, and the block has no payload (impossible now, system every block will have at least one chunk to verify)
//     3. It should find no matched result if there is only result, but no approval
// 4. Matching engine should be able to seal a matched result:
//     1. It should not seal a matched result if:
//         1. the block is missing (consensus hasn’t received this executed block yet)
//         2. the approvals for a certain chunk are insufficient
//         3. there is some chunk didn’t receive enough approvals
//         4. the previous result is not known
//     2. It should seal a matched result if the approvals are sufficient
// 5. Matching engine should request results from execution nodes:
//     1. If there are unsealed and finalized blocks, it should request the execution receipts from the execution nodes.
func TestMatchingEngine(t *testing.T) {
	suite.Run(t, new(MatchingSuite))
}

type MatchingSuite struct {
	suite.Suite

	conID flow.Identifier
	exeID flow.Identifier
	verID flow.Identifier

	identities map[flow.Identifier]*flow.Identity

	approvers flow.IdentityList

	state *protocol.State

	sealedSnapshot *protocol.Snapshot
	finalSnapshot  *protocol.Snapshot

	sealedResults map[flow.Identifier]*flow.ExecutionResult
	blocks        map[flow.Identifier]*flow.Block
	seals         map[flow.Identifier]*flow.Seal // indexed by block id

	resultsDB *storage.ExecutionResults
	headersDB *storage.Headers
	indexDB   *storage.Index
	sealsDB   *storage.Seals

	pendingResults   map[flow.Identifier]*flow.IncorporatedResult
	pendingReceipts  map[flow.Identifier]*flow.ExecutionReceipt
	pendingApprovals map[flow.Identifier]*flow.ResultApproval
	pendingSeals     map[flow.Identifier]*flow.IncorporatedResultSeal

	resultsPL   *mempool.IncorporatedResults
	receiptsPL  *mempool.Receipts
	approvalsPL *mempool.Approvals
	sealsPL     *mempool.IncorporatedResultSeals

	requester *module.Requester

	assigner *module.ChunkAssigner

	matching *Engine
}

func (ms *MatchingSuite) SetupTest() {

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

	ms.state = &protocol.State{}
	ms.state.On("Sealed").Return(
		func() realproto.Snapshot {
			return ms.sealedSnapshot
		},
		nil,
	)
	ms.state.On("Final").Return(
		func() realproto.Snapshot {
			return ms.finalSnapshot
		},
		nil,
	)

	ms.finalSnapshot = &protocol.Snapshot{}
	ms.finalSnapshot.On("Identity", mock.Anything).Return(
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
	ms.finalSnapshot.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter) flow.IdentityList {
			return ms.approvers
		},
		func(selector flow.IdentityFilter) error {
			return nil
		},
	)

	ms.sealedSnapshot = &protocol.Snapshot{}

	ms.sealedResults = make(map[flow.Identifier]*flow.ExecutionResult)
	ms.blocks = make(map[flow.Identifier]*flow.Block)
	ms.seals = make(map[flow.Identifier]*flow.Seal)

	ms.resultsDB = &storage.ExecutionResults{}
	ms.resultsDB.On("ByID", mock.Anything).Return(
		func(resultID flow.Identifier) *flow.ExecutionResult {
			return ms.sealedResults[resultID]
		},
		func(resultID flow.Identifier) error {
			_, found := ms.sealedResults[resultID]
			if !found {
				return storerr.ErrNotFound
			}
			return nil
		},
	)
	ms.resultsDB.On("Index", mock.Anything, mock.Anything).Return(
		func(blockID, resultID flow.Identifier) error {
			return nil
		},
	)

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

	ms.indexDB = &storage.Index{}
	ms.indexDB.On("ByBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) *flow.Index {
			block, found := ms.blocks[blockID]
			if !found {
				return nil
			}
			return block.Payload.Index()
		},
		func(blockID flow.Identifier) error {
			_, found := ms.blocks[blockID]
			if !found {
				return storerr.ErrNotFound
			}
			return nil
		},
	)

	ms.sealsDB = &storage.Seals{}
	ms.sealsDB.On("ByBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) *flow.Seal {
			seal, found := ms.seals[blockID]
			if !found {
				return nil
			}
			return seal
		},
		func(blockID flow.Identifier) error {
			_, found := ms.seals[blockID]
			if !found {
				return storerr.ErrNotFound
			}
			return nil
		},
	)

	ms.pendingResults = make(map[flow.Identifier]*flow.IncorporatedResult)
	ms.pendingReceipts = make(map[flow.Identifier]*flow.ExecutionReceipt)
	ms.pendingApprovals = make(map[flow.Identifier]*flow.ResultApproval)
	ms.pendingSeals = make(map[flow.Identifier]*flow.IncorporatedResultSeal)

	ms.resultsPL = &mempool.IncorporatedResults{}
	ms.resultsPL.On("Size").Return(uint(0)) // only for metrics
	ms.resultsPL.On("ByIncorporatedBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) []*flow.IncorporatedResult {
			res := []*flow.IncorporatedResult{}
			if ir, ok := ms.pendingResults[blockID]; ok {
				res = append(res, ir)
			}
			return res
		},
		func(blockID flow.Identifier) bool {
			_, found := ms.pendingResults[blockID]
			return found
		},
	)
	ms.resultsPL.On("All").Return(
		func() []*flow.IncorporatedResult {
			results := make([]*flow.IncorporatedResult, 0, len(ms.pendingResults))
			for _, result := range ms.pendingResults {
				results = append(results, result)
			}
			return results
		},
	)

	ms.receiptsPL = &mempool.Receipts{}
	ms.receiptsPL.On("Size").Return(uint(0)) // only for metrics
	ms.receiptsPL.On("ByID", mock.Anything).Return(
		func(receiptID flow.Identifier) *flow.ExecutionReceipt {
			return ms.pendingReceipts[receiptID]
		},
		func(receiptID flow.Identifier) bool {
			_, found := ms.pendingReceipts[receiptID]
			return found
		},
	)

	ms.approvalsPL = &mempool.Approvals{}
	ms.approvalsPL.On("Size").Return(uint(0)) // only for metrics

	ms.sealsPL = &mempool.IncorporatedResultSeals{}
	ms.sealsPL.On("Size").Return(uint(0)) // only for metrics

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
		sealsDB:                 ms.sealsDB,
		incorporatedResults:     ms.resultsPL,
		receipts:                ms.receiptsPL,
		approvals:               ms.approvalsPL,
		seals:                   ms.sealsPL,
		checkingSealing:         atomic.NewBool(false),
		requestReceiptThreshold: 10,
		maxUnsealedResults:      200,
		assigner:                ms.assigner,
	}
}

func (ms *MatchingSuite) TestOnReceiptInvalidOrigin() {

	// try to submit a receipt with a random origin ID
	originID := ms.exeID
	receipt := unittest.ExecutionReceiptFixture()

	err := ms.matching.onReceipt(originID, receipt)
	ms.Require().Error(err, "should reject receipt with mismatching origin and executor")

	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.receiptsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestOnReceiptInvalidRole() {

	// try ot submit a receipt from a consensus node
	originID := ms.conID
	receipt := unittest.ExecutionReceiptFixture()
	receipt.ExecutorID = originID

	err := ms.matching.onReceipt(originID, receipt)
	ms.Require().Error(err, "should reject receipt from wrong node role")

	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.receiptsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestOnReceiptUnstakedExecutor() {

	// try ot submit a receipt from an unstaked node
	originID := ms.exeID
	receipt := unittest.ExecutionReceiptFixture()
	receipt.ExecutorID = originID
	ms.identities[originID].Stake = 0

	err := ms.matching.onReceipt(originID, receipt)
	ms.Require().Error(err, "should reject receipt from unstaked node")

	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.receiptsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestOnReceiptSealedBlock() {

	// try to submit a receipt for a sealed result
	originID := ms.exeID
	receipt := unittest.ExecutionReceiptFixture()
	receipt.ExecutorID = originID
	ms.seals[receipt.ExecutionResult.BlockID] = &flow.Seal{}

	err := ms.matching.onReceipt(originID, receipt)
	ms.Require().NoError(err, "should ignore receipt for sealed block")

	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.receiptsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestOnReceiptPendingReceipt() {

	// try to submit a receipt for a sealed result
	originID := ms.exeID
	receipt := unittest.ExecutionReceiptFixture()
	receipt.ExecutorID = originID

	// check parameters are correct for calls
	ms.receiptsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			added := args.Get(0).(*flow.ExecutionReceipt)
			ms.Assert().Equal(receipt, added)
		},
	).Return(false)

	err := ms.matching.onReceipt(originID, receipt)
	ms.Require().NoError(err, "should ignore already pending receipt")

	ms.receiptsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestOnReceiptValid() {

	// try to submit a receipt that should be valid
	originID := ms.exeID
	receipt := unittest.ExecutionReceiptFixture()
	receipt.ExecutorID = originID

	// check parameters are correct for calls
	ms.receiptsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			added := args.Get(0).(*flow.ExecutionReceipt)
			ms.Assert().Equal(receipt, added)
		},
	).Return(true)

	err := ms.matching.onReceipt(originID, receipt)
	ms.Require().NoError(err, "should add receipt to mempool if valid")

	ms.receiptsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestOnApprovalInvalidOrigin() {

	// try to submit an approval with a random origin ID
	originID := ms.verID
	approval := unittest.ResultApprovalFixture()

	err := ms.matching.onApproval(originID, approval)
	ms.Require().Error(err, "should reject approval with mismatching origin and executor")

	ms.approvalsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestOnApprovalInvalidRole() {

	// try to submit an approval from a consensus node
	originID := ms.conID
	approval := unittest.ResultApprovalFixture()
	approval.Body.ApproverID = originID

	err := ms.matching.onApproval(originID, approval)
	ms.Require().Error(err, "should reject approval from wrong approver role")

	ms.approvalsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestOnApprovalInvalidStake() {

	// try to submit an approval from an unstaked approver
	originID := ms.verID
	approval := unittest.ResultApprovalFixture()
	approval.Body.ApproverID = originID
	ms.identities[originID].Stake = 0

	err := ms.matching.onApproval(originID, approval)
	ms.Require().Error(err, "should reject approval from unstaked approver")

	ms.approvalsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestOnApprovalSealedBlock() {

	// try to submit an approval for a sealed block
	originID := ms.verID
	approval := unittest.ResultApprovalFixture()
	approval.Body.ApproverID = originID
	ms.seals[approval.Body.BlockID] = &flow.Seal{}

	err := ms.matching.onApproval(originID, approval)
	ms.Require().NoError(err, "should ignore approval for sealed block")

	ms.approvalsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestOnApprovalPendingApproval() {

	// try to submit an approval that is already in the mempool
	originID := ms.verID
	approval := unittest.ResultApprovalFixture()
	approval.Body.ApproverID = originID

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

func (ms *MatchingSuite) TestOnApprovalValid() {

	// try to submit an approval for a sealed result
	originID := ms.verID
	approval := unittest.ResultApprovalFixture()
	approval.Body.ApproverID = originID

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

func (ms *MatchingSuite) TestMatchedResultsEmptyMempools() {

	// try to get matched results with nothing in memory pools
	results, err := ms.matching.matchedResults()
	ms.Require().NoError(err, "should not error with empty mempools")
	ms.Assert().Empty(results, "should not have matched results with empty mempools")
}

func (ms *MatchingSuite) TestMatchedResultsNoPayload() {

	// add a block with a specific guarantee to the DB
	block := unittest.BlockFixture()
	block.Payload.Guarantees = nil
	ms.blocks[block.Header.ID()] = &block

	// add a result for this block to the mempool
	result := unittest.ResultForBlockFixture(&block)
	incorporationBlockID := unittest.IdentifierFixture()
	incorporatedResult := &flow.IncorporatedResult{
		IncorporatedBlockID: incorporationBlockID,
		Result:              result,
	}
	ms.pendingResults[incorporationBlockID] = incorporatedResult

	assignment := chunks.NewAssignment()
	ms.assigner.On("Assign", result, incorporationBlockID).Return(assignment, nil)

	results, err := ms.matching.matchedResults()
	ms.Require().NoError(err)
	if ms.Assert().Len(results, 1, "should select result for empty block") {
		sealable := results[0]
		ms.Assert().Equal(incorporatedResult, sealable)
	}
}

func (ms *MatchingSuite) TestMatchedResultsInsufficientApprovals() {

	// add a block with a specific guarantee to the DB
	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block

	// add a result for this block to the mempool
	result := unittest.ResultForBlockFixture(&block)
	incorporationBlockID := unittest.IdentifierFixture()
	incorporatedResult := &flow.IncorporatedResult{
		IncorporatedBlockID: incorporationBlockID,
		Result:              result,
	}
	ms.pendingResults[incorporationBlockID] = incorporatedResult

	// check calls have the correct parameters, and return 0 approvals
	ms.approvalsPL.On("ByChunk", mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			resultID := args.Get(0).(flow.Identifier)
			ms.Assert().Equal(result.ID(), resultID)
		},
	).Return(nil, false)

	// only include 3/4 approvals
	assignment := chunks.NewAssignment()
	approvers := ms.approvers[:3]
	for _, chunk := range result.Chunks {
		assignment.Add(chunk, approvers.NodeIDs())
	}

	ms.assigner.On("Assign", result, incorporationBlockID).Return(assignment, nil)

	results, err := ms.matching.matchedResults()
	ms.Require().NoError(err)
	ms.Assert().Empty(results, "should not select result with insufficient approvals")
}

func (ms *MatchingSuite) TestMatchedResultsSufficientApprovals() {

	// add a block with a specific guarantee to the DB
	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block

	// add a result for this block to the mempool
	result := unittest.ResultForBlockFixture(&block)
	incorporationBlockID := unittest.IdentifierFixture()
	incorporatedResult := &flow.IncorporatedResult{
		IncorporatedBlockID: incorporationBlockID,
		Result:              result,
	}
	ms.pendingResults[incorporationBlockID] = incorporatedResult

	assignment := chunks.NewAssignment()
	approvers := ms.approvers[:3]
	for _, chunk := range result.Chunks {
		assignment.Add(chunk, approvers.NodeIDs())
	}
	ms.assigner.On("Assign", result, incorporationBlockID).Return(assignment, nil)

	realApprovalPool, err := stdmap.NewApprovals(1000)
	ms.Require().NoError(err)
	ms.matching.approvals = realApprovalPool

	// add enough approvals for each chunk
	for _, approver := range approvers {
		for index := uint64(0); index < uint64(len(result.Chunks)); index++ {
			approval := unittest.ResultApprovalFixture()
			approval.Body.BlockID = block.Header.ID()
			approval.Body.ExecutionResultID = result.ID()
			approval.Body.ApproverID = approver.NodeID
			approval.Body.ChunkIndex = index
			_, err := ms.matching.approvals.Add(approval)
			ms.Require().NoError(err)
		}
	}

	results, err := ms.matching.matchedResults()
	ms.Require().NoError(err)
	if ms.Assert().Len(results, 1, "should select result with sufficient approvals") {
		sealable := results[0]
		ms.Assert().Equal(incorporatedResult, sealable)
	}
}

func (ms *MatchingSuite) TestMatchedResultsUnassignedVerifiers() {

	// add a block with a specific guarantee to the DB
	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block

	// add a result for this block to the mempool
	result := unittest.ResultForBlockFixture(&block)
	incorporationBlockID := unittest.IdentifierFixture()
	incorporatedResult := &flow.IncorporatedResult{
		IncorporatedBlockID: incorporationBlockID,
		Result:              result,
	}
	ms.pendingResults[incorporationBlockID] = incorporatedResult

	// list of 3 approvers
	assignedApprovers := ms.approvers[:3]

	// create assignment with 3 verification node assigned to every chunk
	assignment := chunks.NewAssignment()
	for _, chunk := range result.Chunks {
		assignment.Add(chunk, assignedApprovers.NodeIDs())
	}

	realApprovalPool, err := stdmap.NewApprovals(1000)
	ms.Require().NoError(err)
	ms.matching.approvals = realApprovalPool

	// approve every chunk by an unassigned verifier.
	unassignedApprover := ms.approvers[3]
	for index := uint64(0); index < uint64(len(result.Chunks)); index++ {
		approval := unittest.ResultApprovalFixture()
		approval.Body.BlockID = block.Header.ID()
		approval.Body.ExecutionResultID = result.ID()
		approval.Body.ApproverID = unassignedApprover.NodeID
		approval.Body.ChunkIndex = index
		_, err := ms.matching.approvals.Add(approval)
		ms.Require().NoError(err)
	}

	// mock assigner
	ms.assigner.On("Assign", result, incorporationBlockID).Return(assignment, nil)

	results, err := ms.matching.matchedResults()
	ms.Require().NoError(err)
	ms.Assert().Len(results, 0, "should not count approvals from unassigned verifiers")
}

func (ms *MatchingSuite) TestSealResultMissingBlock() {

	// try to seal a result for which we don't have the index payload
	incorporatedResult := unittest.IncorporatedResultFixture()

	err := ms.matching.sealResult(incorporatedResult)
	ms.Require().Equal(errUnknownBlock, err, "should get unknown block error on missing block")

	ms.resultsDB.AssertNumberOfCalls(ms.T(), "Store", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestSealResultInvalidChunks() {

	// try to seal a result with a mismatching chunk count
	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block
	result := unittest.ResultForBlockFixture(&block)
	chunk := unittest.ChunkFixture()
	chunk.Index = uint64(len(block.Payload.Guarantees))
	result.Chunks = append(result.Chunks, chunk)
	incorporationBlockID := unittest.IdentifierFixture()
	incorporatedResult := &flow.IncorporatedResult{
		IncorporatedBlockID: incorporationBlockID,
		Result:              result,
	}

	err := ms.matching.sealResult(incorporatedResult)
	ms.Require().Equal(errInvalidChunks, err, "should get invalid chunks error on wrong chunk count")

	ms.resultsDB.AssertNumberOfCalls(ms.T(), "Store", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestSealResultUnknownPrevious() {

	// try to seal a result with a missing previous result
	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block
	incorporatedResult := unittest.IncorporatedResultForBlockFixture(&block)

	err := ms.matching.sealResult(incorporatedResult)
	ms.Require().Equal(errUnknownPrevious, err, "should get unknown previous error")

	ms.resultsDB.AssertNumberOfCalls(ms.T(), "Store", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestSealResultValid() {

	// try to seal a result with a persisted previous result
	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block
	result := unittest.ResultForBlockFixture(&block)
	previous := unittest.ExecutionResultFixture()
	result.PreviousResultID = previous.ID()
	ms.sealedResults[previous.ID()] = previous
	incorporationBlockID := unittest.IdentifierFixture()
	incorporatedResult := &flow.IncorporatedResult{
		IncorporatedBlockID: incorporationBlockID,
		Result:              result,
	}

	// check match when we are storing entities
	ms.resultsDB.On("Store", mock.Anything).Run(
		func(args mock.Arguments) {
			stored := args.Get(0).(*flow.ExecutionResult)
			ms.Assert().Equal(result, stored)
		},
	).Return(nil)

	ms.sealsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			seal := args.Get(0).(*flow.IncorporatedResultSeal)
			ms.Assert().Equal(incorporatedResult.ID(), seal.IncorporatedResult.ID())
			ms.Assert().Equal(result.BlockID, seal.Seal.BlockID)
		},
	).Return(true)

	err := ms.matching.sealResult(incorporatedResult)
	ms.Require().NoError(err, "should generate seal on persisted previous result")

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
		payload := unittest.PayloadFixture(unittest.WithoutIdentities, unittest.WithoutSeals)
		header := headers[i]
		header.PayloadHash = payload.Hash()
		block := flow.Block{
			Header:  &header,
			Payload: payload,
		}
		ms.blocks[block.ID()] = &block
		orderedBlocks = append(orderedBlocks, block)
	}

	ms.finalSnapshot.On("Head").Return(
		func() *flow.Header {
			return orderedBlocks[n-1].Header
		},
		nil,
	)

	ms.sealedSnapshot.On("Head").Return(
		func() *flow.Header {
			return orderedBlocks[0].Header
		},
		nil,
	)

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
	ms.Assert().Equal(n-1, len(requestedBlocks))
}
