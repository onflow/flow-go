package test

import (
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/go-log"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/network/p2p"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// MutableIdentityTableSuite tests that the networking layer responds correctly
// to changes to the identity table. When nodes are added, we should update our
// topology and accept connections from these new nodes. When nodes are removed
// or ejected we should update our topology and restrict connections from these
// nodes.
type MutableIdentityTableSuite struct {
	suite.Suite
	ConduitWrapper
	nets         []*p2p.Network
	removedNets  []*p2p.Network // networks for node which might have been removed from the mesh
	mws          []*p2p.Middleware
	idRefreshers []*p2p.NodeIDRefresher
	engines      []*MeshEngine
	state        *mockprotocol.State
	snapshot     *mockprotocol.Snapshot
	ids          flow.IdentityList
	logger       zerolog.Logger
}

func TestEpochTransitionTestSuite(t *testing.T) {
	suite.Run(t, new(MutableIdentityTableSuite))
}

func (suite *MutableIdentityTableSuite) SetupTest() {
	rand.Seed(time.Now().UnixNano())
	nodeCount := 4
	suite.logger = zerolog.New(os.Stderr).Level(zerolog.TraceLevel)
	log.SetAllLoggers(log.LevelError)

	// create ids

	//ids, mws := GenerateIDsAndMiddlewares(suite.T(), nodeCount, !DryRun, suite.logger)
	//suite.ids = ids
	//suite.mws = mws

	// setup state related mocks
	final := unittest.BlockHeaderFixture()
	suite.state = new(mockprotocol.State)
	suite.snapshot = new(mockprotocol.Snapshot)
	suite.snapshot.On("Head").Return(&final, nil)
	suite.snapshot.On("Phase").Return(flow.EpochPhaseCommitted, nil)
	suite.snapshot.On("Identities", mock.Anything).Return(
		func(flow.IdentityFilter) flow.IdentityList { return suite.ids },
		func(flow.IdentityFilter) error { return nil })
	suite.state.On("Final").Return(suite.snapshot, nil)

	// all nodes use the same state mock
	states := make([]*mockprotocol.State, nodeCount)
	for i := 0; i < nodeCount; i++ {
		states[i] = suite.state
	}

	suite.addNode(nodeCount)

	// create networks using the mocked state and default topology
	//sms := GenerateSubscriptionManagers(suite.T(), mws)
	//nets := GenerateNetworks(suite.T(), suite.logger, ids, mws, 100, nil, sms, !DryRun)
	//suite.nets = nets

	// generate the refreshers
	//suite.idRefreshers = suite.generateNodeIDRefreshers(nets)

	// generate the engines
	//suite.engines = GenerateEngines(suite.T(), nets)

	// simulate a start of an epoch by signaling a change in the identity table
	for _, n := range suite.idRefreshers {
		n.OnIdentityTableChanged()
	}
	// wait for two lip2p heatbeats for the nodes to discover each other and form the mesh
	time.Sleep(2 * time.Second)
}

// TearDownTest closes the networks within a specified timeout
func (suite *MutableIdentityTableSuite) TearDownTest() {
	stopNetworks(suite.T(), suite.nets, 3*time.Second)
	if len(suite.removedNets) > 0 {
		stopNetworks(suite.T(), suite.removedNets, 3*time.Second)
	}
}

// addNode creates count many new nodes and appends them to the suite state variables
func (suite *MutableIdentityTableSuite) addNode(count int) {
	// create the ids, middlewares and networks
	ids, mws, nets := GenerateIDsMiddlewaresNetworks(suite.T(), count, suite.logger, 100, nil, !DryRun)
	suite.ids = append(suite.ids, ids...)
	suite.mws = append(suite.mws, mws...)
	suite.nets = append(suite.nets, nets...)

	// create the engines for the new nodes
	engines := GenerateEngines(suite.T(), nets)
	suite.engines = append(suite.engines, engines...)

	// create the node refreshers
	idRefereshers := suite.generateNodeIDRefreshers(nets)
	suite.idRefreshers = append(suite.idRefreshers, idRefereshers...)
}

func (suite *MutableIdentityTableSuite) removeNode() ([]*flow.Identity, []*p2p.Middleware, []*p2p.Network, []*MeshEngine, []*p2p.NodeIDRefresher) {
	// choose a random node to remove
	removeIndex := rand.Intn(len(suite.ids) - 1)
	removeIndex = 1
	removedID := suite.ids[removeIndex:removeIndex+1]
	removedMiddleware := suite.mws[removeIndex:removeIndex+1]
	removedNet := suite.nets[removeIndex:removeIndex+1]
	removedEngine := suite.engines[removeIndex:removeIndex+1]
	removedRefresher := suite.idRefreshers[removeIndex:removeIndex+1]

	// adjust state
	suite.ids = append(suite.ids[:removeIndex], suite.ids[removeIndex+1:]...)
	suite.mws = append(suite.mws[:removeIndex], suite.mws[removeIndex+1:]...)
	suite.removedNets = removedNet
	suite.nets = append(suite.nets[:removeIndex], suite.nets[removeIndex+1:]...)
	suite.engines = append(suite.engines[:removeIndex], suite.engines[removeIndex+1:]...)
	suite.idRefreshers = append(suite.idRefreshers[:removeIndex], suite.idRefreshers[removeIndex+1:]...)

	return removedID, removedMiddleware, removedNet, removedEngine, removedRefresher
}

// TestNewNodeAdded tests that when a new node is added to the identity list e.g. on an epoch,
// then it can connect to the network.
func (suite *MutableIdentityTableSuite) TestNewNodeAdded() {

	suite.addNode(1)

	// update IDs for all the networks (simulating an epoch)
	for _, n := range suite.idRefreshers {
		n.OnIdentityTableChanged()
	}

	newID := suite.ids[len(suite.ids) - 1]
	newMiddleware := suite.mws[len(suite.mws) - 1]

	// check if the new node has sufficient connections with the existing nodes
	// if it does, then it has been inducted successfully in the network
	assertConnected(suite.T(), newMiddleware, suite.ids.Filter(filter.Not(filter.HasNodeID(newID.NodeID))))

	// check that all the engines on this new epoch can talk to each other using any of the three networking primitives
	suite.exchangeMessages(suite.ids, suite.engines, nil, nil, suite.Publish)
	suite.exchangeMessages(suite.ids, suite.engines, nil, nil, suite.Multicast)
	suite.exchangeMessages(suite.ids, suite.engines, nil, nil, suite.Unicast)
}

// TestNodeRemoved tests that when an existing node is removed from the identity
// list (ie. as a result of an ejection or transition into an epoch where that node
// has un-staked) then it cannot connect to the network.
func (suite *MutableIdentityTableSuite) TestNodeRemoved() {

	fmt.Printf("\nsuite.ids was : %v\n", suite.ids)
	// removed a node
	removedIDs, removedMiddlewares, _, removedEngines, ref := suite.removeNode()
	fmt.Printf("\nsuite.ids is : %v\n", suite.ids)

	ref[0].OnIdentityTableChanged()

	// update IDs for all the remaining nodes
	// the removed node continues with the old identity list and we don't want to rely on it updating its ids list
	for _, n := range suite.idRefreshers {
		n.OnIdentityTableChanged()
	}

	// assert that the removed node has no connections with any of the other nodes
	assertDisconnected(suite.T(), removedMiddlewares[0], suite.ids)

	// check that all remaining engines can still talk to each other while the ones removed can't
	// using any of the three networking primitives
	suite.exchangeMessages(suite.ids, suite.engines, removedIDs, removedEngines, suite.Publish)
	suite.exchangeMessages(suite.ids, suite.engines, removedIDs, removedEngines, suite.Multicast)
	suite.exchangeMessages(suite.ids, suite.engines, removedIDs, removedEngines, suite.Unicast)
}

// assertConnected checks that the middleware of a node is directly connected
// to at least half of the other nodes.
func assertConnected(t *testing.T, mw *p2p.Middleware, ids flow.IdentityList) {
	threshold := len(ids) / 2
	require.Eventually(t, func() bool {
		connections := 0
		for _, id := range ids {
			connected, err := mw.IsConnected(*id)
			require.NoError(t, err)
			if connected {
				connections++
			}
		}
		return connections >= threshold
	}, 5*time.Second, time.Millisecond*100)
}

// assertDisconnected checks that the middleware of a node is not connected to any of the other nodes specified in the
// ids list
func assertDisconnected(t *testing.T, mw *p2p.Middleware, ids flow.IdentityList) {
	require.Eventually(t, func() bool {
		for _, id := range ids {
			connected, err := mw.IsConnected(*id)
			require.NoError(t, err)
			if connected {
				return false
			}
		}
		return true
	}, 5*time.Second, time.Millisecond*100)
}

// exchangeMessages verifies that allowed engines can successfully exchange messages between them while disallowed
// engines can't using the ConduitSendWrapperFunc network primitive
func (suite *MutableIdentityTableSuite) exchangeMessages(
	allowedIDs flow.IdentityList,
	allowedEngs []*MeshEngine,
	disallowedIDs flow.IdentityList,
	disallowedEngs []*MeshEngine,
	send ConduitSendWrapperFunc) {

	// send a message from each of the allowed engine to the other allowed engines
	for i, allowedEng := range allowedEngs {

		fromID := allowedIDs[i].NodeID
		targetIDs := allowedIDs.Filter(filter.Not(filter.HasNodeID(allowedIDs[i].NodeID)))

		err := suite.sendMessage(fromID, allowedEng, targetIDs, send)
		require.NoError(suite.T(), err)
	}

	// send a message from each of the allowed engine to all of the disallowed engines
	if len(disallowedEngs) > 0 {
		for i, fromEng := range allowedEngs {

			fromID := allowedIDs[i].NodeID
			targetIDs := disallowedIDs

			err := suite.sendMessage(fromID, fromEng, targetIDs, send)
			suite.checkSendError(err, send)
		}
	}

	// send a message from each of the disallowed engine to each of the allowed engines
	for i, fromEng := range disallowedEngs {

		fromID := disallowedIDs[i].NodeID
		targetIDs := allowedIDs

		err := suite.sendMessage(fromID, fromEng, targetIDs, send)
		suite.checkSendError(err, send)
	}

	count := len(allowedEngs)
	expectedMsgCnt := count - 1
	wg := sync.WaitGroup{}
	// fires a goroutine for each of the allowed engine to listen for incoming messages
	for i := range allowedEngs {
		wg.Add(expectedMsgCnt)
		go func(e *MeshEngine) {
			for x := 0; x < expectedMsgCnt; x++ {
				<-e.received
				wg.Done()
			}
		}(allowedEngs[i])
	}

	// assert that all allowed engines received expectedMsgCnt number of messages
	unittest.AssertReturnsBefore(suite.T(), wg.Wait, 5*time.Second)
	// assert that all allowed engines received no other messages
	for i := range allowedEngs {
		assert.Empty(suite.T(), allowedEngs[i].received)
	}

	// assert that the disallowed engines didn't receive any message
	for i, eng := range disallowedEngs {
		unittest.RequireNeverReturnBefore(suite.T(), func() {
			<-eng.received
		}, time.Millisecond, fmt.Sprintf("%s engine should not have recevied message", disallowedIDs[i]))
	}
}

func (suite *MutableIdentityTableSuite) sendMessage(fromID flow.Identifier,
	fromEngine *MeshEngine,
	toIDs flow.IdentityList,
	send ConduitSendWrapperFunc) error {

	primitive := runtime.FuncForPC(reflect.ValueOf(send).Pointer()).Name()
	event := &message.TestMessage{
		Text: fmt.Sprintf("hello from node %s using %s", fromID.String(), primitive),
	}

	return send(event, fromEngine.con, toIDs.NodeIDs()...)
}

func (suite *MutableIdentityTableSuite) checkSendError(err error, send ConduitSendWrapperFunc) {
	primitive := runtime.FuncForPC(reflect.ValueOf(send).Pointer()).Name()
	requireError := strings.Contains(primitive, "Unicast")
	if requireError {
		require.Error(suite.T(), err)
	}
}

func (suite *MutableIdentityTableSuite) generateNodeIDRefreshers(nets []*p2p.Network) []*p2p.NodeIDRefresher {
	refreshers := make([]*p2p.NodeIDRefresher, len(nets))
	for i, net := range nets {
		refreshers[i] = p2p.NewNodeIDRefresher(suite.logger, suite.state, net.SetIDs)
	}
	return refreshers
}
