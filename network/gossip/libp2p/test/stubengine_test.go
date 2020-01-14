package test

import (
	"fmt"
	"testing"
	"time"

	golog "github.com/ipfs/go-log"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	mockery "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	gologging "github.com/whyrusleeping/go-logging"

	"github.com/dapperlabs/flow-go/model/flow"
	libp2pmodel "github.com/dapperlabs/flow-go/model/libp2p"
	"github.com/dapperlabs/flow-go/module/mock"
	"github.com/dapperlabs/flow-go/network/codec/json"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
)

// StubEngineTestSuite tests the correctness of the entire pipeline of network -> middleware -> libp2p
// protocol stack. It creates two instances of a stubengine, connects them through network, and sends a
// single message from one engine to the other one.
type StubEngineTestSuite struct {
	suite.Suite
	nets []*libp2p.Network    // used to keep track of the networks
	mws  []*libp2p.Middleware // used to keep track of the middlewares associated with networks
	ids  []flow.Identifier    // used to keep track of the identifiers associated with networks
}

// TestStubEngineTestSuite runs all the test methods in this test suit
func TestStubEngineTestSuite(t *testing.T) {
	suite.Run(t, new(StubEngineTestSuite))
}

func (s *StubEngineTestSuite) SetupTest() {
	const count = 2
	golog.SetAllLoggers(gologging.INFO)
	s.ids = s.createIDs(count)
	s.mws = s.createMiddleware(s.ids)
	s.nets = s.createNetworks(s.mws, s.ids)
}

// TestSingleMessage sends a single message from one network instance to the other one
// it evaluates the correctness of implementation against correct delivery of the message.
func (s *StubEngineTestSuite) TestSingleMessage() {
	// test engine1
	te1 := &StubEngine{
		t: s.Suite.T(),
	}
	c1, err := s.nets[0].Register(1, te1)
	require.NoError(s.Suite.T(), err)

	// test engine 2
	te2 := &StubEngine{
		t:        s.Suite.T(),
		event:    make(chan interface{}, 1),
		received: make(chan struct{}, 1),
	}

	_, err = s.nets[1].Register(1, te2)
	require.NoError(s.Suite.T(), err)

	// Send the message to node 2 using the conduit of node 1
	event := &libp2pmodel.Echo{
		Text: "hello",
	}
	require.NoError(s.Suite.T(), c1.Submit(event, s.ids[1]))

	select {
	case <-te2.received:
		// evaluates reception of message at the other side
		// does not evaluate the content
		require.NotNil(s.Suite.T(), te2.originID)
		require.NotNil(s.Suite.T(), te2.event)
		assert.Equal(s.Suite.T(), s.ids[0], te2.originID)

		// evaluates proper reception of event
		// casts the received event at the receiver side
		rcvEvent, ok := (<-te2.event).(*libp2pmodel.Echo)
		// evaluates correctness of casting
		require.True(s.Suite.T(), ok)
		// evaluates content of received message
		assert.Equal(s.Suite.T(), event, rcvEvent)

	case <-time.After(10 * time.Second):
		assert.Fail(s.Suite.T(), "peer 1 failed to send a message to peer 2")
	}
}

// TestMultiMessageSync sends a multiple messages from one network instance to the other one
// it evaluates the correctness of implementation against correct delivery of the messages.
func (s *StubEngineTestSuite) TestMultiMessageSync() {
	// count defines number of messages
	count := 10
	// test engine1
	te1 := &StubEngine{
		t: s.Suite.T(),
	}
	c1, err := s.nets[0].Register(1, te1)
	require.NoError(s.Suite.T(), err)

	// test engine 2
	te2 := &StubEngine{
		t:        s.Suite.T(),
		event:    make(chan interface{}, count),
		received: make(chan struct{}, count),
	}

	_, err = s.nets[1].Register(1, te2)
	require.NoError(s.Suite.T(), err)

	for i := 0; i < count; i++ {
		// Send the message to node 2 using the conduit of node 1
		event := &libp2pmodel.Echo{
			Text: fmt.Sprintf("hello%d", i),
		}
		require.NoError(s.Suite.T(), c1.Submit(event, s.ids[1]))

		select {
		case <-te2.received:
			// evaluates reception of message at the other side
			// does not evaluate the content
			require.NotNil(s.Suite.T(), te2.originID)
			require.NotNil(s.Suite.T(), te2.event)
			assert.Equal(s.Suite.T(), s.ids[0], te2.originID)

			// evaluates proper reception of event
			// casts the received event at the receiver side
			rcvEvent, ok := (<-te2.event).(*libp2pmodel.Echo)
			// evaluates correctness of casting
			require.True(s.Suite.T(), ok)
			// evaluates content of received message
			assert.Equal(s.Suite.T(), event, rcvEvent)

		case <-time.After(2 * time.Second):
			assert.Fail(s.Suite.T(), "peer 1 failed to send a message to peer 2")
		}
	}
}

// create ids creates and initializes count-many flow identifiers instances
func (s *StubEngineTestSuite) createIDs(count int) []flow.Identifier {
	ids := make([]flow.Identifier, 0)
	for i := 0; i < count; i++ {
		// defining id of node
		var nodeID [32]byte
		nodeID[0] = byte(i + 1)
		ID := flow.Identifier(nodeID)
		ids = append(ids, ID)
	}
	return ids
}

// create middleware receives an ids slice and creates and initializes a middleware instances for each id
func (s *StubEngineTestSuite) createMiddleware(ids []flow.Identifier) []*libp2p.Middleware {
	count := len(ids)
	mws := make([]*libp2p.Middleware, 0)
	for i := 0; i < count; i++ {
		// creating middleware of nodes
		mw, err := libp2p.NewMiddleware(zerolog.Logger{}, json.NewCodec(), uint(count-1), "0.0.0.0:0", ids[i])
		require.NoError(s.Suite.T(), err)
		mws = append(mws, mw)
	}
	return mws
}

// createNetworks receives a slice of middlewares their associated flow identifiers,
// and for each middleware creates a network instance on top
// it returns the slice of created middlewares
func (s *StubEngineTestSuite) createNetworks(mws []*libp2p.Middleware, ids []flow.Identifier) []*libp2p.Network {
	count := len(mws)
	nets := make([]*libp2p.Network, 0)

	for i := 0; i < count; i++ {
		// retrieves IP and port of the middleware
		ip, port := mws[(i+1)%count].GetIPPort()

		// mocks an identity for the middleware
		//
		targetID := flow.Identity{
			NodeID:  ids[(i+1)%count],
			Address: fmt.Sprintf("%s:%s", ip, port),
			Role:    flow.RoleCollection,
		}

		// creates and mocks the state
		state := &protocol.State{}
		snapshot := &protocol.Snapshot{}
		state.On("Final").Return(snapshot).Once()
		snapshot.On("Identities", mockery.Anything).Return(flow.IdentityList{targetID}, nil).Once()

		// creates and mocks me
		// creating network of node-1
		me := &mock.Local{}
		me.On("NodeID").Return(ids[i])
		net, err := libp2p.NewNetwork(zerolog.Logger{}, json.NewCodec(), state, me, mws[i])
		require.NoError(s.Suite.T(), err)

		nets = append(nets, net)

		// starts the middlewares
		done := net.Ready()
		<-done
		time.Sleep(1 * time.Second)
	}

	return nets
}
