// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package propagation

import (
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/dapperlabs/flow-go/model/collection"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/identity"
	module "github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
)

func TestOnGuaranteedCollection(t *testing.T) {

	// initialize the mocks and engine
	pool := &module.Mempool{}
	com := &module.Committee{}
	con := &network.Conduit{}
	e := &Engine{
		pool: pool,
		com:  com,
		con:  con,
	}

	// create random collection
	hash := make([]byte, 32)
	_, _ = rand.Read(hash)
	coll := &collection.GuaranteedCollection{Hash: hash}

	// NOTE: as this function relies on two other functions that have their own
	// unit tests, we only set up and check the behaviour that proxies the
	// errors on the called functions, and check only the end result of the
	// happy path; anything else would be redundant

	// check that we propagate if collection is new
	pool.On("Has", mock.Anything).Return(false).Once()
	pool.On("Add", mock.Anything).Return(nil).Once()
	com.On("Me").Return(flow.Identity{}).Once()
	com.On("Select").Return(flow.IdentityList{}).Once()
	con.On("Submit", mock.Anything, mock.Anything).Return(nil).Once()
	err := e.onGuaranteedCollection("", coll)
	assert.Nil(t, err)
	con.AssertExpectations(t)

	// check that we don't propagate if processing fails
	pool.On("Has", mock.Anything).Return(true).Once()
	err = e.onGuaranteedCollection("", coll)
	assert.NotNil(t, err)
	con.AssertExpectations(t)

	// check that we error if propagation fails
	pool.On("Has", mock.Anything).Return(false).Once()
	pool.On("Add", mock.Anything).Return(nil).Once()
	com.On("Me").Return(flow.Identity{}).Once()
	com.On("Select").Return(flow.IdentityList{}).Once()
	con.On("Submit", mock.Anything, mock.Anything).Return(errors.New("dummy")).Once()
	err = e.onGuaranteedCollection("", coll)
	assert.NotNil(t, err)
	con.AssertExpectations(t)
}

func TestProcessGuaranteedCollection(t *testing.T) {

	// initialize mocks and engine
	pool := &module.Mempool{}
	e := Engine{
		pool: pool,
	}

	// generate n random collections
	n := 3
	collections := make([]*collection.GuaranteedCollection, 0, n)
	for i := 0; i < n; i++ {
		hash := make([]byte, 32)
		_, _ = rand.Read(hash)
		coll := &collection.GuaranteedCollection{Hash: hash}
		collections = append(collections, coll)
	}

	// test processing of collections for the first time
	for i, coll := range collections {
		pool.On("Has", coll.Hash).Return(false).Once()
		pool.On("Add", coll).Return(nil).Once()
		err := e.processGuaranteedCollection(coll)
		assert.Nilf(t, err, "collection %d", i)
	}

	// test processing of collections for the second time
	for i, coll := range collections {
		pool.On("Has", coll.Hash).Return(true).Once()
		err := e.processGuaranteedCollection(coll)
		assert.NotNilf(t, err, "collection %d", i)
	}

	// check that we only had the expected calls
	pool.AssertExpectations(t)

	// test processing of collections when the mempool fails
	for i, coll := range collections {
		pool.On("Has", coll.Hash).Return(false)
		pool.On("Add", coll).Return(errors.New("dummy"))
		err := e.processGuaranteedCollection(coll)
		assert.NotNilf(t, err, "collection %d", i)
	}
}

func TestPropagateGuaranteedCollection(t *testing.T) {

	// initialize mocks and engine
	com := &module.Committee{}
	con := &network.Conduit{}
	e := Engine{
		com: com,
		con: con,
	}

	// generate random collections
	n := 3
	collections := make([]*collection.GuaranteedCollection, 0, n)
	for i := 0; i < n; i++ {
		hash := make([]byte, 32)
		_, _ = rand.Read(hash)
		coll := &collection.GuaranteedCollection{Hash: hash}
		collections = append(collections, coll)
	}

	// generate our own node identity
	var identities flow.IdentityList
	me := flow.Identity{
		NodeID:  "me",
		Address: "home",
		Role:    identity.Consensus,
	}
	identities = append(identities, me)

	// generate another 1000 node identities
	var targetIDs []string
	for i := 0; i < 1000; i++ {
		nodeID := fmt.Sprintf("node%d", i)
		address := fmt.Sprintf("address%d", i)
		var role string
		switch rand.Intn(5) {
		case 0:
			role = identity.Collection
		case 1:
			role = identity.Consensus
			targetIDs = append(targetIDs, nodeID)
		case 2:
			role = identity.Execution
		case 3:
			role = identity.Verification
		case 4:
			role = identity.Observation
		}
		id := flow.Identity{
			NodeID:  nodeID,
			Address: address,
			Role:    role,
		}
		identities = append(identities, id)
	}

	// set up the committee mock for good
	com.On("Me").Return(me)
	com.On("Select").Return(identities)

	// test propagating a collection through the network
	for i, coll := range collections {
		params := []interface{}{coll}
		for _, targetID := range targetIDs {
			params = append(params, targetID)
		}
		con.On("Submit", params...).Return(nil).Once()
		err := e.PropagateGuaranteedCollection(coll)
		assert.Nilf(t, err, "collection %d", i)
	}

	// assert the expectations
	con.AssertExpectations(t)
}
