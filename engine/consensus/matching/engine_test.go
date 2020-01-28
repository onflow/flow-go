// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package matching

import (
	"io/ioutil"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	merr "github.com/dapperlabs/flow-go/module/mempool"
	mempool "github.com/dapperlabs/flow-go/module/mempool/mock"
	module "github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
	serr "github.com/dapperlabs/flow-go/storage"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestNewEngine(t *testing.T) {

	log := zerolog.New(ioutil.Discard)

	net := &module.Network{}
	net.On("Register", uint8(engine.ReceiptProvider), mock.Anything).Return(&network.Conduit{}, nil).Once()
	net.On("Register", uint8(engine.ApprovalProvider), mock.Anything).Return(&network.Conduit{}, nil).Once()

	state := &protocol.State{}
	me := &module.Local{}
	results := &storage.Results{}
	receipts := &mempool.Receipts{}
	approvals := &mempool.Approvals{}
	seals := &mempool.Seals{}

	e, err := New(log, net, state, me, results, receipts, approvals, seals)
	require.NoError(t, err)

	assert.NotNil(t, e.log)
	assert.Same(t, state, e.state)
	assert.Same(t, me, e.me)
	assert.Same(t, results, e.results)
	assert.Same(t, receipts, e.receipts)
	assert.Same(t, approvals, e.approvals)
	assert.Same(t, seals, e.seals)

	net.AssertExpectations(t)
}

func TestOnReceiptCorrectRole(t *testing.T) {

	identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	receipt := unittest.ExecutionReceiptFixture()

	state := &protocol.State{}
	snapshot := &protocol.Snapshot{}
	approvals := &mempool.Approvals{}
	receipts := &mempool.Receipts{}

	state.On("Final").Return(snapshot)
	snapshot.On("Identity", mock.Anything).Return(identity, nil)
	receipts.On("Add", mock.Anything).Return(nil)
	approvals.On("ByResultID", mock.Anything).Return(nil, merr.ErrEntityNotFound)

	e := Engine{
		state:     state,
		approvals: approvals,
		receipts:  receipts,
	}

	err := e.onReceipt(identity.NodeID, receipt)
	assert.NoError(t, err)

	if receipts.AssertNumberOfCalls(t, "Add", 1) {
		receipts.AssertCalled(t, "Add", receipt)
	}
	if approvals.AssertNumberOfCalls(t, "ByResultID", 1) {
		approvals.AssertCalled(t, "ByResultID", receipt.ExecutionResult.ID())
	}
}

func TestOnReceiptWrongRole(t *testing.T) {

	identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	receipt := unittest.ExecutionReceiptFixture()

	state := &protocol.State{}
	snapshot := &protocol.Snapshot{}
	approvals := &mempool.Approvals{}
	receipts := &mempool.Receipts{}

	state.On("Final").Return(snapshot)
	snapshot.On("Identity", mock.Anything).Return(identity, nil)
	receipts.On("Add", mock.Anything).Return(nil)
	approvals.On("ByResultID", mock.Anything).Return(nil, merr.ErrEntityNotFound)

	e := Engine{
		state:     state,
		receipts:  receipts,
		approvals: approvals,
	}

	err := e.onReceipt(identity.NodeID, receipt)
	assert.Error(t, err)

	approvals.AssertNumberOfCalls(t, "Add", 0)
	receipts.AssertNumberOfCalls(t, "ByResultID", 0)
}

func TestOnApprovalCorrectRole(t *testing.T) {

	identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	approval := unittest.ResultApprovalFixture()

	state := &protocol.State{}
	snapshot := &protocol.Snapshot{}
	approvals := &mempool.Approvals{}
	receipts := &mempool.Receipts{}

	state.On("Final").Return(snapshot)
	snapshot.On("Identity", mock.Anything).Return(identity, nil)
	approvals.On("Add", mock.Anything).Return(nil)
	receipts.On("ByResultID", mock.Anything).Return(nil, merr.ErrEntityNotFound)

	e := Engine{
		state:     state,
		receipts:  receipts,
		approvals: approvals,
	}

	err := e.onApproval(identity.NodeID, approval)
	assert.NoError(t, err)

	if approvals.AssertNumberOfCalls(t, "Add", 1) {
		approvals.AssertCalled(t, "Add", approval)
	}
	if receipts.AssertNumberOfCalls(t, "ByResultID", 1) {
		receipts.AssertCalled(t, "ByResultID", approval.ResultApprovalBody.ExecutionResultID)
	}
}

func TestOnApprovalWrongRole(t *testing.T) {

	identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	approval := unittest.ResultApprovalFixture()

	state := &protocol.State{}
	snapshot := &protocol.Snapshot{}
	approvals := &mempool.Approvals{}
	receipts := &mempool.Receipts{}

	state.On("Final").Return(snapshot)
	snapshot.On("Identity", mock.Anything).Return(identity, nil)
	approvals.On("Add", mock.Anything).Return(nil)
	receipts.On("ByResultID", mock.Anything).Return(nil, merr.ErrEntityNotFound)

	e := Engine{
		state:     state,
		receipts:  receipts,
		approvals: approvals,
	}

	err := e.onApproval(identity.NodeID, approval)
	assert.Error(t, err)

	approvals.AssertNumberOfCalls(t, "Add", 0)
	receipts.AssertNumberOfCalls(t, "ByResultID", 0)
}

func TestMatchReceiptAvailableApproval(t *testing.T) {

	receipt := unittest.ExecutionReceiptFixture()
	approval := unittest.ResultApprovalFixture()
	result := unittest.ExecutionResultFixture()

	approvals := &mempool.Approvals{}
	results := &storage.Results{}
	seals := &mempool.Seals{}

	approvals.On("ByResultID", mock.Anything).Return(approval, nil)
	results.On("ByID", mock.Anything).Return(result, nil)
	results.On("Store", mock.Anything).Return(nil)
	seals.On("Add", mock.Anything).Return(nil)

	e := Engine{
		approvals: approvals,
		results:   results,
		seals:     seals,
	}

	err := e.matchReceipt(receipt)
	assert.NoError(t, err)

	if approvals.AssertNumberOfCalls(t, "ByResultID", 1) {
		approvals.AssertCalled(t, "ByResultID", receipt.ExecutionResult.ID())
	}
	if results.AssertNumberOfCalls(t, "ByID", 1) {
		results.AssertCalled(t, "ByID", receipt.ExecutionResult.PreviousResultID)
	}
}

func TestMatchReceiptMissingApproval(t *testing.T) {

	receipt := unittest.ExecutionReceiptFixture()
	result := unittest.ExecutionResultFixture()

	approvals := &mempool.Approvals{}
	results := &storage.Results{}
	seals := &mempool.Seals{}

	approvals.On("ByResultID", mock.Anything).Return(nil, merr.ErrEntityNotFound)
	results.On("ByID", mock.Anything).Return(result, nil)
	results.On("Store", mock.Anything).Return(nil)
	seals.On("Add", mock.Anything).Return(nil)

	e := Engine{
		approvals: approvals,
		results:   results,
		seals:     seals,
	}

	err := e.matchReceipt(receipt)
	assert.NoError(t, err)

	if approvals.AssertNumberOfCalls(t, "ByResultID", 1) {
		approvals.AssertCalled(t, "ByResultID", receipt.ExecutionResult.ID())
	}
	results.AssertNumberOfCalls(t, "ByID", 0)
}

func TestMatchApprovalAvailableReceipt(t *testing.T) {

	approval := unittest.ResultApprovalFixture()
	receipt := unittest.ExecutionReceiptFixture()
	result := unittest.ExecutionResultFixture()

	receipts := &mempool.Receipts{}
	results := &storage.Results{}
	seals := &mempool.Seals{}

	receipts.On("ByResultID", mock.Anything).Return(receipt, nil)
	results.On("ByID", mock.Anything).Return(result, nil)
	results.On("Store", mock.Anything).Return(nil)
	seals.On("Add", mock.Anything).Return(nil)

	e := Engine{
		receipts: receipts,
		results:  results,
		seals:    seals,
	}

	err := e.matchApproval(approval)
	assert.NoError(t, err)

	if receipts.AssertNumberOfCalls(t, "ByResultID", 1) {
		receipts.AssertCalled(t, "ByResultID", approval.ResultApprovalBody.ExecutionResultID)
	}
	if results.AssertNumberOfCalls(t, "ByID", 1) {
		results.AssertCalled(t, "ByID", receipt.ExecutionResult.PreviousResultID)
	}
}

func TestMatchApprovalMissingReceipt(t *testing.T) {

	approval := unittest.ResultApprovalFixture()
	result := unittest.ExecutionResultFixture()

	receipts := &mempool.Receipts{}
	results := &storage.Results{}
	seals := &mempool.Seals{}

	receipts.On("ByResultID", mock.Anything).Return(nil, merr.ErrEntityNotFound)
	results.On("ByID", mock.Anything).Return(result, nil)
	results.On("Store", mock.Anything).Return(nil)
	seals.On("Add", mock.Anything).Return(nil)

	e := Engine{
		receipts: receipts,
		results:  results,
		seals:    seals,
	}

	err := e.matchApproval(approval)
	assert.NoError(t, err)

	if receipts.AssertNumberOfCalls(t, "ByResultID", 1) {
		receipts.AssertCalled(t, "ByResultID", approval.ResultApprovalBody.ExecutionResultID)
	}
	results.AssertNumberOfCalls(t, "ByID", 0)
}

func TestCreateSealResultExists(t *testing.T) {

	receipt := unittest.ExecutionReceiptFixture()
	approval := unittest.ResultApprovalFixture()
	result := unittest.ExecutionResultFixture()
	seal := flow.Seal{
		BlockID:       receipt.ExecutionResult.BlockID,
		PreviousState: result.FinalStateCommit,
		FinalState:    receipt.ExecutionResult.FinalStateCommit,
	}

	results := &storage.Results{}
	seals := &mempool.Seals{}

	results.On("ByID", mock.Anything).Return(result, nil)
	results.On("Store", mock.Anything).Return(nil)
	seals.On("Add", mock.Anything).Return(nil)

	e := Engine{
		results: results,
		seals:   seals,
	}

	err := e.createSeal(receipt, approval)
	assert.NoError(t, err)

	if results.AssertNumberOfCalls(t, "Store", 1) {
		results.AssertCalled(t, "Store", &receipt.ExecutionResult)
	}
	if seals.AssertNumberOfCalls(t, "Add", 1) {
		seals.AssertCalled(t, "Add", &seal)
	}
}

func TestCreateSealResultMissing(t *testing.T) {

	receipt := unittest.ExecutionReceiptFixture()
	approval := unittest.ResultApprovalFixture()

	results := &storage.Results{}
	seals := &mempool.Seals{}

	results.On("ByID", mock.Anything).Return(nil, serr.ErrNotFound)
	results.On("Store", mock.Anything).Return(nil)
	seals.On("Add", mock.Anything).Return(nil)

	e := Engine{
		results: results,
		seals:   seals,
	}

	err := e.createSeal(receipt, approval)
	assert.NoError(t, err)

	results.AssertNumberOfCalls(t, "Store", 0)
	seals.AssertNumberOfCalls(t, "Add", 0)
}
