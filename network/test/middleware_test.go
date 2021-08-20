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
	"github.com/onflow/flow-go/module/observable"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/utils/unittest"
)

const testChannel = "test-channel"

// libp2p emits a call to `Protect` with a topic-specific tag upon establishing each peering connection in a GossipSUb mesh, see:
// https://github.com/libp2p/go-libp2p-pubsub/blob/master/tag_tracer.go
// One way to make sure such a mesh has formed, asynchronously, in unit tests, is to wait for libp2p.GossipSubD such calls,
// and that's what we do with tagsObserver.
//
type tagsObserver struct {
	tags chan string
	log  zerolog.Logger
}

func (co *tagsObserver) OnNext(peertag interface{}) {
	pt, ok := peertag.(PeerTag)

	if ok {
		co.tags <- fmt.Sprintf("peer: %v tag: %v", pt.peer, pt.tag)
	}

}
func (co *tagsObserver) OnError(err error) {
	co.log.Error().Err(err).Msg("Tags Observer closed on an error")
	close(co.tags)
}
func (co *tagsObserver) OnComplete() {
	close(co.tags)
}

type MiddlewareTestSuite struct {
	suite.Suite
	size    int               // used to determine number of middlewares under test
	mws     []*p2p.Middleware // used to keep track of middlewares under test
	ov      []*mocknetwork.Overlay
	obs     chan string // used to keep track of Protect events tagged by pubsub messages
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

	// create and start the middlewares and inject a connection observer
	var obs []observable.Observable
	peerChannel := make(chan string)
	ob := tagsObserver{
		tags: peerChannel,
		log:  logger,
	}

	m.ids, m.mws, obs = GenerateIDsAndMiddlewares(m.T(), m.size, !DryRun, logger)

	for _, observableConnMgr := range obs {
		observableConnMgr.Subscribe(&ob)
	}
	m.obs = peerChannel

	require.Len(m.Suite.T(), obs, m.size)
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

// TestSpoofedPubSubHello evaluates checking the originID of the message w.r.t. its libp2p network ID on PubSub
// we check a pubsub message with a spoofed OriginID does not get delivered
// This would be doubled with cryptographic verification of the libp2p network ID in production (see message signing options in pubSub initialization)
func (m *MiddlewareTestSuite) TestSpoofedPubSubHello() {
	first := 0
	last := m.size - 1
	lastNode := m.ids[last].NodeID

	// initially subscribe the nodes to the channel
	for _, mw := range m.mws {
		err := mw.Subscribe(testChannel)
		require.NoError(m.Suite.T(), err)
	}

	// set up waiting for m.size pubsub tags indicating a mesh has formed
	for i := 0; i < m.size; i++ {
		select {
		case <-m.obs:
		case <-time.After(2 * time.Second):
			assert.FailNow(m.T(), "could not receive pubsub tag indicating mesh formed")
		}
	}

	spoofedID := unittest.IdentifierFixture()

	message1 := createMessage(spoofedID, lastNode, "hello1")

	err := m.mws[first].Publish(message1, testChannel)
	assert.NoError(m.T(), err)

	// assert that the spoofed message is not received by the target node
	assert.Never(m.T(), func() bool {
		return !m.ov[last].AssertNumberOfCalls(m.T(), "Receive", 0)
	}, 2*time.Second, 100*time.Millisecond)

	// invalid message sent by firstNode claims to be from lastNode
	message2 := createMessage(lastNode, lastNode, "hello1")

	err = m.mws[first].Publish(message2, testChannel)
	assert.NoError(m.T(), err)

	// assert that the invalid message is not received by the target node
	assert.Never(m.T(), func() bool {
		return !m.ov[last].AssertNumberOfCalls(m.T(), "Receive", 0)
	}, 2*time.Second, 100*time.Millisecond)

}

// TestSpoofedDirect sends a message from the first middleware of the test suit to the last one
// we check a pubsub message with a spoofed OriginID does not get delivered
func (m *MiddlewareTestSuite) TestSpoofedDirect() {

	// extracts sender id based on the mock option
	var err error
	// mocks Overlay.Receive for middleware.Overlay.Receive(*nodeID, payload)
	firstNode := 0
	lastNode := m.size - 1

	spoofedID := unittest.IdentifierFixture()

	spoofedMsg := createMessage(spoofedID, m.ids[lastNode].NodeID, "hello")

	// sends a direct spoofed message from first node to the last node
	err = m.mws[firstNode].SendDirect(spoofedMsg, m.ids[lastNode].NodeID)
	require.NoError(m.Suite.T(), err)

	// assert that the spoofed message is not received by the target node
	assert.Never(m.T(), func() bool {
		return !m.ov[lastNode].AssertNumberOfCalls(m.T(), "Receive", 0)
	}, 2*time.Second, 100*time.Millisecond)

	invalidID := m.ids[lastNode].NodeID

	invalidMsg := createMessage(invalidID, m.ids[lastNode].NodeID, "hello")

	// sends a direct spoofed message from first node to the last node
	err = m.mws[firstNode].SendDirect(invalidMsg, m.ids[lastNode].NodeID)
	require.NoError(m.Suite.T(), err)

	// assert that the spoofed message is not received by the target node
	assert.Never(m.T(), func() bool {
		return !m.ov[lastNode].AssertNumberOfCalls(m.T(), "Receive", 0)
	}, 2*time.Second, 100*time.Millisecond)

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

	codec := cbor.NewCodec()
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

	codec := cbor.NewCodec()
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

	codec := cbor.NewCodec()
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

	// set up waiting for m.size pubsub tags indicating a mesh has formed
	for i := 0; i < m.size; i++ {
		select {
		case <-m.obs:
		case <-time.After(2 * time.Second):
			assert.FailNow(m.T(), "could not receive pubsub tag indicating mesh formed")
		}
	}

	msgRcvd := make(chan struct{}, 2)
	msgRcvdFun := func() {
		<-msgRcvd
	}
	message1 := createMessage(firstNode, lastNode, "hello1")
	m.ov[last].On("Receive", firstNode, mockery.Anything).Return(nil).Run(func(_ mockery.Arguments) {
		msgRcvd <- struct{}{}
	})

	// first test that when both nodes are subscribed to the channel, the target node receives the message
	err := m.mws[first].Publish(message1, testChannel)
	assert.NoError(m.T(), err)

	unittest.RequireReturnsBefore(m.T(), msgRcvdFun, 2*time.Second, "message not received")

	// now unsubscribe the target node from the channel
	err = m.mws[last].Unsubscribe(testChannel)
	assert.NoError(m.T(), err)

	// create and send a new message on the channel from the origin node
	message2 := createMessage(firstNode, lastNode, "hello2")
	err = m.mws[first].Publish(message2, testChannel)
	assert.NoError(m.T(), err)

	// assert that the new message is not received by the target node
	unittest.RequireNeverReturnBefore(m.T(), msgRcvdFun, 2*time.Second, "message received unexpectedly")
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
