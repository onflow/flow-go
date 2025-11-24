package requester

import (
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/stretchr/testify/suite"
	"math/rand"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v4"
	"go.uber.org/atomic"

	module "github.com/onflow/flow-go/module/mock"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/metrics"
	mocknetwork "github.com/onflow/flow-go/network/mock"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestRequesterEngine(t *testing.T) {
	suite.Run(t, new(RequesterEngineSuite))
}

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

func (s *RequesterEngineSuite) TestEntityByID() {

	now := time.Now().UTC()

	entityID := unittest.IdentifierFixture()
	selector := filter.Any
	s.engine.EntityByID(entityID, selector)

	assert.Len(s.T(), s.engine.items, 1)
	item, contains := s.engine.items[entityID]
	if assert.True(s.T(), contains) {
		assert.Equal(s.T(), item.EntityID, entityID)
		assert.Equal(s.T(), item.NumAttempts, uint(0))
		cutoff := item.LastRequested.Add(item.RetryAfter)
		assert.True(s.T(), cutoff.Before(now)) // make sure we push out immediately
	}
}

func (s *RequesterEngineSuite) TestDispatchRequestVarious() {

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
			assert.ElementsMatch(s.T(), request.EntityIDs, []flow.Identifier{justAdded.EntityID, triedAnciently.EntityID})
		},
	).Return(nil).Once()

	dispatched, err := s.engine.dispatchRequest()
	require.NoError(s.T(), err)
	require.True(s.T(), dispatched)

	s.engine.mu.Lock()
	assert.Contains(s.T(), s.engine.requests, nonce)
	s.engine.mu.Unlock()

	// TODO: racy/slow test
	time.Sleep(2 * cfg.RetryInitial)

	s.engine.mu.Lock()
	assert.NotContains(s.T(), s.engine.requests, nonce)
	s.engine.mu.Unlock()
}

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
		item := &Item{
			EntityID:      unittest.IdentifierFixture(),
			NumAttempts:   0,
			LastRequested: time.Time{},
			RetryAfter:    s.engine.cfg.RetryInitial,
			ExtraSelector: filter.Any,
		}
		s.engine.items[item.EntityID] = item
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

	res := &flow.EntityResponse{
		Nonce:     nonce,
		EntityIDs: []flow.Identifier{wanted1.ID(), wanted2.ID(), unwanted.ID()},
		Blobs:     [][]byte{bwanted1, bwanted2, bunwanted},
	}

	req := &messages.EntityRequest{
		Nonce:     nonce,
		EntityIDs: []flow.Identifier{wanted1.ID(), wanted2.ID(), unavailable.ID()},
	}

	done := make(chan struct{})
	called := *atomic.NewUint64(0)
	s.engine.WithHandle(func(flow.Identifier, flow.Entity) {
		if called.Inc() >= 2 {
			close(done)
		}
	})

	s.engine.items[iwanted1.EntityID] = iwanted1
	s.engine.items[iwanted2.EntityID] = iwanted2
	s.engine.items[iunavailable.EntityID] = iunavailable

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

	// make sure we processed two items
	unittest.AssertClosesBefore(s.T(), done, time.Second)

	// check that the missing items timestamp was reset
	assert.Equal(s.T(), iunavailable.LastRequested, time.Time{})
}

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

	iwanted := &Item{
		EntityID:       wanted.ID(),
		LastRequested:  now,
		ExtraSelector:  filter.Any,
		checkIntegrity: true,
	}

	assert.NotEqual(s.T(), wanted, wanted2)

	// prepare payload from different entity
	bwanted, _ := msgpack.Marshal(wanted2)

	res := &flow.EntityResponse{
		Nonce:     nonce,
		EntityIDs: []flow.Identifier{wanted.ID()},
		Blobs:     [][]byte{bwanted},
	}

	req := &messages.EntityRequest{
		Nonce:     nonce,
		EntityIDs: []flow.Identifier{wanted.ID()},
	}

	called := make(chan struct{})
	s.engine.WithHandle(func(flow.Identifier, flow.Entity) { close(called) })

	s.engine.items[iwanted.EntityID] = iwanted

	s.engine.requests[req.Nonce] = req

	err := s.engine.onEntityResponse(targetID, res)
	assert.NoError(s.T(), err)

	// check that the request was removed
	assert.NotContains(s.T(), s.engine.requests, nonce)

	// check that the provided item wasn't removed
	assert.Contains(s.T(), s.engine.items, wanted.ID())

	iwanted.checkIntegrity = false
	s.engine.items[iwanted.EntityID] = iwanted
	s.engine.requests[req.Nonce] = req

	err = s.engine.onEntityResponse(targetID, res)
	assert.NoError(s.T(), err)

	// make sure we process item without checking integrity
	unittest.AssertClosesBefore(s.T(), called, time.Second)
}

// Verify that the origin should not be checked when ValidateStaking config is set to false
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

	iwanted := &Item{
		EntityID:       wanted.ID(),
		LastRequested:  now,
		ExtraSelector:  filter.HasNodeID[flow.Identity](targetID),
		checkIntegrity: true,
	}

	// prepare payload
	bwanted, _ := msgpack.Marshal(wanted)

	res := &flow.EntityResponse{
		Nonce:     nonce,
		EntityIDs: []flow.Identifier{wanted.ID()},
		Blobs:     [][]byte{bwanted},
	}

	req := &messages.EntityRequest{
		Nonce:     nonce,
		EntityIDs: []flow.Identifier{wanted.ID()},
	}

	network := &mocknetwork.EngineRegistry{}
	network.On("Register", mock.Anything, mock.Anything).Return(nil, nil)

	called := make(chan struct{})

	s.engine.WithHandle(func(origin flow.Identifier, _ flow.Entity) {
		// we expect wrong origin to propagate here with validation disabled
		assert.Equal(s.T(), wrongID, origin)
		close(called)
	})

	s.engine.items[iwanted.EntityID] = iwanted
	s.engine.requests[req.Nonce] = req

	err := s.engine.onEntityResponse(wrongID, res)
	assert.Error(s.T(), err)
	assert.IsType(s.T(), engine.InvalidInputError{}, err)

	s.engine.cfg.ValidateStaking = false

	err = s.engine.onEntityResponse(wrongID, res)
	assert.NoError(s.T(), err)

	// handler are called async, but this should be extremely quick
	unittest.AssertClosesBefore(s.T(), called, time.Second)
}
