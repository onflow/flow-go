package test

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"runtime"
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
	"github.com/onflow/flow-go/network"
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
	testNodes        testNodeList
	removedTestNodes testNodeList // test nodes which might have been removed from the mesh
	state            *mockprotocol.State
	snapshot         *mockprotocol.Snapshot
	logger           zerolog.Logger
	cancels          []context.CancelFunc
}

// testNode encapsulates the node state which includes its identity, middleware, network,
// mesh engine and the id refresher
type testNode struct {
	id     *flow.Identity
	mw     network.Middleware
	net    network.Network
	engine *MeshEngine
}

// testNodeList encapsulates a list of test node and
// has functions to retrieve the different elements of the test nodes in a concurrency safe manner
type testNodeList struct {
	sync.RWMutex
	nodes []testNode
}

func newTestNodeList() testNodeList {
	return testNodeList{}
}

func (t *testNodeList) append(node testNode) {
	t.Lock()
	defer t.Unlock()
	t.nodes = append(t.nodes, node)
}

func (t *testNodeList) remove() testNode {
	t.Lock()
	defer t.Unlock()
	// choose a random node to remove
	i := rand.Intn(len(t.nodes))
	removedNode := t.nodes[i]
	t.nodes = append(t.nodes[:i], t.nodes[i+1:]...)
	return removedNode
}

func (t *testNodeList) ids() flow.IdentityList {
	t.RLock()
	defer t.RUnlock()
	ids := make(flow.IdentityList, len(t.nodes))
	for i, node := range t.nodes {
		ids[i] = node.id
	}
	return ids
}

func (t *testNodeList) lastAdded() (testNode, error) {
	t.RLock()
	defer t.RUnlock()
	if len(t.nodes) > 0 {
		return t.nodes[len(t.nodes)-1], nil
	}
	return testNode{}, fmt.Errorf("node list empty")
}

func (t *testNodeList) engines() []*MeshEngine {
	t.RLock()
	defer t.RUnlock()
	engs := make([]*MeshEngine, len(t.nodes))
	for i, node := range t.nodes {
		engs[i] = node.engine
	}
	return engs
}

func (t *testNodeList) networks() []network.Network {
	t.RLock()
	defer t.RUnlock()
	nets := make([]network.Network, len(t.nodes))
	for i, node := range t.nodes {
		nets[i] = node.net
	}
	return nets
}

func TestEpochTransitionTestSuite(t *testing.T) {
	// Test is flaky, print it in order to avoid the unused linting error
	unittest.SkipUnless(t, unittest.TEST_FLAKY, fmt.Sprintf("test is flaky: %v", &MutableIdentityTableSuite{}))
}

// signalIdentityChanged update IDs for all the current set of nodes (simulating an epoch)
func (suite *MutableIdentityTableSuite) signalIdentityChanged() {
	for _, n := range suite.testNodes.nodes {
		n.mw.UpdateNodeAddresses()
		n.mw.UpdateAllowList()
	}
}

func (suite *MutableIdentityTableSuite) SetupTest() {
	suite.testNodes = newTestNodeList()
	suite.removedTestNodes = newTestNodeList()
	rand.Seed(time.Now().UnixNano())
	nodeCount := 10
	suite.logger = zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
	log.SetAllLoggers(log.LevelError)

	suite.setupStateMock()
	suite.addNodes(nodeCount)

	// simulate a start of an epoch by signaling a change in the identity table
	suite.signalIdentityChanged()

	// wait for two lip2p heatbeats for the nodes to discover each other and form the mesh
	time.Sleep(2 * time.Second)
}

// TearDownTest closes all the networks within a specified timeout
func (suite *MutableIdentityTableSuite) TearDownTest() {
	for _, cancel := range suite.cancels {
		cancel()
	}
	networks := append(suite.testNodes.networks(), suite.removedTestNodes.networks()...)
	stopNetworks(suite.T(), networks, 3*time.Second)
}

// setupStateMock setup state related mocks (all networks share the same state mock)
func (suite *MutableIdentityTableSuite) setupStateMock() {
	final := unittest.BlockHeaderFixture()
	suite.state = new(mockprotocol.State)
	suite.snapshot = new(mockprotocol.Snapshot)
	suite.snapshot.On("Head").Return(&final, nil)
	suite.snapshot.On("Phase").Return(flow.EpochPhaseCommitted, nil)
	// return all the current list of ids for the state.Final.Identities call made by the network
	suite.snapshot.On("Identities", mock.Anything).Return(
		func(flow.IdentityFilter) flow.IdentityList {
			return suite.testNodes.ids()
		},
		func(flow.IdentityFilter) error { return nil })
	suite.state.On("Final").Return(suite.snapshot, nil)
}

// addNodes creates count many new nodes and appends them to the suite state variables
func (suite *MutableIdentityTableSuite) addNodes(count int) {
	ctx, cancel := context.WithCancel(context.Background())

	// create the ids, middlewares and networks
	ids, mws, nets, _ := GenerateIDsMiddlewaresNetworks(
		ctx, suite.T(), count, suite.logger, 100, nil,
	)
	suite.cancels = append(suite.cancels, cancel)

	// create the engines for the new nodes
	engines := GenerateEngines(suite.T(), nets)

	// create the test engines
	for i := 0; i < count; i++ {
		node := testNode{
			id:     ids[i],
			mw:     mws[i],
			net:    nets[i],
			engine: engines[i],
		}
		suite.testNodes.append(node)
	}
}

// removeNode removes a randomly chosen test node from suite.testNodes and adds it to suite.removedTestNodes
func (suite *MutableIdentityTableSuite) removeNode() testNode {
	removedNode := suite.testNodes.remove()
	suite.removedTestNodes.append(removedNode)
	return removedNode
}

// TestNewNodeAdded tests that when a new node is added to the identity list e.g. on an epoch,
// then it can connect to the network.
func (suite *MutableIdentityTableSuite) TestNewNodeAdded() {

	// add a new node the current list of nodes
	suite.addNodes(1)

	newNode, err := suite.testNodes.lastAdded()
	require.NoError(suite.T(), err)
	newID := newNode.id
	newMiddleware := newNode.mw

	suite.logger.Debug().
		Str("new_node", newID.NodeID.String()).
		Msg("added one node")

	// update IDs for all the networks (simulating an epoch)
	suite.signalIdentityChanged()

	ids := suite.testNodes.ids()
	engs := suite.testNodes.engines()

	// check if the new node has sufficient connections with the existing nodes
	// if it does, then it has been inducted successfully in the network
	suite.assertConnected(newMiddleware, ids.Filter(filter.Not(filter.HasNodeID(newID.NodeID))))

	// check that all the engines on this new epoch can talk to each other using any of the three networking primitives
	suite.assertNetworkPrimitives(ids, engs, nil, nil)
}

// TestNodeRemoved tests that when an existing node is removed from the identity
// list (ie. as a result of an ejection or transition into an epoch where that node
// has un-staked) then it cannot connect to the network.
func (suite *MutableIdentityTableSuite) TestNodeRemoved() {

	// removed a node
	removedNode := suite.removeNode()
	removedID := removedNode.id
	removedMiddleware := removedNode.mw
	removedEngine := removedNode.engine

	// update IDs for all the remaining nodes
	// the removed node continues with the old identity list as we don't want to rely on it updating its ids list
	suite.signalIdentityChanged()

	remainingIDs := suite.testNodes.ids()
	remainingEngs := suite.testNodes.engines()

	// assert that the removed node has no connections with any of the other nodes
	suite.assertDisconnected(removedMiddleware, remainingIDs)

	// check that all remaining engines can still talk to each other while the ones removed can't
	// using any of the three networking primitives
	removedIDs := []*flow.Identity{removedID}
	removedEngines := []*MeshEngine{removedEngine}

	// assert that all three network primitives still work
	suite.assertNetworkPrimitives(remainingIDs, remainingEngs, removedIDs, removedEngines)
}

// TestNodesAddedAndRemoved tests that:
// a. a newly added node can exchange messages with the existing nodes
// b. a node that has has been removed cannot exchange messages with the existing nodes
func (suite *MutableIdentityTableSuite) TestNodesAddedAndRemoved() {

	// remove a node
	removedNode := suite.removeNode()
	removedID := removedNode.id
	removedMiddleware := removedNode.mw
	removedEngine := removedNode.engine

	// add a node
	suite.addNodes(1)
	newNode, err := suite.testNodes.lastAdded()
	require.NoError(suite.T(), err)
	newID := newNode.id
	newMiddleware := newNode.mw

	// update all current nodes
	suite.signalIdentityChanged()

	remainingIDs := suite.testNodes.ids()
	remainingEngs := suite.testNodes.engines()

	// check if the new node has sufficient connections with the existing nodes
	suite.assertConnected(newMiddleware, remainingIDs.Filter(filter.Not(filter.HasNodeID(newID.NodeID))))

	// assert that the removed node has no connections with any of the other nodes
	suite.assertDisconnected(removedMiddleware, remainingIDs)

	// check that all remaining engines can still talk to each other while the ones removed can't
	// using any of the three networking primitives
	removedIDs := []*flow.Identity{removedID}
	removedEngines := []*MeshEngine{removedEngine}

	// assert that all three network primitives still work
	suite.assertNetworkPrimitives(remainingIDs, remainingEngs, removedIDs, removedEngines)
}

// assertConnected checks that the middleware of a node is directly connected
// to at least half of the other nodes.
func (suite *MutableIdentityTableSuite) assertConnected(mw network.Middleware, ids flow.IdentityList) {
	t := suite.T()
	threshold := len(ids) / 2
	require.Eventuallyf(t, func() bool {
		connections := 0
		for _, id := range ids {
			connected, err := mw.IsConnected(id.NodeID)
			require.NoError(t, err)
			if connected {
				connections++
			}
		}
		suite.logger.Debug().
			Int("threshold", threshold).
			Int("connections", connections).
			Msg("current connection count")
		return connections >= threshold
	}, 5*time.Second, 100*time.Millisecond, "node is not connected to enough nodes")
}

// assertDisconnected checks that the middleware of a node is not connected to any of the other nodes specified in the
// ids list
func (suite *MutableIdentityTableSuite) assertDisconnected(mw network.Middleware, ids flow.IdentityList) {
	t := suite.T()
	require.Eventuallyf(t, func() bool {
		for _, id := range ids {
			connected, err := mw.IsConnected(id.NodeID)
			require.NoError(t, err)
			if connected {
				return false
			}
		}
		return true
	}, 5*time.Second, 100*time.Millisecond, "node is still connected")
}

// assertNetworkPrimitives asserts that allowed engines can exchange messages between themselves but not with the
// disallowed engines using each of the three network primitives
func (suite *MutableIdentityTableSuite) assertNetworkPrimitives(
	allowedIDs flow.IdentityList,
	allowedEngs []*MeshEngine,
	disallowedIDs flow.IdentityList,
	disallowedEngs []*MeshEngine) {
	suite.Run("Publish", func() {
		suite.exchangeMessages(allowedIDs, allowedEngs, disallowedIDs, disallowedEngs, suite.Publish, false)
	})
	suite.Run("Multicast", func() {
		suite.exchangeMessages(allowedIDs, allowedEngs, disallowedIDs, disallowedEngs, suite.Multicast, false)
	})
	suite.Run("Unicast", func() {
		// unicast send from or to a node that has been evicted should fail with an error
		suite.exchangeMessages(allowedIDs, allowedEngs, disallowedIDs, disallowedEngs, suite.Unicast, true)
	})
}

// exchangeMessages verifies that allowed engines can successfully exchange messages between them while disallowed
// engines can't using the ConduitSendWrapperFunc network primitive
func (suite *MutableIdentityTableSuite) exchangeMessages(
	allowedIDs flow.IdentityList,
	allowedEngs []*MeshEngine,
	disallowedIDs flow.IdentityList,
	disallowedEngs []*MeshEngine,
	send ConduitSendWrapperFunc,
	expectSendErrorForDisallowedIDs bool) {

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
			if expectSendErrorForDisallowedIDs {
				require.Error(suite.T(), err)
			}
		}
	}

	// send a message from each of the disallowed engine to each of the allowed engines
	for i, fromEng := range disallowedEngs {

		fromID := disallowedIDs[i].NodeID
		targetIDs := allowedIDs

		err := suite.sendMessage(fromID, fromEng, targetIDs, send)
		if expectSendErrorForDisallowedIDs {
			require.Error(suite.T(), err)
		}
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
		unittest.RequireNeverClosedWithin(suite.T(), eng.received, time.Millisecond,
			fmt.Sprintf("%s engine should not have recevied message", disallowedIDs[i]))
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
