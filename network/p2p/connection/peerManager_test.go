package connection_test

import (
	"context"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/internal/p2pfixtures"
	"github.com/onflow/flow-go/network/p2p/connection"
	"github.com/onflow/flow-go/network/p2p/keyutils"
	mockp2p "github.com/onflow/flow-go/network/p2p/mock"
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

func (suite *PeerManagerTestSuite) generatePeerIDs(n int) peer.IDSlice {
	pids := peer.IDSlice{}
	for i := 0; i < n; i++ {
		key := p2pfixtures.NetworkingKeyFixtures(suite.T())
		pid, err := keyutils.PeerIDFromFlowPublicKey(key.PublicKey())
		require.NoError(suite.T(), err)
		pids = append(pids, pid)
	}

	return pids
}

// TestUpdatePeers tests that updatePeers calls the connector with the expected list of ids to connect and disconnect
// from. The tests are cumulative and ordered.
func (suite *PeerManagerTestSuite) TestUpdatePeers() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create some test ids
	pids := suite.generatePeerIDs(10)

	// create the connector mock to check ids requested for connect and disconnect
	peerUpdater := mockp2p.NewPeerUpdater(suite.T())
	peerUpdater.On("UpdatePeers", mock.Anything, mock.AnythingOfType("peer.IDSlice")).
		Run(func(args mock.Arguments) {
			idArg := args[1].(peer.IDSlice)
			assert.ElementsMatch(suite.T(), pids, idArg)
		}).
		Return(nil)

	// create the peer manager (but don't start it)
	pm := connection.NewPeerManager(suite.log, connection.DefaultPeerUpdateInterval, peerUpdater)
	pm.SetPeersProvider(func() peer.IDSlice {
		return pids
	})

	// very first call to updatepeer
	suite.Run("updatePeers only connects to all peers the first time", func() {
		pm.ForceUpdatePeers(ctx)
		peerUpdater.AssertNumberOfCalls(suite.T(), "UpdatePeers", 1)
	})

	// a subsequent call to updatePeers should request a connector.UpdatePeers to existing ids and new ids
	suite.Run("updatePeers connects to old and new peers", func() {
		// create a new id
		newPIDs := suite.generatePeerIDs(1)
		pids = append(pids, newPIDs...)

		pm.ForceUpdatePeers(ctx)
		peerUpdater.AssertNumberOfCalls(suite.T(), "UpdatePeers", 2)
	})

	// when ids are only excluded, connector.UpdatePeers should be called
	suite.Run("updatePeers disconnects from extra peers", func() {
		// delete an id
		pids = removeRandomElement(pids)

		pm.ForceUpdatePeers(ctx)
		peerUpdater.AssertNumberOfCalls(suite.T(), "UpdatePeers", 3)
	})

	// addition and deletion of ids should result in a call to connector.UpdatePeers
	suite.Run("updatePeers connects to new peers and disconnects from extra peers", func() {
		// remove a couple of ids
		pids = removeRandomElement(pids)
		pids = removeRandomElement(pids)

		// add a couple of new ids
		newPIDs := suite.generatePeerIDs(2)
		pids = append(pids, newPIDs...)

		pm.ForceUpdatePeers(ctx)

		peerUpdater.AssertNumberOfCalls(suite.T(), "UpdatePeers", 4)
	})
}

func removeRandomElement(pids peer.IDSlice) peer.IDSlice {
	i := rand.Intn(len(pids))
	pids[i] = pids[len(pids)-1]
	return pids[:len(pids)-1]
}

// TestPeriodicPeerUpdate tests that the peer manager runs periodically
func (suite *PeerManagerTestSuite) TestPeriodicPeerUpdate() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalerCtx := irrecoverable.NewMockSignalerContext(suite.T(), ctx)

	// create some test ids
	pids := suite.generatePeerIDs(10)

	peerUpdater := mockp2p.NewPeerUpdater(suite.T())
	wg := &sync.WaitGroup{} // keeps track of number of calls on `ConnectPeers`
	mu := &sync.Mutex{}     // provides mutual exclusion on calls to `ConnectPeers`
	count := 0
	times := 2 // we expect it to be called twice at least
	wg.Add(times)
	peerUpdater.On("UpdatePeers", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		mu.Lock()
		defer mu.Unlock()

		if count < times {
			count++
			wg.Done()
		}
	}).Return(nil)

	peerUpdateInterval := 10 * time.Millisecond
	pm := connection.NewPeerManager(suite.log, peerUpdateInterval, peerUpdater)
	pm.SetPeersProvider(func() peer.IDSlice {
		return pids
	})

	pm.Start(signalerCtx)
	unittest.RequireCloseBefore(suite.T(), pm.Ready(), 2*time.Second, "could not start peer manager")

	unittest.RequireReturnsBefore(suite.T(), wg.Wait, 2*peerUpdateInterval,
		"UpdatePeers is not running on UpdateIntervals")
}

// TestOnDemandPeerUpdate tests that the peer update can be requested on demand and in between the periodic runs
func (suite *PeerManagerTestSuite) TestOnDemandPeerUpdate() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalerCtx := irrecoverable.NewMockSignalerContext(suite.T(), ctx)

	// create some test ids
	pids := suite.generatePeerIDs(10)

	// chooses peer interval rate deliberately long to capture on demand peer update
	peerUpdateInterval := time.Hour

	// creates mock peerUpdater
	wg := &sync.WaitGroup{} // keeps track of number of calls on `ConnectPeers`
	mu := &sync.Mutex{}     // provides mutual exclusion on calls to `ConnectPeers`
	count := 0
	times := 2 // we expect it to be called twice overall
	wg.Add(1)  // this accounts for one invocation, the other invocation is subsequent
	peerUpdater := mockp2p.NewPeerUpdater(suite.T())
	// captures the first periodic update initiated after start to complete
	peerUpdater.On("UpdatePeers", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		mu.Lock()
		defer mu.Unlock()

		if count < times {
			count++
			wg.Done()
		}
	}).Return(nil)

	pm := connection.NewPeerManager(suite.log, peerUpdateInterval, peerUpdater)
	pm.SetPeersProvider(func() peer.IDSlice {
		return pids
	})
	pm.Start(signalerCtx)
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalerCtx := irrecoverable.NewMockSignalerContext(suite.T(), ctx)

	// create some test ids
	pids := suite.generatePeerIDs(10)

	peerUpdater := mockp2p.NewPeerUpdater(suite.T())
	// connectPeerGate channel gates the return of the peerUpdater
	connectPeerGate := make(chan time.Time)
	defer close(connectPeerGate)

	// choose the periodic interval as a high value so that periodic runs don't interfere with this test
	peerUpdateInterval := time.Hour

	peerUpdater.On("UpdatePeers", mock.Anything, mock.Anything).Return(nil).
		WaitUntil(connectPeerGate) // blocks call for connectPeerGate channel
	pm := connection.NewPeerManager(suite.log, peerUpdateInterval, peerUpdater)
	pm.SetPeersProvider(func() peer.IDSlice {
		return pids
	})

	// start the peer manager
	pm.Start(signalerCtx)

	// this should trigger the first update and which will block on the ConnectPeers to return
	unittest.RequireCloseBefore(suite.T(), pm.Ready(), 2*time.Second, "could not start peer manager")

	// assert that the first update started
	assert.Eventually(suite.T(), func() bool {
		return peerUpdater.AssertNumberOfCalls(suite.T(), "UpdatePeers", 1)
	}, 3*time.Second, 100*time.Millisecond)

	// makes 10 concurrent request for peer update
	unittest.RequireConcurrentCallsReturnBefore(suite.T(), pm.RequestPeerUpdate, 10, time.Second,
		"concurrent peer update requests could not return on time")

	// allow the first periodic update (which should be currently blocked) to finish
	connectPeerGate <- time.Now()

	// assert that only two calls to UpdatePeers were made (one by the periodic update and one by the on-demand update)
	assert.Eventually(suite.T(), func() bool {
		return peerUpdater.AssertNumberOfCalls(suite.T(), "UpdatePeers", 2)
	}, 10*time.Second, 100*time.Millisecond)
}
