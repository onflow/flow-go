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
	"go.uber.org/atomic"

	state "github.com/onflow/flow-go/engine/execution/state/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/state/protocol"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestProviderEngine_onChunkDataRequest(t *testing.T) {
	t.Run("non-verification engine", func(t *testing.T) {
		ps := mockprotocol.NewState(t)
		ss := mockprotocol.NewSnapshot(t)
		net := mocknetwork.NewNetwork(t)
		chunkConduit := mocknetwork.NewConduit(t)
		execState := state.NewExecutionState(t)

		net.On("Register", channels.PushReceipts, mock.Anything).Return(&mocknetwork.Conduit{}, nil)
		net.On("Register", channels.ProvideChunks, mock.Anything).Return(chunkConduit, nil)
		requestQueue := queue.NewHeroStore(10, unittest.Logger(), metrics.NewNoopCollector())

		e, err := New(
			unittest.Logger(),
			trace.NewNoopTracer(),
			net,
			ps,
			execState,
			metrics.NewNoopCollector(),
			func(_ flow.Identifier) (bool, error) { return true, nil },
			requestQueue,
			DefaultChunkDataPackRequestWorker,
			DefaultChunkDataPackQueryTimeout,
			DefaultChunkDataPackDeliveryTimeout)
		require.NoError(t, err)

		originID := unittest.IdentifierFixture()
		chunkID := unittest.IdentifierFixture()
		blockID := unittest.IdentifierFixture()
		chunkDataPack := unittest.ChunkDataPackFixture(chunkID)

		ps.On("AtBlockID", blockID).Return(ss).Once()
		ss.On("Identity", originID).Return(unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution)), nil)
		execState.On("ChunkDataPackByChunkID", mock.Anything).Return(chunkDataPack, nil)

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

		require.Eventually(t, func() bool {
			_, ok := requestQueue.Get() // ensuring all requests have been picked up from the queue.
			return !ok
		}, 1*time.Second, 10*time.Millisecond)

		cancel()
		unittest.RequireCloseBefore(t, e.Done(), 100*time.Millisecond, "could not stop engine")

		// no chunk data pack response should be sent to an invalid role's request
		chunkConduit.AssertNotCalled(t, "Unicast")
	})

	t.Run("unauthorized (0 weight) origin", func(t *testing.T) {
		ps := mockprotocol.NewState(t)
		ss := mockprotocol.NewSnapshot(t)
		net := mocknetwork.NewNetwork(t)
		chunkConduit := mocknetwork.NewConduit(t)
		execState := state.NewExecutionState(t)

		net.On("Register", channels.PushReceipts, mock.Anything).Return(&mocknetwork.Conduit{}, nil)
		net.On("Register", channels.ProvideChunks, mock.Anything).Return(chunkConduit, nil)
		requestQueue := queue.NewHeroStore(10, unittest.Logger(), metrics.NewNoopCollector())

		e, err := New(
			unittest.Logger(),
			trace.NewNoopTracer(),
			net,
			ps,
			execState,
			metrics.NewNoopCollector(),
			func(_ flow.Identifier) (bool, error) { return true, nil },
			requestQueue,
			DefaultChunkDataPackRequestWorker,
			DefaultChunkDataPackQueryTimeout,
			DefaultChunkDataPackDeliveryTimeout)
		require.NoError(t, err)

		originID := unittest.IdentifierFixture()
		chunkID := unittest.IdentifierFixture()
		blockID := unittest.IdentifierFixture()
		chunkDataPack := unittest.ChunkDataPackFixture(chunkID)

		ps.On("AtBlockID", blockID).Return(ss).Once()
		ss.On("Identity", originID).Return(unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution), unittest.WithWeight(0)), nil)
		execState.On("ChunkDataPackByChunkID", mock.Anything).Return(chunkDataPack, nil)

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

		require.Eventually(t, func() bool {
			_, ok := requestQueue.Get() // ensuring all requests have been picked up from the queue.
			return !ok
		}, 1*time.Second, 10*time.Millisecond)

		cancel()
		unittest.RequireCloseBefore(t, e.Done(), 100*time.Millisecond, "could not stop engine")

		// no chunk data pack response should be sent to a request coming from 0-weight node
		chunkConduit.AssertNotCalled(t, "Unicast")
	})

	t.Run("un-authorized (not found origin) origin", func(t *testing.T) {
		ps := mockprotocol.NewState(t)
		ss := mockprotocol.NewSnapshot(t)
		net := mocknetwork.NewNetwork(t)
		chunkConduit := mocknetwork.NewConduit(t)
		execState := state.NewExecutionState(t)

		net.On("Register", channels.PushReceipts, mock.Anything).Return(&mocknetwork.Conduit{}, nil)
		net.On("Register", channels.ProvideChunks, mock.Anything).Return(chunkConduit, nil)
		requestQueue := queue.NewHeroStore(10, unittest.Logger(), metrics.NewNoopCollector())

		e, err := New(
			unittest.Logger(),
			trace.NewNoopTracer(),
			net,
			ps,
			execState,
			metrics.NewNoopCollector(),
			func(_ flow.Identifier) (bool, error) { return true, nil },
			requestQueue,
			DefaultChunkDataPackRequestWorker,
			DefaultChunkDataPackQueryTimeout,
			DefaultChunkDataPackDeliveryTimeout)
		require.NoError(t, err)

		originID := unittest.IdentifierFixture()
		chunkID := unittest.IdentifierFixture()
		blockID := unittest.IdentifierFixture()
		chunkDataPack := unittest.ChunkDataPackFixture(chunkID)

		ps.On("AtBlockID", blockID).Return(ss).Once()
		ss.On("Identity", originID).Return(nil, protocol.IdentityNotFoundError{})
		execState.On("ChunkDataPackByChunkID", mock.Anything).Return(chunkDataPack, nil)

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

		require.Eventually(t, func() bool {
			_, ok := requestQueue.Get() // ensuring all requests have been picked up from the queue.
			return !ok
		}, 1*time.Second, 10*time.Millisecond)

		cancel()
		unittest.RequireCloseBefore(t, e.Done(), 100*time.Millisecond, "could not stop engine")

		// no chunk data pack response should be sent to a request coming from a non-existing origin ID
		chunkConduit.AssertNotCalled(t, "Unicast")
	})

	t.Run("non-existent chunk", func(t *testing.T) {
		ps := mockprotocol.NewState(t)
		net := mocknetwork.NewNetwork(t)
		chunkConduit := mocknetwork.NewConduit(t)
		execState := state.NewExecutionState(t)

		net.On("Register", channels.PushReceipts, mock.Anything).Return(&mocknetwork.Conduit{}, nil)
		net.On("Register", channels.ProvideChunks, mock.Anything).Return(chunkConduit, nil)

		execState.On("ChunkDataPackByChunkID", mock.Anything).Return(nil, errors.New("not found!"))
		requestQueue := queue.NewHeroStore(10, unittest.Logger(), metrics.NewNoopCollector())

		e, err := New(
			unittest.Logger(),
			trace.NewNoopTracer(),
			net,
			ps,
			execState,
			metrics.NewNoopCollector(),
			func(_ flow.Identifier) (bool, error) { return true, nil },
			requestQueue,
			DefaultChunkDataPackRequestWorker,
			DefaultChunkDataPackQueryTimeout,
			DefaultChunkDataPackDeliveryTimeout)
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

		require.Eventually(t, func() bool {
			_, ok := requestQueue.Get() // ensuring all requests have been picked up from the queue.
			return !ok
		}, 1*time.Second, 10*time.Millisecond)

		cancel()
		unittest.RequireCloseBefore(t, e.Done(), 100*time.Millisecond, "could not stop engine")

		// no chunk data pack response should be sent to a request coming from a non-existing origin ID
		chunkConduit.AssertNotCalled(t, "Unicast")
	})

	t.Run("success", func(t *testing.T) {
		ps := new(mockprotocol.State)
		ss := new(mockprotocol.Snapshot)
		net := new(mocknetwork.Network)
		chunkConduit := &mocknetwork.Conduit{}
		execState := new(state.ExecutionState)

		net.On("Register", channels.PushReceipts, mock.Anything).Return(&mocknetwork.Conduit{}, nil)
		net.On("Register", channels.ProvideChunks, mock.Anything).Return(chunkConduit, nil)
		requestQueue := queue.NewHeroStore(10, unittest.Logger(), metrics.NewNoopCollector())

		e, err := New(
			unittest.Logger(),
			trace.NewNoopTracer(),
			net,
			ps,
			execState,
			metrics.NewNoopCollector(),
			func(_ flow.Identifier) (bool, error) { return true, nil },
			requestQueue,
			DefaultChunkDataPackRequestWorker,
			DefaultChunkDataPackQueryTimeout,
			DefaultChunkDataPackDeliveryTimeout)
		require.NoError(t, err)

		originIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

		chunkID := unittest.IdentifierFixture()
		chunkDataPack := unittest.ChunkDataPackFixture(chunkID)
		blockID := unittest.IdentifierFixture()

		ps.On("AtBlockID", blockID).Return(ss).Once()
		ss.On("Identity", originIdentity.NodeID).Return(originIdentity, nil)
		chunkConduit.On("Unicast", mock.Anything, originIdentity.NodeID).
			Run(func(args mock.Arguments) {
				res, ok := args[0].(*messages.ChunkDataResponse)
				require.True(t, ok)

				actualChunkID := res.ChunkDataPack.ChunkID
				assert.Equal(t, chunkID, actualChunkID)
			}).
			Return(nil)

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

		require.Eventually(t, func() bool {
			_, ok := requestQueue.Get() // ensuring all requests have been picked up from the queue.
			return !ok
		}, 1*time.Second, 10*time.Millisecond)

		cancel()
		unittest.RequireCloseBefore(t, e.Done(), 100*time.Millisecond, "could not stop engine")
	})

	t.Run("reply to chunk data pack request only when authorized", func(t *testing.T) {
		currentAuthorizedState := atomic.Bool{}
		currentAuthorizedState.Store(true)
		ps := mockprotocol.NewState(t)
		ss := mockprotocol.NewSnapshot(t)
		net := mocknetwork.NewNetwork(t)
		chunkConduit := mocknetwork.NewConduit(t)
		execState := state.NewExecutionState(t)

		net.On("Register", channels.PushReceipts, mock.Anything).Return(&mocknetwork.Conduit{}, nil)
		net.On("Register", channels.ProvideChunks, mock.Anything).Return(chunkConduit, nil)
		requestQueue := queue.NewHeroStore(10, unittest.Logger(), metrics.NewNoopCollector())

		e, err := New(
			unittest.Logger(),
			trace.NewNoopTracer(),
			net,
			ps,
			execState,
			metrics.NewNoopCollector(),
			func(_ flow.Identifier) (bool, error) { return currentAuthorizedState.Load(), nil },
			requestQueue,
			DefaultChunkDataPackRequestWorker,
			DefaultChunkDataPackQueryTimeout,
			DefaultChunkDataPackDeliveryTimeout)
		require.NoError(t, err)

		originIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

		chunkID := unittest.IdentifierFixture()
		chunkDataPack := unittest.ChunkDataPackFixture(chunkID)
		blockID := unittest.IdentifierFixture()

		ps.On("AtBlockID", blockID).Return(ss).Once()
		ss.On("Identity", originIdentity.NodeID).Return(originIdentity, nil).Once()

		// channel tracking for the first chunk data pack request responded.
		chunkConduit.On("Unicast", mock.Anything, originIdentity.NodeID).
			Run(func(args mock.Arguments) {
				res, ok := args[0].(*messages.ChunkDataResponse)
				require.True(t, ok)

				actualChunkID := res.ChunkDataPack.ChunkID
				assert.Equal(t, chunkID, actualChunkID)
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

		require.Eventually(t, func() bool {
			_, ok := requestQueue.Get() // ensuring first request has been picked up from the queue.
			return !ok
		}, 1*time.Second, 100*time.Millisecond)
		currentAuthorizedState.Store(false)

		require.NoError(t, e.Process(channels.RequestChunks, originIdentity.NodeID, req))
		require.Eventually(t, func() bool {
			_, ok := requestQueue.Get() // ensuring second request has been picked up from the queue as well.
			return !ok
		}, 1*time.Second, 10*time.Millisecond)

		cancel()
		unittest.RequireCloseBefore(t, e.Done(), 100*time.Millisecond, "could not stop engine")
	})
}
