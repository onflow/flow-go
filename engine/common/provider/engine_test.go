package provider

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module/metrics"
	network "github.com/dapperlabs/flow-go/network/mock"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestOnEntityRequestFull(t *testing.T) {

	entities := make(map[flow.Identifier]flow.Entity)

	identities := unittest.IdentityListFixture(8)
	selector := filter.HasNodeID(identities.NodeIDs()...)
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

	final := &protocol.Snapshot{}
	final.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter) flow.IdentityList {
			return identities.Filter(selector)
		},
		nil,
	)

	state := &protocol.State{}
	state.On("Final").Return(final, nil)

	con := &network.Conduit{}
	con.On("Submit", mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			response := args.Get(0).(*messages.EntityResponse)
			nodeID := args.Get(1).(flow.Identifier)
			assert.Equal(t, nodeID, originID)
			assert.ElementsMatch(t, response.Entities, []flow.Entity{coll1, coll2, coll3, coll4, coll5})
		},
	).Return(nil)

	provide := Engine{
		metrics:  metrics.NewNoopCollector(),
		state:    state,
		con:      con,
		selector: selector,
		retrieve: retrieve,
	}

	request := &messages.EntityRequest{
		Nonce:     rand.Uint64(),
		EntityIDs: []flow.Identifier{coll1.ID(), coll2.ID(), coll3.ID(), coll4.ID(), coll5.ID()},
	}
	err := provide.onEntityRequest(originID, request)
	assert.NoError(t, err, "should not error on full response")

	con.AssertExpectations(t)
}

func TestOnEntityRequestPartial(t *testing.T) {

	entities := make(map[flow.Identifier]flow.Entity)

	identities := unittest.IdentityListFixture(8)
	selector := filter.HasNodeID(identities.NodeIDs()...)
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

	final := &protocol.Snapshot{}
	final.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter) flow.IdentityList {
			return identities.Filter(selector)
		},
		nil,
	)

	state := &protocol.State{}
	state.On("Final").Return(final, nil)

	con := &network.Conduit{}
	con.On("Submit", mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			response := args.Get(0).(*messages.EntityResponse)
			nodeID := args.Get(1).(flow.Identifier)
			assert.Equal(t, nodeID, originID)
			assert.ElementsMatch(t, response.Entities, []flow.Entity{coll1, coll3, coll5})
		},
	).Return(nil)

	provide := Engine{
		metrics:  metrics.NewNoopCollector(),
		state:    state,
		con:      con,
		selector: selector,
		retrieve: retrieve,
	}

	request := &messages.EntityRequest{
		Nonce:     rand.Uint64(),
		EntityIDs: []flow.Identifier{coll1.ID(), coll2.ID(), coll3.ID(), coll4.ID(), coll5.ID()},
	}
	err := provide.onEntityRequest(originID, request)
	assert.NoError(t, err, "should not error on partial response")

	con.AssertExpectations(t)
}

func TestOnEntityRequestEmpty(t *testing.T) {

	entities := make(map[flow.Identifier]flow.Entity)

	identities := unittest.IdentityListFixture(8)
	selector := filter.HasNodeID(identities.NodeIDs()...)
	originID := identities[0].NodeID

	coll1 := unittest.CollectionFixture(1)
	coll2 := unittest.CollectionFixture(2)
	coll3 := unittest.CollectionFixture(3)
	coll4 := unittest.CollectionFixture(4)
	coll5 := unittest.CollectionFixture(5)

	// entities[coll1.ID()] = coll1
	// entities[coll2.ID()] = coll2
	// entities[coll3.ID()] = coll3
	// entities[coll4.ID()] = coll4
	// entities[coll5.ID()] = coll5

	retrieve := func(entityID flow.Identifier) (flow.Entity, error) {
		entity, ok := entities[entityID]
		if !ok {
			return nil, storage.ErrNotFound
		}
		return entity, nil
	}

	final := &protocol.Snapshot{}
	final.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter) flow.IdentityList {
			return identities.Filter(selector)
		},
		nil,
	)

	state := &protocol.State{}
	state.On("Final").Return(final, nil)

	con := &network.Conduit{}
	con.On("Submit", mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			response := args.Get(0).(*messages.EntityResponse)
			nodeID := args.Get(1).(flow.Identifier)
			assert.Equal(t, nodeID, originID)
			assert.Empty(t, response.Entities)
		},
	).Return(nil)

	provide := Engine{
		metrics:  metrics.NewNoopCollector(),
		state:    state,
		con:      con,
		selector: selector,
		retrieve: retrieve,
	}

	request := &messages.EntityRequest{
		Nonce:     rand.Uint64(),
		EntityIDs: []flow.Identifier{coll1.ID(), coll2.ID(), coll3.ID(), coll4.ID(), coll5.ID()},
	}
	err := provide.onEntityRequest(originID, request)
	assert.NoError(t, err, "should not error on empty response")

	con.AssertExpectations(t)
}

func TestOnEntityRequestInvalidOrigin(t *testing.T) {

	entities := make(map[flow.Identifier]flow.Entity)

	identities := unittest.IdentityListFixture(8)
	selector := filter.HasNodeID(identities.NodeIDs()...)
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

	final := &protocol.Snapshot{}
	final.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter) flow.IdentityList {
			return identities.Filter(selector)
		},
		nil,
	)

	state := &protocol.State{}
	state.On("Final").Return(final, nil)

	con := &network.Conduit{}

	provide := Engine{
		metrics:  metrics.NewNoopCollector(),
		state:    state,
		con:      con,
		selector: selector,
		retrieve: retrieve,
	}

	request := &messages.EntityRequest{
		Nonce:     rand.Uint64(),
		EntityIDs: []flow.Identifier{coll1.ID(), coll2.ID(), coll3.ID(), coll4.ID(), coll5.ID()},
	}
	err := provide.onEntityRequest(originID, request)
	assert.Error(t, err, "should error on invalid origin")
	assert.True(t, engine.IsInvalidInputError(err), "should return invalid input error")

	con.AssertExpectations(t)
}
