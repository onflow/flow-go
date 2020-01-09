package middleware

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
	m.ids, m.mws = m.createAndStartMiddleWares(m.size)
	require.Len(m.Suite.T(), m.ids, m.size)
	require.Len(m.Suite.T(), m.mws, m.size)
}

// TestSendAndReceive sends a single message from one middleware to the other
// middleware, and checks the reception
func (m *MiddlewareTestSuit) TestSendAndReceive() {
	msg := []byte("hello")
	err := m.mws[0].Send(m.ids[m.size-1], msg)
	require.NoError(m.Suite.T(), err)

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
		m.ov[i].On("Identity").Return(flowID, nil).Once()

		// mocks Overlay.Receive for  middleware.Overlay.Receive(*nodeID, payload)
		m.ov[i].On("Receive", mockery.Anything).Return(nil).Once()
	}

	// starting the middleware
	for i := 0; i < m.size; i++ {
		m.mws[i].Start(m.ov[i])
		time.Sleep(1 * time.Second)
	}

	// evaluates the mock calls
	for i := 0; i < m.size; i++ {
		m.ov[i].AssertExpectations(m.T())
	}
}

func (m *MiddlewareTestSuit) createAndStartMiddleWares(count int) ([]flow.Identifier, []*Middleware) {
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

	// mocks an overlay (i.e., network) for each middleware
	for i := 0; i < count; i++ {
		overlay := &mock.Overlay{}
		//// mocks Overlay.Identity for middleware.Overlay.Identity()
		//overlay.On("Identity").Return(flowID, nil).Once()
		//
		//// mocks Overlay.Receive for  middleware.Overlay.Receive(*nodeID, payload)
		//overlay.On("Receive", mockery.Anything).Return(nil).Once()
		m.ov = append(m.ov, overlay)
	}

	return ids, mws
}
