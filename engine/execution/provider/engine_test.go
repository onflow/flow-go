package provider

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	state "github.com/dapperlabs/flow-go/engine/execution/state/mock"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	network "github.com/dapperlabs/flow-go/network/mock"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestProviderEngine_OnExecutionStateRequest(t *testing.T) {
	t.Run("non-verification engine", func(t *testing.T) {
		ps := new(protocol.State)
		ss := new(protocol.Snapshot)

		execState := new(state.ExecutionState)

		e := Engine{state: ps, execState: execState}

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
		execState.AssertExpectations(t)
	})

	t.Run("non-existent chunk", func(t *testing.T) {
		ps := new(protocol.State)
		ss := new(protocol.Snapshot)

		execState := new(state.ExecutionState)

		e := Engine{state: ps, execState: execState}

		originIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

		chunkID := unittest.IdentifierFixture()

		ps.On("Final").Return(ss)
		ss.On("Identity", originIdentity.NodeID).Return(originIdentity, nil)
		execState.On("GetChunkRegisters", chunkID).Return(nil, fmt.Errorf("state error"))

		req := &messages.ExecutionStateRequest{ChunkID: chunkID}

		err := e.onExecutionStateRequest(originIdentity.NodeID, req)
		assert.Error(t, err)

		ps.AssertExpectations(t)
		ss.AssertExpectations(t)
		execState.AssertExpectations(t)
	})

	t.Run("success", func(t *testing.T) {
		ps := new(protocol.State)
		ss := new(protocol.Snapshot)
		con := new(network.Conduit)

		execState := new(state.ExecutionState)

		e := Engine{state: ps, execStateCon: con, execState: execState}

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

		execState.On("GetChunkRegisters", chunkID).Return(expectedRegisters, nil)

		req := &messages.ExecutionStateRequest{ChunkID: chunkID}

		err := e.onExecutionStateRequest(originIdentity.NodeID, req)
		assert.NoError(t, err)

		ps.AssertExpectations(t)
		ss.AssertExpectations(t)
		con.AssertExpectations(t)
		execState.AssertExpectations(t)
	})
}
