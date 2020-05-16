// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package matching

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

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

	log := zerolog.New(ioutil.Discard)
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

func (ms *MatchingSuite) TestOnReceiptValid() {

	// try to submit a receipt that should be valid
	originID := ms.exeID
	receipt := unittest.ExecutionReceiptFixture()
	receipt.ExecutorID = originID
	ms.receiptsPL.On("Add", mock.Anything).Return(true)
	ms.resultsPL.On("Add", mock.Anything).Return(true)

	err := ms.matching.onReceipt(originID, receipt)
	ms.Require().NoError(err, "should ignore receipt for already pending result")

	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
	ms.receiptsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
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
	ms.receiptsPL.On("Add", mock.Anything).Return(false)

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
	ms.receiptsPL.On("Add", mock.Anything).Return(true)
	ms.resultsPL.On("Add", mock.Anything).Return(false)

	err := ms.matching.onReceipt(originID, receipt)
	ms.Require().NoError(err, "should ignore receipt for already pending result")

	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
	ms.receiptsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestOnApprovalValid() {

	// try to submit an approval for a sealed result
	originID := ms.verID
	approval := unittest.ResultApprovalFixture()
	approval.Body.ApproverID = originID
	ms.approvalsPL.On("Add", mock.Anything).Return(true)

	err := ms.matching.onApproval(originID, approval)
	ms.Require().NoError(err, "should ignore approval for sealed result")

	ms.approvalsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
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
	ms.approvalsPL.On("Add", mock.Anything).Return(false)

	err := ms.matching.onApproval(originID, approval)
	ms.Require().NoError(err, "should ignore approval for sealed result")

	ms.approvalsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestSealableResultsNoResults() {

}

func (ms *MatchingSuite) TestSealableResultsNoApprovals() {
}

func (ms *MatchingSuite) TestSealableResultsInsufficientStake() {
}

func (ms *MatchingSuite) TestSealResultValid() {
}

func (ms *MatchingSuite) TestSealResultUnknownBlock() {
}

func (ms *MatchingSuite) TestSealResultUnknownPrevious() {
}
