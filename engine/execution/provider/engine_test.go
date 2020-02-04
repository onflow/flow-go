package provider

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	state "github.com/dapperlabs/flow-go/engine/execution/execution/state/mock"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	module "github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestExecutionReceiptProviderEngine_ProcessExecutionResult(t *testing.T) {
	targetIDs := flow.IdentityList{
		unittest.IdentityFixture(func(id *flow.Identity) {
			id.Role = flow.RoleConsensus
		}),
		unittest.IdentityFixture(func(id *flow.Identity) {
			id.Role = flow.RoleVerification
		}),
	}

	result := unittest.ExecutionResultFixture()

	t.Run("failed to load identities", func(t *testing.T) {
		state := &protocol.State{}
		ss := &protocol.Snapshot{}
		con := &network.Conduit{}
		me := &module.Local{}
		me.On("NodeID").Return(flow.ZeroID)

		e := Engine{
			state:      state,
			receiptCon: con,
			me:         me,
		}

		state.On("Final").Return(ss)
		ss.On("Identities", mock.Anything, mock.Anything).
			Return(nil, fmt.Errorf("identity error"))

		err := e.onExecutionResult(e.me.NodeID(), result)
		assert.Error(t, err)

		state.AssertExpectations(t)
		ss.AssertExpectations(t)
	})

	t.Run("failed to broadcast", func(t *testing.T) {
		state := &protocol.State{}
		ss := &protocol.Snapshot{}
		con := &network.Conduit{}
		me := &module.Local{}
		me.On("NodeID").Return(flow.ZeroID)

		e := Engine{
			state:      state,
			receiptCon: con,
			me:         me,
		}

		state.On("Final").Return(ss)
		ss.On("Identities", mock.Anything, mock.Anything).Return(targetIDs, nil)
		con.On(
			"Submit",
			mock.Anything,
			targetIDs[0].NodeID,
			targetIDs[1].NodeID,
		).
			Return(fmt.Errorf("network error"))

		err := e.onExecutionResult(e.me.NodeID(), result)
		assert.Error(t, err)

		state.AssertExpectations(t)
		ss.AssertExpectations(t)
		con.AssertExpectations(t)
	})

	t.Run("non-local engine", func(t *testing.T) {
		me := &module.Local{}
		me.On("NodeID").Return(flow.ZeroID)

		e := Engine{me: me}

		// submit using origin ID that does not match node ID
		err := e.onExecutionResult(flow.Identifier{42}, result)
		assert.Error(t, err)
	})

	t.Run("success", func(t *testing.T) {
		state := &protocol.State{}
		ss := &protocol.Snapshot{}
		con := &network.Conduit{}
		me := &module.Local{}
		me.On("NodeID").Return(flow.ZeroID)

		e := Engine{
			state:      state,
			receiptCon: con,
			me:         me,
		}

		state.On("Final").Return(ss)
		ss.On("Identities", mock.Anything, mock.Anything).Return(targetIDs, nil)
		con.On(
			"Submit",
			mock.Anything,
			targetIDs[0].NodeID,
			targetIDs[1].NodeID,
		).
			Run(func(args mock.Arguments) {
				// check the receipt is properly formed
				receipt := args[0].(*flow.ExecutionReceipt)
				assert.Equal(t, result, &receipt.ExecutionResult)
			}).
			Return(nil)

		err := e.onExecutionResult(e.me.NodeID(), result)
		assert.NoError(t, err)

		state.AssertExpectations(t)
		ss.AssertExpectations(t)
		con.AssertExpectations(t)
	})
}

func TestExecutionEngine_OnExecutionStateRequest(t *testing.T) {
	t.Run("non-verification engine", func(t *testing.T) {
		ps := new(protocol.State)
		ss := new(protocol.Snapshot)

		e := Engine{state: ps}

		originID := unittest.IdentifierFixture()
		chunkID := unittest.IdentifierFixture()

		ps.On("Final").Return(ss)
		ss.On("Identity", originID).
			Return(unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution)), nil)

		req := &messages.ExecutionStateRequest{ChunkID: chunkID}

		// submit using origin ID with invalid role
		err := e.onExecutionStateRequest(originID, req)
		assert.Error(t, err)

		ps.AssertExpectations(t)
		ss.AssertExpectations(t)
	})

	t.Run("non-existent chunk", func(t *testing.T) {
		ps := new(protocol.State)
		ss := new(protocol.Snapshot)
		es := new(state.ExecutionState)

		e := Engine{state: ps}

		originIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

		chunkID := unittest.IdentifierFixture()

		ps.On("Final").Return(ss)
		ss.On("Identity", originIdentity.NodeID).Return(originIdentity, nil)
		es.On("GetChunkRegisters", chunkID).Return(nil, fmt.Errorf("state error"))

		req := &messages.ExecutionStateRequest{ChunkID: chunkID}

		err := e.onExecutionStateRequest(originIdentity.NodeID, req)
		assert.Error(t, err)

		ps.AssertExpectations(t)
		ss.AssertExpectations(t)
		es.AssertExpectations(t)
	})

	t.Run("success", func(t *testing.T) {
		ps := new(protocol.State)
		ss := new(protocol.Snapshot)
		con := new(network.Conduit)

		e := Engine{state: ps, execStateCon: con}

		originIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

		chunkHeader := unittest.ChunkHeaderFixture()
		chunkID := chunkHeader.ChunkID

		registerIDs := chunkHeader.RegisterIDs
		registerValues := []flow.RegisterValue{{1}, {2}, {3}}

		expectedRegisters := flow.Ledger{
			string(registerIDs[0]): registerValues[0],
			string(registerIDs[1]): registerValues[1],
			string(registerIDs[2]): registerValues[2],
		}

		ps.On("Final").Return(ss)
		ss.On("Identity", originIdentity.NodeID).Return(originIdentity, nil)
		//es.On("GetChunkRegisters", chunkID).Return(expectedRegisters, nil)
		con.On("Submit", mock.Anything, originIdentity.NodeID).
			Run(func(args mock.Arguments) {
				res, ok := args[0].(*messages.ExecutionStateResponse)
				require.True(t, ok)

				actualChunkID := res.State.ChunkID
				actualRegisters := res.State.Registers

				assert.Equal(t, chunkID, actualChunkID)
				assert.EqualValues(t, expectedRegisters, actualRegisters)
			}).
			Return(nil)

		req := &messages.ExecutionStateRequest{ChunkID: chunkID}

		err := e.onExecutionStateRequest(originIdentity.NodeID, req)
		assert.NoError(t, err)

		ps.AssertExpectations(t)
		ss.AssertExpectations(t)
		//es.AssertExpectations(t)
		con.AssertExpectations(t)
	})
}
