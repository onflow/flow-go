package test

import (
	"fmt"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	golog "github.com/ipfs/go-log"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/network/gossip/libp2p"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

type EpochTransitionTestSuite struct {
	suite.Suite
	ConduitWrapper
	nets              []*libp2p.Network
	mws               []*libp2p.Middleware
	idRefreshers      []*libp2p.NodeIDRefresher
	engines           []*MeshEngine
	state             *protocol.ReadOnlyState
	snapshot          *protocol.Snapshot
	ids               flow.IdentityList
	currentEpoch      uint64 //counter to track the current epoch
	currentEpochPhase flow.EpochPhase
	epochQuery        *mocks.EpochQuery
	logger            zerolog.Logger
}

func TestEpochTransitionTestSuite(t *testing.T) {
	suite.Run(t, new(EpochTransitionTestSuite))
}

func (suite *EpochTransitionTestSuite) SetupTest() {
	rand.Seed(time.Now().UnixNano())
	nodeCount := 10
	suite.logger = zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
	golog.SetAllLoggers(golog.LevelError)

	// create ids
	ids, mws := GenerateIDsAndMiddlewares(suite.T(), nodeCount, !DryRun, suite.logger)
	suite.ids = ids
	suite.mws = mws

	// setup current epoch
	suite.currentEpoch = 0
	suite.currentEpochPhase = flow.EpochPhaseStaking
	suite.epochQuery = mocks.NewEpochQuery(suite.T(), suite.currentEpoch)
	suite.addEpoch(suite.currentEpoch, suite.ids)

	// setup state related mocks
	suite.state = new(protocol.ReadOnlyState)
	suite.snapshot = new(protocol.Snapshot)
	suite.snapshot.On("Identities", testifymock.Anything).Return(ids, nil)
	suite.snapshot.On("Epochs").Return(suite.epochQuery)
	suite.snapshot.On("Phase").Return(
		func() flow.EpochPhase { return suite.currentEpochPhase },
		func() error { return nil },
	)
	suite.state.On("Final").Return(suite.snapshot, nil)

	// all nodes use the same state mock
	states := make([]*protocol.ReadOnlyState, nodeCount)
	for i := 0; i < nodeCount; i++ {
		states[i] = suite.state
	}

	// create networks using the mocked state and default topology
	sms := GenerateSubscriptionManagers(suite.T(), mws)
	nets := GenerateNetworks(suite.T(), suite.logger, ids, mws, 100, nil, sms, !DryRun)
	suite.nets = nets

	// generate the refreshers
	suite.idRefreshers = suite.generateNodeIDRefreshers(nets)

	// generate the engines
	suite.engines = GenerateEngines(suite.T(), nets)
}

// TearDownTest closes the networks within a specified timeout
func (suite *EpochTransitionTestSuite) TearDownTest() {
	stopNetworks(suite.T(), suite.nets, 3*time.Second)
}

// TestNewNodeAdded tests that an additional node in the next epoch gets connected to other nodes and can exchange messages
// in the current epoch
func (suite *EpochTransitionTestSuite) TestNewNodeAdded() {
	// create the id, middleware and network for a new node
	ids, mws, nets := GenerateIDsMiddlewaresNetworks(suite.T(), 1, suite.logger, 100, nil, !DryRun)
	newMiddleware := mws[0]

	newIDs := append(suite.ids, ids...)

	// create a new refresher
	newIDRefresher := suite.generateNodeIDRefreshers(nets)
	newIDRefreshers := append(suite.idRefreshers, newIDRefresher...)

	// create the engine for the new node
	newEngine := GenerateEngines(suite.T(), nets)
	newEngines := append(suite.engines, newEngine...)

	// update epoch query mock to return new IDs for the next epoch
	nextEpoch := suite.currentEpoch + 1
	suite.addEpoch(nextEpoch, newIDs)

	// adjust the epoch phase
	suite.currentEpochPhase = flow.EpochPhaseSetup

	// trigger an epoch phase change for all networks going from flow.EpochPhaseStaking to flow.EpochPhaseSetup
	for _, n := range newIDRefreshers {
		n.EpochSetupPhaseStarted(nextEpoch, nil)
	}

	// check if the new node has sufficient connections with the existing nodes
	// if it does, then it has been inducted successfully in the network
	checkConnectivity(suite.T(), newMiddleware, ids)

	// check that all the engines on this new epoch can talk to each other
	sendMessagesAndVerify(suite.T(), newIDs, newEngines, suite.Publish)
}

// TestNodeRemoved tests that a node that is removed in the next epoch remains connected for the current epoch
func (suite *EpochTransitionTestSuite) TestNodeRemoved() {
	// choose a random node to remove
	removeIndex := rand.Intn(len(suite.ids))
	removedID := suite.ids[removeIndex]
	removedMW := suite.mws[removeIndex]

	// remove the identity at that index from the ids
	newIDs := suite.ids.Filter(filter.Not(filter.HasNodeID(removedID.NodeID)))

	// update epoch query mock to return new IDs for the next epoch
	nextEpoch := suite.currentEpoch + 1
	suite.addEpoch(nextEpoch, newIDs)

	// adjust the epoch phase
	suite.currentEpochPhase = flow.EpochPhaseSetup

	// trigger an epoch phase change for all nodes
	// from flow.EpochPhaseStaking to flow.EpochPhaseSetup
	for _, n := range suite.idRefreshers {
		n.EpochSetupPhaseStarted(nextEpoch, nil)
	}

	// check if the evicted node still has sufficient connections with the existing nodes
	checkConnectivity(suite.T(), removedMW, newIDs)

	// check that all the engines on this new epoch can still talk to each other
	sendMessagesAndVerify(suite.T(), suite.ids, suite.engines, suite.Publish)
}

// checkConnectivity checks that the middleware of a node is directly connected to atleast half of the other nodes
func checkConnectivity(t *testing.T, mw *libp2p.Middleware, ids flow.IdentityList) {
	threshold := len(ids) / 2
	assert.Eventually(t, func() bool {
		connections := 0
		for _, id := range ids {
			connected, err := mw.IsConnected(*id)
			require.NoError(t, err)
			if connected {
				connections++
			}
		}
		return connections >= threshold
	}, 5*time.Second, time.Millisecond)
}

// sendMessagesAndVerify sends a message from each engine to the other engines and verifies that all the messages are
// delivered
func sendMessagesAndVerify(t *testing.T, ids flow.IdentityList, engs []*MeshEngine, send ConduitSendWrapperFunc) {

	// allows nodes to find each other in case of Mulitcast and Publish
	optionalSleep(send)

	count := len(engs)
	expectedMsgCnt := count - 1

	// each node broadcasting a message to all others
	for i, eng := range engs {
		event := &message.TestMessage{
			Text: fmt.Sprintf("hello from node %d", i),
		}
		others := ids.Filter(filter.Not(filter.HasNodeID(ids[i].NodeID))).NodeIDs()
		require.NoError(t, send(event, eng.con, others...))
	}

	wg := sync.WaitGroup{}
	// fires a goroutine for each engine to listen to incoming messages
	for i := range engs {
		wg.Add(expectedMsgCnt)
		go func(e *MeshEngine) {
			for x := 0; x < expectedMsgCnt; x++ {
				<-e.received
				wg.Done()
			}
		}(engs[i])
	}

	unittest.AssertReturnsBefore(t, wg.Wait, 5*time.Second)
}

// addEpoch adds an epoch with the given counter.
func (suite *EpochTransitionTestSuite) addEpoch(counter uint64, ids flow.IdentityList) {
	epoch := new(protocol.Epoch)
	epoch.On("InitialIdentities").Return(ids, nil)
	epoch.On("Counter").Return(counter, nil)
	suite.epochQuery.Add(epoch)
}

func (suite *EpochTransitionTestSuite) generateNodeIDRefreshers(nets []*libp2p.Network) []*libp2p.NodeIDRefresher {
	refreshers := make([]*libp2p.NodeIDRefresher, len(nets))
	for i, net := range nets {
		refreshers[i] = libp2p.NewNodeIDRefresher(suite.logger, suite.state, net.SetIDs)
	}
	return refreshers
}
