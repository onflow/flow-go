package test

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
	"github.com/stretchr/testify/require"

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

// setupNodesAndNetworks is executed prior to each test in this test suite. It creates and initializes
// a set of network instances, sets up connection managers, nodes, identities, observables, etc.
// This setup ensures that all necessary configurations are in place before running the tests.
func setupNodesAndNetworks(t *testing.T, ctx context.Context) ([]*p2pnet.Network, []p2p.LibP2PNode, flow.IdentityList, chan string) {
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

	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	sporkId := unittest.IdentifierFixture()
	libP2PNodes := make([]p2p.LibP2PNode, 0)
	identities := make(flow.IdentityList, 0)
	tagObservables := make([]observable.Observable, 0)
	idProvider := unittest.NewUpdatableIDProvider(flow.IdentityList{})
	defaultFlowConfig, err := config.DefaultConfig()
	require.NoError(t, err)
	opts := []p2ptest.NodeFixtureParameterOption{p2ptest.WithUnicastHandlerFunc(nil)}

	for i := 0; i < count; i++ {
		connManager, err := testutils.NewTagWatchingConnManager(
			unittest.Logger(),
			metrics.NewNoopCollector(),
			&defaultFlowConfig.NetworkConfig.ConnectionManagerConfig)
		require.NoError(t, err)

		opts = append(opts, p2ptest.WithConnectionManager(connManager))
		node, nodeId := p2ptest.NodeFixture(t,
			sporkId,
			t.Name(),
			idProvider,
			opts...)
		libP2PNodes = append(libP2PNodes, node)
		identities = append(identities, &nodeId)
		tagObservables = append(tagObservables, connManager)
	}
	idProvider.SetIdentities(identities)
	networks, _ := testutils.NetworksFixture(t, sporkId, identities, libP2PNodes)
	// starts the nodes and networks
	testutils.StartNodes(signalerCtx, t, libP2PNodes)
	for _, net := range networks {
		testutils.StartNetworks(signalerCtx, t, []network.EngineRegistry{net})
		unittest.RequireComponentsReadyBefore(t, 5*time.Second, net)
	}

	for _, observableConnMgr := range tagObservables {
		observableConnMgr.Subscribe(&ob)
	}

	ctx10s, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	p2ptest.LetNodesDiscoverEachOther(t, ctx10s, libP2PNodes, identities)

	return networks, libP2PNodes, identities, peerChannel
}

// TestAllToAll_Publish evaluates the network of mesh engines against allToAllScenario scenario.
// Network instances during this test use their Publish method to disseminate messages.
func TestAllToAll_Publish(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	networks, libp2pNodes, ids, obs := setupNodesAndNetworks(t, ctx)
	conduit := testutils.ConduitWrapper{}
	allToAllScenario(t, conduit.Publish, networks, ids, obs)

	defer func() {
		cancel()
		testutils.StopComponents(t, networks, 3*time.Second)
		testutils.StopComponents(t, libp2pNodes, 3*time.Second)
	}()
}

// TestAllToAll_Multicast evaluates the network of mesh engines against allToAllScenario scenario.
// Network instances during this test use their Multicast method to disseminate messages.
func TestAllToAll_Multicast(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	networks, libp2pNodes, ids, obs := setupNodesAndNetworks(t, ctx)
	conduit := testutils.ConduitWrapper{}
	allToAllScenario(t, conduit.Multicast, networks, ids, obs)

	defer func() {
		cancel()
		testutils.StopComponents(t, networks, 3*time.Second)
		testutils.StopComponents(t, libp2pNodes, 3*time.Second)
	}()
}

// TestAllToAll_Unicast evaluates the network of mesh engines against allToAllScenario scenario.
// Network instances during this test use their Unicast method to disseminate messages.
func TestAllToAll_Unicast(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	networks, libp2pNodes, ids, obs := setupNodesAndNetworks(t, ctx)
	conduit := testutils.ConduitWrapper{}
	allToAllScenario(t, conduit.Unicast, networks, ids, obs)

	defer func() {
		cancel()
		testutils.StopComponents(t, networks, 3*time.Second)
		testutils.StopComponents(t, libp2pNodes, 3*time.Second)
	}()
}

// TestTargetedValidators_Unicast tests if only the intended recipients in a 1-k messaging actually receive the message.
// The messages are disseminated through the Unicast method of conduits.
func TestTargetedValidators_Unicast(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	networks, libp2pNodes, ids, obs := setupNodesAndNetworks(t, ctx)
	conduit := testutils.ConduitWrapper{}
	targetValidatorScenario(t, conduit.Unicast, networks, ids, obs)

	defer func() {
		cancel()
		testutils.StopComponents(t, networks, 3*time.Second)
		testutils.StopComponents(t, libp2pNodes, 3*time.Second)
	}()
}

// TestTargetedValidators_Multicast tests if only the intended recipients in a 1-k messaging actually receive the
// message.
// The messages are disseminated through the Multicast method of conduits.
func TestTargetedValidators_Multicast(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_FLAKY, "passes on local, fails on CI")
	ctx, cancel := context.WithCancel(context.Background())
	networks, libp2pNodes, ids, obs := setupNodesAndNetworks(t, ctx)
	conduit := testutils.ConduitWrapper{}
	targetValidatorScenario(t, conduit.Multicast, networks, ids, obs)

	defer func() {
		cancel()
		testutils.StopComponents(t, networks, 3*time.Second)
		testutils.StopComponents(t, libp2pNodes, 3*time.Second)
	}()
}

// TestTargetedValidators_Publish tests if only the intended recipients in a 1-k messaging actually receive the message.
// The messages are disseminated through the Multicast method of conduits.
func TestTargetedValidators_Publish(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_FLAKY, "passes on local, fails on CI")
	
	ctx, cancel := context.WithCancel(context.Background())
	networks, libp2pNodes, ids, obs := setupNodesAndNetworks(t, ctx)
	conduit := testutils.ConduitWrapper{}
	targetValidatorScenario(t, conduit.Publish, networks, ids, obs)

	defer func() {
		cancel()
		testutils.StopComponents(t, networks, 3*time.Second)
		testutils.StopComponents(t, libp2pNodes, 3*time.Second)
	}()
}

// TestMaxMessageSize_Unicast evaluates the messageSizeScenario scenario using
// the Unicast method of conduits.
func TestMaxMessageSize_Unicast(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_FLAKY, "passes on local, fails on CI")
	ctx, cancel := context.WithCancel(context.Background())
	networks, libp2pNodes, ids, obs := setupNodesAndNetworks(t, ctx)
	conduit := testutils.ConduitWrapper{}
	messageSizeScenario(t, p2pnode.DefaultMaxPubSubMsgSize, conduit.Unicast, networks, ids, obs)

	defer func() {
		cancel()
		testutils.StopComponents(t, networks, 3*time.Second)
		testutils.StopComponents(t, libp2pNodes, 3*time.Second)
	}()
}

// TestMaxMessageSize_Multicast evaluates the messageSizeScenario scenario using
// the Multicast method of conduits.
func TestMaxMessageSize_Multicast(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	networks, libp2pNodes, ids, obs := setupNodesAndNetworks(t, ctx)
	conduit := testutils.ConduitWrapper{}
	messageSizeScenario(t, p2pnode.DefaultMaxPubSubMsgSize, conduit.Multicast, networks, ids, obs)

	defer func() {
		cancel()
		testutils.StopComponents(t, networks, 3*time.Second)
		testutils.StopComponents(t, libp2pNodes, 3*time.Second)
	}()
}

// TestMaxMessageSize_Publish evaluates the messageSizeScenario scenario using the
// Publish method of conduits.
func TestMaxMessageSize_Publish(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	networks, libp2pNodes, ids, obs := setupNodesAndNetworks(t, ctx)
	conduit := testutils.ConduitWrapper{}
	messageSizeScenario(t, p2pnode.DefaultMaxPubSubMsgSize, conduit.Publish, networks, ids, obs)

	defer func() {
		cancel()
		testutils.StopComponents(t, networks, 3*time.Second)
		testutils.StopComponents(t, libp2pNodes, 3*time.Second)
	}()
}

// TestUnregister_Publish tests that an engine cannot send any message using Publish
// or receive any messages after the conduit is closed
func TestUnregister_Publish(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	networks, libp2pNodes, ids, obs := setupNodesAndNetworks(t, ctx)
	conduit := testutils.ConduitWrapper{}
	conduitCloseScenario(t, conduit.Publish, networks, ids, obs)

	defer func() {
		cancel()
		testutils.StopComponents(t, networks, 3*time.Second)
		testutils.StopComponents(t, libp2pNodes, 3*time.Second)
	}()
}

// TestUnregister_Publish tests that an engine cannot send any message using Multicast
// or receive any messages after the conduit is closed
func TestUnregister_Multicast(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	networks, libp2pNodes, ids, obs := setupNodesAndNetworks(t, ctx)
	conduit := testutils.ConduitWrapper{}
	conduitCloseScenario(t, conduit.Multicast, networks, ids, obs)

	defer func() {
		cancel()
		testutils.StopComponents(t, networks, 3*time.Second)
		testutils.StopComponents(t, libp2pNodes, 3*time.Second)
	}()
}

// TestUnregister_Publish tests that an engine cannot send any message using Unicast
// or receive any messages after the conduit is closed
func TestUnregister_Unicast(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	networks, libp2pNodes, ids, obs := setupNodesAndNetworks(t, ctx)
	conduit := testutils.ConduitWrapper{}
	conduitCloseScenario(t, conduit.Unicast, networks, ids, obs)

	defer func() {
		cancel()
		testutils.StopComponents(t, networks, 3*time.Second)
		testutils.StopComponents(t, libp2pNodes, 3*time.Second)
	}()
}

// allToAllScenario creates a complete mesh of the engines, where each engine x sends a
// "hello from node x" to other engines. It then evaluates the correctness of message
// delivery as well as the content of the messages. This scenario tests the capability of
// the engines to communicate in a fully connected graph, ensuring both the reachability
// of messages and the integrity of their contents.
func allToAllScenario(t *testing.T, send testutils.ConduitSendWrapperFunc, networks []*p2pnet.Network, ids flow.IdentityList, obs chan string) {
	// allows nodes to find each other in case of Mulitcast and Publish
	testutils.OptionalSleep(send)

	// creating engines
	count := len(networks)
	engs := make([]*testutils.MeshEngine, 0)
	wg := sync.WaitGroup{}

	// logs[i][j] keeps the message that node i sends to node j
	logs := make(map[int][]string)
	for i := range networks {
		eng := testutils.NewMeshEngine(t, networks[i], count-1, channels.TestNetworkChannel)
		engs = append(engs, eng)
		logs[i] = make([]string, 0)
	}

	// allow nodes to heartbeat and discover each other
	// each node will register ~D protect messages, where D is the default out-degree
	for i := 0; i < pubsub.GossipSubD*count; i++ {
		select {
		case <-obs:
		case <-time.After(8 * time.Second):
			require.FailNow(t, "could not receive pubsub tag indicating mesh formed")
		}
	}

	// Each node broadcasting a message to all others
	for i := range networks {
		event := &message.TestMessage{
			Text: fmt.Sprintf("hello from node %v", i),
		}

		// others keeps the identifier of all nodes except ith node
		others := ids.Filter(filter.Not(filter.HasNodeID(ids[i].NodeID))).NodeIDs()
		require.NoError(t, send(event, engs[i].Con, others...))
		wg.Add(count - 1)
	}

	// fires a goroutine for each engine that listens to incoming messages
	for i := range networks {
		go func(e *testutils.MeshEngine) {
			for x := 0; x < count-1; x++ {
				<-e.Received
				wg.Done()
			}
		}(engs[i])
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 30*time.Second, "message not received")

	// evaluates that all messages are received
	for index, e := range engs {
		// confirms the number of received messages at each node
		if len(e.Event) != (count - 1) {
			require.Fail(t,
				fmt.Sprintf("Message reception mismatch at node %v. Expected: %v, Got: %v", index, count-1, len(e.Event)))
		}

		for i := 0; i < count-1; i++ {
			unittest.RequireReturnsBefore(t, func() {
				require.Equal(t, channels.TestNetworkChannel, <-e.Channel)
			}, 1*time.Second, "channel mismatch")
		}

		// extracts failed messages
		receivedIndices, err := extractSenderID(count, e.Event, "hello from node")
		require.NoError(t, err)

		for j := 0; j < count; j++ {
			// evaluates self-gossip
			if j == index {
				require.False(t, (receivedIndices)[index], fmt.Sprintf("self gossiped for node %v detected", index))
			}
			// evaluates content
			if !(receivedIndices)[j] {
				require.False(t, (receivedIndices)[index],
					fmt.Sprintf("Message not found in node #%v's messages. Expected: Message from node %v. Got: No message", index, j))
			}
		}
	}
}

// targetValidatorScenario sends a single message from last node to the first half of the nodes
// based on identifiers list.
// It then verifies that only the intended recipients receive the message.
// Message dissemination is done using the send wrapper of conduit.
func targetValidatorScenario(t *testing.T, send testutils.ConduitSendWrapperFunc, networks []*p2pnet.Network, ids flow.IdentityList, obs chan string) {
	// creating engines
	count := len(networks)
	meshEngines := make([]*testutils.MeshEngine, 0)
	wg := sync.WaitGroup{}

	for i := range networks {
		eng := testutils.NewMeshEngine(t, networks[i], count-1, channels.TestNetworkChannel)
		meshEngines = append(meshEngines, eng)
	}

	// allow nodes to heartbeat and discover each other
	// each node will register ~D protect messages, where D is the default out-degree
	for i := 0; i < pubsub.GossipSubD*count; i++ {
		select {
		case <-obs:
		case <-time.After(2 * time.Second):
			require.FailNow(t, "could not receive pubsub tag indicating mesh formed")
		}
	}

	// choose half of the nodes as target
	allIds := ids.NodeIDs()
	var targets []flow.Identifier
	// create a target list of half of the nodes
	for i := 0; i < len(allIds)/2; i++ {
		targets = append(targets, allIds[i])
	}

	// node 0 broadcasting a message to all targets
	event := &message.TestMessage{
		Text: "hello from node 0",
	}
	require.NoError(t, send(event, meshEngines[len(meshEngines)-1].Con, targets...))

	// fires a goroutine for all engines to listens for the incoming message
	for i := 0; i < len(allIds)/2; i++ {
		wg.Add(1)
		go func(e *testutils.MeshEngine) {
			<-e.Received
			wg.Done()
		}(meshEngines[i])
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 10*time.Second, "message not received")

	// evaluates that all messages are received
	for index, e := range meshEngines {
		if index < len(meshEngines)/2 {
			require.Len(t, e.Event, 1, fmt.Sprintf("message not received %v", index))
			unittest.RequireReturnsBefore(t, func() {
				require.Equal(t, channels.TestNetworkChannel, <-e.Channel)
			}, 1*time.Second, "channel mismatch")
		} else {
			require.Len(t, e.Event, 0, fmt.Sprintf("message received when none was expected %v", index))
		}
	}
}

// messageSizeScenario provides a scenario to check if a message of maximum permissible size can be sent
// successfully.
// It broadcasts a message from the first node to all the nodes in the identifiers list using send wrapper function.
func messageSizeScenario(t *testing.T, size uint, send testutils.ConduitSendWrapperFunc, networks []*p2pnet.Network, ids flow.IdentityList, obs chan string) {
	// creating engines
	count := len(networks)
	meshEngines := make([]*testutils.MeshEngine, 0)
	wg := sync.WaitGroup{}

	for i := range networks {
		eng := testutils.NewMeshEngine(t, networks[i], count-1, channels.TestNetworkChannel)
		meshEngines = append(meshEngines, eng)
	}

	// allow nodes to heartbeat and discover each other
	// each node will register ~D protect messages per mesh setup, where D is the default out-degree
	for i := 0; i < pubsub.GossipSubD*count; i++ {
		select {
		case <-obs:
		case <-time.After(8 * time.Second):
			require.FailNow(t, "could not receive pubsub tag indicating mesh formed")
		}
	}
	// others keeps the identifier of all nodes except node that is sender.
	others := ids.Filter(filter.Not(filter.HasNodeID(ids[0].NodeID))).NodeIDs()

	// generates and sends an event of custom size to the network
	payload := testutils.NetworkPayloadFixture(t, size)
	event := &message.TestMessage{
		Text: string(payload),
	}

	require.NoError(t, send(event, meshEngines[0].Con, others...))

	// fires a goroutine for all engines (except sender) to listen for the incoming message
	for _, eng := range meshEngines[1:] {
		wg.Add(1)
		go func(e *testutils.MeshEngine) {
			<-e.Received
			wg.Done()
		}(eng)
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 30*time.Second, "message not received")

	// evaluates that all messages are received
	for index, e := range meshEngines[1:] {
		require.Len(t, e.Event, 1, "message not received by engine %d", index+1)
		unittest.RequireReturnsBefore(t, func() {
			require.Equal(t, channels.TestNetworkChannel, <-e.Channel)
		}, 1*time.Second, "channel mismatch")
	}
}

// conduitCloseScenario tests after a Conduit is closed, an engine cannot send or receive a message for that channel.
func conduitCloseScenario(t *testing.T, send testutils.ConduitSendWrapperFunc, networks []*p2pnet.Network, ids flow.IdentityList, obs chan string) {
	testutils.OptionalSleep(send)

	// creating engines
	count := len(networks)
	engs := make([]*testutils.MeshEngine, 0)
	wg := sync.WaitGroup{}

	for i := range networks {
		eng := testutils.NewMeshEngine(t, networks[i], count-1, channels.TestNetworkChannel)
		engs = append(engs, eng)
	}

	// allow nodes to heartbeat and discover each other
	// each node will register ~D protect messages, where D is the default out-degree
	for i := 0; i < pubsub.GossipSubD*count; i++ {
		select {
		case <-obs:
		case <-time.After(2 * time.Second):
			require.FailNow(t, "could not receive pubsub tag indicating mesh formed")
		}
	}

	// unregister a random engine from the test topic by calling close on it's conduit
	unregisterIndex := rand.Intn(count)
	err := engs[unregisterIndex].Con.Close()
	require.NoError(t, err)

	// waits enough for peer manager to unsubscribe the node from the topic
	// while libp2p is unsubscribing the node, the topology gets unstable
	// and connections to the node may be refused (although very unlikely).
	time.Sleep(2 * time.Second)

	// each node attempts to broadcast a message to all others
	for i := range networks {
		event := &message.TestMessage{
			Text: fmt.Sprintf("hello from node %v", i),
		}

		// others keeps the identifier of all nodes except ith node and the node that unregistered from the topic.
		// nodes without valid topic registration for a channel will reject messages on that channel via unicast.
		others := ids.Filter(filter.Not(filter.HasNodeID(ids[i].NodeID, ids[unregisterIndex].NodeID))).NodeIDs()

		if i == unregisterIndex {
			// require that unsubscribed engine cannot publish on that topic
			require.Error(t, send(event, engs[i].Con, others...))
			continue
		}

		require.NoError(t, send(event, engs[i].Con, others...))
	}

	// fire a goroutine to listen for incoming messages for each engine except for the one which unregistered
	for i := range networks {
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

	// require every one except the unsubscribed engine received the message
	unittest.RequireReturnsBefore(t, wg.Wait, 2*time.Second, "message not received")

	// asserts that the unregistered engine did not receive the message
	unregisteredEng := engs[unregisterIndex]
	require.Emptyf(t, unregisteredEng.Received, "unregistered engine received the topic message")
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
