package cohort1

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/go-log"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/observable"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/internal/testutils"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/p2pnet"
	"github.com/onflow/flow-go/network/p2p/p2pnode"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/utils/unittest"
)

// MeshEngineTestSuite evaluates the message delivery functionality for the overlay
// of engines over a complete graph
type MeshEngineTestSuite struct {
	suite.Suite
	testutils.ConduitWrapper                   // used as a wrapper around conduit methods
	networks                 []*p2pnet.Network // used to keep track of the networks
	libp2pNodes              []p2p.LibP2PNode  // used to keep track of the libp2p nodes
	ids                      flow.IdentityList // used to keep track of the identifiers associated with networks
	obs                      chan string       // used to keep track of Protect events tagged by pubsub messages
	cancel                   context.CancelFunc
}

// TestMeshNetTestSuite runs all tests in this test suit
func TestMeshNetTestSuite(t *testing.T) {
	suite.Run(t, new(MeshEngineTestSuite))
}

// SetupTest is executed prior to each test in this test suite. It creates and initializes
// a set of network instances, sets up connection managers, nodes, identities, observables, etc.
// This setup ensures that all necessary configurations are in place before running the tests.
func (suite *MeshEngineTestSuite) SetupTest() {
	// defines total number of nodes in our network (minimum 3 needed to use 1-k messaging)
	const count = 10
	logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
	log.SetAllLoggers(log.LevelError)

	// set up a channel to receive pubsub tags from connManagers of the nodes
	peerChannel := make(chan string)

	// Tag Observables Usage Explanation:
	// The tagsObserver is used to observe connections tagged by pubsub messages. This is instrumental in understanding
	// the connectivity between different peers and verifying the formation of the mesh within this test suite.
	// Issues:
	// - Deviation from Production Code: The usage of tag observables here may not reflect the behavior in the production environment.
	// - Mask Issues in the Production Environment: The observables tied to testing might lead to behaviors or errors that are
	//   masked or not evident within the actual production code.
	// TODO: Evaluate the necessity of tag observables in this test and consider addressing the deviation from production
	// code and potential mask issues. Evaluate the possibility of removing this part eventually.
	ob := tagsObserver{
		tags: peerChannel,
		log:  logger,
	}

	ctx, cancel := context.WithCancel(context.Background())
	suite.cancel = cancel

	signalerCtx := irrecoverable.NewMockSignalerContext(suite.T(), ctx)

	sporkId := unittest.IdentifierFixture()
	libP2PNodes := make([]p2p.LibP2PNode, 0)
	identities := make(flow.IdentityList, 0)
	tagObservables := make([]observable.Observable, 0)
	idProvider := unittest.NewUpdatableIDProvider(flow.IdentityList{})
	defaultFlowConfig, err := config.DefaultConfig()
	require.NoError(suite.T(), err)
	opts := []p2ptest.NodeFixtureParameterOption{p2ptest.WithUnicastHandlerFunc(nil)}

	for i := 0; i < count; i++ {
		connManager, err := testutils.NewTagWatchingConnManager(
			unittest.Logger(),
			metrics.NewNoopCollector(),
			&defaultFlowConfig.NetworkConfig.ConnectionManagerConfig)
		require.NoError(suite.T(), err)

		opts = append(opts, p2ptest.WithConnectionManager(connManager))
		node, nodeId := p2ptest.NodeFixture(suite.T(),
			sporkId,
			suite.T().Name(),
			idProvider,
			opts...)
		libP2PNodes = append(libP2PNodes, node)
		identities = append(identities, &nodeId)
		tagObservables = append(tagObservables, connManager)
	}
	idProvider.SetIdentities(identities)

	suite.libp2pNodes = libP2PNodes
	suite.ids = identities

	suite.networks, _ = testutils.NetworksFixture(suite.T(), sporkId, suite.ids, suite.libp2pNodes)
	// starts the nodes and networks
	testutils.StartNodes(signalerCtx, suite.T(), suite.libp2pNodes)
	for _, net := range suite.networks {
		testutils.StartNetworks(signalerCtx, suite.T(), []network.EngineRegistry{net})
		unittest.RequireComponentsReadyBefore(suite.T(), 1*time.Second, net)
	}

	for _, observableConnMgr := range tagObservables {
		observableConnMgr.Subscribe(&ob)
	}
	suite.obs = peerChannel
}

// TearDownTest closes the networks within a specified timeout
func (suite *MeshEngineTestSuite) TearDownTest() {
	suite.cancel()
	testutils.StopComponents(suite.T(), suite.networks, 3*time.Second)
	testutils.StopComponents(suite.T(), suite.libp2pNodes, 3*time.Second)
}

// TestAllToAll_Publish evaluates the network of mesh engines against allToAllScenario scenario.
// Network instances during this test use their Publish method to disseminate messages.
func (suite *MeshEngineTestSuite) TestAllToAll_Publish() {
	suite.allToAllScenario(suite.Publish)
}

// TestAllToAll_Multicast evaluates the network of mesh engines against allToAllScenario scenario.
// Network instances during this test use their Multicast method to disseminate messages.
func (suite *MeshEngineTestSuite) TestAllToAll_Multicast() {
	suite.allToAllScenario(suite.Multicast)
}

// TestAllToAll_Unicast evaluates the network of mesh engines against allToAllScenario scenario.
// Network instances during this test use their Unicast method to disseminate messages.
func (suite *MeshEngineTestSuite) TestAllToAll_Unicast() {
	suite.allToAllScenario(suite.Unicast)
}

// TestTargetedValidators_Unicast tests if only the intended recipients in a 1-k messaging actually receive the message.
// The messages are disseminated through the Unicast method of conduits.
func (suite *MeshEngineTestSuite) TestTargetedValidators_Unicast() {
	suite.targetValidatorScenario(suite.Unicast)
}

// TestTargetedValidators_Multicast tests if only the intended recipients in a 1-k messaging actually receive the
// message.
// The messages are disseminated through the Multicast method of conduits.
func (suite *MeshEngineTestSuite) TestTargetedValidators_Multicast() {
	suite.targetValidatorScenario(suite.Multicast)
}

// TestTargetedValidators_Publish tests if only the intended recipients in a 1-k messaging actually receive the message.
// The messages are disseminated through the Multicast method of conduits.
func (suite *MeshEngineTestSuite) TestTargetedValidators_Publish() {
	suite.targetValidatorScenario(suite.Publish)
}

// TestMaxMessageSize_Unicast evaluates the messageSizeScenario scenario using
// the Unicast method of conduits.
func (suite *MeshEngineTestSuite) TestMaxMessageSize_Unicast() {
	suite.messageSizeScenario(suite.Unicast, p2pnet.DefaultMaxUnicastMsgSize)
}

// TestMaxMessageSize_Multicast evaluates the messageSizeScenario scenario using
// the Multicast method of conduits.
func (suite *MeshEngineTestSuite) TestMaxMessageSize_Multicast() {
	suite.messageSizeScenario(suite.Multicast, p2pnode.DefaultMaxPubSubMsgSize)
}

// TestMaxMessageSize_Publish evaluates the messageSizeScenario scenario using the
// Publish method of conduits.
func (suite *MeshEngineTestSuite) TestMaxMessageSize_Publish() {
	suite.messageSizeScenario(suite.Publish, p2pnode.DefaultMaxPubSubMsgSize)
}

// TestUnregister_Publish tests that an engine cannot send any message using Publish
// or receive any messages after the conduit is closed
func (suite *MeshEngineTestSuite) TestUnregister_Publish() {
	suite.conduitCloseScenario(suite.Publish)
}

// TestUnregister_Publish tests that an engine cannot send any message using Multicast
// or receive any messages after the conduit is closed
func (suite *MeshEngineTestSuite) TestUnregister_Multicast() {
	suite.conduitCloseScenario(suite.Multicast)
}

// TestUnregister_Publish tests that an engine cannot send any message using Unicast
// or receive any messages after the conduit is closed
func (suite *MeshEngineTestSuite) TestUnregister_Unicast() {
	suite.conduitCloseScenario(suite.Unicast)
}

// allToAllScenario creates a complete mesh of the engines, where each engine x sends a
// "hello from node x" to other engines. It then evaluates the correctness of message
// delivery as well as the content of the messages. This scenario tests the capability of
// the engines to communicate in a fully connected graph, ensuring both the reachability
// of messages and the integrity of their contents.
func (suite *MeshEngineTestSuite) allToAllScenario(send testutils.ConduitSendWrapperFunc) {
	// allows nodes to find each other in case of Mulitcast and Publish
	testutils.OptionalSleep(send)

	// creating engines
	count := len(suite.networks)
	engs := make([]*testutils.MeshEngine, 0)
	wg := sync.WaitGroup{}

	// logs[i][j] keeps the message that node i sends to node j
	logs := make(map[int][]string)
	for i := range suite.networks {
		eng := testutils.NewMeshEngine(suite.Suite.T(), suite.networks[i], count-1, channels.TestNetworkChannel)
		engs = append(engs, eng)
		logs[i] = make([]string, 0)
	}

	// allow nodes to heartbeat and discover each other
	// each node will register ~D protect messages, where D is the default out-degree
	for i := 0; i < pubsub.GossipSubD*count; i++ {
		select {
		case <-suite.obs:
		case <-time.After(8 * time.Second):
			assert.FailNow(suite.T(), "could not receive pubsub tag indicating mesh formed")
		}
	}

	// Each node broadcasting a message to all others
	for i := range suite.networks {
		event := &message.TestMessage{
			Text: fmt.Sprintf("hello from node %v", i),
		}

		// others keeps the identifier of all nodes except ith node
		others := suite.ids.Filter(filter.Not(filter.HasNodeID(suite.ids[i].NodeID))).NodeIDs()
		require.NoError(suite.Suite.T(), send(event, engs[i].Con, others...))
		wg.Add(count - 1)
	}

	// fires a goroutine for each engine that listens to incoming messages
	for i := range suite.networks {
		go func(e *testutils.MeshEngine) {
			for x := 0; x < count-1; x++ {
				<-e.Received
				wg.Done()
			}
		}(engs[i])
	}

	unittest.AssertReturnsBefore(suite.Suite.T(), wg.Wait, 30*time.Second)

	// evaluates that all messages are received
	for index, e := range engs {
		// confirms the number of received messages at each node
		if len(e.Event) != (count - 1) {
			assert.Fail(suite.Suite.T(),
				fmt.Sprintf("Message reception mismatch at node %v. Expected: %v, Got: %v", index, count-1, len(e.Event)))
		}

		for i := 0; i < count-1; i++ {
			assertChannelReceived(suite.T(), e, channels.TestNetworkChannel)
		}

		// extracts failed messages
		receivedIndices, err := extractSenderID(count, e.Event, "hello from node")
		require.NoError(suite.Suite.T(), err)

		for j := 0; j < count; j++ {
			// evaluates self-gossip
			if j == index {
				assert.False(suite.Suite.T(), (receivedIndices)[index], fmt.Sprintf("self gossiped for node %v detected", index))
			}
			// evaluates content
			if !(receivedIndices)[j] {
				assert.False(suite.Suite.T(), (receivedIndices)[index],
					fmt.Sprintf("Message not found in node #%v's messages. Expected: Message from node %v. Got: No message", index, j))
			}
		}
	}
}

// targetValidatorScenario sends a single message from last node to the first half of the nodes
// based on identifiers list.
// It then verifies that only the intended recipients receive the message.
// Message dissemination is done using the send wrapper of conduit.
func (suite *MeshEngineTestSuite) targetValidatorScenario(send testutils.ConduitSendWrapperFunc) {
	// creating engines
	count := len(suite.networks)
	engs := make([]*testutils.MeshEngine, 0)
	wg := sync.WaitGroup{}

	for i := range suite.networks {
		eng := testutils.NewMeshEngine(suite.Suite.T(), suite.networks[i], count-1, channels.TestNetworkChannel)
		engs = append(engs, eng)
	}

	// allow nodes to heartbeat and discover each other
	// each node will register ~D protect messages, where D is the default out-degree
	for i := 0; i < pubsub.GossipSubD*count; i++ {
		select {
		case <-suite.obs:
		case <-time.After(2 * time.Second):
			assert.FailNow(suite.T(), "could not receive pubsub tag indicating mesh formed")
		}
	}

	// choose half of the nodes as target
	allIds := suite.ids.NodeIDs()
	var targets []flow.Identifier
	// create a target list of half of the nodes
	for i := 0; i < len(allIds)/2; i++ {
		targets = append(targets, allIds[i])
	}

	// node 0 broadcasting a message to all targets
	event := &message.TestMessage{
		Text: "hello from node 0",
	}
	require.NoError(suite.Suite.T(), send(event, engs[len(engs)-1].Con, targets...))

	// fires a goroutine for all engines to listens for the incoming message
	for i := 0; i < len(allIds)/2; i++ {
		wg.Add(1)
		go func(e *testutils.MeshEngine) {
			<-e.Received
			wg.Done()
		}(engs[i])
	}

	unittest.AssertReturnsBefore(suite.T(), wg.Wait, 10*time.Second)

	// evaluates that all messages are received
	for index, e := range engs {
		if index < len(engs)/2 {
			assert.Len(suite.Suite.T(), e.Event, 1, fmt.Sprintf("message not received %v", index))
			assertChannelReceived(suite.T(), e, channels.TestNetworkChannel)
		} else {
			assert.Len(suite.Suite.T(), e.Event, 0, fmt.Sprintf("message received when none was expected %v", index))
		}
	}
}

// messageSizeScenario provides a scenario to check if a message of maximum permissible size can be sent
// successfully.
// It broadcasts a message from the first node to all the nodes in the identifiers list using send wrapper function.
func (suite *MeshEngineTestSuite) messageSizeScenario(send testutils.ConduitSendWrapperFunc, size uint) {
	// creating engines
	count := len(suite.networks)
	engs := make([]*testutils.MeshEngine, 0)
	wg := sync.WaitGroup{}

	for i := range suite.networks {
		eng := testutils.NewMeshEngine(suite.Suite.T(), suite.networks[i], count-1, channels.TestNetworkChannel)
		engs = append(engs, eng)
	}

	// allow nodes to heartbeat and discover each other
	// each node will register ~D protect messages per mesh setup, where D is the default out-degree
	for i := 0; i < pubsub.GossipSubD*count; i++ {
		select {
		case <-suite.obs:
		case <-time.After(8 * time.Second):
			assert.FailNow(suite.T(), "could not receive pubsub tag indicating mesh formed")
		}
	}
	// others keeps the identifier of all nodes except node that is sender.
	others := suite.ids.Filter(filter.Not(filter.HasNodeID(suite.ids[0].NodeID))).NodeIDs()

	// generates and sends an event of custom size to the network
	payload := testutils.NetworkPayloadFixture(suite.T(), size)
	event := &message.TestMessage{
		Text: string(payload),
	}

	require.NoError(suite.T(), send(event, engs[0].Con, others...))

	// fires a goroutine for all engines (except sender) to listen for the incoming message
	for _, eng := range engs[1:] {
		wg.Add(1)
		go func(e *testutils.MeshEngine) {
			<-e.Received
			wg.Done()
		}(eng)
	}

	unittest.AssertReturnsBefore(suite.Suite.T(), wg.Wait, 30*time.Second)

	// evaluates that all messages are received
	for index, e := range engs[1:] {
		assert.Len(suite.Suite.T(), e.Event, 1, "message not received by engine %d", index+1)
		assertChannelReceived(suite.T(), e, channels.TestNetworkChannel)
	}
}

// conduitCloseScenario tests after a Conduit is closed, an engine cannot send or receive a message for that channel.
func (suite *MeshEngineTestSuite) conduitCloseScenario(send testutils.ConduitSendWrapperFunc) {

	testutils.OptionalSleep(send)

	// creating engines
	count := len(suite.networks)
	engs := make([]*testutils.MeshEngine, 0)
	wg := sync.WaitGroup{}

	for i := range suite.networks {
		eng := testutils.NewMeshEngine(suite.Suite.T(), suite.networks[i], count-1, channels.TestNetworkChannel)
		engs = append(engs, eng)
	}

	// allow nodes to heartbeat and discover each other
	// each node will register ~D protect messages, where D is the default out-degree
	for i := 0; i < pubsub.GossipSubD*count; i++ {
		select {
		case <-suite.obs:
		case <-time.After(2 * time.Second):
			assert.FailNow(suite.T(), "could not receive pubsub tag indicating mesh formed")
		}
	}

	// unregister a random engine from the test topic by calling close on it's conduit
	unregisterIndex := rand.Intn(count)
	err := engs[unregisterIndex].Con.Close()
	assert.NoError(suite.T(), err)

	// waits enough for peer manager to unsubscribe the node from the topic
	// while libp2p is unsubscribing the node, the topology gets unstable
	// and connections to the node may be refused (although very unlikely).
	time.Sleep(2 * time.Second)

	// each node attempts to broadcast a message to all others
	for i := range suite.networks {
		event := &message.TestMessage{
			Text: fmt.Sprintf("hello from node %v", i),
		}

		// others keeps the identifier of all nodes except ith node and the node that unregistered from the topic.
		// nodes without valid topic registration for a channel will reject messages on that channel via unicast.
		others := suite.ids.Filter(filter.Not(filter.HasNodeID(suite.ids[i].NodeID, suite.ids[unregisterIndex].NodeID))).NodeIDs()

		if i == unregisterIndex {
			// assert that unsubscribed engine cannot publish on that topic
			require.Error(suite.Suite.T(), send(event, engs[i].Con, others...))
			continue
		}

		require.NoError(suite.Suite.T(), send(event, engs[i].Con, others...))
	}

	// fire a goroutine to listen for incoming messages for each engine except for the one which unregistered
	for i := range suite.networks {
		if i == unregisterIndex {
			continue
		}
		wg.Add(1)
		go func(e *testutils.MeshEngine) {
			expectedMsgCnt := count - 2 // count less self and unsubscribed engine
			for x := 0; x < expectedMsgCnt; x++ {
				<-e.Received
			}
			wg.Done()
		}(engs[i])
	}

	// assert every one except the unsubscribed engine received the message
	unittest.AssertReturnsBefore(suite.Suite.T(), wg.Wait, 2*time.Second)

	// assert that the unregistered engine did not receive the message
	unregisteredEng := engs[unregisterIndex]
	assert.Emptyf(suite.T(), unregisteredEng.Received, "unregistered engine received the topic message")
}

// assertChannelReceived asserts that the given channel was received on the given engine
func assertChannelReceived(t *testing.T, e *testutils.MeshEngine, channel channels.Channel) {
	unittest.AssertReturnsBefore(t, func() {
		assert.Equal(t, channel, <-e.Channel)
	}, 100*time.Millisecond)
}

// extractSenderID returns a bool array with the index i true if there is a message from node i in the provided messages.
// enginesNum is the number of engines
// events is the channel of received events
// expectedMsgTxt is the common prefix among all the messages that we expect to receive, for example
// we expect to receive "hello from node x" in this test, and then expectedMsgTxt is "hello form node"
func extractSenderID(enginesNum int, events chan interface{}, expectedMsgTxt string) ([]bool, error) {
	indices := make([]bool, enginesNum)
	expectedMsgSize := len(expectedMsgTxt)
	for i := 0; i < enginesNum-1; i++ {
		var event interface{}
		select {
		case event = <-events:
		default:
			continue
		}
		echo := event.(*message.TestMessage)
		msg := echo.Text
		if len(msg) < expectedMsgSize {
			return nil, fmt.Errorf("invalid message format")
		}
		senderIndex := msg[expectedMsgSize:]
		senderIndex = strings.TrimLeft(senderIndex, " ")
		nodeID, err := strconv.Atoi(senderIndex)
		if err != nil {
			return nil, fmt.Errorf("could not extract the node id from: %v", msg)
		}

		if indices[nodeID] {
			return nil, fmt.Errorf("duplicate message reception: %v", msg)
		}

		if msg == fmt.Sprintf("%s %v", expectedMsgTxt, nodeID) {
			indices[nodeID] = true
		}
	}
	return indices, nil
}
