package provider

import (
	"context"
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	_ "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/state/protocol"

	_ "github.com/onflow/flow-go/engine"
	state "github.com/onflow/flow-go/engine/execution/state/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/mocknetwork"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestProviderEngine_onChunkDataRequest(t *testing.T) {
	t.Run("non-verification engine", func(t *testing.T) {
		ps := new(mockprotocol.State)
		ss := new(mockprotocol.Snapshot)
		net := new(mocknetwork.Network)
		chunkConduit := &mocknetwork.Conduit{}
		net.On("Register", channels.PushReceipts, mock.Anything).Return(&mocknetwork.Conduit{}, nil)
		net.On("Register", channels.ProvideChunks, mock.Anything).Return(chunkConduit, nil)
		execState := new(state.ExecutionState)

		e, err := New(
			unittest.Logger(),
			trace.NewNoopTracer(),
			net,
			ps,
			execState,
			metrics.NewNoopCollector(),
			func(_ flow.Identifier) (bool, error) { return true, nil },
			queue.NewChunkDataPackRequestQueue(10, unittest.Logger(), metrics.NewNoopCollector()),
			DefaultChunkDataPackQueryTimeout,
			DefaultChunkDataPackDeliveryTimeout,
			DefaultChunkDataPackRequestWorker)
		require.NoError(t, err)

		originID := unittest.IdentifierFixture()
		chunkID := unittest.IdentifierFixture()
		blockID := unittest.IdentifierFixture()
		chunkDataPack := unittest.ChunkDataPackFixture(chunkID)

		ps.On("AtBlockID", blockID).Return(ss)
		ss.On("Identity", originID).Return(unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution)), nil)
		execState.On("ChunkDataPackByChunkID", mock.Anything).Return(chunkDataPack, nil)
		execState.On("GetBlockIDByChunkID", chunkID).Return(blockID, nil)

		req := &messages.ChunkDataRequest{
			ChunkID: chunkID,
			Nonce:   rand.Uint64(),
		}

		cancelCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ctx, _ := irrecoverable.WithSignaler(cancelCtx)
		e.Start(ctx)
		// submit using origin ID with invalid role
		unittest.RequireCloseBefore(t, e.Ready(), 100*time.Millisecond, "could not start engine")
		require.NoError(t, e.Process(channels.RequestChunks, originID, req))
		cancel()
		unittest.RequireCloseBefore(t, e.Done(), 100*time.Millisecond, "could not stop engine")

		// no chunk data pack response should be sent to an invalid role's request
		chunkConduit.AssertNotCalled(t, "Unicast")

		ps.AssertExpectations(t)
		ss.AssertExpectations(t)
		execState.AssertExpectations(t)
		chunkConduit.AssertExpectations(t)
	})

	t.Run("unauthorized (0 weight) origin", func(t *testing.T) {
		ps := new(mockprotocol.State)
		ss := new(mockprotocol.Snapshot)
		net := new(mocknetwork.Network)
		chunkConduit := &mocknetwork.Conduit{}
		net.On("Register", channels.PushReceipts, mock.Anything).Return(&mocknetwork.Conduit{}, nil)
		net.On("Register", channels.ProvideChunks, mock.Anything).Return(chunkConduit, nil)
		execState := new(state.ExecutionState)

		e, err := New(
			unittest.Logger(),
			trace.NewNoopTracer(),
			net,
			ps,
			execState,
			metrics.NewNoopCollector(),
			func(_ flow.Identifier) (bool, error) { return true, nil },
			queue.NewChunkDataPackRequestQueue(10, unittest.Logger(), metrics.NewNoopCollector()),
			DefaultChunkDataPackQueryTimeout,
			DefaultChunkDataPackDeliveryTimeout,
			DefaultChunkDataPackRequestWorker)
		require.NoError(t, err)

		originID := unittest.IdentifierFixture()
		chunkID := unittest.IdentifierFixture()
		blockID := unittest.IdentifierFixture()
		chunkDataPack := unittest.ChunkDataPackFixture(chunkID)

		ps.On("AtBlockID", blockID).Return(ss)
		ss.On("Identity", originID).Return(unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution), unittest.WithWeight(0)), nil)
		execState.On("ChunkDataPackByChunkID", mock.Anything).Return(chunkDataPack, nil)
		execState.On("GetBlockIDByChunkID", chunkID).Return(blockID, nil)

		req := &messages.ChunkDataRequest{
			ChunkID: chunkID,
			Nonce:   rand.Uint64(),
		}
		cancelCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ctx, _ := irrecoverable.WithSignaler(cancelCtx)
		e.Start(ctx)
		// submit using origin ID with zero weight
		unittest.RequireCloseBefore(t, e.Ready(), 100*time.Millisecond, "could not start engine")
		require.NoError(t, e.Process(channels.RequestChunks, originID, req))
		cancel()
		unittest.RequireCloseBefore(t, e.Done(), 100*time.Millisecond, "could not stop engine")

		// no chunk data pack response should be sent to a request coming from 0-weight node
		chunkConduit.AssertNotCalled(t, "Unicast")

		ps.AssertExpectations(t)
		ss.AssertExpectations(t)
		execState.AssertExpectations(t)
		chunkConduit.AssertExpectations(t)
	})

	t.Run("un-authorized (not found origin) origin", func(t *testing.T) {
		ps := new(mockprotocol.State)
		ss := new(mockprotocol.Snapshot)
		net := new(mocknetwork.Network)
		chunkConduit := &mocknetwork.Conduit{}
		net.On("Register", channels.PushReceipts, mock.Anything).Return(&mocknetwork.Conduit{}, nil)
		net.On("Register", channels.ProvideChunks, mock.Anything).Return(chunkConduit, nil)
		execState := new(state.ExecutionState)

		e, err := New(
			unittest.Logger(),
			trace.NewNoopTracer(),
			net,
			ps,
			execState,
			metrics.NewNoopCollector(),
			func(_ flow.Identifier) (bool, error) { return true, nil },
			queue.NewChunkDataPackRequestQueue(10, unittest.Logger(), metrics.NewNoopCollector()),
			DefaultChunkDataPackQueryTimeout,
			DefaultChunkDataPackDeliveryTimeout,
			DefaultChunkDataPackRequestWorker)
		require.NoError(t, err)

		originID := unittest.IdentifierFixture()
		chunkID := unittest.IdentifierFixture()
		blockID := unittest.IdentifierFixture()
		chunkDataPack := unittest.ChunkDataPackFixture(chunkID)

		ps.On("AtBlockID", blockID).Return(ss)
		ss.On("Identity", originID).Return(nil, protocol.IdentityNotFoundError{})
		execState.On("ChunkDataPackByChunkID", mock.Anything).Return(chunkDataPack, nil)
		execState.On("GetBlockIDByChunkID", chunkID).Return(blockID, nil)

		req := &messages.ChunkDataRequest{
			ChunkID: chunkID,
			Nonce:   rand.Uint64(),
		}
		cancelCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ctx, _ := irrecoverable.WithSignaler(cancelCtx)
		e.Start(ctx)
		// submit using non-existing origin ID
		unittest.RequireCloseBefore(t, e.Ready(), 100*time.Millisecond, "could not start engine")
		require.NoError(t, e.Process(channels.RequestChunks, originID, req))
		cancel()
		unittest.RequireCloseBefore(t, e.Done(), 100*time.Millisecond, "could not stop engine")

		// no chunk data pack response should be sent to a request coming from a non-existing origin ID
		chunkConduit.AssertNotCalled(t, "Unicast")

		ps.AssertExpectations(t)
		ss.AssertExpectations(t)
		execState.AssertExpectations(t)
		chunkConduit.AssertExpectations(t)
	})

	t.Run("non-existent chunk", func(t *testing.T) {
		ps := new(mockprotocol.State)
		ss := new(mockprotocol.Snapshot)
		net := new(mocknetwork.Network)
		chunkConduit := &mocknetwork.Conduit{}
		net.On("Register", channels.PushReceipts, mock.Anything).Return(&mocknetwork.Conduit{}, nil)
		net.On("Register", channels.ProvideChunks, mock.Anything).Return(chunkConduit, nil)

		execState := new(state.ExecutionState)
		execState.On("ChunkDataPackByChunkID", mock.Anything).Return(nil, errors.New("not found!"))

		e, err := New(
			unittest.Logger(),
			trace.NewNoopTracer(),
			net,
			ps,
			execState,
			metrics.NewNoopCollector(),
			func(_ flow.Identifier) (bool, error) { return true, nil },
			queue.NewChunkDataPackRequestQueue(10, unittest.Logger(), metrics.NewNoopCollector()),
			DefaultChunkDataPackQueryTimeout,
			DefaultChunkDataPackDeliveryTimeout,
			DefaultChunkDataPackRequestWorker)
		require.NoError(t, err)

		originIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

		chunkID := unittest.IdentifierFixture()

		req := &messages.ChunkDataRequest{
			ChunkID: chunkID,
			Nonce:   rand.Uint64(),
		}

		cancelCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ctx, _ := irrecoverable.WithSignaler(cancelCtx)
		e.Start(ctx)
		// submit using non-existing origin ID
		unittest.RequireCloseBefore(t, e.Ready(), 100*time.Millisecond, "could not start engine")
		require.NoError(t, e.Process(channels.RequestChunks, originIdentity.NodeID, req))
		cancel()
		unittest.RequireCloseBefore(t, e.Done(), 100*time.Millisecond, "could not stop engine")

		// no chunk data pack response should be sent to a request coming from a non-existing origin ID
		chunkConduit.AssertNotCalled(t, "Unicast")

		ps.AssertExpectations(t)
		ss.AssertExpectations(t)
		execState.AssertExpectations(t)
		chunkConduit.AssertExpectations(t)
	})

	t.Run("success", func(t *testing.T) {
		ps := new(mockprotocol.State)
		ss := new(mockprotocol.Snapshot)
		net := new(mocknetwork.Network)
		chunkConduit := &mocknetwork.Conduit{}
		execState := new(state.ExecutionState)

		net.On("Register", channels.PushReceipts, mock.Anything).Return(&mocknetwork.Conduit{}, nil)
		net.On("Register", channels.ProvideChunks, mock.Anything).Return(chunkConduit, nil)

		e, err := New(
			unittest.Logger(),
			trace.NewNoopTracer(),
			net,
			ps,
			execState,
			metrics.NewNoopCollector(),
			func(_ flow.Identifier) (bool, error) { return true, nil },
			queue.NewChunkDataPackRequestQueue(10, unittest.Logger(), metrics.NewNoopCollector()),
			DefaultChunkDataPackQueryTimeout,
			DefaultChunkDataPackDeliveryTimeout,
			DefaultChunkDataPackRequestWorker)
		require.NoError(t, err)

		originIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

		chunkID := unittest.IdentifierFixture()
		chunkDataPack := unittest.ChunkDataPackFixture(chunkID)
		blockID := unittest.IdentifierFixture()

		ps.On("AtBlockID", blockID).Return(ss)
		ss.On("Identity", originIdentity.NodeID).Return(originIdentity, nil)
		chunkConduit.On("Unicast", mock.Anything, originIdentity.NodeID).
			Run(func(args mock.Arguments) {
				res, ok := args[0].(*messages.ChunkDataResponse)
				require.True(t, ok)

				actualChunkID := res.ChunkDataPack.ChunkID
				assert.Equal(t, chunkID, actualChunkID)
			}).
			Return(nil)

		execState.On("GetBlockIDByChunkID", chunkID).Return(blockID, nil)
		execState.On("ChunkDataPackByChunkID", chunkID).Return(chunkDataPack, nil)

		req := &messages.ChunkDataRequest{
			ChunkID: chunkID,
			Nonce:   rand.Uint64(),
		}

		cancelCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ctx, _ := irrecoverable.WithSignaler(cancelCtx)
		e.Start(ctx)
		// submit using non-existing origin ID
		unittest.RequireCloseBefore(t, e.Ready(), 100*time.Millisecond, "could not start engine")
		require.NoError(t, e.Process(channels.RequestChunks, originIdentity.NodeID, req))
		cancel()
		unittest.RequireCloseBefore(t, e.Done(), 100*time.Millisecond, "could not stop engine")

		ps.AssertExpectations(t)
		ss.AssertExpectations(t)
		execState.AssertExpectations(t)
		chunkConduit.AssertExpectations(t)
	})

	t.Run("reply to chunk data pack request only when authorized", func(t *testing.T) {
		currentAuthorizedState := true

		ps := new(mockprotocol.State)
		ss := new(mockprotocol.Snapshot)
		net := new(mocknetwork.Network)
		chunkConduit := &mocknetwork.Conduit{}
		execState := new(state.ExecutionState)

		net.On("Register", channels.PushReceipts, mock.Anything).Return(&mocknetwork.Conduit{}, nil)
		net.On("Register", channels.ProvideChunks, mock.Anything).Return(chunkConduit, nil)

		e, err := New(
			unittest.Logger(),
			trace.NewNoopTracer(),
			net,
			ps,
			execState,
			metrics.NewNoopCollector(),
			func(_ flow.Identifier) (bool, error) { return currentAuthorizedState, nil },
			queue.NewChunkDataPackRequestQueue(10, unittest.Logger(), metrics.NewNoopCollector()),
			DefaultChunkDataPackQueryTimeout,
			DefaultChunkDataPackDeliveryTimeout,
			DefaultChunkDataPackRequestWorker)
		require.NoError(t, err)

		originIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

		chunkID := unittest.IdentifierFixture()
		chunkDataPack := unittest.ChunkDataPackFixture(chunkID)
		blockID := unittest.IdentifierFixture()

		execState.On("GetBlockIDByChunkID", chunkID).Return(blockID, nil)
		ps.On("AtBlockID", blockID).Return(ss)

		ss.On("Identity", originIdentity.NodeID).Return(originIdentity, nil).Once()

		// channel tracking for the first chunk data pack request responded.
		responded := make(chan struct{})
		chunkConduit.On("Unicast", mock.Anything, originIdentity.NodeID).
			Run(func(args mock.Arguments) {
				res, ok := args[0].(*messages.ChunkDataResponse)
				require.True(t, ok)

				actualChunkID := res.ChunkDataPack.ChunkID
				assert.Equal(t, chunkID, actualChunkID)
				close(responded)
			}).
			Return(nil).Once()

		execState.On("ChunkDataPackByChunkID", chunkID).Return(chunkDataPack, nil).Twice()

		req := &messages.ChunkDataRequest{
			ChunkID: chunkID,
			Nonce:   rand.Uint64(),
		}

		cancelCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ctx, _ := irrecoverable.WithSignaler(cancelCtx)
		e.Start(ctx)
		// submit using non-existing origin ID
		unittest.RequireCloseBefore(t, e.Ready(), 100*time.Millisecond, "could not start engine")
		require.NoError(t, e.Process(channels.RequestChunks, originIdentity.NodeID, req))

		// waits till first chunk data pack is replied and then makes the requester unauthorized.
		unittest.RequireCloseBefore(t, responded, 1*time.Second, "could not get first chunk data pack responded")
		currentAuthorizedState = false

		require.NoError(t, e.Process(channels.RequestChunks, originIdentity.NodeID, req))
		cancel()
		unittest.RequireCloseBefore(t, e.Done(), 100*time.Millisecond, "could not stop engine")

		ps.AssertExpectations(t)
		ss.AssertExpectations(t)
		execState.AssertExpectations(t)
		chunkConduit.AssertExpectations(t)
	})
}
