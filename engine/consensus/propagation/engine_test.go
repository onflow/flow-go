// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package propagation

import (
	"io/ioutil"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/dapperlabs/flow-go/model/flow"
	mempool "github.com/dapperlabs/flow-go/module/mempool/mock"
	"github.com/dapperlabs/flow-go/module/metrics"
	module "github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestOnCollectionGuarantee(t *testing.T) {

	// create random collection guarantee & identities
	guarantee := unittest.CollectionGuaranteeFixture()
	identities := unittest.IdentityListFixture(1)

	// initialize the mocks and engine
	log := zerolog.New(ioutil.Discard)
	metrics := metrics.NewNoopCollector()
	con := &network.Conduit{}
	state := &protocol.State{}
	ss := &protocol.Snapshot{}
	me := &module.Local{}
	guarantees := &mempool.Guarantees{}

	// set up shared behaviour
	me.On("NodeID").Return(flow.Identifier{})
	state.On("Final").Return(ss)
	ss.On("Identities", mock.Anything, mock.Anything).Return(identities, nil)
	guarantees.On("Size").Return(uint(0))
	con.On("Submit", mock.Anything, mock.Anything).Return(nil)

	e := &Engine{
		log:        log,
		metrics:    metrics,
		mempool:    metrics,
		spans:      metrics,
		con:        con,
		state:      state,
		me:         me,
		guarantees: guarantees,
	}

	// check that we don't propagate if processing fails
	guarantees.On("Add", mock.Anything).Return(false).Once()
	err := e.onGuarantee(flow.ZeroID, guarantee)
	assert.NoError(t, err)
	con.AssertNumberOfCalls(t, "Submit", 0)

	// check that we propagate if collection is new
	guarantees.On("Add", mock.Anything).Return(true).Once()
	err = e.onGuarantee(flow.ZeroID, guarantee)
	assert.NoError(t, err)
	con.AssertNumberOfCalls(t, "Submit", 1)

}
