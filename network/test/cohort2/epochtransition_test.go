package cohort2

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
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/internal/testutils"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/underlay"
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
	testutils.ConduitWrapper
	testNodes        testNodeList
	removedTestNodes testNodeList // test nodes which might have been removed from the mesh
	state            *mockprotocol.State
	snapshot         *mockprotocol.Snapshot
	logger           zerolog.Logger
	cancels          []context.CancelFunc
}

// testNode encapsulates the node state which includes its identity, libp2p node, network,
// mesh engine and the id refresher
type testNode struct {
	id         *flow.Identity
	libp2pNode p2p.LibP2PNode
	network    *underlay.Network
	engine     *testutils.MeshEngine
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

func (t *testNodeList) engines() []*testutils.MeshEngine {
	t.RLock()
	defer t.RUnlock()
	engs := make([]*testutils.MeshEngine, len(t.nodes))
	for i, node := range t.nodes {
		engs[i] = node.engine
	}
	return engs
}

func (t *testNodeList) networks() []network.EngineRegistry {
	t.RLock()
	defer t.RUnlock()
	nets := make([]network.EngineRegistry, len(t.nodes))
	for i, node := range t.nodes {
		nets[i] = node.network
	}
	return nets
}

func (t *testNodeList) libp2pNodes() []p2p.LibP2PNode {
	t.RLock()
	defer t.RUnlock()
	nodes := make([]p2p.LibP2PNode, len(t.nodes))
	for i, node := range t.nodes {
		nodes[i] = node.libp2pNode
	}
	return nodes
}

func TestMutableIdentityTable(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_TODO, "broken test")
	suite.Run(t, new(MutableIdentityTableSuite))
}

// signalIdentityChanged update IDs for all the current set of nodes (simulating an epoch)
func (suite *MutableIdentityTableSuite) signalIdentityChanged() {
	for _, n := range suite.testNodes.nodes {
		n.network.UpdateNodeAddresses()
	}
}

func (suite *MutableIdentityTableSuite) SetupTest() {
	suite.testNodes = newTestNodeList()
	suite.removedTestNodes = newTestNodeList()

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
	testutils.StopComponents(suite.T(), networks, 3*time.Second)
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
	signalerCtx := irrecoverable.NewMockSignalerContext(suite.T(), ctx)
	sporkId := unittest.IdentifierFixture()
	ids, nodes := testutils.LibP2PNodeForNetworkFixture(suite.T(), sporkId, count)
	nets, _ := testutils.NetworksFixture(suite.T(), sporkId, ids, nodes)
	suite.cancels = append(suite.cancels, cancel)

	// starts the nodes and networks
	testutils.StartNodes(signalerCtx, suite.T(), nodes)
	for _, net := range nets {
		testutils.StartNetworks(signalerCtx, suite.T(), []network.EngineRegistry{net})
		unittest.RequireComponentsReadyBefore(suite.T(), 1*time.Second, net)
	}

	// create the engines for the new nodes
	engines := make([]*testutils.MeshEngine, count)
	for i, n := range nets {
		eng := testutils.NewMeshEngine(suite.T(), n, 100, channels.TestNetworkChannel)
		engines[i] = eng
	}

	// create the test engines
	for i := 0; i < count; i++ {
		node := testNode{
			id:         ids[i],
			libp2pNode: nodes[i],
			network:    nets[i],
			engine:     engines[i],
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

	suite.logger.Debug().
		Str("new_node", newID.NodeID.String()).
		Msg("added one node")

	// update IDs for all the networks (simulating an epoch)
	suite.signalIdentityChanged()

	ids := suite.testNodes.ids()
	engs := suite.testNodes.engines()

	// check if the new node has sufficient connections with the existing nodes
	// if it does, then it has been inducted successfully in the network
	suite.assertConnected(newNode.libp2pNode, suite.testNodes.libp2pNodes())

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
	removedEngine := removedNode.engine

	// update IDs for all the remaining nodes
	// the removed node continues with the old identity list as we don't want to rely on it updating its ids list
	suite.signalIdentityChanged()

	remainingIDs := suite.testNodes.ids()
	remainingEngs := suite.testNodes.engines()

	// assert that the removed node has no connections with any of the other nodes
	suite.assertDisconnected(removedNode.libp2pNode, suite.testNodes.libp2pNodes())

	// check that all remaining engines can still talk to each other while the ones removed can't
	// using any of the three networking primitives
	removedIDs := []*flow.Identity{removedID}
	removedEngines := []*testutils.MeshEngine{removedEngine}

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
	removedEngine := removedNode.engine

	// add a node
	suite.addNodes(1)
	newNode, err := suite.testNodes.lastAdded()
	require.NoError(suite.T(), err)

	// update all current nodes
	suite.signalIdentityChanged()

	remainingIDs := suite.testNodes.ids()
	remainingEngs := suite.testNodes.engines()

	// check if the new node has sufficient connections with the existing nodes
	suite.assertConnected(newNode.libp2pNode, suite.testNodes.libp2pNodes())

	// assert that the removed node has no connections with any of the other nodes
	suite.assertDisconnected(removedNode.libp2pNode, suite.testNodes.libp2pNodes())

	// check that all remaining engines can still talk to each other while the ones removed can't
	// using any of the three networking primitives
	removedIDs := []*flow.Identity{removedID}
	removedEngines := []*testutils.MeshEngine{removedEngine}

	// assert that all three network primitives still work
	suite.assertNetworkPrimitives(remainingIDs, remainingEngs, removedIDs, removedEngines)
}

// assertConnected checks that a libp2p node is directly connected
// to at least half of the other nodes.
func (suite *MutableIdentityTableSuite) assertConnected(thisNode p2p.LibP2PNode, allNodes []p2p.LibP2PNode) {
	t := suite.T()
	threshold := len(allNodes) / 2
	require.Eventuallyf(t, func() bool {
		connections := 0
		for _, node := range allNodes {
			if node == thisNode {
				// we don't want to check if a node is connected to itself
				continue
			}
			connected, err := thisNode.IsConnected(node.ID())
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

// assertDisconnected checks that a libp2p node is not connected to any of the other nodes specified in the
// ids list.
func (suite *MutableIdentityTableSuite) assertDisconnected(thisNode p2p.LibP2PNode, allNodes []p2p.LibP2PNode) {
	t := suite.T()
	require.Eventuallyf(t, func() bool {
		for _, node := range allNodes {
			connected, err := thisNode.IsConnected(node.ID())
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
	allowedEngs []*testutils.MeshEngine,
	disallowedIDs flow.IdentityList,
	disallowedEngs []*testutils.MeshEngine) {
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
	allowedEngs []*testutils.MeshEngine,
	disallowedIDs flow.IdentityList,
	disallowedEngs []*testutils.MeshEngine,
	send testutils.ConduitSendWrapperFunc,
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
		go func(e *testutils.MeshEngine) {
			for x := 0; x < expectedMsgCnt; x++ {
				<-e.Received
				wg.Done()
			}
		}(allowedEngs[i])
	}

	// assert that all allowed engines received expectedMsgCnt number of messages
	unittest.AssertReturnsBefore(suite.T(), wg.Wait, 5*time.Second)
	// assert that all allowed engines received no other messages
	for i := range allowedEngs {
		assert.Empty(suite.T(), allowedEngs[i].Received)
	}

	// assert that the disallowed engines didn't receive any message
	for i, eng := range disallowedEngs {
		unittest.RequireNeverClosedWithin(suite.T(), eng.Received, time.Millisecond,
			fmt.Sprintf("%s engine should not have recevied message", disallowedIDs[i]))
	}
}

func (suite *MutableIdentityTableSuite) sendMessage(fromID flow.Identifier,
	fromEngine *testutils.MeshEngine,
	toIDs flow.IdentityList,
	send testutils.ConduitSendWrapperFunc) error {

	primitive := runtime.FuncForPC(reflect.ValueOf(send).Pointer()).Name()
	event := &message.TestMessage{
		Text: fmt.Sprintf("hello from node %s using %s", fromID.String(), primitive),
	}

	return send(event, fromEngine.Con, toIDs.NodeIDs()...)
}
