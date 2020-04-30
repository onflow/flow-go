// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package propagation

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/order"
	mempool "github.com/dapperlabs/flow-go/module/mempool/mock"
	module "github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
)

func TestOnCollectionGuarantee(t *testing.T) {

	// initialize the mocks and engine
	con := &network.Conduit{}
	state := &protocol.State{}
	ss := &protocol.Snapshot{}
	me := &module.Local{}
	pool := &mempool.Guarantees{}
	e := &Engine{
		con:   con,
		state: state,
		me:    me,
		pool:  pool,
	}

	rand.Seed(time.Now().UnixNano())

	// create random collection
	var collID flow.Identifier
	_, _ = rand.Read(collID[:])
	coll := &flow.CollectionGuarantee{CollectionID: collID}

	// NOTE: as this function relies on two other functions that have their own
	// unit tests, we only set up and check the behaviour that proxies the
	// errors on the called functions, and check only the end result of the
	// happy path; anything else would be redundant

	// check that we propagate if collection is new
	pool.On("Add", mock.Anything).Return(nil).Once()
	me.On("NodeID").Return(flow.Identifier{}).Once()
	state.On("Final").Return(ss).Once()
	ss.On("Identities", mock.Anything, mock.Anything).Return(flow.IdentityList{}, nil).Once()
	con.On("Submit", mock.Anything, mock.Anything).Return(nil).Once()
	err := e.onGuarantee(flow.Identifier{}, coll)
	assert.Nil(t, err)
	con.AssertExpectations(t)

	// check that we don't propagate if processing fails
	pool.On("Add", mock.Anything).Return(errors.New("dummy")).Once()
	err = e.onGuarantee(flow.Identifier{}, coll)
	assert.NotNil(t, err)
	con.AssertExpectations(t)

	// check that we error if propagation fails
	pool.On("Add", mock.Anything).Return(nil).Once()
	me.On("NodeID").Return(flow.Identifier{}).Once()
	state.On("Final").Return(ss).Once()
	ss.On("Identities", mock.Anything, mock.Anything).Return(flow.IdentityList{}, nil).Once()
	con.On("Submit", mock.Anything, mock.Anything).Return(errors.New("dummy")).Once()
	err = e.onGuarantee(flow.Identifier{}, coll)
	assert.NotNil(t, err)
	con.AssertExpectations(t)
}

func TestProcessCollectionGuarantee(t *testing.T) {

	// initialize mocks and engine
	pool := &mempool.Guarantees{}
	e := Engine{
		pool: pool,
	}

	rand.Seed(time.Now().UnixNano())
	// generate n random collections
	n := 3
	collections := make([]*flow.CollectionGuarantee, 0, n)
	for i := 0; i < n; i++ {
		var collID flow.Identifier
		_, _ = rand.Read(collID[:])
		coll := &flow.CollectionGuarantee{CollectionID: collID}
		collections = append(collections, coll)
	}

	// test storing of collections for the first time
	for i, coll := range collections {
		pool.On("Add", coll).Return(nil).Once()
		err := e.storeGuarantee(coll)
		assert.Nilf(t, err, "collection %d", i)
	}

	// test storing of collections for the second time
	for i, coll := range collections {
		pool.On("Add", coll).Return(errors.New("dummy")).Once()
		err := e.storeGuarantee(coll)
		assert.NotNilf(t, err, "collection %d", i)
	}

	// check that we only had the expected calls
	pool.AssertExpectations(t)

	// test storing of collections when the mempool fails
	for i, coll := range collections {
		pool.On("Has", coll.ID()).Return(false)
		pool.On("Add", coll).Return(errors.New("dummy"))
		err := e.storeGuarantee(coll)
		assert.NotNilf(t, err, "collection %d", i)
	}
}

func TestPropagateCollectionGuarantee(t *testing.T) {

	rand.Seed(time.Now().UnixNano())

	// initialize mocks and engine
	state := &protocol.State{}
	ss := &protocol.Snapshot{}
	me := &module.Local{}
	con := &network.Conduit{}
	e := Engine{
		state: state,
		con:   con,
		me:    me,
	}

	// generate random collections
	n := 3
	collections := make([]*flow.CollectionGuarantee, 0, n)
	for i := 0; i < n; i++ {
		var collID flow.Identifier
		_, _ = rand.Read(collID[:])
		coll := &flow.CollectionGuarantee{CollectionID: collID}
		collections = append(collections, coll)
	}

	// generate our own node identity
	var identities flow.IdentityList
	identity := &flow.Identity{
		NodeID:  flow.Identifier{0x09, 0x09, 0x09, 0x09},
		Address: "home",
		Role:    flow.RoleConsensus,
	}
	identities = append(identities, identity)

	// generate another 1000 node identities
	var targetIDs flow.IdentityList
	for i := 0; i < 1000; i++ {
		var nodeID flow.Identifier
		_, _ = rand.Read(nodeID[:])
		address := fmt.Sprintf("address%d", i)
		var role flow.Role
		switch rand.Intn(5) {
		case 0:
			role = flow.RoleCollection
		case 1:
			role = flow.RoleConsensus
		case 2:
			role = flow.RoleExecution
		case 3:
			role = flow.RoleVerification
		case 4:
			role = flow.RoleAccess
		}
		identity := &flow.Identity{
			NodeID:  nodeID,
			Address: address,
			Role:    role,
		}
		identities = append(identities, identity)
		if role == flow.RoleConsensus {
			targetIDs = append(targetIDs, identity)
		}
	}

	// sort by node identity
	sort.Slice(identities, func(i int, j int) bool {
		return order.ByNodeIDAsc(identities[i], identities[j])
	})

	// set up the committee mock for good
	me.On("NodeID").Return(identity.NodeID)
	state.On("Final").Return(ss)
	ss.On("Identities", mock.Anything, mock.Anything).Return(targetIDs, nil)

	// test propagating a collection through the network
	for i, coll := range collections {
		params := []interface{}{coll}
		for _, targetID := range targetIDs {
			params = append(params, targetID.NodeID)
		}
		con.On("Submit", params...).Return(nil).Once()
		err := e.propagateGuarantee(coll)
		assert.Nilf(t, err, "collection %d", i)
	}

	// assert the expectations
	con.AssertExpectations(t)
}
