package peermgr

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	mock2 "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/network/gossip/libp2p"
	"github.com/onflow/flow-go/network/gossip/libp2p/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type PeerManagerTestSuite struct {
	suite.Suite
	log zerolog.Logger
	ctx context.Context
}

func TestPeerManagerTestSuite(t *testing.T) {
	suite.Run(t, new(PeerManagerTestSuite))
}

func (ts *PeerManagerTestSuite) SetupTest() {
	ts.log = ts.log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	ts.ctx = context.Background()
	libp2p.InitializePeerInfoCache()
}

func (ts *PeerManagerTestSuite) TearDownTest() {
}

func (ts *PeerManagerTestSuite) TestUpdatePeers() {
	currentIDs := unittest.IdentityListFixture(10)
	idProvider := func() (flow.IdentityList, error) {
		return currentIDs, nil
	}
	var extraIDs flow.IdentityList

	connector := new(mock.Connector)
	connector.On("ConnectPeers", ts.ctx, mock2.AnythingOfType("flow.IdentityList")).
		Run(func(args mock2.Arguments) {
			idArg := args[1].(flow.IdentityList)
			assertListsEqual(ts.T(), currentIDs, idArg)
		}).
		Return(nil)
	connector.On("DisconnectPeers", ts.ctx, mock2.AnythingOfType("flow.IdentityList")).
		Run(func(args mock2.Arguments) {
			idArg := args[1].(flow.IdentityList)
			assertListsEqual(ts.T(), extraIDs, idArg)
			assertListsDisjoint(ts.T(), currentIDs, extraIDs)
		}).
		Return(nil)

	pm := NewPeerManager(ts.ctx, ts.log, idProvider, connector)

	ts.Run("updatePeers only connects to all peers the first time", func() {
		pm.updatePeers()
		connector.AssertNumberOfCalls(ts.T(), "ConnectPeers", 1)
		connector.AssertNotCalled(ts.T(), "DisconnectPeers")
	})

	ts.Run("updatePeers connects to old and new peers", func() {
		newIDs := unittest.IdentityListFixture(1)
		currentIDs = append(currentIDs, newIDs...)
		pm.updatePeers()
		connector.AssertNumberOfCalls(ts.T(), "ConnectPeers", 2)
		connector.AssertNotCalled(ts.T(), "DisconnectPeers")
	})

	ts.Run("updatePeers disconnects from extra peers", func() {
		extraIDs = currentIDs.Sample(1)
		currentIDs = currentIDs.Filter(filter.Not(filter.In(extraIDs)))
		pm.updatePeers()
		connector.AssertNumberOfCalls(ts.T(), "ConnectPeers", 3)
		connector.AssertNumberOfCalls(ts.T(), "DisconnectPeers", 1)
	})

	ts.Run("updatePeers connects to new peers and disconnects from extra peers", func() {
		// remove a couple of ids
		extraIDs = currentIDs.Sample(2)
		currentIDs = currentIDs.Filter(filter.Not(filter.In(extraIDs)))

		// add a couple of new ids
		newIDs := unittest.IdentityListFixture(2)
		currentIDs = append(currentIDs, newIDs...)

		pm.updatePeers()

		connector.AssertNumberOfCalls(ts.T(), "ConnectPeers", 4)
		connector.AssertNumberOfCalls(ts.T(), "DisconnectPeers", 2)
	})
}

func (ts *PeerManagerTestSuite) TestPeriodicPeerUpdate() {
	currentIDs := unittest.IdentityListFixture(10)
	idProvider := func() (flow.IdentityList, error) {
		return currentIDs, nil
	}

	connector := new(mock.Connector)
	connector.On("ConnectPeers", ts.ctx, mock2.Anything).Return(nil)
	connector.On("DisconnectPeers", ts.ctx, mock2.Anything).Return(nil)
	pm := NewPeerManager(ts.ctx, ts.log, idProvider, connector)

	PeerUpdateInterval = 5 * time.Millisecond
	err := pm.Start()
	assert.NoError(ts.T(), err)
	assert.Eventually(ts.T(), func() bool {
		return connector.AssertNumberOfCalls(ts.T(), "ConnectPeers", 2)
	}, 2*PeerUpdateInterval + 2 * time.Millisecond, 2*PeerUpdateInterval)
}

func (ts *PeerManagerTestSuite) TestOnDemandPeerUpdate() {
	currentIDs := unittest.IdentityListFixture(10)
	idProvider := func() (flow.IdentityList, error) {
		return currentIDs, nil
	}

	connector := new(mock.Connector)
	connector.On("ConnectPeers", ts.ctx, mock2.Anything).Return(nil)
	connector.On("DisconnectPeers", ts.ctx, mock2.Anything).Return(nil)
	pm := NewPeerManager(ts.ctx, ts.log, idProvider, connector)

	PeerUpdateInterval = time.Hour
	err := pm.Start()
	assert.NoError(ts.T(), err)

	// wait for the first periodic update initiated after start to finish
	assert.Eventually(ts.T(), func() bool {
		return connector.AssertNumberOfCalls(ts.T(), "ConnectPeers", 1)
	}, 2 * time.Millisecond, 1 * time.Millisecond)

	// make a request for peer update
	pm.RequestPeerUpdate()

	// assert that a call to connect to peers is made
	assert.Eventually(ts.T(), func() bool {
		return connector.AssertNumberOfCalls(ts.T(), "ConnectPeers", 2)
	}, 2 * time.Millisecond, 1 * time.Millisecond)
}

func (ts *PeerManagerTestSuite) TestOnDemandPeerUpdate() {
	currentIDs := unittest.IdentityListFixture(10)
	idProvider := func() (flow.IdentityList, error) {
		return currentIDs, nil
	}

	connector := new(mock.Connector)
	connector.On("ConnectPeers", ts.ctx, mock2.Anything).Return(nil)
	connector.On("DisconnectPeers", ts.ctx, mock2.Anything).Return(nil)
	pm := NewPeerManager(ts.ctx, ts.log, idProvider, connector)

	PeerUpdateInterval = time.Hour
	err := pm.Start()
	assert.NoError(ts.T(), err)

	// wait for the first periodic update initiated after start to finish
	assert.Eventually(ts.T(), func() bool {
		return connector.AssertNumberOfCalls(ts.T(), "ConnectPeers", 1)
	}, 2 * time.Millisecond, 1 * time.Millisecond)

	// make a request for peer update
	pm.RequestPeerUpdate()

	// assert that a call to connect to peers is made
	assert.Eventually(ts.T(), func() bool {
		return connector.AssertNumberOfCalls(ts.T(), "ConnectPeers", 2)
	}, 2 * time.Millisecond, 1 * time.Millisecond)
}

func assertListsEqual(t *testing.T, list1, list2 flow.IdentityList) {
	list1 = list1.Order(order.ByNodeIDAsc)
	list2 = list2.Order(order.ByNodeIDAsc)
	assert.Equal(t, list1, list2)
}

func assertListsDisjoint(t *testing.T, list1, list2 flow.IdentityList) {
	common := list1.Filter(filter.In(list2))
	assert.Zero(t, len(common))
}
