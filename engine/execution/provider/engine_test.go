package provider

import (
	"context"
	"errors"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	state "github.com/onflow/flow-go/engine/execution/state/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/state/protocol"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestProviderEngine_onChunkDataRequest(t *testing.T) {
	t.Run("non-verification engine", func(t *testing.T) {
		ps := new(mockprotocol.State)
		ss := new(mockprotocol.Snapshot)

		execState := new(state.ExecutionState)

		e := Engine{state: ps, execState: execState, metrics: metrics.NewNoopCollector(), staker: unittest.NewFixedStaker(true)}

		originID := unittest.IdentifierFixture()
		chunkID := unittest.IdentifierFixture()
		blockID := unittest.IdentifierFixture()
		collectionID := unittest.IdentifierFixture()

		ps.On("AtBlockID", blockID).Return(ss)
		ss.On("Identity", originID).
			Return(unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution)), nil)
		execState.
			On("ChunkDataPackByChunkID", mock.Anything, mock.Anything).
			Return(&flow.ChunkDataPack{CollectionID: collectionID, ChunkID: chunkID}, nil)
		execState.
			On("GetBlockIDByChunkID", chunkID).
			Return(blockID, nil)

		req := &messages.ChunkDataRequest{
			ChunkID: chunkID,
			Nonce:   rand.Uint64(),
		}
		// submit using origin ID with invalid role
		err := e.onChunkDataRequest(context.Background(), originID, req)
		assert.Error(t, err)

		ps.AssertExpectations(t)
		ss.AssertExpectations(t)
		execState.AssertExpectations(t)
	})

	t.Run("unstaked (0 stake) origin", func(t *testing.T) {
		staker := &unittest.FixedStaker{Staked: true}
		ps := new(mockprotocol.State)
		ss := new(mockprotocol.Snapshot)

		execState := new(state.ExecutionState)

		e := Engine{state: ps, execState: execState, metrics: metrics.NewNoopCollector(), staker: staker}

		originID := unittest.IdentifierFixture()
		chunkID := unittest.IdentifierFixture()
		blockID := unittest.IdentifierFixture()
		collectionID := unittest.IdentifierFixture()

		ps.On("AtBlockID", blockID).Return(ss)
		ss.On("Identity", originID).
			Return(unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution), unittest.WithStake(0)), nil)
		execState.
			On("ChunkDataPackByChunkID", mock.Anything, mock.Anything).
			Return(&flow.ChunkDataPack{CollectionID: collectionID, ChunkID: chunkID}, nil)
		execState.
			On("GetBlockIDByChunkID", chunkID).
			Return(blockID, nil)

		req := &messages.ChunkDataRequest{
			ChunkID: chunkID,
			Nonce:   rand.Uint64(),
		}
		// submit using origin ID with zero stake
		err := e.onChunkDataRequest(context.Background(), originID, req)
		assert.Error(t, err)

		ps.AssertExpectations(t)
		ss.AssertExpectations(t)
		execState.AssertExpectations(t)
	})

	t.Run("unstaked (not found origin) origin", func(t *testing.T) {
		staker := &unittest.FixedStaker{Staked: true}
		ps := new(mockprotocol.State)
		ss := new(mockprotocol.Snapshot)

		execState := new(state.ExecutionState)

		e := Engine{state: ps, execState: execState, metrics: metrics.NewNoopCollector(), staker: staker}

		originID := unittest.IdentifierFixture()
		chunkID := unittest.IdentifierFixture()
		blockID := unittest.IdentifierFixture()
		collectionID := unittest.IdentifierFixture()

		ps.On("AtBlockID", blockID).Return(ss)
		ss.On("Identity", originID).
			Return(nil, protocol.IdentityNotFoundError{})
		execState.
			On("ChunkDataPackByChunkID", mock.Anything, mock.Anything).
			Return(&flow.ChunkDataPack{CollectionID: collectionID, ChunkID: chunkID}, nil)
		execState.
			On("GetBlockIDByChunkID", chunkID).
			Return(blockID, nil)

		req := &messages.ChunkDataRequest{
			ChunkID: chunkID,
			Nonce:   rand.Uint64(),
		}
		// submit using non-existing origin ID
		err := e.onChunkDataRequest(context.Background(), originID, req)
		assert.Error(t, err)

		ps.AssertExpectations(t)
		ss.AssertExpectations(t)
		execState.AssertExpectations(t)
	})

	t.Run("non-existent chunk", func(t *testing.T) {
		ps := new(mockprotocol.State)
		ss := new(mockprotocol.Snapshot)

		execState := new(state.ExecutionState)
		execState.
			On("ChunkDataPackByChunkID", mock.Anything, mock.Anything).
			Return(nil, errors.New("not found!"))

		e := Engine{state: ps, execState: execState, metrics: metrics.NewNoopCollector(), staker: unittest.NewFixedStaker(true)}

		originIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

		chunkID := unittest.IdentifierFixture()

		req := &messages.ChunkDataRequest{
			ChunkID: chunkID,
			Nonce:   rand.Uint64(),
		}
		err := e.onChunkDataRequest(context.Background(), originIdentity.NodeID, req)
		assert.Error(t, err)

		ps.AssertExpectations(t)
		ss.AssertExpectations(t)
		execState.AssertExpectations(t)
	})

	t.Run("success", func(t *testing.T) {
		ps := new(mockprotocol.State)
		ss := new(mockprotocol.Snapshot)
		con := new(mocknetwork.Conduit)

		execState := new(state.ExecutionState)

		e := Engine{state: ps, chunksConduit: con, execState: execState, metrics: metrics.NewNoopCollector(), staker: unittest.NewFixedStaker(true)}

		originIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

		chunkID := unittest.IdentifierFixture()
		chunkDataPack := unittest.ChunkDataPackFixture(chunkID)
		collectionID := unittest.IdentifierFixture()
		chunkDataPack.CollectionID = collectionID
		collection := unittest.CollectionFixture(1)
		blockID := unittest.IdentifierFixture()

		ps.On("AtBlockID", blockID).Return(ss)
		ss.On("Identity", originIdentity.NodeID).Return(originIdentity, nil)
		con.On("Unicast", mock.Anything, originIdentity.NodeID).
			Run(func(args mock.Arguments) {
				res, ok := args[0].(*messages.ChunkDataResponse)
				require.True(t, ok)

				actualChunkID := res.ChunkDataPack.ChunkID
				assert.Equal(t, chunkID, actualChunkID)
			}).
			Return(nil)

		execState.
			On("GetBlockIDByChunkID", chunkID).
			Return(blockID, nil)
		execState.
			On("ChunkDataPackByChunkID", mock.Anything, chunkID).
			Return(chunkDataPack, nil)

		execState.On("GetCollection", chunkDataPack.CollectionID).Return(&collection, nil)

		req := &messages.ChunkDataRequest{
			ChunkID: chunkID,
			Nonce:   rand.Uint64(),
		}

		err := e.onChunkDataRequest(context.Background(), originIdentity.NodeID, req)
		assert.NoError(t, err)

		ps.AssertExpectations(t)
		ss.AssertExpectations(t)
		con.AssertExpectations(t)
		execState.AssertExpectations(t)
	})

	t.Run("reply to chunk data pack request only when staked", func(t *testing.T) {

		ps := new(mockprotocol.State)
		ss := new(mockprotocol.Snapshot)
		con := new(mocknetwork.Conduit)

		execState := new(state.ExecutionState)

		staker := unittest.NewFixedStaker(true)
		e := Engine{state: ps, chunksConduit: con, execState: execState, metrics: metrics.NewNoopCollector(), staker: staker}

		originIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

		chunkID := unittest.IdentifierFixture()
		chunkDataPack := unittest.ChunkDataPackFixture(chunkID)
		collection := unittest.CollectionFixture(1)
		blockID := unittest.IdentifierFixture()

		execState.
			On("GetBlockIDByChunkID", chunkID).
			Return(blockID, nil)
		//ps.On("Final").Return(ss).Once()
		ps.On("AtBlockID", blockID).Return(ss)

		ss.On("Identity", originIdentity.NodeID).Return(originIdentity, nil).Once()
		con.On("Unicast", mock.Anything, originIdentity.NodeID).
			Run(func(args mock.Arguments) {
				res, ok := args[0].(*messages.ChunkDataResponse)
				require.True(t, ok)

				actualChunkID := res.ChunkDataPack.ChunkID
				assert.Equal(t, chunkID, actualChunkID)
			}).
			Return(nil).Once()

		execState.
			On("ChunkDataPackByChunkID", mock.Anything, chunkID).
			Return(chunkDataPack, nil).Twice()

		execState.On("GetCollection", chunkDataPack.CollectionID).Return(&collection, nil)

		req := &messages.ChunkDataRequest{
			ChunkID: chunkID,
			Nonce:   rand.Uint64(),
		}

		err := e.onChunkDataRequest(context.Background(), originIdentity.NodeID, req)
		assert.NoError(t, err)

		staker.Staked = false

		err = e.onChunkDataRequest(context.Background(), originIdentity.NodeID, req)
		assert.Error(t, err)

		ps.AssertExpectations(t)
		ss.AssertExpectations(t)
		con.AssertExpectations(t)
		execState.AssertExpectations(t)
	})
}
