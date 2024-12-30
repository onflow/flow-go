package pusher_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/collection/pusher"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/mocknetwork"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite

	identities   flow.IdentityList
	state        *protocol.State
	snapshot     *protocol.Snapshot
	conduit      *mocknetwork.Conduit
	me           *module.Local
	collections  *storage.Collections
	transactions *storage.Transactions

	engine *pusher.Engine
}

func (suite *Suite) SetupTest() {
	var err error

	// add some dummy identities so we have one of each role
	suite.identities = unittest.IdentityListFixture(5, unittest.WithAllRoles())
	me := suite.identities.Filter(filter.HasRole[flow.Identity](flow.RoleCollection))[0]

	suite.state = new(protocol.State)
	suite.snapshot = new(protocol.Snapshot)
	suite.snapshot.On("Identities", mock.Anything).Return(func(filter flow.IdentityFilter[flow.Identity]) flow.IdentityList {
		return suite.identities.Filter(filter)
	}, func(filter flow.IdentityFilter[flow.Identity]) error {
		return nil
	})
	suite.state.On("Final").Return(suite.snapshot)

	metrics := metrics.NewNoopCollector()

	net := new(mocknetwork.Network)
	suite.conduit = new(mocknetwork.Conduit)
	net.On("Register", mock.Anything, mock.Anything).Return(suite.conduit, nil)

	suite.me = new(module.Local)
	suite.me.On("NodeID").Return(me.NodeID)

	suite.collections = new(storage.Collections)
	suite.transactions = new(storage.Transactions)

	suite.engine, err = pusher.New(
		zerolog.New(io.Discard),
		net,
		suite.state,
		metrics,
		metrics,
		suite.me,
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
	ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(suite.T(), context.Background())
	suite.engine.Start(ctx)
	defer cancel()
	done := make(chan struct{})

	guarantee := unittest.CollectionGuaranteeFixture()

	// should submit the collection to consensus nodes
	consensus := suite.identities.Filter(filter.HasRole[flow.Identity](flow.RoleConsensus))
	suite.conduit.On("Publish", guarantee, consensus[0].NodeID).
		Run(func(_ mock.Arguments) { close(done) }).Return(nil).Once()

	suite.engine.SubmitCollectionGuarantee(guarantee)

	unittest.RequireCloseBefore(suite.T(), done, time.Second, "message not sent")

	suite.conduit.AssertExpectations(suite.T())
}

// should be able to submit collection guarantees to consensus nodes
func (suite *Suite) TestSubmitCollectionGuaranteeNonLocal() {

	guarantee := unittest.CollectionGuaranteeFixture()

	// verify that pusher.Engine handles any (potentially byzantine) input:
	// A byzantine peer could target the collector node's pusher engine with messages
	// The pusher should discard those and explicitly not get tricked into broadcasting
	// collection guarantees which a byzantine peer might try to inject into the system.
	sender := suite.identities.Filter(filter.HasRole[flow.Identity](flow.RoleVerification))[0]

	err := suite.engine.Process(channels.PushGuarantees, sender.NodeID, guarantee)
	suite.Require().NoError(err)
	suite.conduit.AssertNumberOfCalls(suite.T(), "Multicast", 0)
}
