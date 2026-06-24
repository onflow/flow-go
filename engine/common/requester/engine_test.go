package requester

import (
	"math/rand"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/vmihailenco/msgpack/v4"
	"go.uber.org/atomic"

	module "github.com/onflow/flow-go/module/mock"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/module/metrics"
	mocknetwork "github.com/onflow/flow-go/network/mock"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestRequesterEngine(t *testing.T) {
	suite.Run(t, new(RequesterEngineSuite))
}

// RequesterEngineSuite is a test suite for the requester engine that holds minimal state for testing.
type RequesterEngineSuite struct {
	suite.Suite
	con   *mocknetwork.Conduit
	final *protocol.Snapshot

	engine *Engine
}

func (s *RequesterEngineSuite) SetupTest() {
	s.final = protocol.NewSnapshot(s.T())

	state := protocol.NewState(s.T())
	state.On("Final").Return(s.final).Maybe()

	me := module.NewLocal(s.T())
	localID := unittest.IdentifierFixture()
	me.On("NodeID").Return(localID).Maybe()

	s.con = mocknetwork.NewConduit(s.T())

	network := mocknetwork.NewEngineRegistry(s.T())
	network.On("Register", mock.Anything, mock.Anything).Return(s.con, nil)
	// CAUTION: the HeroStore fifo queue is NOT BFT. It should be used for messages from trusted sources only!
	// In the requester engine, the injected fifo queue is used to hold [flow.EntityResponse] messages from other
	// potentially byzantine peers. In PRODUCTION, you can NOT use a HeroStore here. However, for testing we
	// use the HeroStore for its better performance (reduced GC load on the maxed-out testing server).
	requestQueue := queue.NewHeroStore(10, unittest.Logger(), metrics.NewNoopCollector())
	var err error
	s.engine, err = New(
		zerolog.Nop(),
		metrics.NewNoopCollector(),
		network,
		me,
		state,
		requestQueue,
		"",
		filter.Any,
		func() flow.Entity { return &flow.Collection{} },
	)
	require.NoError(s.T(), err)
}

// TestEntityByID verifies that calling EntityByID adds the correct entry
// to the requester's internal map of entities to be requested.
func (s *RequesterEngineSuite) TestEntityByID() {
	now := time.Now().UTC()
	entityID := unittest.IdentifierFixture()
	selector := filter.Any
	s.engine.EntityByID(entityID, selector)

	assert.Len(s.T(), s.engine.items, 1)
	item, contains := s.engine.items[entityID]
	if assert.True(s.T(), contains) {
		assert.Equal(s.T(), item.QueryKey, entityID)
		assert.Equal(s.T(), item.NumAttempts, uint(0))
		cutoff := item.LastRequested.Add(item.RetryAfter)
		assert.True(s.T(), cutoff.Before(now)) // make sure we push out immediately
	}
}

// TestDispatchRequestVarious verifies that we only dispatch requests for items
// that are eligible based on their retry policy.
func (s *RequesterEngineSuite) TestDispatchRequestVarious() {
	synctest.Test(s.T(), func(t *testing.T) {
		identities := unittest.IdentityListFixture(16)
		targetID := identities[0].NodeID

		s.final.On("Identities", mock.Anything).Return(
			func(selector flow.IdentityFilter[flow.Identity]) flow.IdentityList {
				return identities.Filter(selector)
			},
			nil,
		)

		cfg := Config{
			BatchInterval:  200 * time.Millisecond,
			BatchThreshold: 999,
			RetryInitial:   100 * time.Millisecond,
			RetryFunction:  RetryLinear(10 * time.Millisecond),
			RetryAttempts:  2,
			RetryMaximum:   300 * time.Millisecond,
		}

		// item that has just been added, should be included
		justAdded := &Request{
			QueryKey:      unittest.IdentifierFixture(),
			NumAttempts:   0,
			LastRequested: time.Time{},
			RetryAfter:    cfg.RetryInitial,
			ExtraSelector: filter.Any,
		}

		// item was tried long time ago, should be included
		triedAnciently := &Request{
			QueryKey:      unittest.IdentifierFixture(),
			NumAttempts:   1,
			LastRequested: time.Now().UTC().Add(-cfg.RetryMaximum),
			RetryAfter:    cfg.RetryFunction(cfg.RetryInitial),
			ExtraSelector: filter.Any,
		}

		// item that was just tried, should be excluded
		triedRecently := &Request{
			QueryKey:      unittest.IdentifierFixture(),
			NumAttempts:   1,
			LastRequested: time.Now().UTC(),
			RetryAfter:    cfg.RetryFunction(cfg.RetryInitial),
		}

		// item was tried twice, should be excluded
		triedTwice := &Request{
			QueryKey:      unittest.IdentifierFixture(),
			NumAttempts:   2,
			LastRequested: time.Time{},
			RetryAfter:    cfg.RetryInitial,
			ExtraSelector: filter.Any,
		}

		items := make(map[flow.Identifier]*Request)
		items[justAdded.QueryKey] = justAdded
		items[triedAnciently.QueryKey] = triedAnciently
		items[triedRecently.QueryKey] = triedRecently
		items[triedTwice.QueryKey] = triedTwice
		s.engine.cfg = cfg
		s.engine.items = items
		s.engine.selector = filter.HasNodeID[flow.Identity](targetID)

		var nonce uint64

		s.con.On("Unicast", mock.Anything, mock.Anything).Run(
			func(args mock.Arguments) {
				request := args.Get(0).(*messages.EntityRequest)
				originID := args.Get(1).(flow.Identifier)
				nonce = request.Nonce
				assert.Equal(s.T(), originID, targetID)
				assert.ElementsMatch(s.T(), request.EntityIDs, []flow.Identifier{justAdded.QueryKey, triedAnciently.QueryKey})
			},
		).Return(nil).Once()

		dispatched, err := s.engine.dispatchRequest()
		require.NoError(s.T(), err)
		require.True(s.T(), dispatched)

		s.engine.mu.Lock()
		assert.Contains(s.T(), s.engine.requests, nonce)
		s.engine.mu.Unlock()

		time.Sleep(cfg.BatchInterval)
		unittest.RequireReturnsBefore(s.T(), synctest.Wait, 5*time.Second, "should return before timeout")

		s.engine.mu.Lock()
		assert.NotContains(s.T(), s.engine.requests, nonce)
		s.engine.mu.Unlock()
	})
}

// TestDispatchRequestBatchSize verifies that we respect the batch size limit when dispatching requests.
func (s *RequesterEngineSuite) TestDispatchRequestBatchSize() {
	batchLimit := uint(16)
	totalItems := uint(99)

	identities := unittest.IdentityListFixture(16)
	s.final.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter[flow.Identity]) flow.IdentityList {
			return identities.Filter(selector)
		},
		nil,
	)

	s.engine.cfg = Config{
		BatchInterval:  24 * time.Hour,
		BatchThreshold: batchLimit,
		RetryInitial:   24 * time.Hour,
		RetryFunction:  RetryLinear(1),
		RetryAttempts:  1,
		RetryMaximum:   24 * time.Hour,
	}

	// item that has just been added, should be included
	for i := uint(0); i < totalItems; i++ {
		item := &Request{
			QueryKey:      unittest.IdentifierFixture(),
			NumAttempts:   0,
			LastRequested: time.Time{},
			RetryAfter:    s.engine.cfg.RetryInitial,
			ExtraSelector: filter.Any,
		}
		s.engine.items[item.QueryKey] = item
	}

	s.con.On("Unicast", mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			request := args.Get(0).(*messages.EntityRequest)
			assert.Len(s.T(), request.EntityIDs, int(batchLimit))
		},
	).Return(nil).Once()

	dispatched, err := s.engine.dispatchRequest()
	require.NoError(s.T(), err)
	require.True(s.T(), dispatched)
}

// TestOnEntityResponseValid verifies that we correctly process a valid entity response, even if
// (i) they only contain a subset of the requested entities.
// (ii) contain extra entities that were not requested.
// Specifically, we expect that only requested entities are processed and removed from the pending items.
// Furthermore, we expect that missing entities are not removed from the pending items, and their
// last requested timestamp is reset to allow for immediate re-requesting.
func (s *RequesterEngineSuite) TestOnEntityResponseValid() {
	identities := unittest.IdentityListFixture(16)
	targetID := identities[0].NodeID

	s.final.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter[flow.Identity]) flow.IdentityList {
			return identities.Filter(selector)
		},
		nil,
	)

	nonce := rand.Uint64()

	wanted1 := unittest.CollectionFixture(1)
	wanted2 := unittest.CollectionFixture(2)
	unavailable := unittest.CollectionFixture(3)
	unwanted := unittest.CollectionFixture(4)

	now := time.Now()

	iwanted1 := &Request{
		QueryKey:      wanted1.ID(),
		LastRequested: now,
		ExtraSelector: filter.Any,
	}
	iwanted2 := &Request{
		QueryKey:      wanted2.ID(),
		LastRequested: now,
		ExtraSelector: filter.Any,
	}
	iunavailable := &Request{
		QueryKey:      unavailable.ID(),
		LastRequested: now,
		ExtraSelector: filter.Any,
	}

	bwanted1, _ := msgpack.Marshal(wanted1)
	bwanted2, _ := msgpack.Marshal(wanted2)
	bunwanted, _ := msgpack.Marshal(unwanted)

	req := &messages.EntityRequest{
		Nonce:     nonce,
		EntityIDs: []flow.Identifier{wanted1.ID(), wanted2.ID(), unavailable.ID()},
	}

	res := &flow.EntityResponse{
		Nonce:     nonce,
		EntityIDs: []flow.Identifier{wanted1.ID(), wanted2.ID(), unwanted.ID()},
		Blobs:     [][]byte{bwanted1, bwanted2, bunwanted},
	}

	done := make(chan struct{})
	called := *atomic.NewUint64(0)
	s.engine.WithHandle(func(flow.Identifier, flow.Entity) {
		if called.Inc() >= 2 {
			close(done)
		}
	})

	s.engine.items[iwanted1.QueryKey] = iwanted1
	s.engine.items[iwanted2.QueryKey] = iwanted2
	s.engine.items[iunavailable.QueryKey] = iunavailable

	s.engine.requests[req.Nonce] = req

	err := s.engine.onEntityResponse(targetID, res)
	assert.NoError(s.T(), err)

	// check that the request was removed
	assert.NotContains(s.T(), s.engine.requests, nonce)

	// check that the provided items were removed
	assert.NotContains(s.T(), s.engine.items, wanted1.ID())
	assert.NotContains(s.T(), s.engine.items, wanted2.ID())

	// check that the missing item is still there
	assert.Contains(s.T(), s.engine.items, unavailable.ID())

	// make sure we processed only two items: this indicates that the unwanted item was ignored
	unittest.AssertClosesBefore(s.T(), done, time.Second)

	// check that the missing items timestamp was reset
	assert.Equal(s.T(), iunavailable.LastRequested, time.Time{})
}

// TestOnEntityIntegrityCheck verifies that
// (i) the structural integrity of received [flow.EntityResponse] messages is properly checked against the hash by which the
// item was requested. This check should be performed if and only if `queryByContentHash` is set to `true` for a requested item.
// (ii) If and only if `queryByContentHash` is `false`, the received entity should not be compared against the requested key.
func (s *RequesterEngineSuite) TestOnEntityIntegrityCheck() {
	identities := unittest.IdentityListFixture(16)
	targetID := identities[0].NodeID

	s.final.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter[flow.Identity]) flow.IdentityList {
			return identities.Filter(selector)
		},
		nil,
	)

	nonce := rand.Uint64()
	wanted := unittest.CollectionFixture(1)
	wanted2 := unittest.CollectionFixture(2)
	now := time.Now()

	iwanted := &Request{
		QueryKey:           wanted.ID(),
		LastRequested:      now,
		ExtraSelector:      filter.Any,
		queryByContentHash: true,
	}

	assert.NotEqual(s.T(), wanted, wanted2) // sanity check

	req := &messages.EntityRequest{
		Nonce:     nonce,
		EntityIDs: []flow.Identifier{wanted.ID()},
	}

	// prepare payload from different entity
	bwanted, _ := msgpack.Marshal(wanted2)
	res := &flow.EntityResponse{
		Nonce:     nonce,
		EntityIDs: []flow.Identifier{wanted.ID()},
		Blobs:     [][]byte{bwanted},
	}

	called := make(chan struct{})
	s.engine.WithHandle(func(flow.Identifier, flow.Entity) { close(called) })

	// PART (i)
	s.engine.items[iwanted.QueryKey] = iwanted
	s.engine.requests[req.Nonce] = req

	err := s.engine.onEntityResponse(targetID, res)
	assert.NoError(s.T(), err)

	// check that the request was removed, because it was answered by the selected provider
	assert.NotContains(s.T(), s.engine.requests, nonce)

	// However, since the provider sent an entity that does not match the requested content hash, the
	// request should not be considered fulfilled. Instead, the item should remain in the pending `items` map.
	assert.Contains(s.T(), s.engine.items, wanted.ID())

	// PART (ii)
	iwanted.queryByContentHash = false
	s.engine.items[iwanted.QueryKey] = iwanted
	s.engine.requests[req.Nonce] = req

	err = s.engine.onEntityResponse(targetID, res)
	assert.NoError(s.T(), err)

	// Since `queryByContentHash` is `false`, the entity should be propagated to the handler,
	// despite its hash not matching the requested key.
	unittest.AssertClosesBefore(s.T(), called, time.Second)
}

// TestOriginValidation verifies that responses from unexpected origins are rejected.
func (s *RequesterEngineSuite) TestOriginValidation() {
	identities := unittest.IdentityListFixture(16)
	targetID := identities[0].NodeID
	wrongID := unittest.IdentifierFixture()

	s.final.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter[flow.Identity]) flow.IdentityList {
			return identities.Filter(selector)
		},
		nil,
	)

	nonce := rand.Uint64()
	wanted := unittest.CollectionFixture(1)
	now := time.Now()
	iwanted := &Request{
		QueryKey:           wanted.ID(),
		LastRequested:      now,
		ExtraSelector:      filter.HasNodeID[flow.Identity](targetID),
		queryByContentHash: true,
	}

	req := &messages.EntityRequest{
		Nonce:     nonce,
		EntityIDs: []flow.Identifier{wanted.ID()},
	}

	// prepare byzantine response: it contains the correct entity, but is from an invalid data source (e.g. ejected peer)
	bwanted, _ := msgpack.Marshal(wanted)
	res := &flow.EntityResponse{
		Nonce:     nonce,
		EntityIDs: []flow.Identifier{wanted.ID()},
		Blobs:     [][]byte{bwanted},
	}

	network := &mocknetwork.EngineRegistry{}
	network.On("Register", mock.Anything, mock.Anything).Return(nil, nil)

	called := make(chan struct{})
	s.engine.WithHandle(func(origin flow.Identifier, _ flow.Entity) {
		// we expect wrong origin to propagate here with validation disabled
		assert.Equal(s.T(), wrongID, origin)
		close(called)
	})

	s.engine.items[iwanted.QueryKey] = iwanted
	s.engine.requests[req.Nonce] = req

	err := s.engine.onEntityResponse(wrongID, res)
	assert.True(s.T(), engine.IsInvalidInputError(err))
}
