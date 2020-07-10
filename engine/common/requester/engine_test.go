package requester

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module/metrics"
	network "github.com/dapperlabs/flow-go/network/mock"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
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

	con := &network.Conduit{}
	con.On("Submit", mock.Anything, mock.Anything).Run(
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

	time.Sleep(cfg.RetryInitial)

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

	con := &network.Conduit{}
	con.On("Submit", mock.Anything, mock.Anything).Run(
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
