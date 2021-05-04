package p2p

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/go-log"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
)

type PeerManagerTestSuite struct {
	suite.Suite
	log zerolog.Logger
}

func TestPeerManagerTestSuite(t *testing.T) {
	suite.Run(t, new(PeerManagerTestSuite))
}

func (suite *PeerManagerTestSuite) SetupTest() {
	suite.log = zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
	log.SetAllLoggers(log.LevelError)
}

// TestUpdatePeers tests that updatePeers calls the connector with the expected list of ids to connect and disconnect
// from. The tests are cumulative and ordered.
func (suite *PeerManagerTestSuite) TestUpdatePeers() {

	// create some test ids
	currentIDs := unittest.IdentityListFixture(10)

	// setup a ID provider callback to return currentIDs
	idProvider := func() (flow.IdentityList, error) {
		return currentIDs, nil
	}

	// track IDs that should be disconnected
	var extraIDs flow.IdentityList

	// create the connector mock to check ids requested for connect and disconnect
	connector := new(mocknetwork.Connector)
	connector.On("UpdatePeers", mock.Anything, mock.AnythingOfType("flow.IdentityList")).
		Run(func(args mock.Arguments) {
			idArg := args[1].(flow.IdentityList)
			assertListsEqual(suite.T(), currentIDs, idArg)
		}).
		Return(nil)

	// create the peer manager (but don't start it)
	pm := NewPeerManager(suite.log, idProvider, connector)

	// very first call to updatepeer
	suite.Run("updatePeers only connects to all peers the first time", func() {
		pm.updatePeers()
		connector.AssertNumberOfCalls(suite.T(), "UpdatePeers", 1)
	})

	// a subsequent call to updatePeers should request a connector.UpdatePeers to existing ids and new ids
	suite.Run("updatePeers connects to old and new peers", func() {
		// create a new id
		newIDs := unittest.IdentityListFixture(1)
		currentIDs = append(currentIDs, newIDs...)

		pm.updatePeers()
		connector.AssertNumberOfCalls(suite.T(), "UpdatePeers", 2)
	})

	// when ids are only excluded, connector.UpdatePeers should be called
	suite.Run("updatePeers disconnects from extra peers", func() {
		// delete an id
		extraIDs = currentIDs.Sample(1)
		currentIDs = currentIDs.Filter(filter.Not(filter.In(extraIDs)))

		pm.updatePeers()
		connector.AssertNumberOfCalls(suite.T(), "UpdatePeers", 3)
	})

	// addition and deletion of ids should result in a call to connector.UpdatePeers
	suite.Run("updatePeers connects to new peers and disconnects from extra peers", func() {
		// remove a couple of ids
		extraIDs = currentIDs.Sample(2)
		currentIDs = currentIDs.Filter(filter.Not(filter.In(extraIDs)))

		// add a couple of new ids
		newIDs := unittest.IdentityListFixture(2)
		currentIDs = append(currentIDs, newIDs...)

		pm.updatePeers()

		connector.AssertNumberOfCalls(suite.T(), "UpdatePeers", 4)
	})
}

// TestPeriodicPeerUpdate tests that the peer manager runs periodically
func (suite *PeerManagerTestSuite) TestPeriodicPeerUpdate() {
	currentIDs := unittest.IdentityListFixture(10)
	idProvider := func() (flow.IdentityList, error) {
		return currentIDs, nil
	}

	connector := new(mocknetwork.Connector)
	wg := &sync.WaitGroup{} // keeps track of number of calls on `ConnectPeers`
	mu := &sync.Mutex{}     // provides mutual exclusion on calls to `ConnectPeers`
	count := 0
	times := 2 // we expect it to be called twice at least
	wg.Add(times)
	connector.On("UpdatePeers", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		mu.Lock()
		defer mu.Unlock()

		if count < times {
			count++
			wg.Done()
		}
	}).Return(nil)
	pm := NewPeerManager(suite.log, idProvider, connector)
	PeerUpdateInterval = 5 * time.Millisecond
	unittest.RequireCloseBefore(suite.T(), pm.Ready(), 2*time.Second, "could not start peer manager")

	unittest.RequireReturnsBefore(suite.T(), wg.Wait, 2*PeerUpdateInterval,
		"UpdatePeers is not running on UpdateIntervals")
}

// TestOnDemandPeerUpdate tests that the a peer update can be requested on demand and in between the periodic runs
func (suite *PeerManagerTestSuite) TestOnDemandPeerUpdate() {
	currentIDs := unittest.IdentityListFixture(10)
	idProvider := func() (flow.IdentityList, error) {
		return currentIDs, nil
	}

	// chooses peer interval rate deliberately long to capture on demand peer update
	PeerUpdateInterval = time.Hour

	// creates mock connector
	wg := &sync.WaitGroup{} // keeps track of number of calls on `ConnectPeers`
	mu := &sync.Mutex{}     // provides mutual exclusion on calls to `ConnectPeers`
	count := 0
	times := 2 // we expect it to be called twice overall
	wg.Add(1)  // this accounts for one invocation, the other invocation is subsequent
	connector := new(mocknetwork.Connector)
	// captures the first periodic update initiated after start to complete
	connector.On("UpdatePeers", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		mu.Lock()
		defer mu.Unlock()

		if count < times {
			count++
			wg.Done()
		}
	}).Return(nil)

	pm := NewPeerManager(suite.log, idProvider, connector)
	unittest.RequireCloseBefore(suite.T(), pm.Ready(), 2*time.Second, "could not start peer manager")

	unittest.RequireReturnsBefore(suite.T(), wg.Wait, 1*time.Second,
		"UpdatePeers is not running on startup")

	// makes a request for peer update
	wg.Add(1) // expects a call to `ConnectPeers` by requesting peer update
	pm.RequestPeerUpdate()

	// assert that a call to connect to peers is made
	unittest.RequireReturnsBefore(suite.T(), wg.Wait, 1*time.Second,
		"UpdatePeers is not running on request")
}

// TestConcurrentOnDemandPeerUpdate tests that concurrent on-demand peer update request never block
func (suite *PeerManagerTestSuite) TestConcurrentOnDemandPeerUpdate() {
	currentIDs := unittest.IdentityListFixture(10)
	idProvider := func() (flow.IdentityList, error) {
		return currentIDs, nil
	}

	connector := new(mocknetwork.Connector)
	// connectPeerGate channel gates the return of the connector
	connectPeerGate := make(chan time.Time)
	defer close(connectPeerGate)

	connector.On("UpdatePeers", mock.Anything, mock.Anything).Return(nil).
		WaitUntil(connectPeerGate) // blocks call for connectPeerGate channel
	pm := NewPeerManager(suite.log, idProvider, connector)

	// set the periodic interval to a high value so that periodic runs don't interfere with this test
	PeerUpdateInterval = time.Hour

	// start the peer manager
	// this should trigger the first update and which will block on the ConnectPeers to return
	unittest.RequireCloseBefore(suite.T(), pm.Ready(), 2*time.Second, "could not start peer manager")

	// assert that the first update started
	assert.Eventually(suite.T(), func() bool {
		return connector.AssertNumberOfCalls(suite.T(), "UpdatePeers", 1)
	}, 3*time.Second, 100*time.Millisecond)

	// makes 10 concurrent request for peer update
	unittest.RequireConcurrentCallsReturnBefore(suite.T(), pm.RequestPeerUpdate, 10, time.Second,
		"concurrent peer update requests could not return on time")

	// allow the first periodic update (which should be currently blocked) to finish
	connectPeerGate <- time.Now()

	// assert that only two calls to UpdatePeers were made (one by the periodic update and one by the on-demand update)
	assert.Eventually(suite.T(), func() bool {
		return connector.AssertNumberOfCalls(suite.T(), "UpdatePeers", 2)
	}, 10*time.Second, 100*time.Millisecond)
}

// assertListsEqual asserts that two identity list are equal ignoring the order
func assertListsEqual(t *testing.T, list1, list2 flow.IdentityList) {
	list1 = list1.Sort(order.ByNodeIDAsc)
	list2 = list2.Sort(order.ByNodeIDAsc)
	assert.Equal(t, list1, list2)
}
