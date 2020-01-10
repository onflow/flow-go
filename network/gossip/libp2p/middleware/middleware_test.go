package middleware

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

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network/codec/json"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/mock"
)

type MiddlewareTestSuit struct {
	suite.Suite
	size int           // used to determine number of middlewares under test
	mws  []*Middleware // used to keep track of middlewares under test
	ov   []*mock.Overlay
	ids  []flow.Identifier
}

// TestMiddlewareTestSuit runs all the test methods in this test suit
func TestMiddlewareTestSuit(t *testing.T) {
	suite.Run(t, new(MiddlewareTestSuit))
}

// SetupTest initiates the test setups prior to each test
func (m *MiddlewareTestSuit) SetupTest() {
	m.size = 2 // operates on two middlewares
	// create the middlewares
	m.ids, m.mws = m.createMiddleWares(m.size)
	require.Len(m.Suite.T(), m.ids, m.size)
	require.Len(m.Suite.T(), m.mws, m.size)
	// starts the middlewares
	m.StartMiddlewares()
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
	m.Ping(mockery.Anything, mockery.AnythingOfType("[]uint8"))
}

// TestPingTypeReception tests the middleware against type of received payload
// upon reception at the receiver side
// it does not evaluate content of the payload
// it does not evaluate anything related to the sender id
func (m *MiddlewareTestSuit) TestMultiPing() {
	m.MultiPing(1)

	m.MultiPing(100)
}

// TestPingIDType tests the middleware against both the type of sender id
// and content of the payload of the event upon reception at the receiver side
// it does not evaluate the actual value of the sender ID
func (m *MiddlewareTestSuit) TestPingIDType() {
	m.Ping(mockery.AnythingOfType("flow.Identifier"), []byte("Hello, World!"))
}

// TestPingContentReception tests the middleware against both
// the payload and sender ID of the event upon reception at the receiver side
func (m *MiddlewareTestSuit) TestPingContentReception() {
	m.Ping(m.mws[0].me, []byte("Hello, World!"))
}

// StartMiddleware creates mock overlays for each middleware, and starts the middlewares
func (m *MiddlewareTestSuit) StartMiddlewares() {
	// generates and mocks an overlay for each middleware
	for i := 0; i < m.size; i++ {
		target := i + 1
		if i == m.size-1 {
			target = 0
		}
		ip, port := m.mws[target].libP2PNode.GetIPPort()

		// mocks an identity
		flowID := flow.Identity{
			NodeID:  m.ids[target],
			Address: fmt.Sprintf("%s:%s", ip, port),
			Role:    flow.RoleCollection,
		}

		// mocks Overlay.Identity for middleware.Overlay.Identity()
		m.ov[i].On("Identity").Return(flowID, nil)

	}

	// starting the middleware
	for i := 0; i < m.size; i++ {
		m.mws[i].Start(m.ov[i])
		time.Sleep(1 * time.Second)
	}

	for i := 0; i < m.size; i++ {
		m.ov[i].AssertExpectations(m.T())
	}
}

// Ping sends a message from the first middleware of the test suit to the last one
// expectID and expectPayload are what we expect the receiver side to evaluate the
// incoming ping against, it can be mocked or typed data
func (m *MiddlewareTestSuit) Ping(expectID, expectPayload interface{}) {

	ch := make(chan struct{})
	// extracts sender id based on the mock option
	var err error
	// mocks Overlay.Receive for  middleware.Overlay.Receive(*nodeID, payload)
	m.ov[m.size-1].On("Receive", expectID, expectPayload).Return(nil).Once().
		Run(func(args mockery.Arguments) {
			ch <- struct{}{}
		})

	var msg []byte
	switch x := expectPayload.(type) {
	case []byte:
		msg = x
	default:
		msg = []byte("hello")
	}

	err = m.mws[0].Send(m.ids[m.size-1], msg)
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
	for i := 0; i < count; i++ {
		wg.Add(1)
		msg := []byte(fmt.Sprintf("hello from: %d", i))
		m.ov[m.size-1].On("Receive", m.mws[0].me, msg).Return(nil).Once().
			Run(func(args mockery.Arguments) {
				wg.Done()
			})
		go func() {
			err = m.mws[0].Send(m.ids[m.size-1], msg)
			require.NoError(m.Suite.T(), err)
		}()
	}

	wg.Wait()

	// evaluates the mock calls
	for i := 1; i < m.size; i++ {
		m.ov[i].AssertExpectations(m.T())
	}
}

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

		// creates new middleware
		mw, err := New(logger, codec, uint(count-1), "0.0.0.0:0", targetID)
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
