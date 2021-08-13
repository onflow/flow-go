package provider

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/onflow/flow-go/engine"
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
		chunkConduit := mocknetwork.Conduit{}

		e := Engine{
			state:              ps,
			unit:               engine.NewUnit(),
			execState:          execState,
			metrics:            metrics.NewNoopCollector(),
			chunksConduit:      &chunkConduit,
			checkStakedAtBlock: func(_ flow.Identifier) (bool, error) { return true, nil }}

		originID := unittest.IdentifierFixture()
		chunkID := unittest.IdentifierFixture()
		blockID := unittest.IdentifierFixture()
		chunkDataPack := unittest.ChunkDataPackFixture(chunkID)

		ps.On("AtBlockID", blockID).Return(ss)
		ss.On("Identity", originID).Return(unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution)), nil)
		execState.On("ChunkDataPackByChunkID", mock.Anything, mock.Anything).Return(chunkDataPack, nil)
		execState.On("GetBlockIDByChunkID", chunkID).Return(blockID, nil)

		req := &messages.ChunkDataRequest{
			ChunkID: chunkID,
			Nonce:   rand.Uint64(),
		}
		// submit using origin ID with invalid role
		unittest.RequireCloseBefore(t, e.Ready(), 100*time.Millisecond, "could not start engine")
		e.onChunkDataRequest(context.Background(), originID, req)
		unittest.RequireCloseBefore(t, e.Done(), 100*time.Millisecond, "could not stop engine")

		// no chunk data pack response should be sent to an invalid role's request
		chunkConduit.AssertNotCalled(t, "Unicast")

		ps.AssertExpectations(t)
		ss.AssertExpectations(t)
		execState.AssertExpectations(t)
		chunkConduit.AssertExpectations(t)
	})

	t.Run("unstaked (0 stake) origin", func(t *testing.T) {
		ps := new(mockprotocol.State)
		ss := new(mockprotocol.Snapshot)

		execState := new(state.ExecutionState)
		chunkConduit := mocknetwork.Conduit{}

		e := Engine{
			state:              ps,
			unit:               engine.NewUnit(),
			execState:          execState,
			metrics:            metrics.NewNoopCollector(),
			chunksConduit:      &chunkConduit,
			checkStakedAtBlock: func(_ flow.Identifier) (bool, error) { return true, nil }}

		originID := unittest.IdentifierFixture()
		chunkID := unittest.IdentifierFixture()
		blockID := unittest.IdentifierFixture()
		chunkDataPack := unittest.ChunkDataPackFixture(chunkID)

		ps.On("AtBlockID", blockID).Return(ss)
		ss.On("Identity", originID).Return(unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution), unittest.WithStake(0)), nil)
		execState.On("ChunkDataPackByChunkID", mock.Anything, mock.Anything).Return(chunkDataPack, nil)
		execState.On("GetBlockIDByChunkID", chunkID).Return(blockID, nil)

		req := &messages.ChunkDataRequest{
			ChunkID: chunkID,
			Nonce:   rand.Uint64(),
		}
		// submit using origin ID with zero stake
		unittest.RequireCloseBefore(t, e.Ready(), 100*time.Millisecond, "could not start engine")
		e.onChunkDataRequest(context.Background(), originID, req)
		unittest.RequireCloseBefore(t, e.Done(), 100*time.Millisecond, "could not stop engine")

		// no chunk data pack response should be sent to a request coming from (0)-stake node
		chunkConduit.AssertNotCalled(t, "Unicast")

		ps.AssertExpectations(t)
		ss.AssertExpectations(t)
		execState.AssertExpectations(t)
		chunkConduit.AssertExpectations(t)
	})

	t.Run("unstaked (not found origin) origin", func(t *testing.T) {
		ps := new(mockprotocol.State)
		ss := new(mockprotocol.Snapshot)

		execState := new(state.ExecutionState)
		chunkConduit := mocknetwork.Conduit{}

		e := Engine{
			state:              ps,
			unit:               engine.NewUnit(),
			execState:          execState,
			metrics:            metrics.NewNoopCollector(),
			chunksConduit:      &chunkConduit,
			checkStakedAtBlock: func(_ flow.Identifier) (bool, error) { return true, nil }}

		originID := unittest.IdentifierFixture()
		chunkID := unittest.IdentifierFixture()
		blockID := unittest.IdentifierFixture()
		chunkDataPack := unittest.ChunkDataPackFixture(chunkID)

		ps.On("AtBlockID", blockID).Return(ss)
		ss.On("Identity", originID).Return(nil, protocol.IdentityNotFoundError{})
		execState.On("ChunkDataPackByChunkID", mock.Anything, mock.Anything).Return(chunkDataPack, nil)
		execState.On("GetBlockIDByChunkID", chunkID).Return(blockID, nil)

		req := &messages.ChunkDataRequest{
			ChunkID: chunkID,
			Nonce:   rand.Uint64(),
		}
		// submit using non-existing origin ID
		unittest.RequireCloseBefore(t, e.Ready(), 100*time.Millisecond, "could not start engine")
		e.onChunkDataRequest(context.Background(), originID, req)
		unittest.RequireCloseBefore(t, e.Done(), 100*time.Millisecond, "could not stop engine")

		// no chunk data pack response should be sent to a request coming from a non-existing origin ID
		chunkConduit.AssertNotCalled(t, "Unicast")

		ps.AssertExpectations(t)
		ss.AssertExpectations(t)
		execState.AssertExpectations(t)
		chunkConduit.AssertExpectations(t)
	})

	//t.Run("non-existent chunk", func(t *testing.T) {
	//	ps := new(mockprotocol.State)
	//	ss := new(mockprotocol.Snapshot)
	//
	//	execState := new(state.ExecutionState)
	//	execState.On("ChunkDataPackByChunkID", mock.Anything, mock.Anything).Return(nil, errors.New("not found!"))
	//
	//	e := Engine{state: ps, unit: engine.NewUnit(), execState: execState, metrics: metrics.NewNoopCollector(), checkStakedAtBlock: func(_ flow.Identifier) (bool, error) { return true, nil }}
	//
	//	originIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	//
	//	chunkID := unittest.IdentifierFixture()
	//
	//	req := &messages.ChunkDataRequest{
	//		ChunkID: chunkID,
	//		Nonce:   rand.Uint64(),
	//	}
	//	err := e.onChunkDataRequest(context.Background(), originIdentity.NodeID, req)
	//	assert.Error(t, err)
	//
	//	ps.AssertExpectations(t)
	//	ss.AssertExpectations(t)
	//	execState.AssertExpectations(t)
	//})
	//
	//t.Run("success", func(t *testing.T) {
	//	ps := new(mockprotocol.State)
	//	ss := new(mockprotocol.Snapshot)
	//	con := new(mocknetwork.Conduit)
	//
	//	execState := new(state.ExecutionState)
	//
	//	e := Engine{state: ps, unit: engine.NewUnit(), chunksConduit: con, execState: execState, metrics: metrics.NewNoopCollector(), checkStakedAtBlock: func(_ flow.Identifier) (bool, error) { return true, nil }}
	//
	//	originIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	//
	//	chunkID := unittest.IdentifierFixture()
	//	chunkDataPack := unittest.ChunkDataPackFixture(chunkID)
	//	blockID := unittest.IdentifierFixture()
	//
	//	ps.On("AtBlockID", blockID).Return(ss)
	//	ss.On("Identity", originIdentity.NodeID).Return(originIdentity, nil)
	//	con.On("Unicast", mock.Anything, originIdentity.NodeID).
	//		Run(func(args mock.Arguments) {
	//			res, ok := args[0].(*messages.ChunkDataResponse)
	//			require.True(t, ok)
	//
	//			actualChunkID := res.ChunkDataPack.ChunkID
	//			assert.Equal(t, chunkID, actualChunkID)
	//		}).
	//		Return(nil)
	//
	//	execState.On("GetBlockIDByChunkID", chunkID).Return(blockID, nil)
	//	execState.On("ChunkDataPackByChunkID", mock.Anything, chunkID).Return(chunkDataPack, nil)
	//
	//	req := &messages.ChunkDataRequest{
	//		ChunkID: chunkID,
	//		Nonce:   rand.Uint64(),
	//	}
	//
	//	err := e.onChunkDataRequest(context.Background(), originIdentity.NodeID, req)
	//	assert.NoError(t, err)
	//
	//	ps.AssertExpectations(t)
	//	ss.AssertExpectations(t)
	//	execState.AssertExpectations(t)
	//	assert.Eventually(t, func() bool {
	//		return con.AssertExpectations(t)
	//	}, time.Second, time.Millisecond)
	//})
	//
	//t.Run("reply to chunk data pack request only when staked", func(t *testing.T) {
	//
	//	ps := new(mockprotocol.State)
	//	ss := new(mockprotocol.Snapshot)
	//	con := new(mocknetwork.Conduit)
	//
	//	execState := new(state.ExecutionState)
	//
	//	currentStakedState := true
	//	checkStakedAtBlock := func(_ flow.Identifier) (bool, error) { return currentStakedState, nil }
	//
	//	e := Engine{
	//		state:              ps,
	//		unit:               engine.NewUnit(),
	//		chunksConduit:      con,
	//		execState:          execState,
	//		metrics:            metrics.NewNoopCollector(),
	//		checkStakedAtBlock: checkStakedAtBlock,
	//	}
	//
	//	originIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	//
	//	chunkID := unittest.IdentifierFixture()
	//	chunkDataPack := unittest.ChunkDataPackFixture(chunkID)
	//	blockID := unittest.IdentifierFixture()
	//
	//	execState.On("GetBlockIDByChunkID", chunkID).Return(blockID, nil)
	//	ps.On("AtBlockID", blockID).Return(ss)
	//
	//	ss.On("Identity", originIdentity.NodeID).Return(originIdentity, nil).Once()
	//	con.On("Unicast", mock.Anything, originIdentity.NodeID).
	//		Run(func(args mock.Arguments) {
	//			res, ok := args[0].(*messages.ChunkDataResponse)
	//			require.True(t, ok)
	//
	//			actualChunkID := res.ChunkDataPack.ChunkID
	//			assert.Equal(t, chunkID, actualChunkID)
	//		}).
	//		Return(nil).Once()
	//
	//	execState.On("ChunkDataPackByChunkID", mock.Anything, chunkID).Return(chunkDataPack, nil).Twice()
	//
	//	req := &messages.ChunkDataRequest{
	//		ChunkID: chunkID,
	//		Nonce:   rand.Uint64(),
	//	}
	//
	//	err := e.onChunkDataRequest(context.Background(), originIdentity.NodeID, req)
	//	assert.NoError(t, err)
	//
	//	currentStakedState = false
	//
	//	err = e.onChunkDataRequest(context.Background(), originIdentity.NodeID, req)
	//	assert.Error(t, err)
	//
	//	ps.AssertExpectations(t)
	//	ss.AssertExpectations(t)
	//	execState.AssertExpectations(t)
	//
	//	assert.Eventually(t, func() bool {
	//		return con.AssertExpectations(t)
	//	}, time.Second, time.Millisecond)
	//})
}
