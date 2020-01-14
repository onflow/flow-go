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
	golog.SetAllLoggers(gologging.INFO)
	s.nets = make([]*libp2p.Network, 0)
	s.mws = make([]*libp2p.Middleware, 0)
	s.ids = make([]flow.Identifier, 0)
}

// TestSingleMessage sends a single message from one network instance to the other one
// it evaluates the correctness of implementation against correct delivery of the message.
func (s *StubEngineTestSuite) TestSingleMessage() {
	const count = 2

	for i := 0; i < count; i++ {
		// defining id of node
		var nodeID [32]byte
		nodeID[0] = byte(i + 1)
		ID := flow.Identifier(nodeID)
		s.ids = append(s.ids, ID)

		// creating middleware of nodes
		mw, err := libp2p.NewMiddleware(zerolog.Logger{}, json.NewCodec(), count-1, "0.0.0.0:0", s.ids[i])
		require.NoError(s.Suite.T(), err)
		s.mws = append(s.mws, mw)
	}

	for i := 0; i < count; i++ {
		ip, port := s.mws[(i+1)%count].GetIPPort()
		// mocks an identity
		targetID := flow.Identity{
			NodeID:  s.ids[(i+1)%count],
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
		me.On("NodeID").Return(s.ids[i])
		net, err := libp2p.NewNetwork(zerolog.Logger{}, json.NewCodec(), state, me, s.mws[i])
		require.NoError(s.Suite.T(), err)

		s.nets = append(s.nets, net)

		done := net.Ready()
		<-done
		time.Sleep(1 * time.Second)
	}

	// test engine1
	te1 := &StubEngine{
		t: s.Suite.T(),
	}
	c1, err := s.nets[0].Register(1, te1)
	require.NoError(s.Suite.T(), err)

	// test engine 2
	te2 := &StubEngine{
		t:        s.Suite.T(),
		received: make(chan struct{}),
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
		rcvEvent, ok := te2.event.(*libp2pmodel.Echo)
		// evaluates correctness of casting
		require.True(s.Suite.T(), ok)
		// evaluates content of received message
		assert.Equal(s.Suite.T(), event, rcvEvent)

	case <-time.After(10 * time.Second):
		assert.Fail(s.Suite.T(), "peer 1 failed to send a message to peer 2")
	}
}
