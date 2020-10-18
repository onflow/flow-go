package libp2p

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
	err := InitializePeerInfoCache()
	assert.NoError(ts.T(), err)
}

// TestUpdatePeers tests that updatePeers calls the connector with the expected list of ids to connect and disconnect
// from
func (ts *PeerManagerTestSuite) TestUpdatePeers() {

	// create some test ids
	currentIDs := unittest.IdentityListFixture(10)

	// setup a ID provider callback to return currentIDs
	idProvider := func() (flow.IdentityList, error) {
		return currentIDs, nil
	}

	// track IDs that should be disconnect from
	var extraIDs flow.IdentityList

	// create the connector mock to check ids requested for connect and disconnect
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
			// assert that ids passed to disconnect have no id in common with those passed to connect
			assertListsDisjoint(ts.T(), currentIDs, extraIDs)
		}).
		Return(nil)

	// create the peer manager (but don't start it)
	pm := NewPeerManager(ts.ctx, ts.log, idProvider, connector)

	// very first call to updatepeer
	ts.Run("updatePeers only connects to all peers the first time", func() {

		pm.updatePeers()

		connector.AssertNumberOfCalls(ts.T(), "ConnectPeers", 1)
		connector.AssertNotCalled(ts.T(), "DisconnectPeers")
	})

	// a subsequent call to updatepeer should request a connect to existing ids and new ids
	ts.Run("updatePeers connects to old and new peers", func() {
		// create a new id
		newIDs := unittest.IdentityListFixture(1)
		currentIDs = append(currentIDs, newIDs...)

		pm.updatePeers()

		connector.AssertNumberOfCalls(ts.T(), "ConnectPeers", 2)
		connector.AssertNotCalled(ts.T(), "DisconnectPeers")
	})

	// when ids are excluded, they should be requested to be disconnected
	ts.Run("updatePeers disconnects from extra peers", func() {
		// delete an id
		extraIDs = currentIDs.Sample(1)
		currentIDs = currentIDs.Filter(filter.Not(filter.In(extraIDs)))

		pm.updatePeers()

		connector.AssertNumberOfCalls(ts.T(), "ConnectPeers", 3)
		connector.AssertNumberOfCalls(ts.T(), "DisconnectPeers", 1)
	})

	// addition and deletion of ids should result in appropriate connect and disconnect calls
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

// TestPeriodicPeerUpdate tests that the peermanager runs periodically
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
	}, 2*PeerUpdateInterval+2*time.Millisecond, 2*PeerUpdateInterval)
}

// TestOnDemandPeerUpdate tests that the a peer update can be requested on demand and in between the periodic runs
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
	}, 2*time.Millisecond, 1*time.Millisecond)

	// make a request for peer update
	pm.RequestPeerUpdate()

	// assert that a call to connect to peers is made
	assert.Eventually(ts.T(), func() bool {
		return connector.AssertNumberOfCalls(ts.T(), "ConnectPeers", 2)
	}, 2*time.Millisecond, 1*time.Millisecond)
}

// assertListsEqual asserts that two identity list are equal ignoring the order
func assertListsEqual(t *testing.T, list1, list2 flow.IdentityList) {
	list1 = list1.Order(order.ByNodeIDAsc)
	list2 = list2.Order(order.ByNodeIDAsc)
	assert.Equal(t, list1, list2)
}

// assertListsDisjoint asserts that list1 and list2 have no element in common
func assertListsDisjoint(t *testing.T, list1, list2 flow.IdentityList) {
	common := list1.Filter(filter.In(list2))
	assert.Zero(t, len(common))
}
