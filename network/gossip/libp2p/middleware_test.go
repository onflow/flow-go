package libp2p

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	mockery "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	message2 "github.com/dapperlabs/flow-go/model/libp2p/message"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/network/codec/json"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/message"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

const testChannel = "test-channel"

var rootID = unittest.BlockFixture().ID().String()

type MiddlewareTestSuite struct {
	suite.Suite
	size    int           // used to determine number of middlewares under test
	mws     []*Middleware // used to keep track of middlewares under test
	ov      []*mock.Overlay
	ids     []flow.Identifier
	metrics *metrics.NoopCollector // no-op performance monitoring simulation
}

// TestMiddlewareTestSuit runs all the test methods in this test suit
func TestMiddlewareTestSuit(t *testing.T) {
	suite.Run(t, new(MiddlewareTestSuite))
}

// SetupTest initiates the test setups prior to each test
func (m *MiddlewareTestSuite) SetupTest() {

	m.size = 2 // operates on two middlewares

	m.metrics = metrics.NewNoopCollector()

	// create the middlewares
	m.ids, m.mws = m.createMiddleWares(m.size)
	require.Len(m.Suite.T(), m.ids, m.size)
	require.Len(m.Suite.T(), m.mws, m.size)
	// starts the middlewares
	m.startMiddlewares()
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
	msg := createMessage(m.ids[0], m.ids[1])
	m.Ping(mockery.AnythingOfType("flow.Identifier"), msg)
}

// TestPingContentReception tests the middleware against both
// the payload and sender ID of the event upon reception at the receiver side
func (m *MiddlewareTestSuite) TestPingContentReception() {
	msg := createMessage(m.ids[0], m.ids[1])
	m.Ping(m.mws[0].me, msg)
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

// startMiddleware creates mock overlays for each middleware, and starts the middlewares
func (m *MiddlewareTestSuite) startMiddlewares() {

	idMaps := make([]map[flow.Identifier]flow.Identity, m.size)

	// start all the middlewares
	for i := 0; i < m.size; i++ {
		idMap := make(map[flow.Identifier]flow.Identity)
		// mocks Overlay.Identity with an empty map for now (till we start middleware)
		m.ov[i].On("Identity").Maybe().Return(idMap, nil)
		m.ov[i].On("Topology").Maybe().Return(idMap, nil)

		// start the middleware
		err := m.mws[i].Start(m.ov[i])
		require.NoError(m.Suite.T(), err)

		idMaps[i] = idMap
	}

	// change the overlay mock to return valid ids (now that middleware have been started and we have valid IP & port)
	for i := 0; i < m.size; i++ {
		target := i + 1
		if i == m.size-1 {
			target = 0
		}

		ip, port, err := m.mws[target].GetIPPort()
		require.NoError(m.Suite.T(), err)
		key := m.mws[target].PublicKey()

		// mocks an identity
		flowID := flow.Identity{
			NodeID:        m.ids[target],
			Address:       fmt.Sprintf("%s:%s", ip, port),
			Role:          flow.RoleCollection,
			NetworkPubKey: key,
		}
		idMap := idMaps[i]
		idMap[flowID.NodeID] = flowID
	}

	for _, mw := range m.mws {
		// update whitelist so that nodes can talk to each other
		err := mw.UpdateAllowList()
		require.NoError(m.Suite.T(), err)
	}
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

	msg := createMessage(m.ids[firstNode], m.ids[lastNode])

	// sends a direct message from first node to the last node
	err = m.mws[firstNode].SendDirect(msg, m.ids[lastNode])
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
	var err error
	// mocks Overlay.Receive for  middleware.Overlay.Receive(*nodeID, payload)
	firstNode := 0
	lastNode := m.size - 1
	for i := 0; i < count; i++ {
		wg.Add(1)
		msg := createMessage(m.ids[firstNode], m.ids[lastNode], fmt.Sprintf("hello from: %d", i))
		m.ov[lastNode].On("Receive", m.mws[firstNode].me, msg).Return(nil).Once().
			Run(func(args mockery.Arguments) {
				wg.Done()
			})
		go func() {
			// sends a direct message from first node to the last node
			err = m.mws[firstNode].SendDirect(msg, m.ids[lastNode])
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
	firstNode := 0
	lastNode := m.size - 1

	sendMsg := createMessage(m.ids[firstNode], m.ids[lastNode], "hello")
	replyMsg := createMessage(m.ids[lastNode], m.ids[firstNode], "hello back")

	// last node
	m.ov[lastNode].On("Receive", m.mws[firstNode].me, sendMsg).Return(nil).Once().
		Run(func(args mockery.Arguments) {
			wg.Done()
			// echos back the same message back to the sender
			err = m.mws[lastNode].SendDirect(replyMsg, m.mws[firstNode].me)
			assert.NoError(m.T(), err)

		})

	// first node
	m.ov[firstNode].On("Receive", m.mws[lastNode].me, replyMsg).Return(nil).Once().
		Run(func(args mockery.Arguments) {
			wg.Done()
		})

	// sends a direct message from first node to the last node
	err = m.mws[firstNode].SendDirect(sendMsg, m.ids[lastNode])
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
	firstNode := 0
	lastNode := m.size - 1

	msg := createMessage(m.ids[firstNode], m.ids[lastNode], "")

	// creates a network payload beyond the maximum message size
	// Note: NetworkPayloadFixture considers 1000 bytes as the overhead of the encoded message,
	// so the generated payload is 1000 bytes below the maximum unicast message size.
	// We hence add up 1000 bytes to the input of network payload fixture to make
	// sure that payload is beyond the permissible size.
	payload := NetworkPayloadFixture(m.T(), uint(DefaultMaxUnicastMsgSize)+1000)
	event := &message2.TestMessage{
		Text: string(payload),
	}

	codec := json.NewCodec()
	encodedEvent, err := codec.Encode(event)
	require.NoError(m.T(), err)

	msg.Payload = encodedEvent

	// sends a direct message from first node to the last node
	err = m.mws[firstNode].SendDirect(msg, m.ids[lastNode])
	require.Error(m.Suite.T(), err)
}

// TestMaxMessageSize_Publish evaluates that invoking Publish method of the middleware on a message
// size beyond the permissible publish message size returns an error.
func (m *MiddlewareTestSuite) TestMaxMessageSize_Publish() {
	firstNode := 0
	lastNode := m.size - 1

	msg := createMessage(m.ids[firstNode], m.ids[lastNode], "")
	// adds another node as the target id to imitate publishing
	msg.TargetIDs = append(msg.TargetIDs, m.ids[lastNode-1][:])

	// creates a network payload beyond the maximum message size
	// Note: NetworkPayloadFixture considers 1000 bytes as the overhead of the encoded message,
	// so the generated payload is 1000 bytes below the maximum publish message size.
	// We hence add up 1000 bytes to the input of network payload fixture to make
	// sure that payload is beyond the permissible size.
	payload := NetworkPayloadFixture(m.T(), uint(DefaultMaxPubSubMsgSize)+1000)
	event := &message2.TestMessage{
		Text: string(payload),
	}

	codec := json.NewCodec()
	encodedEvent, err := codec.Encode(event)
	require.NoError(m.T(), err)

	msg.Payload = encodedEvent

	// sends a direct message from first node to the last node
	err = m.mws[firstNode].Publish(msg, testChannel)
	require.Error(m.Suite.T(), err)
}

// TestUnsubscribe tests that an engine can unsubscribe from a topic it was earlier subscribed to and stop receiving
// messages
func (m *MiddlewareTestSuite) TestUnsubscribe() {

	// initially subscribe the nodes to the channel
	for _, mw := range m.mws {
		err := mw.Subscribe(testChannel)
		require.NoError(m.Suite.T(), err)
	}

	// wait for nodes to form a mesh
	time.Sleep(2 * time.Second)

	origin := 0
	target := m.size - 1

	originID := m.ids[origin]
	message1 := createMessage(m.ids[origin], m.ids[target], "hello1")

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
	message2 := createMessage(m.ids[origin], m.ids[target], "hello2")
	err = m.mws[origin].Publish(message2, testChannel)
	assert.NoError(m.T(), err)

	// assert that the new message is not received by the target node
	assert.Never(m.T(), func() bool {
		return !m.ov[target].AssertNumberOfCalls(m.T(), "Receive", 1)
	}, 2*time.Second, time.Millisecond)
}

// createMiddelwares creates middlewares with mock overlay for each middleware
func (m *MiddlewareTestSuite) createMiddleWares(count int) ([]flow.Identifier, []*Middleware) {
	var mws []*Middleware
	var ids []flow.Identifier

	// creates the middlewares
	for i := 0; i < count; i++ {
		// generating ids of the nodes
		// as [32]byte{(i+1),0,...,0}
		var target [32]byte
		target[0] = byte(i + 1)
		targetID := flow.Identifier(target)
		ids = append(ids, targetID)

		// generates logger and coder of the nodes
		logger := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
		codec := json.NewCodec()

		key := m.generateNetworkingKey(target[:])

		// creates new middleware (with an arbitrary genesis block id)
		mw, err := NewMiddleware(logger,
			codec,
			"0.0.0.0:0",
			targetID,
			key,
			m.metrics,
			DefaultMaxUnicastMsgSize,
			DefaultMaxPubSubMsgSize,
			rootID)
		require.NoError(m.Suite.T(), err)

		mws = append(mws, mw)
	}

	// create the mock overlay (i.e., network) for each middleware
	for i := 0; i < count; i++ {
		overlay := &mock.Overlay{}
		m.ov = append(m.ov, overlay)
	}

	return ids, mws
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

// generateNetworkingKey generates a Flow ECDSA key using the given seed
func (m *MiddlewareTestSuite) generateNetworkingKey(seed []byte) crypto.PrivateKey {
	s := make([]byte, crypto.KeyGenSeedMinLenECDSASecp256k1)
	copy(s, seed)
	prvKey, err := crypto.GeneratePrivateKey(crypto.ECDSASecp256k1, s)
	require.NoError(m.Suite.T(), err)
	return prvKey
}
