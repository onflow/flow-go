// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package matching

import (
	"fmt"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	mempool "github.com/dapperlabs/flow-go/module/mempool/mock"
	"github.com/dapperlabs/flow-go/module/metrics"
	realproto "github.com/dapperlabs/flow-go/state/protocol"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
	storerr "github.com/dapperlabs/flow-go/storage"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

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

	state    *protocol.State
	snapshot *protocol.Snapshot

	results map[flow.Identifier]*flow.ExecutionResult
	blocks  map[flow.Identifier]*flow.Block

	resultsDB *storage.ExecutionResults
	headersDB *storage.Headers
	indexDB   *storage.Index

	pendingResults   map[flow.Identifier]*flow.ExecutionResult
	pendingReceipts  map[flow.Identifier]*flow.ExecutionReceipt
	pendingApprovals map[flow.Identifier]*flow.ResultApproval
	pendingSeals     map[flow.Identifier]*flow.Seal

	resultsPL   *mempool.Results
	receiptsPL  *mempool.Receipts
	approvalsPL *mempool.Approvals
	sealsPL     *mempool.Seals

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
	ms.state.On("Final").Return(
		func() realproto.Snapshot {
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
	ms.snapshot.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter) flow.IdentityList {
			return ms.approvers
		},
		nil,
	)

	ms.results = make(map[flow.Identifier]*flow.ExecutionResult)
	ms.blocks = make(map[flow.Identifier]*flow.Block)

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

	ms.indexDB = &storage.Index{}
	ms.indexDB.On("ByBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) flow.Index {
			block, found := ms.blocks[blockID]
			if !found {
				return flow.Index{}
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

	ms.pendingResults = make(map[flow.Identifier]*flow.ExecutionResult)
	ms.pendingReceipts = make(map[flow.Identifier]*flow.ExecutionReceipt)
	ms.pendingApprovals = make(map[flow.Identifier]*flow.ResultApproval)
	ms.pendingSeals = make(map[flow.Identifier]*flow.Seal)

	ms.resultsPL = &mempool.Results{}
	ms.resultsPL.On("Size").Return(uint(0)) // only for metrics
	ms.resultsPL.On("ByID", mock.Anything).Return(
		func(resultID flow.Identifier) *flow.ExecutionResult {
			return ms.pendingResults[resultID]
		},
		func(resultID flow.Identifier) bool {
			_, found := ms.pendingResults[resultID]
			return found
		},
	)
	ms.resultsPL.On("All").Return(
		func() []*flow.ExecutionResult {
			results := make([]*flow.ExecutionResult, 0, len(ms.pendingResults))
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
	ms.approvalsPL.On("ByID", mock.Anything).Return(
		func(approvalID flow.Identifier) *flow.ResultApproval {
			return ms.pendingApprovals[approvalID]
		},
		func(approvalID flow.Identifier) bool {
			_, found := ms.pendingApprovals[approvalID]
			return found
		},
	)
	ms.approvalsPL.On("All").Return(
		func() []*flow.ResultApproval {
			approvals := make([]*flow.ResultApproval, 0, len(ms.pendingApprovals))
			for _, approval := range ms.pendingApprovals {
				approvals = append(approvals, approval)
			}
			return approvals
		},
	)

	ms.sealsPL = &mempool.Seals{}
	ms.sealsPL.On("Size").Return(uint(0)) // only for metrics
	ms.sealsPL.On("ByID", mock.Anything).Return(
		func(sealID flow.Identifier) *flow.Seal {
			return ms.pendingSeals[sealID]
		},
		func(sealID flow.Identifier) bool {
			_, found := ms.pendingSeals[sealID]
			return found
		},
	)

	ms.matching = &Engine{
		unit:      unit,
		log:       log,
		metrics:   metrics,
		mempool:   metrics,
		state:     ms.state,
		resultsDB: ms.resultsDB,
		headersDB: ms.headersDB,
		indexDB:   ms.indexDB,
		results:   ms.resultsPL,
		receipts:  ms.receiptsPL,
		approvals: ms.approvalsPL,
		seals:     ms.sealsPL,
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

func (ms *MatchingSuite) TestOnReceiptSealedResult() {

	// try to submit a receipt for a sealed result
	originID := ms.exeID
	receipt := unittest.ExecutionReceiptFixture()
	receipt.ExecutorID = originID
	ms.results[receipt.ExecutionResult.ID()] = &receipt.ExecutionResult

	err := ms.matching.onReceipt(originID, receipt)
	ms.Require().NoError(err, "should ignore receipt for sealed result")

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

	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.receiptsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestOnReceiptPendingResult() {

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
	).Return(true)
	ms.resultsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			result := args.Get(0).(*flow.ExecutionResult)
			ms.Assert().Equal(result, &receipt.ExecutionResult)
		},
	).Return(false)

	err := ms.matching.onReceipt(originID, receipt)
	ms.Require().NoError(err, "should ignore receipt for already pending result")

	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
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
	ms.resultsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			result := args.Get(0).(*flow.ExecutionResult)
			ms.Assert().Equal(result, &receipt.ExecutionResult)
		},
	).Return(true)

	err := ms.matching.onReceipt(originID, receipt)
	ms.Require().NoError(err, "should ignore receipt for already pending result")

	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
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

func (ms *MatchingSuite) TestOnApprovalSealedResult() {

	// try to submit an approval for a sealed result
	originID := ms.verID
	approval := unittest.ResultApprovalFixture()
	approval.Body.ApproverID = originID
	ms.results[approval.Body.ExecutionResultID] = unittest.ExecutionResultFixture()

	err := ms.matching.onApproval(originID, approval)
	ms.Require().NoError(err, "should ignore approval for sealed result")

	ms.approvalsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestOnApprovalPendingApproval() {

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
	).Return(false)

	err := ms.matching.onApproval(originID, approval)
	ms.Require().NoError(err, "should ignore approval for sealed result")

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
	).Return(true)

	err := ms.matching.onApproval(originID, approval)
	ms.Require().NoError(err, "should ignore approval for sealed result")

	ms.approvalsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestSealableResultsEmptyMempools() {

	// try to get sealable results with nothing in memory pools

	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err, "should not error with empty mempools")
	ms.Assert().Empty(results, "should not have sealable results with empty mempools")
}

func (ms *MatchingSuite) TestSealableResultsNoPayload() {

	// add a block with a specific guarantee to the DB
	block := unittest.BlockFixture()
	block.Payload.Guarantees = nil
	ms.blocks[block.Header.ID()] = &block

	// add a result for this block to the DB
	result := unittest.ResultForBlockFixture(&block)
	ms.pendingResults[result.ID()] = result

	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	if ms.Assert().Len(results, 1, "should select result for empty block") {
		sealable := results[0]
		ms.Assert().Equal(result, sealable)
	}
}

func (ms *MatchingSuite) TestSealableResultsNoApprovals() {

	// add a block with a specific guarantee to the DB
	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block

	// add a result for this block to the DB
	result := unittest.ResultForBlockFixture(&block)
	ms.pendingResults[result.ID()] = result

	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	ms.Assert().Empty(results, "should not select result with no approvals")
}

func (ms *MatchingSuite) TestSealableResultsInsufficientApprovals() {

	// add a block with a specific guarantee to the DB
	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block

	// add a result for this block to the DB
	result := unittest.ResultForBlockFixture(&block)
	ms.pendingResults[result.ID()] = result

	// add approvals for each chunk that have insufficient stake
	for n := 0; n < 3; n++ {
		for index := uint64(0); index < uint64(len(result.Chunks)); index++ {
			// skip last chunk for last approval
			if n == 2 && index == uint64(len(result.Chunks)-1) {
				break
			}
			approval := unittest.ResultApprovalFixture()
			approval.Body.BlockID = block.Header.ID()
			approval.Body.ExecutionResultID = result.ID()
			approval.Body.ApproverID = ms.approvers[n].NodeID
			approval.Body.ChunkIndex = index
			ms.pendingApprovals[approval.ID()] = approval
		}
	}

	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	ms.Assert().Empty(results, "should not select result with insufficient approvals")
}

func (ms *MatchingSuite) TestSealableResultsSufficientApprovals() {

	// add a block with a specific guarantee to the DB
	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block

	// add a result for this block to the DB
	result := unittest.ResultForBlockFixture(&block)
	ms.pendingResults[result.ID()] = result

	// add approvals for each chunk that have insufficient stake
	for n := 0; n < 3; n++ {
		for index := uint64(0); index < uint64(len(result.Chunks)); index++ {
			approval := unittest.ResultApprovalFixture()
			approval.Body.BlockID = block.Header.ID()
			approval.Body.ExecutionResultID = result.ID()
			approval.Body.ApproverID = ms.approvers[n].NodeID
			approval.Body.ChunkIndex = index
			ms.pendingApprovals[approval.ID()] = approval
		}
	}

	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	if ms.Assert().Len(results, 1, "should select result with sufficient approvals") {
		sealable := results[0]
		ms.Assert().Equal(result, sealable)
	}
}

func (ms *MatchingSuite) TestSealResultMissingBlock() {

	// try to seal a result for which we don't have the index payload
	result := unittest.ExecutionResultFixture()

	err := ms.matching.sealResult(result)
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

	err := ms.matching.sealResult(result)
	ms.Require().Equal(errInvalidChunks, err, "should get invalid chunks error on wrong chunk count")

	ms.resultsDB.AssertNumberOfCalls(ms.T(), "Store", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestSealResultMissingPrevious() {

	// try to seal a result with a missing previous result
	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block
	result := unittest.ResultForBlockFixture(&block)

	err := ms.matching.sealResult(result)
	ms.Require().Equal(errUnknownPrevious, err, "should get unknown previous error on missing previous result")

	ms.resultsDB.AssertNumberOfCalls(ms.T(), "Store", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestSealResultPendingPrevious() {

	// try to seal a result with a missing previous result
	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block
	result := unittest.ResultForBlockFixture(&block)
	previous := unittest.ExecutionResultFixture()
	result.PreviousResultID = previous.ID()
	ms.pendingResults[previous.ID()] = previous

	// check match when we are storing entities
	ms.resultsDB.On("Store", mock.Anything).Run(
		func(args mock.Arguments) {
			stored := args.Get(0).(*flow.ExecutionResult)
			ms.Assert().Equal(result, stored)
		},
	).Return(nil)
	ms.sealsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			seal := args.Get(0).(*flow.Seal)
			ms.Assert().Equal(result.ID(), seal.ResultID)
			ms.Assert().Equal(result.BlockID, seal.BlockID)
		},
	).Return(true)

	err := ms.matching.sealResult(result)
	ms.Require().NoError(err, "should generate seal on pending previous result")

	ms.resultsDB.AssertNumberOfCalls(ms.T(), "Store", 1)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
}

func (ms *MatchingSuite) TestSealResultValid() {

	// try to seal a result with a persisted previous result
	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block
	result := unittest.ResultForBlockFixture(&block)
	previous := unittest.ExecutionResultFixture()
	result.PreviousResultID = previous.ID()
	ms.results[previous.ID()] = previous

	// check match when we are storing entities
	ms.resultsDB.On("Store", mock.Anything).Run(
		func(args mock.Arguments) {
			stored := args.Get(0).(*flow.ExecutionResult)
			ms.Assert().Equal(result, stored)
		},
	).Return(nil)
	ms.sealsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			seal := args.Get(0).(*flow.Seal)
			ms.Assert().Equal(result.ID(), seal.ResultID)
			ms.Assert().Equal(result.BlockID, seal.BlockID)
		},
	).Return(true)

	err := ms.matching.sealResult(result)
	ms.Require().NoError(err, "should generate seal on persisted previous result")

	ms.resultsDB.AssertNumberOfCalls(ms.T(), "Store", 1)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
}
