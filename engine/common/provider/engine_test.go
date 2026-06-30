package provider_test

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v4"

	"github.com/onflow/flow-go/engine/common/provider"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/module/metrics"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/channels"
	mocknetwork "github.com/onflow/flow-go/network/mock"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestOnEntityRequestFull(t *testing.T) {
	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := irrecoverable.NewMockSignalerContext(t, cancelCtx)

	entities := make(map[flow.Identifier]flow.Entity)

	identities := unittest.IdentityListFixture(8)
	selector := filter.HasNodeID[flow.Identity](identities.NodeIDs()...)
	originID := identities[0].NodeID

	coll1 := unittest.CollectionFixture(1)
	coll2 := unittest.CollectionFixture(2)
	coll3 := unittest.CollectionFixture(3)
	coll4 := unittest.CollectionFixture(4)
	coll5 := unittest.CollectionFixture(5)

	entities[coll1.ID()] = coll1
	entities[coll2.ID()] = coll2
	entities[coll3.ID()] = coll3
	entities[coll4.ID()] = coll4
	entities[coll5.ID()] = coll5

	retrieve := func(entityID flow.Identifier) (flow.Entity, error) {
		entity, ok := entities[entityID]
		if !ok {
			return nil, storage.ErrNotFound
		}
		return entity, nil
	}

	final := protocol.NewSnapshot(t)
	final.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter[flow.Identity]) flow.IdentityList {
			return identities.Filter(selector)
		},
		nil,
	)

	state := protocol.NewState(t)
	state.On("Final").Return(final, nil)

	net := mocknetwork.NewEngineRegistry(t)
	con := mocknetwork.NewConduit(t)
	net.On("Register", mock.Anything, mock.Anything).Return(con, nil)
	con.On("Unicast", mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			defer cancel()

			response := args.Get(0).(*messages.EntityResponse)
			nodeID := args.Get(1).(flow.Identifier)
			assert.Equal(t, nodeID, originID)
			var entities []flow.Entity
			for _, blob := range response.Blobs {
				coll := &flow.Collection{}
				_ = msgpack.Unmarshal(blob, &coll)
				entities = append(entities, coll)
			}
			assert.ElementsMatch(t, entities, []flow.Entity{&coll1, &coll2, &coll3, &coll4, &coll5})
		},
	).Return(nil)

	me := mockmodule.NewLocal(t)
	me.On("NodeID").Return(unittest.IdentifierFixture())
	// CAUTION: the HeroStore fifo queue is NOT BFT. It should be used for messages from trusted sources only!
	// In the requester engine, the injected fifo queue is used to hold [flow.EntityResponse] messages from other
	// potentially byzantine peers. In PRODUCTION, you can NOT use a HeroStore here. However, for testing we
	// use the HeroStore for its better performance (reduced GC load on the maxed-out testing server).
	requestQueue := queue.NewHeroStore(10, unittest.Logger(), metrics.NewNoopCollector())

	e, err := provider.New(
		unittest.Logger(),
		metrics.NewNoopCollector(),
		net,
		me,
		state,
		requestQueue,
		provider.DefaultRequestProviderWorkers,
		channels.TestNetworkChannel,
		selector,
		retrieve)
	require.NoError(t, err)

	request := &flow.EntityRequest{
		Nonce:     rand.Uint64(),
		EntityIDs: []flow.Identifier{coll1.ID(), coll2.ID(), coll3.ID(), coll4.ID(), coll5.ID()},
	}

	e.Start(ctx)

	unittest.RequireCloseBefore(t, e.Ready(), 100*time.Millisecond, "could not start engine")

	err = e.Process(channels.TestNetworkChannel, originID, request)
	require.NoError(t, err, "should not error on full response")

	unittest.RequireCloseBefore(t, e.Done(), 100*time.Millisecond, "could not stop engine")
}

func TestOnEntityRequestPartial(t *testing.T) {
	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := irrecoverable.NewMockSignalerContext(t, cancelCtx)

	entities := make(map[flow.Identifier]flow.Entity)

	identities := unittest.IdentityListFixture(8)
	selector := filter.HasNodeID[flow.Identity](identities.NodeIDs()...)
	originID := identities[0].NodeID

	coll1 := unittest.CollectionFixture(1)
	coll2 := unittest.CollectionFixture(2)
	coll3 := unittest.CollectionFixture(3)
	coll4 := unittest.CollectionFixture(4)
	coll5 := unittest.CollectionFixture(5)

	entities[coll1.ID()] = coll1
	// entities[coll2.ID()] = coll2
	entities[coll3.ID()] = coll3
	// entities[coll4.ID()] = coll4
	entities[coll5.ID()] = coll5

	retrieve := func(entityID flow.Identifier) (flow.Entity, error) {
		entity, ok := entities[entityID]
		if !ok {
			return nil, storage.ErrNotFound
		}
		return entity, nil
	}

	final := protocol.NewSnapshot(t)
	final.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter[flow.Identity]) flow.IdentityList {
			return identities.Filter(selector)
		},
		nil,
	)

	state := protocol.NewState(t)
	state.On("Final").Return(final, nil)

	net := mocknetwork.NewEngineRegistry(t)
	con := mocknetwork.NewConduit(t)
	net.On("Register", mock.Anything, mock.Anything).Return(con, nil)
	con.On("Unicast", mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			defer cancel()

			response := args.Get(0).(*messages.EntityResponse)
			nodeID := args.Get(1).(flow.Identifier)
			assert.Equal(t, nodeID, originID)
			var entities []flow.Entity
			for _, blob := range response.Blobs {
				coll := &flow.Collection{}
				_ = msgpack.Unmarshal(blob, &coll)
				entities = append(entities, coll)
			}
			assert.ElementsMatch(t, entities, []flow.Entity{&coll1, &coll3, &coll5})
		},
	).Return(nil)

	me := mockmodule.NewLocal(t)
	me.On("NodeID").Return(unittest.IdentifierFixture())
	// CAUTION: the HeroStore fifo queue is NOT BFT. It should be used for messages from trusted sources only!
	// In the requester engine, the injected fifo queue is used to hold [flow.EntityResponse] messages from other
	// potentially byzantine peers. In PRODUCTION, you can NOT use a HeroStore here. However, for testing we
	// use the HeroStore for its better performance (reduced GC load on the maxed-out testing server).
	requestQueue := queue.NewHeroStore(10, unittest.Logger(), metrics.NewNoopCollector())

	e, err := provider.New(
		unittest.Logger(),
		metrics.NewNoopCollector(),
		net,
		me,
		state,
		requestQueue,
		provider.DefaultRequestProviderWorkers,
		channels.TestNetworkChannel,
		selector,
		retrieve)
	require.NoError(t, err)

	request := &flow.EntityRequest{
		Nonce:     rand.Uint64(),
		EntityIDs: []flow.Identifier{coll1.ID(), coll2.ID(), coll3.ID(), coll4.ID(), coll5.ID()},
	}

	e.Start(ctx)

	unittest.RequireCloseBefore(t, e.Ready(), 100*time.Millisecond, "could not start engine")
	err = e.Process(channels.TestNetworkChannel, originID, request)
	require.NoError(t, err, "should not error on full response")
	unittest.RequireCloseBefore(t, e.Done(), 100*time.Millisecond, "could not stop engine")
}

func TestOnEntityRequestDuplicates(t *testing.T) {
	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := irrecoverable.NewMockSignalerContext(t, cancelCtx)

	entities := make(map[flow.Identifier]flow.Entity)

	identities := unittest.IdentityListFixture(8)
	selector := filter.HasNodeID[flow.Identity](identities.NodeIDs()...)
	originID := identities[0].NodeID

	coll1 := unittest.CollectionFixture(1)
	coll2 := unittest.CollectionFixture(2)
	coll3 := unittest.CollectionFixture(3)

	entities[coll1.ID()] = coll1
	entities[coll2.ID()] = coll2
	entities[coll3.ID()] = coll3

	retrieve := func(entityID flow.Identifier) (flow.Entity, error) {
		entity, ok := entities[entityID]
		if !ok {
			return nil, storage.ErrNotFound
		}
		return entity, nil
	}

	final := protocol.NewSnapshot(t)
	final.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter[flow.Identity]) flow.IdentityList {
			return identities.Filter(selector)
		},
		nil,
	)

	state := protocol.NewState(t)
	state.On("Final").Return(final, nil)

	net := mocknetwork.NewEngineRegistry(t)
	con := mocknetwork.NewConduit(t)
	net.On("Register", mock.Anything, mock.Anything).Return(con, nil)
	con.On("Unicast", mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			defer cancel()

			response := args.Get(0).(*messages.EntityResponse)
			nodeID := args.Get(1).(flow.Identifier)
			assert.Equal(t, nodeID, originID)
			var entities []flow.Entity
			for _, blob := range response.Blobs {
				coll := &flow.Collection{}
				_ = msgpack.Unmarshal(blob, &coll)
				entities = append(entities, coll)
			}
			assert.ElementsMatch(t, entities, []flow.Entity{&coll1, &coll2, &coll3})
		},
	).Return(nil)

	me := mockmodule.NewLocal(t)
	me.On("NodeID").Return(unittest.IdentifierFixture())
	// CAUTION: the HeroStore fifo queue is NOT BFT. It should be used for messages from trusted sources only!
	// In the requester engine, the injected fifo queue is used to hold [flow.EntityResponse] messages from other
	// potentially byzantine peers. In PRODUCTION, you can NOT use a HeroStore here. However, for testing we
	// use the HeroStore for its better performance (reduced GC load on the maxed-out testing server).
	requestQueue := queue.NewHeroStore(10, unittest.Logger(), metrics.NewNoopCollector())

	e, err := provider.New(
		unittest.Logger(),
		metrics.NewNoopCollector(),
		net,
		me,
		state,
		requestQueue,
		provider.DefaultRequestProviderWorkers,
		channels.TestNetworkChannel,
		selector,
		retrieve)
	require.NoError(t, err)

	// create entity requests with some duplicate entity IDs
	request := &flow.EntityRequest{
		Nonce:     rand.Uint64(),
		EntityIDs: []flow.Identifier{coll1.ID(), coll2.ID(), coll3.ID(), coll3.ID(), coll2.ID(), coll1.ID()},
	}

	e.Start(ctx)
	unittest.RequireCloseBefore(t, e.Ready(), 100*time.Millisecond, "could not start engine")
	err = e.Process(channels.TestNetworkChannel, originID, request)
	require.NoError(t, err, "should not error on full response")
	unittest.RequireCloseBefore(t, e.Done(), 100*time.Millisecond, "could not stop engine")
}

func TestOnEntityRequestEmpty(t *testing.T) {
	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := irrecoverable.NewMockSignalerContext(t, cancelCtx)

	entities := make(map[flow.Identifier]flow.Entity)
	identities := unittest.IdentityListFixture(8)
	selector := filter.HasNodeID[flow.Identity](identities.NodeIDs()...)
	originID := identities[0].NodeID

	coll1 := unittest.CollectionFixture(1)
	coll2 := unittest.CollectionFixture(2)
	coll3 := unittest.CollectionFixture(3)
	coll4 := unittest.CollectionFixture(4)
	coll5 := unittest.CollectionFixture(5)

	retrieve := func(entityID flow.Identifier) (flow.Entity, error) {
		entity, ok := entities[entityID]
		if !ok {
			return nil, storage.ErrNotFound
		}
		return entity, nil
	}

	final := protocol.NewSnapshot(t)
	final.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter[flow.Identity]) flow.IdentityList {
			return identities.Filter(selector)
		},
		nil,
	)

	state := protocol.NewState(t)
	state.On("Final").Return(final, nil)

	net := mocknetwork.NewEngineRegistry(t)
	con := mocknetwork.NewConduit(t)
	net.On("Register", mock.Anything, mock.Anything).Return(con, nil)
	con.On("Unicast", mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			defer cancel()

			response := args.Get(0).(*messages.EntityResponse)
			nodeID := args.Get(1).(flow.Identifier)
			assert.Equal(t, nodeID, originID)
			assert.Empty(t, response.Blobs)
		},
	).Return(nil)

	me := mockmodule.NewLocal(t)
	me.On("NodeID").Return(unittest.IdentifierFixture())
	// CAUTION: the HeroStore fifo queue is NOT BFT. It should be used for messages from trusted sources only!
	// In the requester engine, the injected fifo queue is used to hold [flow.EntityResponse] messages from other
	// potentially byzantine peers. In PRODUCTION, you can NOT use a HeroStore here. However, for testing we
	// use the HeroStore for its better performance (reduced GC load on the maxed-out testing server).
	requestQueue := queue.NewHeroStore(10, unittest.Logger(), metrics.NewNoopCollector())

	e, err := provider.New(
		unittest.Logger(),
		metrics.NewNoopCollector(),
		net,
		me,
		state,
		requestQueue,
		provider.DefaultRequestProviderWorkers,
		channels.TestNetworkChannel,
		selector,
		retrieve)
	require.NoError(t, err)

	request := &flow.EntityRequest{
		Nonce:     rand.Uint64(),
		EntityIDs: []flow.Identifier{coll1.ID(), coll2.ID(), coll3.ID(), coll4.ID(), coll5.ID()},
	}

	e.Start(ctx)
	unittest.RequireCloseBefore(t, e.Ready(), 100*time.Millisecond, "could not start engine")
	err = e.Process(channels.TestNetworkChannel, originID, request)
	require.NoError(t, err, "should not error on full response")
	unittest.RequireCloseBefore(t, e.Done(), 100*time.Millisecond, "could not stop engine")
}

func TestOnEntityRequestInvalidOrigin(t *testing.T) {
	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := irrecoverable.NewMockSignalerContext(t, cancelCtx)

	entities := make(map[flow.Identifier]flow.Entity)
	identities := unittest.IdentityListFixture(8)
	selector := filter.HasNodeID[flow.Identity](identities.NodeIDs()...)
	originID := unittest.IdentifierFixture()

	coll1 := unittest.CollectionFixture(1)
	coll2 := unittest.CollectionFixture(2)
	coll3 := unittest.CollectionFixture(3)
	coll4 := unittest.CollectionFixture(4)
	coll5 := unittest.CollectionFixture(5)

	entities[coll1.ID()] = coll1
	entities[coll2.ID()] = coll2
	entities[coll3.ID()] = coll3
	entities[coll4.ID()] = coll4
	entities[coll5.ID()] = coll5

	retrieve := func(entityID flow.Identifier) (flow.Entity, error) {
		entity, ok := entities[entityID]
		if !ok {
			return nil, storage.ErrNotFound
		}
		return entity, nil
	}

	final := protocol.NewSnapshot(t)
	final.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter[flow.Identity]) flow.IdentityList {
			defer cancel()
			return identities.Filter(selector)
		},
		nil,
	)

	state := protocol.NewState(t)
	state.On("Final").Return(final, nil)

	net := mocknetwork.NewEngineRegistry(t)
	con := mocknetwork.NewConduit(t)
	net.On("Register", mock.Anything, mock.Anything).Return(con, nil)
	me := mockmodule.NewLocal(t)
	me.On("NodeID").Return(unittest.IdentifierFixture())
	// CAUTION: the HeroStore fifo queue is NOT BFT. It should be used for messages from trusted sources only!
	// In the requester engine, the injected fifo queue is used to hold [flow.EntityResponse] messages from other
	// potentially byzantine peers. In PRODUCTION, you can NOT use a HeroStore here. However, for testing we
	// use the HeroStore for its better performance (reduced GC load on the maxed-out testing server).
	requestQueue := queue.NewHeroStore(10, unittest.Logger(), metrics.NewNoopCollector())

	e, err := provider.New(
		unittest.Logger(),
		metrics.NewNoopCollector(),
		net,
		me,
		state,
		requestQueue,
		provider.DefaultRequestProviderWorkers,
		channels.TestNetworkChannel,
		selector,
		retrieve)
	require.NoError(t, err)

	request := &flow.EntityRequest{
		Nonce:     rand.Uint64(),
		EntityIDs: []flow.Identifier{coll1.ID(), coll2.ID(), coll3.ID(), coll4.ID(), coll5.ID()},
	}

	e.Start(ctx)
	unittest.RequireCloseBefore(t, e.Ready(), 100*time.Millisecond, "could not start engine")
	err = e.Process(channels.TestNetworkChannel, originID, request)
	require.NoError(t, err)
	unittest.RequireCloseBefore(t, e.Done(), 100*time.Millisecond, "could not stop engine")
}

// TestOnEntityRequestTruncatesExcessiveIDs verifies that requests with more entity IDs than
// the maximum limit are truncated. This prevents amplification attacks where a small request
// triggers excessive retrieval work.
func TestOnEntityRequestTruncatesExcessiveIDs(t *testing.T) {
	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := irrecoverable.NewMockSignalerContext(t, cancelCtx)

	// Set a low limit for testing
	const maxEntityIDs = 3

	entities := make(map[flow.Identifier]flow.Entity)
	identities := unittest.IdentityListFixture(8)
	selector := filter.HasNodeID[flow.Identity](identities.NodeIDs()...)
	originID := identities[0].NodeID

	// Create 5 collections, but only the first 3 should be returned due to the limit
	coll1 := unittest.CollectionFixture(1)
	coll2 := unittest.CollectionFixture(2)
	coll3 := unittest.CollectionFixture(3)
	coll4 := unittest.CollectionFixture(4)
	coll5 := unittest.CollectionFixture(5)

	entities[coll1.ID()] = coll1
	entities[coll2.ID()] = coll2
	entities[coll3.ID()] = coll3
	entities[coll4.ID()] = coll4
	entities[coll5.ID()] = coll5

	retrieve := func(entityID flow.Identifier) (flow.Entity, error) {
		entity, ok := entities[entityID]
		if !ok {
			return nil, storage.ErrNotFound
		}
		return entity, nil
	}

	final := protocol.NewSnapshot(t)
	final.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter[flow.Identity]) flow.IdentityList {
			return identities.Filter(selector)
		},
		nil,
	)

	state := protocol.NewState(t)
	state.On("Final").Return(final, nil)

	net := mocknetwork.NewEngineRegistry(t)
	con := mocknetwork.NewConduit(t)
	net.On("Register", mock.Anything, mock.Anything).Return(con, nil)
	con.On("Unicast", mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			defer cancel()

			response := args.Get(0).(*messages.EntityResponse)
			nodeID := args.Get(1).(flow.Identifier)
			assert.Equal(t, nodeID, originID)

			// Only the first 3 collections should be returned due to the maxEntityIDs limit
			assert.Len(t, response.Blobs, maxEntityIDs, "response should contain exactly maxEntityIDs entities")

			var returnedEntities []flow.Entity
			for _, blob := range response.Blobs {
				coll := &flow.Collection{}
				_ = msgpack.Unmarshal(blob, &coll)
				returnedEntities = append(returnedEntities, coll)
			}
			// The request was [coll1, coll2, coll3, coll4, coll5], truncated to first 3
			assert.ElementsMatch(t, returnedEntities, []flow.Entity{&coll1, &coll2, &coll3})
		},
	).Return(nil)

	me := mockmodule.NewLocal(t)
	me.On("NodeID").Return(unittest.IdentifierFixture())
	requestQueue := queue.NewHeroStore(10, unittest.Logger(), metrics.NewNoopCollector())

	e, err := provider.New(
		unittest.Logger(),
		metrics.NewNoopCollector(),
		net,
		me,
		state,
		requestQueue,
		provider.DefaultRequestProviderWorkers,
		channels.TestNetworkChannel,
		selector,
		retrieve,
		provider.WithMaxEntityIDs(maxEntityIDs))
	require.NoError(t, err)

	// Request 5 entities, but only 3 should be returned
	request := &flow.EntityRequest{
		Nonce:     rand.Uint64(),
		EntityIDs: []flow.Identifier{coll1.ID(), coll2.ID(), coll3.ID(), coll4.ID(), coll5.ID()},
	}

	e.Start(ctx)
	unittest.RequireCloseBefore(t, e.Ready(), 100*time.Millisecond, "could not start engine")
	err = e.Process(channels.TestNetworkChannel, originID, request)
	require.NoError(t, err, "should not error when truncating excessive IDs")
	unittest.RequireCloseBefore(t, e.Done(), 100*time.Millisecond, "could not stop engine")
}

// TestOnEntityRequestResponseByteBudget verifies that the response byte budget is enforced.
// The provider should stop adding entities once the cumulative encoded size exceeds the budget,
// preventing unbounded serialization work.
func TestOnEntityRequestResponseByteBudget(t *testing.T) {
	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := irrecoverable.NewMockSignalerContext(t, cancelCtx)

	entities := make(map[flow.Identifier]flow.Entity)
	identities := unittest.IdentityListFixture(8)
	selector := filter.HasNodeID[flow.Identity](identities.NodeIDs()...)
	originID := identities[0].NodeID

	// Create collections with varying sizes
	coll1 := unittest.CollectionFixture(1)
	coll2 := unittest.CollectionFixture(2)
	coll3 := unittest.CollectionFixture(3)

	entities[coll1.ID()] = coll1
	entities[coll2.ID()] = coll2
	entities[coll3.ID()] = coll3

	// Calculate the encoded size of the first collection to set a budget that allows
	// only 1-2 collections
	blob1, err := msgpack.Marshal(coll1)
	require.NoError(t, err)
	blob2, err := msgpack.Marshal(coll2)
	require.NoError(t, err)

	// Set budget to allow first two collections but not the third
	maxResponseByteSize := len(blob1) + len(blob2) + 10 // small buffer, but not enough for coll3

	retrieve := func(entityID flow.Identifier) (flow.Entity, error) {
		entity, ok := entities[entityID]
		if !ok {
			return nil, storage.ErrNotFound
		}
		return entity, nil
	}

	final := protocol.NewSnapshot(t)
	final.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter[flow.Identity]) flow.IdentityList {
			return identities.Filter(selector)
		},
		nil,
	)

	state := protocol.NewState(t)
	state.On("Final").Return(final, nil)

	net := mocknetwork.NewEngineRegistry(t)
	con := mocknetwork.NewConduit(t)
	net.On("Register", mock.Anything, mock.Anything).Return(con, nil)
	con.On("Unicast", mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			defer cancel()

			response := args.Get(0).(*messages.EntityResponse)
			nodeID := args.Get(1).(flow.Identifier)
			assert.Equal(t, nodeID, originID)

			// Should have at most 2 entities due to byte budget
			assert.LessOrEqual(t, len(response.Blobs), 2, "response should not exceed byte budget")

			// Calculate total response size
			totalSize := 0
			for _, blob := range response.Blobs {
				totalSize += len(blob)
			}
			assert.LessOrEqual(t, totalSize, maxResponseByteSize, "total response size should not exceed budget")
		},
	).Return(nil)

	me := mockmodule.NewLocal(t)
	me.On("NodeID").Return(unittest.IdentifierFixture())
	requestQueue := queue.NewHeroStore(10, unittest.Logger(), metrics.NewNoopCollector())

	e, err := provider.New(
		unittest.Logger(),
		metrics.NewNoopCollector(),
		net,
		me,
		state,
		requestQueue,
		provider.DefaultRequestProviderWorkers,
		channels.TestNetworkChannel,
		selector,
		retrieve,
		provider.WithMaxResponseByteSize(maxResponseByteSize))
	require.NoError(t, err)

	// Request all 3 entities, but byte budget should limit response
	request := &flow.EntityRequest{
		Nonce:     rand.Uint64(),
		EntityIDs: []flow.Identifier{coll1.ID(), coll2.ID(), coll3.ID()},
	}

	e.Start(ctx)
	unittest.RequireCloseBefore(t, e.Ready(), 100*time.Millisecond, "could not start engine")
	err = e.Process(channels.TestNetworkChannel, originID, request)
	require.NoError(t, err, "should not error when byte budget is exceeded")
	unittest.RequireCloseBefore(t, e.Done(), 100*time.Millisecond, "could not stop engine")
}
