// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package matching

import (
	"errors"
	"io/ioutil"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	mempool "github.com/dapperlabs/flow-go/module/mempool/mock"
	module "github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

var reachedEnd = errors.New("reached end of test")

func TestNewEngine(t *testing.T) {

	log := zerolog.New(ioutil.Discard)

	net := &module.Network{}
	net.On("Register", uint8(engine.ExecutionReceiptProvider), mock.Anything).Return(&network.Conduit{}, nil).Once()
	net.On("Register", uint8(engine.ApprovalProvider), mock.Anything).Return(&network.Conduit{}, nil).Once()

	state := &protocol.State{}
	me := &module.Local{}
	results := &storage.ExecutionResults{}
	receipts := &mempool.Receipts{}
	approvals := &mempool.Approvals{}
	seals := &mempool.Seals{}

	e, err := New(log, net, state, me, results, receipts, approvals, seals)
	require.NoError(t, err)

	assert.NotNil(t, e.unit)
	assert.NotNil(t, e.log)
	assert.Same(t, state, e.state)
	assert.Same(t, me, e.me)
	assert.Same(t, results, e.results)
	assert.Same(t, receipts, e.receipts)
	assert.Same(t, approvals, e.approvals)
	assert.Same(t, seals, e.seals)

	net.AssertExpectations(t)
}

func TestOnReceiptValid(t *testing.T) {

	identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	receipt := unittest.ExecutionReceiptFixture()
	receipt.ExecutorID = identity.NodeID

	state := &protocol.State{}
	snapshot := &protocol.Snapshot{}
	boundary := &protocol.Snapshot{}
	approvals := &mempool.Approvals{}
	receipts := &mempool.Receipts{}

	state.On("Final").Return(snapshot)

	state.On("AtBlockID", mock.Anything).Return(boundary)
	snapshot.On("Identity", mock.Anything).Return(identity, nil)
	receipts.On("Add", mock.Anything).Return(nil)
	boundary.On("Identities", mock.Anything, mock.Anything).Return(nil, reachedEnd)

	e := Engine{
		state:     state,
		approvals: approvals,
		receipts:  receipts,
	}

	err := e.onReceipt(identity.NodeID, receipt)
	assert.True(t, errors.Is(err, reachedEnd))

	if receipts.AssertNumberOfCalls(t, "Add", 1) {
		receipts.AssertCalled(t, "Add", receipt)
	}
}

func TestOnReceiptWrongExecutor(t *testing.T) {

	identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	receipt := unittest.ExecutionReceiptFixture()
	receipt.ExecutorID = flow.ZeroID

	state := &protocol.State{}
	snapshot := &protocol.Snapshot{}
	boundary := &protocol.Snapshot{}
	approvals := &mempool.Approvals{}
	receipts := &mempool.Receipts{}

	state.On("Final").Return(snapshot)

	state.On("AtBlockID", mock.Anything).Return(boundary)
	snapshot.On("Identity", mock.Anything).Return(identity, nil)
	receipts.On("Add", mock.Anything).Return(nil)
	boundary.On("Identities", mock.Anything, mock.Anything).Return(nil, reachedEnd)

	e := Engine{
		state:     state,
		approvals: approvals,
		receipts:  receipts,
	}

	err := e.onReceipt(identity.NodeID, receipt)
	assert.Error(t, err)
	assert.False(t, errors.Is(err, reachedEnd))

	receipts.AssertNumberOfCalls(t, "Add", 0)
}

func TestOnReceiptMissingIdentity(t *testing.T) {

	identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	receipt := unittest.ExecutionReceiptFixture()
	receipt.ExecutorID = identity.NodeID

	state := &protocol.State{}
	snapshot := &protocol.Snapshot{}
	boundary := &protocol.Snapshot{}
	approvals := &mempool.Approvals{}
	receipts := &mempool.Receipts{}

	state.On("Final").Return(snapshot)

	state.On("AtBlockID", mock.Anything).Return(boundary)
	snapshot.On("Identity", mock.Anything).Return(nil, errors.New("identity missing"))
	receipts.On("Add", mock.Anything).Return(nil)
	boundary.On("Identities", mock.Anything, mock.Anything).Return(nil, reachedEnd)

	e := Engine{
		state:     state,
		approvals: approvals,
		receipts:  receipts,
	}

	err := e.onReceipt(identity.NodeID, receipt)
	assert.Error(t, err)
	assert.False(t, errors.Is(err, reachedEnd))

	receipts.AssertNumberOfCalls(t, "Add", 0)
}

func TestOnReceiptZeroStake(t *testing.T) {

	identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	receipt := unittest.ExecutionReceiptFixture()
	receipt.ExecutorID = identity.NodeID
	identity.Stake = 0

	state := &protocol.State{}
	snapshot := &protocol.Snapshot{}
	boundary := &protocol.Snapshot{}
	approvals := &mempool.Approvals{}
	receipts := &mempool.Receipts{}

	state.On("Final").Return(snapshot)

	state.On("AtBlockID", mock.Anything).Return(boundary)
	snapshot.On("Identity", mock.Anything).Return(identity, nil)
	receipts.On("Add", mock.Anything).Return(nil)
	boundary.On("Identities", mock.Anything, mock.Anything).Return(nil, reachedEnd)

	e := Engine{
		state:     state,
		approvals: approvals,
		receipts:  receipts,
	}

	err := e.onReceipt(identity.NodeID, receipt)
	assert.Error(t, err)
	assert.False(t, errors.Is(err, reachedEnd))

	receipts.AssertNumberOfCalls(t, "Add", 0)
}

func TestOnReceiptWrongRole(t *testing.T) {

	identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	receipt := unittest.ExecutionReceiptFixture()
	receipt.ExecutorID = identity.NodeID

	state := &protocol.State{}
	snapshot := &protocol.Snapshot{}
	boundary := &protocol.Snapshot{}
	approvals := &mempool.Approvals{}
	receipts := &mempool.Receipts{}

	state.On("Final").Return(snapshot)

	state.On("AtBlockID", mock.Anything).Return(boundary)
	snapshot.On("Identity", mock.Anything).Return(identity, nil)
	receipts.On("Add", mock.Anything).Return(nil)
	boundary.On("Identities", mock.Anything, mock.Anything).Return(nil, reachedEnd)

	e := Engine{
		state:     state,
		approvals: approvals,
		receipts:  receipts,
	}

	err := e.onReceipt(identity.NodeID, receipt)
	assert.Error(t, err)
	assert.False(t, errors.Is(err, reachedEnd))

	receipts.AssertNumberOfCalls(t, "Add", 0)
}

func TestOnApprovalValid(t *testing.T) {

	identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	approval := unittest.ResultApprovalFixture()
	approval.ResultApprovalBody.ApproverID = identity.NodeID

	state := &protocol.State{}
	snapshot := &protocol.Snapshot{}
	boundary := &protocol.Snapshot{}
	approvals := &mempool.Approvals{}
	receipts := &mempool.Receipts{}

	state.On("Final").Return(snapshot)

	state.On("AtBlockID", mock.Anything).Return(boundary)
	snapshot.On("Identity", mock.Anything).Return(identity, nil)
	approvals.On("Add", mock.Anything).Return(nil)
	boundary.On("Identities", mock.Anything, mock.Anything).Return(nil, reachedEnd)

	e := Engine{
		state:     state,
		receipts:  receipts,
		approvals: approvals,
	}

	err := e.onApproval(identity.NodeID, approval)
	assert.True(t, errors.Is(err, reachedEnd))

	if approvals.AssertNumberOfCalls(t, "Add", 1) {
		approvals.AssertCalled(t, "Add", approval)
	}
}

func TestOnApprovalWrongApprover(t *testing.T) {

	identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	approval := unittest.ResultApprovalFixture()
	approval.ResultApprovalBody.ApproverID = flow.ZeroID

	state := &protocol.State{}
	snapshot := &protocol.Snapshot{}
	boundary := &protocol.Snapshot{}
	approvals := &mempool.Approvals{}
	receipts := &mempool.Receipts{}

	state.On("Final").Return(snapshot)

	state.On("AtBlockID", mock.Anything).Return(boundary)
	snapshot.On("Identity", mock.Anything).Return(identity, nil)
	approvals.On("Add", mock.Anything).Return(nil)
	boundary.On("Identities", mock.Anything, mock.Anything).Return(nil, reachedEnd)

	e := Engine{
		state:     state,
		receipts:  receipts,
		approvals: approvals,
	}

	err := e.onApproval(identity.NodeID, approval)
	assert.Error(t, err)
	assert.False(t, errors.Is(err, reachedEnd))

	approvals.AssertNumberOfCalls(t, "Add", 0)
}

func TestOnApprovalMissingIdentity(t *testing.T) {

	identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	approval := unittest.ResultApprovalFixture()
	approval.ResultApprovalBody.ApproverID = identity.NodeID

	state := &protocol.State{}
	snapshot := &protocol.Snapshot{}
	boundary := &protocol.Snapshot{}
	approvals := &mempool.Approvals{}
	receipts := &mempool.Receipts{}

	state.On("Final").Return(snapshot)
	state.On("AtBlockID", mock.Anything).Return(boundary)
	snapshot.On("Identity", mock.Anything).Return(nil, errors.New("identity missing"))
	approvals.On("Add", mock.Anything).Return(nil)
	boundary.On("Identities", mock.Anything, mock.Anything).Return(nil, reachedEnd)

	e := Engine{
		state:     state,
		receipts:  receipts,
		approvals: approvals,
	}

	err := e.onApproval(identity.NodeID, approval)
	assert.Error(t, err)
	assert.False(t, errors.Is(err, reachedEnd))

	approvals.AssertNumberOfCalls(t, "Add", 0)
}

func TestOnApprovalZeroStake(t *testing.T) {

	identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	approval := unittest.ResultApprovalFixture()
	approval.ResultApprovalBody.ApproverID = identity.NodeID
	identity.Stake = 0

	state := &protocol.State{}
	snapshot := &protocol.Snapshot{}
	boundary := &protocol.Snapshot{}
	approvals := &mempool.Approvals{}
	receipts := &mempool.Receipts{}

	state.On("Final").Return(snapshot)
	state.On("AtBlockID", mock.Anything).Return(boundary)
	snapshot.On("Identity", mock.Anything).Return(identity, nil)
	approvals.On("Add", mock.Anything).Return(nil)
	boundary.On("Identities", mock.Anything, mock.Anything).Return(nil, reachedEnd)

	e := Engine{
		state:     state,
		receipts:  receipts,
		approvals: approvals,
	}

	err := e.onApproval(identity.NodeID, approval)
	assert.Error(t, err)
	assert.False(t, errors.Is(err, reachedEnd))

	approvals.AssertNumberOfCalls(t, "Add", 0)
}

func TestOnApprovalWrongRole(t *testing.T) {

	identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	approval := unittest.ResultApprovalFixture()
	approval.ResultApprovalBody.ApproverID = identity.NodeID

	state := &protocol.State{}
	snapshot := &protocol.Snapshot{}
	boundary := &protocol.Snapshot{}
	approvals := &mempool.Approvals{}
	receipts := &mempool.Receipts{}

	state.On("Final").Return(snapshot)
	state.On("AtBlockID", mock.Anything).Return(boundary)
	snapshot.On("Identity", mock.Anything).Return(identity, nil)
	approvals.On("Add", mock.Anything).Return(nil)
	boundary.On("Identities", mock.Anything, mock.Anything).Return(nil, reachedEnd)

	e := Engine{
		state:     state,
		receipts:  receipts,
		approvals: approvals,
	}

	err := e.onApproval(identity.NodeID, approval)
	assert.Error(t, err)
	assert.False(t, errors.Is(err, reachedEnd))

	approvals.AssertNumberOfCalls(t, "Add", 0)
}
