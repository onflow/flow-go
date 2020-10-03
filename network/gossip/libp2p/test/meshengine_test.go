package test

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	golog "github.com/ipfs/go-log"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/network/gossip/libp2p"
	"github.com/onflow/flow-go/utils/unittest"
)

// MeshEngineTestSuite evaluates the message delivery functionality for the overlay
// of engines over a complete graph
type MeshEngineTestSuite struct {
	suite.Suite
	ConduitWrapper                      // used as a wrapper around conduit methods
	nets           []*libp2p.Network    // used to keep track of the networks
	mws            []*libp2p.Middleware // used to keep track of the middlewares associated with networks
	ids            flow.IdentityList    // used to keep track of the identifiers associated with networks
}

// TestMeshNetTestSuite runs all tests in this test suit
func TestMeshNetTestSuite(t *testing.T) {
	suite.Run(t, new(MeshEngineTestSuite))
}

// SetupTest is executed prior to each test in this test suit
// it creates and initializes a set of network instances
func (m *MeshEngineTestSuite) SetupTest() {
	// defines total number of nodes in our network (minimum 3 needed to use 1-k messaging)
	const count = 10
	const cacheSize = 100
	golog.SetAllLoggers(golog.LevelInfo)

	m.ids = CreateIDs(count)

	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	mws, err := createMiddleware(logger, m.ids)
	require.NoError(m.Suite.T(), err)
	m.mws = mws

	nets, err := createNetworks(logger, m.mws, m.ids, cacheSize, false)
	require.NoError(m.Suite.T(), err)
	m.nets = nets
}

// TearDownTest closes the networks within a specified timeout
func (m *MeshEngineTestSuite) TearDownTest() {
	for _, net := range m.nets {
		select {
		// closes the network
		case <-net.Done():
			continue
		case <-time.After(3 * time.Second):
			m.Suite.Fail("could not stop the network")
		}
	}
}

// TestAllToAll_Submit evaluates the network of mesh engines against allToAllScenario scenario.
// Network instances during this test use their Submit method to disseminate messages.
func (m *MeshEngineTestSuite) TestAllToAll_Submit() {
	m.allToAllScenario(m.Submit)
}

// TestAllToAll_Publish evaluates the network of mesh engines against allToAllScenario scenario.
// Network instances during this test use their Publish method to disseminate messages.
func (m *MeshEngineTestSuite) TestAllToAll_Publish() {
	m.allToAllScenario(m.Publish)
}

// TestAllToAll_Multicast evaluates the network of mesh engines against allToAllScenario scenario.
// Network instances during this test use their Multicast method to disseminate messages.
func (m *MeshEngineTestSuite) TestAllToAll_Multicast() {
	m.allToAllScenario(m.Multicast)
}

// TestAllToAll_Unicast evaluates the network of mesh engines against allToAllScenario scenario.
// Network instances during this test use their Unicast method to disseminate messages.
func (m *MeshEngineTestSuite) TestAllToAll_Unicast() {
	m.allToAllScenario(m.Unicast)
}

// TestTargetedValidators_Submit tests if only the intended recipients in a 1-k messaging actually receive the message.
// The messages are disseminated through the Submit method of conduits.
func (m *MeshEngineTestSuite) TestTargetedValidators_Submit() {
	m.targetValidatorScenario(m.Submit)
}

// TestTargetedValidators_Unicast tests if only the intended recipients in a 1-k messaging actually receive the message.
// The messages are disseminated through the Unicast method of conduits.
func (m *MeshEngineTestSuite) TestTargetedValidators_Unicast() {
	m.targetValidatorScenario(m.Unicast)
}

// TestTargetedValidators_Multicast tests if only the intended recipients in a 1-k messaging actually receive the
//message.
// The messages are disseminated through the Multicast method of conduits.
func (m *MeshEngineTestSuite) TestTargetedValidators_Multicast() {
	m.targetValidatorScenario(m.Multicast)
}

// TestTargetedValidators_Publish tests if only the intended recipients in a 1-k messaging actually receive the message.
// The messages are disseminated through the Multicast method of conduits.
func (m *MeshEngineTestSuite) TestTargetedValidators_Publish() {
	m.targetValidatorScenario(m.Publish)
}

// TestMaxMessageSize_Submit evaluates the messageSizeScenario scenario using
// the Submit method of conduits.
func (m *MeshEngineTestSuite) TestMaxMessageSize_Submit() {
	m.messageSizeScenario(m.Submit, libp2p.DefaultMaxPubSubMsgSize)
}

// TestMaxMessageSize_Unicast evaluates the messageSizeScenario scenario using
// the Unicast method of conduits.
func (m *MeshEngineTestSuite) TestMaxMessageSize_Unicast() {
	m.messageSizeScenario(m.Unicast, libp2p.DefaultMaxUnicastMsgSize)
}

// TestMaxMessageSize_Multicast evaluates the messageSizeScenario scenario using
// the Multicast method of conduits.
func (m *MeshEngineTestSuite) TestMaxMessageSize_Multicast() {
	m.messageSizeScenario(m.Multicast, libp2p.DefaultMaxPubSubMsgSize)
}

// TestMaxMessageSize_Publish evaluates the messageSizeScenario scenario using the
// Publish method of conduits.
func (m *MeshEngineTestSuite) TestMaxMessageSize_Publish() {
	m.messageSizeScenario(m.Publish, libp2p.DefaultMaxPubSubMsgSize)
}

// TestUnregister_Publish tests that an engine cannot send any message using Publish
// or receive any messages after the conduit is closed
func (m *MeshEngineTestSuite) TestUnregister_Publish() {
	m.conduitCloseScenario(m.Publish)
}

// TestUnregister_Publish tests that an engine cannot send any message using Multicast
// or receive any messages after the conduit is closed
func (m *MeshEngineTestSuite) TestUnregister_Multicast() {
	m.conduitCloseScenario(m.Multicast)
}

// TestUnregister_Publish tests that an engine cannot send any message using Submit
// or receive any messages after the conduit is closed
func (m *MeshEngineTestSuite) TestUnregister_Submit() {
	m.conduitCloseScenario(m.Submit)
}

// TestUnregister_Publish tests that an engine cannot send any message using Unicast
// or receive any messages after the conduit is closed
func (m *MeshEngineTestSuite) TestUnregister_Unicast() {
	m.conduitCloseScenario(m.Unicast)
}

// allToAllScenario creates a complete mesh of the engines
// each engine x then sends a "hello from node x" to other engines
// it evaluates the correctness of message delivery as well as content of the message
func (m *MeshEngineTestSuite) allToAllScenario(send ConduitSendWrapperFunc) {
	// allows nodes to find each other in case of Mulitcast and Publish
	optionalSleep(send)

	// creating engines
	count := len(m.nets)
	engs := make([]*MeshEngine, 0)
	wg := sync.WaitGroup{}

	// logs[i][j] keeps the message that node i sends to node j
	logs := make(map[int][]string)
	for i := range m.nets {
		eng := NewMeshEngine(m.Suite.T(), m.nets[i], count-1, engine.TestNetwork)
		engs = append(engs, eng)
		logs[i] = make([]string, 0)
	}

	// allow nodes to heartbeat and discover each other
	time.Sleep(2 * time.Second)

	// Each node broadcasting a message to all others
	for i := range m.nets {
		event := &message.TestMessage{
			Text: fmt.Sprintf("hello from node %v", i),
		}

		// others keeps the identifier of all nodes except ith node
		others := m.ids.Filter(filter.Not(filter.HasNodeID(m.ids[i].NodeID))).NodeIDs()
		require.NoError(m.Suite.T(), send(event, engs[i].con, others...))
		wg.Add(count - 1)
	}

	// fires a goroutine for each engine that listens to incoming messages
	for i := range m.nets {
		go func(e *MeshEngine) {
			for x := 0; x < count-1; x++ {
				<-e.received
				wg.Done()
			}
		}(engs[i])
	}

	unittest.AssertReturnsBefore(m.Suite.T(), wg.Wait, 30*time.Second)

	// evaluates that all messages are received
	for index, e := range engs {
		// confirms the number of received messages at each node
		if len(e.event) != (count - 1) {
			assert.Fail(m.Suite.T(),
				fmt.Sprintf("Message reception mismatch at node %v. Expected: %v, Got: %v", index, count-1, len(e.event)))
		}

		// extracts failed messages
		receivedIndices, err := extractSenderID(count, e.event, "hello from node")
		require.NoError(m.Suite.T(), err)

		for j := 0; j < count; j++ {
			// evaluates self-gossip
			if j == index {
				assert.False(m.Suite.T(), (receivedIndices)[index], fmt.Sprintf("self gossiped for node %v detected", index))
			}
			// evaluates content
			if !(receivedIndices)[j] {
				assert.False(m.Suite.T(), (receivedIndices)[index],
					fmt.Sprintf("Message not found in node #%v's messages. Expected: Message from node %v. Got: No message", index, j))
			}
		}
	}
}

// targetValidatorScenario sends a single message from last node to the first half of the nodes
// based on identifiers list.
// It then verifies that only the intended recipients receive the message.
// Message dissemination is done using the send wrapper of conduit.
func (m *MeshEngineTestSuite) targetValidatorScenario(send ConduitSendWrapperFunc) {
	// creating engines
	count := len(m.nets)
	engs := make([]*MeshEngine, 0)
	wg := sync.WaitGroup{}

	for i := range m.nets {
		eng := NewMeshEngine(m.Suite.T(), m.nets[i], count-1, engine.TestNetwork)
		engs = append(engs, eng)
	}

	// allow nodes to heartbeat and discover each other
	time.Sleep(5 * time.Second)

	// choose half of the nodes as target
	allIds := m.ids.NodeIDs()
	var targets []flow.Identifier
	// create a target list of half of the nodes
	for i := 0; i < len(allIds)/2; i++ {
		targets = append(targets, allIds[i])
	}

	// node 0 broadcasting a message to all targets
	event := &message.TestMessage{
		Text: "hello from node 0",
	}
	require.NoError(m.Suite.T(), send(event, engs[len(engs)-1].con, targets...))

	// fires a goroutine for all engines to listens for the incoming message
	for i := 0; i < len(allIds)/2; i++ {
		wg.Add(1)
		go func(e *MeshEngine) {
			<-e.received
			wg.Done()
		}(engs[i])
	}

	unittest.AssertReturnsBefore(m.T(), wg.Wait, 10*time.Second)

	// evaluates that all messages are received
	for index, e := range engs {
		if index < len(engs)/2 {
			assert.Len(m.Suite.T(), e.event, 1, fmt.Sprintf("message not received %v", index))
		} else {
			assert.Len(m.Suite.T(), e.event, 0, fmt.Sprintf("message received when none was expected %v", index))
		}
	}
}

// messageSizeScenario provides a scenario to check if a message of maximum permissible size can be sent
//successfully.
// It broadcasts a message from the first node to all the nodes in the identifiers list using send wrapper function.
func (m *MeshEngineTestSuite) messageSizeScenario(send ConduitSendWrapperFunc, size uint) {
	// creating engines
	count := len(m.nets)
	engs := make([]*MeshEngine, 0)
	wg := sync.WaitGroup{}

	for i := range m.nets {
		eng := NewMeshEngine(m.Suite.T(), m.nets[i], count-1, engine.TestNetwork)
		engs = append(engs, eng)
	}

	// allow nodes to heartbeat and discover each other
	time.Sleep(2 * time.Second)

	// others keeps the identifier of all nodes except node that is sender.
	others := m.ids.Filter(filter.Not(filter.HasNodeID(m.ids[0].NodeID))).NodeIDs()

	// generates and sends an event of custom size to the network
	payload := libp2p.NetworkPayloadFixture(m.T(), size)
	event := &message.TestMessage{
		Text: string(payload),
	}

	require.NoError(m.T(), send(event, engs[0].con, others...))

	// fires a goroutine for all engines (except sender) to listen for the incoming message
	for _, eng := range engs[1:] {
		wg.Add(1)
		go func(e *MeshEngine) {
			<-e.received
			wg.Done()
		}(eng)
	}

	unittest.AssertReturnsBefore(m.Suite.T(), wg.Wait, 30*time.Second)

	// evaluates that all messages are received
	for index, e := range engs[1:] {
		assert.Len(m.Suite.T(), e.event, 1, "message not received by engine %d", index+1)
	}
}

// conduitCloseScenario tests after a Conduit is closed, an engine cannot send or receive a message for that channel ID
func (m *MeshEngineTestSuite) conduitCloseScenario(send ConduitSendWrapperFunc) {

	optionalSleep(send)

	// creating engines
	count := len(m.nets)
	engs := make([]*MeshEngine, 0)
	wg := sync.WaitGroup{}

	for i := range m.nets {
		eng := NewMeshEngine(m.Suite.T(), m.nets[i], count-1, engine.TestNetwork)
		engs = append(engs, eng)
	}

	// allow nodes to heartbeat and discover each other
	time.Sleep(2 * time.Second)

	// unregister a random engine from the test topic by calling close on it's conduit
	unregisterIndex := rand.Intn(count)
	err := engs[unregisterIndex].con.Close()
	assert.NoError(m.T(), err)

	// each node attempts to broadcast a message to all others
	for i := range m.nets {
		event := &message.TestMessage{
			Text: fmt.Sprintf("hello from node %v", i),
		}

		// others keeps the identifier of all nodes except ith node
		others := m.ids.Filter(filter.Not(filter.HasNodeID(m.ids[i].NodeID))).NodeIDs()

		if i == unregisterIndex {
			// assert that unsubscribed engine cannot publish on that topic
			require.Error(m.Suite.T(), send(event, engs[i].con, others...))
			continue
		}

		require.NoError(m.Suite.T(), send(event, engs[i].con, others...))
	}

	// fire a goroutine to listen for incoming messages for each engine except for the one which unregistered
	for i := range m.nets {
		if i == unregisterIndex {
			continue
		}
		wg.Add(1)
		go func(e *MeshEngine) {
			expectedMsgCnt := count - 2 // count less self and unsubscribed engine
			for x := 0; x < expectedMsgCnt; x++ {
				<-e.received
			}
			wg.Done()
		}(engs[i])
	}

	// assert every one except the unsubscribed engine received the message
	unittest.AssertReturnsBefore(m.Suite.T(), wg.Wait, 2*time.Second)

	// assert that the unregistered engine did not receive the message
	unregisteredEng := engs[unregisterIndex]
	assert.Emptyf(m.T(), unregisteredEng.received, "unregistered engine received the topic message")
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
