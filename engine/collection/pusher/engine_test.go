package pusher_test

import (
	"io/ioutil"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine/collection/pusher"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	mempool "github.com/dapperlabs/flow-go/module/mempool/mock"
	metrics "github.com/dapperlabs/flow-go/module/metrics"
	module "github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite

	identities   flow.IdentityList
	state        *protocol.State
	snapshot     *protocol.Snapshot
	conduit      *network.Conduit
	me           *module.Local
	pool         *mempool.Transactions
	collections  *storage.Collections
	transactions *storage.Transactions

	engine *pusher.Engine
}

func (suite *Suite) SetupTest() {
	var err error

	// add some dummy identities so we have one of each role
	suite.identities = unittest.IdentityListFixture(5, unittest.WithAllRoles())
	me := suite.identities.Filter(filter.HasRole(flow.RoleCollection))[0]

	suite.state = new(protocol.State)
	suite.snapshot = new(protocol.Snapshot)
	suite.snapshot.On("Identities", mock.Anything).Return(func(filter flow.IdentityFilter) flow.IdentityList {
		return suite.identities.Filter(filter)
	}, func(filter flow.IdentityFilter) error {
		return nil
	})
	suite.state.On("Final").Return(suite.snapshot)

	metrics := metrics.NewNoopCollector()

	net := new(module.Network)
	suite.conduit = new(network.Conduit)
	net.On("Register", mock.Anything, mock.Anything).Return(suite.conduit, nil)

	suite.me = new(module.Local)
	suite.me.On("NodeID").Return(me.NodeID)

	suite.pool = new(mempool.Transactions)
	suite.collections = new(storage.Collections)
	suite.transactions = new(storage.Transactions)

	suite.engine, err = pusher.New(
		zerolog.New(ioutil.Discard),
		net,
		suite.state,
		metrics,
		metrics,
		suite.me,
		suite.pool,
		suite.collections,
		suite.transactions,
	)
	suite.Require().Nil(err)
}

func TestPusherEngine(t *testing.T) {
	suite.Run(t, new(Suite))
}

// should be able to submit collection guarantees to consensus nodes
func (suite *Suite) TestSubmitCollectionGuarantee() {

	guarantee := unittest.CollectionGuaranteeFixture()

	// should submit the collection to consensus nodes
	suite.conduit.On("Multicast", guarantee, uint(3), mock.Anything).Return(nil)

	msg := &messages.SubmitCollectionGuarantee{
		Guarantee: *guarantee,
	}
	err := suite.engine.ProcessLocal(msg)
	suite.Require().Nil(err)

	suite.conduit.AssertExpectations(suite.T())
}

// should be able to submit collection guarantees to consensus nodes
func (suite *Suite) TestSubmitCollectionGuaranteeNonLocal() {

	guarantee := unittest.CollectionGuaranteeFixture()

	// send from a non-allowed role
	sender := suite.identities.Filter(filter.HasRole(flow.RoleVerification))[0]

	msg := &messages.SubmitCollectionGuarantee{
		Guarantee: *guarantee,
	}
	err := suite.engine.Process(sender.NodeID, msg)
	suite.Require().Error(err)

	suite.conduit.AssertNotCalled(suite.T(), "Multicast", guarantee, uint(3), mock.Anything)
}
