package requester

import (
	"math/rand"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack"

	"github.com/onflow/flow-go/engine/common/splitter/network"
	module "github.com/onflow/flow-go/module/mock"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/mocknetwork"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestEntityByID(t *testing.T) {

	request := Engine{
		unit:  engine.NewUnit(),
		items: make(map[flow.Identifier]*Item),
	}

	now := time.Now().UTC()

	entityID := unittest.IdentifierFixture()
	selector := filter.Any
	request.EntityByID(entityID, selector)

	assert.Len(t, request.items, 1)
	item, contains := request.items[entityID]
	if assert.True(t, contains) {
		assert.Equal(t, item.EntityID, entityID)
		assert.Equal(t, item.NumAttempts, uint(0))
		cutoff := item.LastRequested.Add(item.RetryAfter)
		assert.True(t, cutoff.Before(now)) // make sure we push out immediately
	}
}

func TestDispatchRequestVarious(t *testing.T) {

	identities := unittest.IdentityListFixture(16)
	targetID := identities[0].NodeID

	final := &protocol.Snapshot{}
	final.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter) flow.IdentityList {
			return identities.Filter(selector)
		},
		nil,
	)

	state := &protocol.State{}
	state.On("Final").Return(final)

	cfg := Config{
		BatchInterval:  200 * time.Millisecond,
		BatchThreshold: 999,
		RetryInitial:   100 * time.Millisecond,
		RetryFunction:  RetryLinear(10 * time.Millisecond),
		RetryAttempts:  2,
		RetryMaximum:   300 * time.Millisecond,
	}

	// item that has just been added, should be included
	justAdded := &Item{
		EntityID:      unittest.IdentifierFixture(),
		NumAttempts:   0,
		LastRequested: time.Time{},
		RetryAfter:    cfg.RetryInitial,
		ExtraSelector: filter.Any,
	}

	// item was tried long time ago, should be included
	triedAnciently := &Item{
		EntityID:      unittest.IdentifierFixture(),
		NumAttempts:   1,
		LastRequested: time.Now().UTC().Add(-cfg.RetryMaximum),
		RetryAfter:    cfg.RetryFunction(cfg.RetryInitial),
		ExtraSelector: filter.Any,
	}

	// item that was just tried, should be excluded
	triedRecently := &Item{
		EntityID:      unittest.IdentifierFixture(),
		NumAttempts:   1,
		LastRequested: time.Now().UTC(),
		RetryAfter:    cfg.RetryFunction(cfg.RetryInitial),
	}

	// item was tried twice, should be excluded
	triedTwice := &Item{
		EntityID:      unittest.IdentifierFixture(),
		NumAttempts:   2,
		LastRequested: time.Time{},
		RetryAfter:    cfg.RetryInitial,
		ExtraSelector: filter.Any,
	}

	items := make(map[flow.Identifier]*Item)
	items[justAdded.EntityID] = justAdded
	items[triedAnciently.EntityID] = triedAnciently
	items[triedRecently.EntityID] = triedRecently
	items[triedTwice.EntityID] = triedTwice

	var nonce uint64

	con := &mocknetwork.Conduit{}
	con.On("Unicast", mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			request := args.Get(0).(*messages.EntityRequest)
			originID := args.Get(1).(flow.Identifier)
			nonce = request.Nonce
			assert.Equal(t, originID, targetID)
			assert.ElementsMatch(t, request.EntityIDs, []flow.Identifier{justAdded.EntityID, triedAnciently.EntityID})
		},
	).Return(nil)

	request := Engine{
		unit:     engine.NewUnit(),
		metrics:  metrics.NewNoopCollector(),
		cfg:      cfg,
		state:    state,
		con:      con,
		items:    items,
		requests: make(map[uint64]*messages.EntityRequest),
		selector: filter.HasNodeID(targetID),
	}
	dispatched, err := request.dispatchRequest()
	require.NoError(t, err)
	require.True(t, dispatched)

	con.AssertExpectations(t)

	assert.Contains(t, request.requests, nonce)

	time.Sleep(2 * cfg.RetryInitial)

	assert.NotContains(t, request.requests, nonce)
}

func TestDispatchRequestBatchSize(t *testing.T) {

	batchLimit := uint(16)
	totalItems := uint(99)

	identities := unittest.IdentityListFixture(16)

	final := &protocol.Snapshot{}
	final.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter) flow.IdentityList {
			return identities.Filter(selector)
		},
		nil,
	)

	state := &protocol.State{}
	state.On("Final").Return(final)

	cfg := Config{
		BatchInterval:  24 * time.Hour,
		BatchThreshold: batchLimit,
		RetryInitial:   24 * time.Hour,
		RetryFunction:  RetryLinear(1),
		RetryAttempts:  1,
		RetryMaximum:   24 * time.Hour,
	}

	// item that has just been added, should be included
	items := make(map[flow.Identifier]*Item)
	for i := uint(0); i < totalItems; i++ {
		item := &Item{
			EntityID:      unittest.IdentifierFixture(),
			NumAttempts:   0,
			LastRequested: time.Time{},
			RetryAfter:    cfg.RetryInitial,
			ExtraSelector: filter.Any,
		}
		items[item.EntityID] = item
	}

	con := &mocknetwork.Conduit{}
	con.On("Unicast", mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			request := args.Get(0).(*messages.EntityRequest)
			assert.Len(t, request.EntityIDs, int(batchLimit))
		},
	).Return(nil)

	request := Engine{
		unit:     engine.NewUnit(),
		metrics:  metrics.NewNoopCollector(),
		cfg:      cfg,
		state:    state,
		con:      con,
		items:    items,
		requests: make(map[uint64]*messages.EntityRequest),
		selector: filter.Any,
	}
	dispatched, err := request.dispatchRequest()
	require.NoError(t, err)
	require.True(t, dispatched)

	con.AssertExpectations(t)
}

func TestOnEntityResponseValid(t *testing.T) {

	identities := unittest.IdentityListFixture(16)
	targetID := identities[0].NodeID

	final := &protocol.Snapshot{}
	final.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter) flow.IdentityList {
			return identities.Filter(selector)
		},
		nil,
	)

	state := &protocol.State{}
	state.On("Final").Return(final)

	nonce := rand.Uint64()

	wanted1 := unittest.CollectionFixture(1)
	wanted2 := unittest.CollectionFixture(2)
	unavailable := unittest.CollectionFixture(3)
	unwanted := unittest.CollectionFixture(4)

	now := time.Now()

	iwanted1 := &Item{
		EntityID:      wanted1.ID(),
		LastRequested: now,
		ExtraSelector: filter.Any,
	}
	iwanted2 := &Item{
		EntityID:      wanted2.ID(),
		LastRequested: now,
		ExtraSelector: filter.Any,
	}
	iunavailable := &Item{
		EntityID:      unavailable.ID(),
		LastRequested: now,
		ExtraSelector: filter.Any,
	}

	bwanted1, _ := msgpack.Marshal(wanted1)
	bwanted2, _ := msgpack.Marshal(wanted2)
	bunwanted, _ := msgpack.Marshal(unwanted)

	res := &messages.EntityResponse{
		Nonce:     nonce,
		EntityIDs: []flow.Identifier{wanted1.ID(), wanted2.ID(), unwanted.ID()},
		Blobs:     [][]byte{bwanted1, bwanted2, bunwanted},
	}

	req := &messages.EntityRequest{
		Nonce:     nonce,
		EntityIDs: []flow.Identifier{wanted1.ID(), wanted2.ID(), unavailable.ID()},
	}

	called := 0
	request := Engine{
		unit:     engine.NewUnit(),
		metrics:  metrics.NewNoopCollector(),
		state:    state,
		items:    make(map[flow.Identifier]*Item),
		requests: make(map[uint64]*messages.EntityRequest),
		selector: filter.HasNodeID(targetID),
		create:   func() flow.Entity { return &flow.Collection{} },
		handle:   func(flow.Identifier, flow.Entity) { called++ },
	}

	request.items[iwanted1.EntityID] = iwanted1
	request.items[iwanted2.EntityID] = iwanted2
	request.items[iunavailable.EntityID] = iunavailable

	request.requests[req.Nonce] = req

	err := request.onEntityResponse(targetID, res)
	assert.NoError(t, err)

	// check that the request was removed
	assert.NotContains(t, request.requests, nonce)

	// check that the provided items were removed
	assert.NotContains(t, request.items, wanted1.ID())
	assert.NotContains(t, request.items, wanted2.ID())

	// check that the missing item is still there
	assert.Contains(t, request.items, unavailable.ID())

	time.Sleep(100 * time.Millisecond)

	// make sure we processed two items
	assert.Equal(t, called, 2)

	// check that the missing items timestamp was reset
	assert.Equal(t, iunavailable.LastRequested, time.Time{})
}

func TestOnEntityIntegrityCheck(t *testing.T) {
	identities := unittest.IdentityListFixture(16)
	targetID := identities[0].NodeID

	final := &protocol.Snapshot{}
	final.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter) flow.IdentityList {
			return identities.Filter(selector)
		},
		nil,
	)

	state := &protocol.State{}
	state.On("Final").Return(final)

	nonce := rand.Uint64()

	wanted := unittest.CollectionFixture(1)
	wanted2 := unittest.CollectionFixture(2)

	now := time.Now()

	iwanted := &Item{
		EntityID:       wanted.ID(),
		LastRequested:  now,
		ExtraSelector:  filter.Any,
		checkIntegrity: true,
	}

	assert.NotEqual(t, wanted, wanted2)

	// prepare payload from different entity
	bwanted, _ := msgpack.Marshal(wanted2)

	res := &messages.EntityResponse{
		Nonce:     nonce,
		EntityIDs: []flow.Identifier{wanted.ID()},
		Blobs:     [][]byte{bwanted},
	}

	req := &messages.EntityRequest{
		Nonce:     nonce,
		EntityIDs: []flow.Identifier{wanted.ID()},
	}

	called := 0
	request := Engine{
		unit:     engine.NewUnit(),
		metrics:  metrics.NewNoopCollector(),
		state:    state,
		items:    make(map[flow.Identifier]*Item),
		requests: make(map[uint64]*messages.EntityRequest),
		selector: filter.HasNodeID(targetID),
		create:   func() flow.Entity { return &flow.Collection{} },
		handle:   func(flow.Identifier, flow.Entity) { called++ },
	}

	request.items[iwanted.EntityID] = iwanted

	request.requests[req.Nonce] = req

	err := request.onEntityResponse(targetID, res)
	assert.NoError(t, err)

	// check that the request was removed
	assert.NotContains(t, request.requests, nonce)

	// check that the provided item wasn't removed
	assert.Contains(t, request.items, wanted.ID())

	// make sure we didn't process items
	assert.Equal(t, 0, called)

	iwanted.checkIntegrity = false
	request.items[iwanted.EntityID] = iwanted
	request.requests[req.Nonce] = req

	err = request.onEntityResponse(targetID, res)
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// make sure we process item without checking integrity
	assert.Equal(t, 1, called)
}

// Verify that the origin should not be checked when ValidateStaking config is set to false
func TestOriginValidation(t *testing.T) {
	identities := unittest.IdentityListFixture(16)
	targetID := identities[0].NodeID
	wrongID := identities[1].NodeID
	meID := identities[3].NodeID

	final := &protocol.Snapshot{}
	final.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter) flow.IdentityList {
			return identities.Filter(selector)
		},
		nil,
	)

	state := &protocol.State{}
	state.On("Final").Return(final)

	me := &module.Local{}

	me.On("NodeID").Return(meID)

	nonce := rand.Uint64()

	wanted := unittest.CollectionFixture(1)

	now := time.Now()

	iwanted := &Item{
		EntityID:       wanted.ID(),
		LastRequested:  now,
		ExtraSelector:  filter.HasNodeID(targetID),
		checkIntegrity: true,
	}

	// prepare payload
	bwanted, _ := msgpack.Marshal(wanted)

	res := &messages.EntityResponse{
		Nonce:     nonce,
		EntityIDs: []flow.Identifier{wanted.ID()},
		Blobs:     [][]byte{bwanted},
	}

	req := &messages.EntityRequest{
		Nonce:     nonce,
		EntityIDs: []flow.Identifier{wanted.ID()},
	}

	network := &network.Network{}
	network.On("Register", mock.Anything, mock.Anything).Return(nil, nil)

	e, err := New(
		zerolog.Nop(),
		metrics.NewNoopCollector(),
		network,
		me,
		state,
		"",
		filter.HasNodeID(targetID),
		func() flow.Entity { return &flow.Collection{} },
	)
	assert.NoError(t, err)

	called := false

	e.WithHandle(func(origin flow.Identifier, _ flow.Entity) {
		// we expect wrong origin to propagate here with validation disabled
		assert.Equal(t, wrongID, origin)
		called = true
	})

	e.items[iwanted.EntityID] = iwanted
	e.requests[req.Nonce] = req

	err = e.onEntityResponse(wrongID, res)
	assert.Error(t, err)
	assert.IsType(t, engine.InvalidInputError{}, err)

	e.cfg.ValidateStaking = false

	err = e.onEntityResponse(wrongID, res)
	assert.NoError(t, err)

	// handler are called async, but this should be extremely quick
	require.Eventually(t, func() bool { return called }, 100*time.Millisecond, 10*time.Millisecond)
}
