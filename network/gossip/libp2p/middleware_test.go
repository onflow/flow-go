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
	mockmodule "github.com/dapperlabs/flow-go/module/mock"
	"github.com/dapperlabs/flow-go/network/codec/json"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/message"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/mock"
)

type MiddlewareTestSuit struct {
	suite.Suite
	size    int           // used to determine number of middlewares under test
	mws     []*Middleware // used to keep track of middlewares under test
	ov      []*mock.Overlay
	ids     []flow.Identifier
	metrics *mockmodule.Metrics // mocks performance monitoring metrics
}

// TestMiddlewareTestSuit runs all the test methods in this test suit
func TestMiddlewareTestSuit(t *testing.T) {
	suite.Run(t, new(MiddlewareTestSuit))
}

// SetupTest initiates the test setups prior to each test
func (m *MiddlewareTestSuit) SetupTest() {
	m.size = 2 // operates on two middlewares

	m.metrics = &mockmodule.Metrics{}
	m.metrics.On("NetworkMessageSent", mockery.Anything, mockery.Anything).Return()
	m.metrics.On("NetworkMessageReceived", mockery.Anything, mockery.Anything).Return()

	// create the middlewares
	m.ids, m.mws = m.createMiddleWares(m.size)
	require.Len(m.Suite.T(), m.ids, m.size)
	require.Len(m.Suite.T(), m.mws, m.size)
	// starts the middlewares
	m.StartMiddlewares()
}

func (m *MiddlewareTestSuit) TearDownTest() {
	m.StopMiddlewares()
}

// TestPingRawReception tests the middleware for solely the
// reception of a single ping message by a node that is sent from another node
// it does not evaluate the type and content of the message
func (m *MiddlewareTestSuit) TestPingRawReception() {
	m.Ping(mockery.Anything, mockery.Anything)
}

// TestPingTypeReception tests the middleware against type of received payload
// upon reception at the receiver side
// it does not evaluate content of the payload
// it does not evaluate anything related to the sender id
func (m *MiddlewareTestSuit) TestPingTypeReception() {
	m.Ping(mockery.Anything, mockery.AnythingOfType("*message.Message"))
}

// TestPingIDType tests the middleware against both the type of sender id
// and content of the payload of the event upon reception at the receiver side
// it does not evaluate the actual value of the sender ID
func (m *MiddlewareTestSuit) TestPingIDType() {
	msg := createMessage(m.ids[0], m.ids[1])
	m.Ping(mockery.AnythingOfType("flow.Identifier"), msg)
}

// TestPingContentReception tests the middleware against both
// the payload and sender ID of the event upon reception at the receiver side
func (m *MiddlewareTestSuit) TestPingContentReception() {
	msg := createMessage(m.ids[0], m.ids[1])
	m.Ping(m.mws[0].me, msg)
}

// TestMultiPing tests the middleware against type of received payload
// of distinct messages that are sent concurrently from a node to another
func (m *MiddlewareTestSuit) TestMultiPing() {
	// one distinct message
	m.MultiPing(1)

	// two distinct messages
	m.MultiPing(2)

	// 10 distinct messages
	m.MultiPing(10)
}

// StartMiddleware creates mock overlays for each middleware, and starts the middlewares
func (m *MiddlewareTestSuit) StartMiddlewares() {

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

		ip, port := m.mws[target].GetIPPort()
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
}

// Ping sends a message from the first middleware of the test suit to the last one
// expectID and expectPayload are what we expect the receiver side to evaluate the
// incoming ping against, it can be mocked or typed data
func (m *MiddlewareTestSuit) Ping(expectID, expectPayload interface{}) {

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

	err = m.mws[firstNode].Send(0, msg, m.ids[lastNode])
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
func (m *MiddlewareTestSuit) MultiPing(count int) {
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
			err = m.mws[firstNode].Send(0, msg, m.ids[lastNode])
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
func (m *MiddlewareTestSuit) TestEcho() {

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
			err = m.mws[lastNode].Send(0, replyMsg, m.mws[firstNode].me)
			assert.NoError(m.T(), err)

		})

	// first node
	m.ov[firstNode].On("Receive", m.mws[lastNode].me, replyMsg).Return(nil).Once().
		Run(func(args mockery.Arguments) {
			wg.Done()
		})

	err = m.mws[firstNode].Send(0, sendMsg, m.ids[lastNode])
	require.NoError(m.Suite.T(), err)

	wg.Wait()

	// evaluates the mock calls
	for i := 1; i < m.size; i++ {
		m.ov[i].AssertExpectations(m.T())
	}
}

// createMiddelwares creates middlewares with mock overlay for each middleware
func (m *MiddlewareTestSuit) createMiddleWares(count int) ([]flow.Identifier, []*Middleware) {
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

		// creates new middleware
		mw, err := NewMiddleware(logger, codec, "0.0.0.0:0", targetID, key, m.metrics, DefaultMaxPubSubMsgSize)
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

	message := &message.Message{
		ChannelID: 1,
		EventID:   []byte("1"),
		OriginID:  originID[:],
		TargetIDs: [][]byte{targetID[:]},
		Payload:   []byte(payload),
	}

	return message
}

func (m *MiddlewareTestSuit) StopMiddlewares() {
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
func (m *MiddlewareTestSuit) generateNetworkingKey(seed []byte) crypto.PrivateKey {
	s := make([]byte, crypto.KeyGenSeedMinLenECDSASecp256k1)
	copy(s, seed)
	prvKey, err := crypto.GeneratePrivateKey(crypto.ECDSASecp256k1, s)
	require.NoError(m.Suite.T(), err)
	return prvKey
}
