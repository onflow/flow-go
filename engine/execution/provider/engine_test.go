package provider

import (
	"context"
	"errors"
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

func TestProviderEngine_onChunkDataPackRequest(t *testing.T) {
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

		req := &messages.ChunkDataPackRequest{ChunkID: chunkID}
		// submit using origin ID with invalid role
		err := e.onChunkDataPackRequest(context.Background(), originID, req)
		assert.Error(t, err)

		ps.AssertExpectations(t)
		ss.AssertExpectations(t)
		execState.AssertExpectations(t)
	})

	t.Run("non-existent chunk", func(t *testing.T) {
		ps := new(protocol.State)
		ss := new(protocol.Snapshot)

		execState := new(state.ExecutionState)
		execState.
			On("ChunkDataPackByChunkID", mock.Anything, mock.Anything).
			Return(nil, errors.New("not found!"))

		e := Engine{state: ps, execState: execState}

		originIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

		chunkID := unittest.IdentifierFixture()

		ps.On("Final").Return(ss)
		ss.On("Identity", originIdentity.NodeID).Return(originIdentity, nil)

		req := &messages.ChunkDataPackRequest{ChunkID: chunkID}
		err := e.onChunkDataPackRequest(context.Background(), originIdentity.NodeID, req)
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

		e := Engine{state: ps, chunksConduit: con, execState: execState}

		originIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

		chunkID := unittest.IdentifierFixture()
		chunkDataPack := unittest.ChunkDataPackFixture(chunkID)

		ps.On("Final").Return(ss)
		ss.On("Identity", originIdentity.NodeID).Return(originIdentity, nil)
		con.On("Submit", mock.Anything, originIdentity.NodeID).
			Run(func(args mock.Arguments) {
				res, ok := args[0].(*messages.ChunkDataPackResponse)
				require.True(t, ok)

				actualChunkID := res.Data.ChunkID
				assert.Equal(t, chunkID, actualChunkID)
			}).
			Return(nil)

		execState.
			On("ChunkDataPackByChunkID", mock.Anything, chunkID).
			Return(&chunkDataPack, nil)

		req := &messages.ChunkDataPackRequest{ChunkID: chunkID}

		err := e.onChunkDataPackRequest(context.Background(), originIdentity.NodeID, req)
		assert.NoError(t, err)

		ps.AssertExpectations(t)
		ss.AssertExpectations(t)
		con.AssertExpectations(t)
		execState.AssertExpectations(t)
	})
}
