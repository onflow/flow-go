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

	state "github.com/onflow/flow-go/engine/execution/state/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/mocknetwork"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestProviderEngine_onChunkDataRequest(t *testing.T) {

	t.Run("non-existent chunk", func(t *testing.T) {
		net := mocknetwork.NewNetwork(t)
		chunkConduit := mocknetwork.NewConduit(t)
		net.On("Register", channels.PushReceipts, mock.Anything).Return(&mocknetwork.Conduit{}, nil)
		net.On("Register", channels.ProvideChunks, mock.Anything).Return(chunkConduit, nil)
		e, _, es, requestQueue := newTestEngine(t, net, true)

		es.On("ChunkDataPackByChunkID", mock.Anything).Return(nil, errors.New("not found!"))

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
		net := mocknetwork.NewNetwork(t)
		chunkConduit := &mocknetwork.Conduit{}
		net.On("Register", channels.PushReceipts, mock.Anything).Return(&mocknetwork.Conduit{}, nil)
		net.On("Register", channels.ProvideChunks, mock.Anything).Return(chunkConduit, nil)
		e, _, es, requestQueue := newTestEngine(t, net, true)

		originIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

		chunkID := unittest.IdentifierFixture()
		chunkDataPack := unittest.ChunkDataPackFixture(chunkID)

		chunkConduit.On("Unicast", mock.Anything, originIdentity.NodeID).
			Run(func(args mock.Arguments) {
				res, ok := args[0].(*messages.ChunkDataResponse)
				require.True(t, ok)

				actualChunkID := res.ChunkDataPack.ChunkID
				assert.Equal(t, chunkID, actualChunkID)
			}).
			Return(nil)

		es.On("ChunkDataPackByChunkID", chunkID).Return(chunkDataPack, nil)

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

}

func TestProviderEngine_BroadcastExecutionReceipt(t *testing.T) {
	// prepare
	net := mocknetwork.NewNetwork(t)
	chunkConduit := mocknetwork.NewConduit(t)
	receiptConduit := mocknetwork.NewConduit(t)
	net.On("Register", channels.PushReceipts, mock.Anything).Return(receiptConduit, nil)
	net.On("Register", channels.ProvideChunks, mock.Anything).Return(chunkConduit, nil)
	e, ps, _, _ := newTestEngine(t, net, true)

	sealedBlock := unittest.BlockHeaderFixture()
	sealed := new(mockprotocol.Snapshot)
	sealed.On("Head").Return(sealedBlock, nil)
	ps.On("Sealed").Return(sealed)
	sealedHeight := sealedBlock.Height

	receivers := unittest.IdentityListFixture(1)
	snap := new(mockprotocol.Snapshot)
	snap.On("Identities", mock.Anything).Return(receivers, nil)
	ps.On("Final").Return(snap)

	// verify that above the sealed height will be broadcasted
	receipt1 := unittest.ExecutionReceiptFixture()
	receiptConduit.On("Publish", receipt1, receivers.NodeIDs()[0]).Return(nil)

	broadcasted, err := e.BroadcastExecutionReceipt(context.Background(), sealedHeight+1, receipt1)
	require.NoError(t, err)
	require.True(t, broadcasted)

	// verify that equal the sealed height will NOT be broadcasted
	receipt2 := unittest.ExecutionReceiptFixture()
	broadcasted, err = e.BroadcastExecutionReceipt(context.Background(), sealedHeight, receipt2)
	require.NoError(t, err)
	require.False(t, broadcasted)

	// verify that below the sealed height will NOT be broadcasted
	broadcasted, err = e.BroadcastExecutionReceipt(context.Background(), sealedHeight-1, receipt2)
	require.NoError(t, err)
	require.False(t, broadcasted)
}

func TestProviderEngine_BroadcastExecutionUnauthorized(t *testing.T) {
	net := mocknetwork.NewNetwork(t)
	chunkConduit := mocknetwork.NewConduit(t)
	receiptConduit := mocknetwork.NewConduit(t)
	net.On("Register", channels.PushReceipts, mock.Anything).Return(receiptConduit, nil)
	net.On("Register", channels.ProvideChunks, mock.Anything).Return(chunkConduit, nil)
	// make sure the node is not authorized for broadcasting
	authorized := false
	e, ps, _, _ := newTestEngine(t, net, authorized)

	sealedBlock := unittest.BlockHeaderFixture()
	sealed := mockprotocol.NewSnapshot(t)
	sealed.On("Head").Return(sealedBlock, nil)
	ps.On("Sealed").Return(sealed)
	sealedHeight := sealedBlock.Height

	// verify that unstaked node will NOT broadcast
	receipt2 := unittest.ExecutionReceiptFixture()
	broadcasted, err := e.BroadcastExecutionReceipt(context.Background(), sealedHeight+1, receipt2)
	require.NoError(t, err)
	require.False(t, broadcasted)
}

func newTestEngine(t *testing.T, net *mocknetwork.Network, authorized bool) (
	*Engine,
	*mockprotocol.State,
	*state.ExecutionState,
	*queue.HeroStore,
) {
	ps := mockprotocol.NewState(t)
	execState := state.NewExecutionState(t)
	requestQueue := queue.NewHeroStore(10, unittest.Logger(), metrics.NewNoopCollector())

	e, err := New(
		unittest.Logger(),
		trace.NewNoopTracer(),
		net,
		ps,
		execState,
		metrics.NewNoopCollector(),
		func(_ flow.Identifier) (bool, error) { return authorized, nil },
		requestQueue,
		DefaultChunkDataPackRequestWorker,
		DefaultChunkDataPackQueryTimeout,
		DefaultChunkDataPackDeliveryTimeout)
	require.NoError(t, err)
	return e, ps, execState, requestQueue
}
