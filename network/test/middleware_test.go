package test

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/go-log"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	mockery "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	libp2pmessage "github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/codec/json"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/utils/unittest"
)

const testChannel = "test-channel"

type MiddlewareTestSuite struct {
	suite.Suite
	size    int               // used to determine number of middlewares under test
	mws     []*p2p.Middleware // used to keep track of middlewares under test
	ov      []*mocknetwork.Overlay
	ids     []*flow.Identity
	metrics *metrics.NoopCollector // no-op performance monitoring simulation
}

// TestMiddlewareTestSuit runs all the test methods in this test suit
func TestMiddlewareTestSuit(t *testing.T) {
	suite.Run(t, new(MiddlewareTestSuite))
}

// SetupTest initiates the test setups prior to each test
func (m *MiddlewareTestSuite) SetupTest() {
	logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
	log.SetAllLoggers(log.LevelError)

	m.size = 2 // operates on two middlewares
	m.metrics = metrics.NewNoopCollector()
	// create and start the middlewares
	m.ids, m.mws = GenerateIDsAndMiddlewares(m.T(), m.size, !DryRun, logger)

	require.Len(m.Suite.T(), m.ids, m.size)
	require.Len(m.Suite.T(), m.mws, m.size)

	// create the mock overlays
	for i := 0; i < m.size; i++ {
		overlay := &mocknetwork.Overlay{}
		m.ov = append(m.ov, overlay)

		identifierToID := make(map[flow.Identifier]flow.Identity)
		for _, id := range m.ids {
			identifierToID[id.NodeID] = *id
		}
		overlay.On("Identity").Maybe().Return(identifierToID, nil)
		overlay.On("Topology").Maybe().Return(flow.IdentityList(m.ids), nil)
	}
	for i, mw := range m.mws {
		assert.NoError(m.T(), mw.Start(m.ov[i]))
		err := mw.UpdateAllowList()
		require.NoError(m.T(), err)
	}
}

func (m *MiddlewareTestSuite) TearDownTest() {
	m.stopMiddlewares()
}

// TestPingRawReception tests the middleware for solely the
// reception of a single ping message by a node that is sent from another node
// it does not evaluate the type and content of the message
func (m *MiddlewareTestSuite) TestPingRawReception() {
	m.Ping(mockery.Anything, mockery.Anything)
}

// TestPingTypeReception tests the middleware against type of received payload
// upon reception at the receiver side
// it does not evaluate content of the payload
// it does not evaluate anything related to the sender id
func (m *MiddlewareTestSuite) TestPingTypeReception() {
	m.Ping(mockery.Anything, mockery.AnythingOfType("*message.Message"))
}

// TestPingIDType tests the middleware against both the type of sender id
// and content of the payload of the event upon reception at the receiver side
// it does not evaluate the actual value of the sender ID
func (m *MiddlewareTestSuite) TestPingIDType() {
	msg := createMessage(m.ids[0].NodeID, m.ids[1].NodeID)
	m.Ping(mockery.AnythingOfType("flow.Identifier"), msg)
}

// TestPingContentReception tests the middleware against both
// the payload and sender ID of the event upon reception at the receiver side
func (m *MiddlewareTestSuite) TestPingContentReception() {
	msg := createMessage(m.ids[0].NodeID, m.ids[1].NodeID)
	m.Ping(m.ids[0].NodeID, msg)
}

// TestMultiPing tests the middleware against type of received payload
// of distinct messages that are sent concurrently from a node to another
func (m *MiddlewareTestSuite) TestMultiPing() {
	// one distinct message
	m.MultiPing(1)

	// two distinct messages
	m.MultiPing(2)

	// 10 distinct messages
	m.MultiPing(10)
}

// Ping sends a message from the first middleware of the test suit to the last one
// expectID and expectPayload are what we expect the receiver side to evaluate the
// incoming ping against, it can be mocked or typed data
func (m *MiddlewareTestSuite) Ping(expectID, expectPayload interface{}) {

	ch := make(chan struct{})
	// extracts sender id based on the mock option
	var err error
	// mocks Overlay.Receive for middleware.Overlay.Receive(*nodeID, payload)
	firstNode := 0
	lastNode := m.size - 1
	m.ov[lastNode].On("Receive", expectID, expectPayload).Return(nil).Once().
		Run(func(args mockery.Arguments) {
			ch <- struct{}{}
		})

	msg := createMessage(m.ids[firstNode].NodeID, m.ids[lastNode].NodeID)

	// sends a direct message from first node to the last node
	err = m.mws[firstNode].SendDirect(msg, m.ids[lastNode].NodeID)
	require.NoError(m.Suite.T(), err)

	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		assert.Fail(m.T(), "peer 1 failed to send a message to peer 2")
	}

	// evaluates the mock calls
	for i := 1; i < m.size; i++ {
		m.ov[i].AssertExpectations(m.T())
	}
}

// Ping sends count-many distinct messages concurrently from the first middleware of the test suit to the last one
// It evaluates the correctness of reception of the content of the messages, as well as the sender ID
func (m *MiddlewareTestSuite) MultiPing(count int) {
	wg := sync.WaitGroup{}
	// extracts sender id based on the mock option
	// mocks Overlay.Receive for  middleware.Overlay.Receive(*nodeID, payload)
	firstNode := 0
	lastNode := m.size - 1
	for i := 0; i < count; i++ {
		wg.Add(1)
		msg := createMessage(m.ids[firstNode].NodeID, m.ids[lastNode].NodeID, fmt.Sprintf("hello from: %d", i))
		m.ov[lastNode].On("Receive", m.ids[firstNode].NodeID, msg).Return(nil).Once().
			Run(func(args mockery.Arguments) {
				wg.Done()
			})
		go func() {
			// sends a direct message from first node to the last node
			err := m.mws[firstNode].SendDirect(msg, m.ids[lastNode].NodeID)
			require.NoError(m.Suite.T(), err)
		}()
	}

	wg.Wait()

	// evaluates the mock calls
	for i := 1; i < m.size; i++ {
		m.ov[i].AssertExpectations(m.T())
	}
}

// TestEcho sends an echo message from first middleware to the last middleware
// the last middleware echos back the message. The test evaluates the correctness
// of the message reception as well as its content
func (m *MiddlewareTestSuite) TestEcho() {

	wg := sync.WaitGroup{}
	// extracts sender id based on the mock option
	var err error

	wg.Add(2)
	// mocks Overlay.Receive for middleware.Overlay.Receive(*nodeID, payload)
	first := 0
	last := m.size - 1
	firstNode := m.ids[first].NodeID
	lastNode := m.ids[last].NodeID

	sendMsg := createMessage(firstNode, lastNode, "hello")
	replyMsg := createMessage(lastNode, firstNode, "hello back")

	// last node
	m.ov[last].On("Receive", firstNode, sendMsg).Return(nil).Once().
		Run(func(args mockery.Arguments) {
			wg.Done()
			// echos back the same message back to the sender
			err := m.mws[last].SendDirect(replyMsg, firstNode)
			assert.NoError(m.T(), err)

		})

	// first node
	m.ov[first].On("Receive", lastNode, replyMsg).Return(nil).Once().
		Run(func(args mockery.Arguments) {
			wg.Done()
		})

	// sends a direct message from first node to the last node
	err = m.mws[first].SendDirect(sendMsg, m.ids[last].NodeID)
	require.NoError(m.Suite.T(), err)

	wg.Wait()

	// evaluates the mock calls
	for i := 1; i < m.size; i++ {
		m.ov[i].AssertExpectations(m.T())
	}
}

// TestMaxMessageSize_SendDirect evaluates that invoking SendDirect method of the middleware on a message
// size beyond the permissible unicast message size returns an error.
func (m *MiddlewareTestSuite) TestMaxMessageSize_SendDirect() {
	first := 0
	last := m.size - 1
	firstNode := m.ids[first].NodeID
	lastNode := m.ids[last].NodeID

	msg := createMessage(firstNode, lastNode, "")

	// creates a network payload beyond the maximum message size
	// Note: networkPayloadFixture considers 1000 bytes as the overhead of the encoded message,
	// so the generated payload is 1000 bytes below the maximum unicast message size.
	// We hence add up 1000 bytes to the input of network payload fixture to make
	// sure that payload is beyond the permissible size.
	payload := networkPayloadFixture(m.T(), uint(p2p.DefaultMaxUnicastMsgSize)+1000)
	event := &libp2pmessage.TestMessage{
		Text: string(payload),
	}

	codec := json.NewCodec()
	encodedEvent, err := codec.Encode(event)
	require.NoError(m.T(), err)

	msg.Payload = encodedEvent

	// sends a direct message from first node to the last node
	err = m.mws[first].SendDirect(msg, lastNode)
	require.Error(m.Suite.T(), err)
}

// TestLargeMessageSize_SendDirect asserts that a ChunkDataResponse is treated as a large message and can be unicasted
// successfully even though it's size is greater than the default message size.
func (m *MiddlewareTestSuite) TestLargeMessageSize_SendDirect() {
	sourceIndex := 0
	targetIndex := m.size - 1
	sourceNode := m.ids[sourceIndex].NodeID
	targetNode := m.ids[targetIndex].NodeID

	msg := createMessage(sourceNode, targetNode, "")

	// creates a network payload with a size greater than the default max size
	payload := networkPayloadFixture(m.T(), uint(p2p.DefaultMaxUnicastMsgSize)+1000)
	event := &libp2pmessage.TestMessage{
		Text: string(payload),
	}

	// set the message type to a known large message type
	msg.Type = "messages.ChunkDataResponse"

	codec := json.NewCodec()
	encodedEvent, err := codec.Encode(event)
	require.NoError(m.T(), err)

	// set the message payload as the large message
	msg.Payload = encodedEvent

	// expect one message to be received by the target
	ch := make(chan struct{})
	m.ov[targetIndex].On("Receive", sourceNode, msg).Return(nil).Once().
		Run(func(args mockery.Arguments) {
			close(ch)
		})

	// sends a direct message from source node to the target node
	err = m.mws[sourceIndex].SendDirect(msg, targetNode)
	// SendDirect should not error since this is a known large message
	require.NoError(m.Suite.T(), err)

	// check message reception on target
	unittest.RequireCloseBefore(m.T(), ch, 15*time.Second, "source node failed to send large message to target")

	m.ov[targetIndex].AssertExpectations(m.T())
}

// TestMaxMessageSize_Publish evaluates that invoking Publish method of the middleware on a message
// size beyond the permissible publish message size returns an error.
func (m *MiddlewareTestSuite) TestMaxMessageSize_Publish() {
	first := 0
	last := m.size - 1
	firstNode := m.ids[first].NodeID
	lastNode := m.ids[last].NodeID

	msg := createMessage(firstNode, lastNode, "")
	// adds another node as the target id to imitate publishing
	msg.TargetIDs = append(msg.TargetIDs, lastNode[:])

	// creates a network payload beyond the maximum message size
	// Note: networkPayloadFixture considers 1000 bytes as the overhead of the encoded message,
	// so the generated payload is 1000 bytes below the maximum publish message size.
	// We hence add up 1000 bytes to the input of network payload fixture to make
	// sure that payload is beyond the permissible size.
	payload := networkPayloadFixture(m.T(), uint(p2p.DefaultMaxPubSubMsgSize)+1000)
	event := &libp2pmessage.TestMessage{
		Text: string(payload),
	}

	codec := json.NewCodec()
	encodedEvent, err := codec.Encode(event)
	require.NoError(m.T(), err)

	msg.Payload = encodedEvent

	// sends a direct message from first node to the last node
	err = m.mws[first].Publish(msg, testChannel)
	require.Error(m.Suite.T(), err)
}

// TestUnsubscribe tests that an engine can unsubscribe from a topic it was earlier subscribed to and stop receiving
// messages.
func (m *MiddlewareTestSuite) TestUnsubscribe() {

	first := 0
	last := m.size - 1
	firstNode := m.ids[first].NodeID
	lastNode := m.ids[last].NodeID

	// initially subscribe the nodes to the channel
	for _, mw := range m.mws {
		err := mw.Subscribe(testChannel)
		require.NoError(m.Suite.T(), err)
	}

	// wait for nodes to form a mesh
	time.Sleep(2 * time.Second)

	origin := 0
	target := m.size - 1

	originID := m.ids[origin].NodeID
	message1 := createMessage(firstNode, lastNode, "hello1")

	m.ov[target].On("Receive", originID, mockery.Anything).Return(nil).Once()

	// first test that when both nodes are subscribed to the channel, the target node receives the message
	err := m.mws[origin].Publish(message1, testChannel)
	assert.NoError(m.T(), err)

	assert.Eventually(m.T(), func() bool {
		return m.ov[target].AssertCalled(m.T(), "Receive", originID, mockery.Anything)
	}, 2*time.Second, time.Millisecond)

	// now unsubscribe the target node from the channel
	err = m.mws[target].Unsubscribe(testChannel)
	assert.NoError(m.T(), err)

	// create and send a new message on the channel from the origin node
	message2 := createMessage(firstNode, lastNode, "hello2")
	err = m.mws[origin].Publish(message2, testChannel)
	assert.NoError(m.T(), err)

	// assert that the new message is not received by the target node
	assert.Never(m.T(), func() bool {
		return !m.ov[target].AssertNumberOfCalls(m.T(), "Receive", 1)
	}, 2*time.Second, time.Millisecond)
}

func createMessage(originID flow.Identifier, targetID flow.Identifier, msg ...string) *message.Message {
	payload := "hello"

	if len(msg) > 0 {
		payload = msg[0]
	}

	return &message.Message{
		ChannelID: testChannel,
		EventID:   []byte("1"),
		OriginID:  originID[:],
		TargetIDs: [][]byte{targetID[:]},
		Payload:   []byte(payload),
	}
}

func (m *MiddlewareTestSuite) stopMiddlewares() {
	// start all the middlewares
	for i := 0; i < m.size; i++ {
		// start the middleware
		m.mws[i].Stop()
	}
	m.mws = nil
	m.ov = nil
	m.ids = nil
	m.size = 0
}
