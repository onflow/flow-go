package test

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/libp2p/message"
	"github.com/dapperlabs/flow-go/network/codec/json"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// MeshNetTestSuite evaluates the message delivery functionality for the overlay
// of engines over a complete graph
type MeshNetTestSuite struct {
	suite.Suite
	nets []*libp2p.Network    // used to keep track of the networks
	mws  []*libp2p.Middleware // used to keep track of the middlewares associated with networks
	ids  flow.IdentityList    // used to keep track of the identifiers associated with networks
}

// TestMeshNetTestSuite runs all tests in this test suit
func TestMeshNetTestSuite(t *testing.T) {
	suite.Run(t, new(MeshNetTestSuite))
}

// SetupTest is executed prior to each test in this test suit
// it creates and initializes a set of network instances
func (m *MeshNetTestSuite) SetupTest() {
	// defines total number of nodes in our network (minimum 3 needed to use 1-k messaging)
	const count = 10
	const cacheSize = 100
	//golog.SetAllLoggers(gologging.INFO)

	m.ids = CreateIDs(count)

	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	mws, err := CreateMiddleware(logger, m.ids)
	require.NoError(m.Suite.T(), err)
	m.mws = mws

	nets, err := CreateNetworks(logger, m.mws, m.ids, cacheSize, false)
	require.NoError(m.Suite.T(), err)
	m.nets = nets

	// allows nodes to find each other
	time.Sleep(5 * time.Second)
}

// TearDownTest closes the networks within a specified timeout
func (s *MeshNetTestSuite) TearDownTest() {
	for _, net := range s.nets {
		select {
		// closes the network
		case <-net.Done():
			continue
		case <-time.After(3 * time.Second):
			s.Suite.Fail("could not stop the network")
		}
	}
}

// TestAllToAll creates a complete mesh of the engines
// each engine x then sends a "hello from node x" to other engines
// it evaluates the correctness of message delivery as well as content of the message
func (m *MeshNetTestSuite) TestAllToAll() {
	// creating engines
	count := len(m.nets)
	engs := make([]*MeshEngine, 0)
	wg := sync.WaitGroup{}

	// log[i][j] keeps the message that node i sends to node j
	log := make(map[int][]string)
	for i := range m.nets {
		eng := NewMeshEngine(m.Suite.T(), m.nets[i], count-1, engine.CollectionProvider)
		engs = append(engs, eng)
		log[i] = make([]string, 0)
	}

	// allow nodes to heartbeat and discover each other
	time.Sleep(5 * time.Second)

	// Each node broadcasting a message to all others
	for i := range m.nets {
		event := &message.Echo{
			Text: fmt.Sprintf("hello from node %v", i),
		}
		require.NoError(m.Suite.T(), engs[i].con.Submit(event, m.ids.NodeIDs()...))
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

	c := make(chan struct{})
	go func() {
		wg.Wait()
		c <- struct{}{}
	}()

	select {
	case <-c:
	case <-time.After(10 * time.Second):
		assert.Fail(m.Suite.T(), "test timed out on broadcast dissemination")
	}

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

// TestTargetValidator tests if only the intended recipients in a 1-k messaging actually receive the message
func (m *MeshNetTestSuite) TestTargetValidator() {
	// creating engines
	count := len(m.nets)
	engs := make([]*MeshEngine, 0)
	wg := sync.WaitGroup{}

	for i := range m.nets {
		eng := NewMeshEngine(m.Suite.T(), m.nets[i], count-1, engine.CollectionIngest)
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
	event := &message.Echo{
		Text: "hello from node 0",
	}
	require.NoError(m.Suite.T(), engs[0].con.Submit(event, targets...))

	// fires a goroutine for all engines to listens for the incoming message
	for i := 1; i < len(allIds)/2; i++ {
		wg.Add(1)
		go func(e *MeshEngine) {
			<-e.received
			wg.Done()
		}(engs[i])
	}

	c := make(chan struct{})
	go func() {
		wg.Wait()
		c <- struct{}{}
	}()

	select {
	case <-c:
	case <-time.After(10 * time.Second):
		assert.Fail(m.Suite.T(), "test timed out on broadcast dissemination")
	}

	// evaluates that all messages are received
	for index, e := range engs {
		if index < len(engs)/2 && index != 0 {
			assert.Len(m.Suite.T(), e.event, 1, fmt.Sprintf("message not received %v", index))
		} else {
			assert.Len(m.Suite.T(), e.event, 0, fmt.Sprintf("message received when none was expected %v", index))
		}
	}
}

// TestMaxMessageSize tests if a message of maximum permissible size can be sent successfully
func (m *MeshNetTestSuite) TestMaxMessageSize() {
	// creating engines
	count := len(m.nets)
	engs := make([]*MeshEngine, 0)
	wg := sync.WaitGroup{}

	for i := range m.nets {
		eng := NewMeshEngine(m.Suite.T(), m.nets[i], count-1, 1)
		engs = append(engs, eng)
	}

	// allow nodes to heartbeat and discover each other
	time.Sleep(10 * time.Second)

	allIds := m.ids.NodeIDs()
	var targets = allIds[1:] // node 0 broadcasts a message to all the other nodes

	// create an empty message
	emptyEvent := &message.Echo{
		Text: "",
	}
	codec := json.NewCodec()
	empty, err := codec.Encode(emptyEvent)
	require.NoError(m.T(), err)

	// create a large message approximately equal to max message size
	overhead := 500                                                       // approx 500 bytes overhead for message headers & encoding overhead
	payloadSize := libp2p.DefaultMaxPubSubMsgSize - overhead - len(empty) // max possible payload size
	payload := make([]byte, payloadSize)                                  // create a message of max possible payload size
	for i := range payload {
		payload[i] = 'a' // a utf-8 char that translates to 1-byte when converted to a string
	}

	event := emptyEvent
	event.Text = string(payload)

	// encode event the way the network would encode it to get the size of the message
	encodedEvent, err := codec.Encode(event)
	require.NoError(m.T(), err)

	// check that message payload is approx equal to max size
	require.InDelta(m.Suite.T(), len(encodedEvent), libp2p.DefaultMaxPubSubMsgSize, float64(overhead))

	// submit the message
	require.NoError(m.Suite.T(), engs[0].con.Submit(event, targets...))

	// fires a goroutine for all engines to listens for the incoming message
	for i := 1; i < len(allIds); i++ {
		wg.Add(1)
		go func(e *MeshEngine) {
			<-e.received
			wg.Done()
		}(engs[i])
	}

	unittest.AssertReturnsBefore(m.Suite.T(), wg.Wait, 30*time.Second)

	// evaluates that all messages are received
	for index, e := range engs[1:] {
		assert.Len(m.Suite.T(), e.event, 1, "message not received by engine %d", index+1)
	}
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
		echo := event.(*message.Echo)
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
