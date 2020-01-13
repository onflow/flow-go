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
	"github.com/dapperlabs/flow-go/model/libp2p"
	"github.com/dapperlabs/flow-go/module/mock"
	"github.com/dapperlabs/flow-go/network/codec/json"
	libp2p2 "github.com/dapperlabs/flow-go/network/gossip/libp2p"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
)

// StubEngine is a simple engine that is used for testing the correctness of
// driving the engines with libp2p
type StubEngine struct {
	t        *testing.T
	net      libp2p2.Network // used to communicate with the network layer
	originID flow.Identifier
	event    interface{}   // used to keep track of the events that the node receives
	received chan struct{} // used as an indicator on reception of messages for testing
}

type Local struct {
	me flow.Identifier
}

func (l Local) NodeID() flow.Identifier {
	return l.me
}

func (l Local) Address() string {
	return ""
}

type StubEngineTestSuite struct {
	suite.Suite
	state *protocol.State
	me    *mock.Local
}

// SubmitLocal is implemented for a valid type assertion to Engine
// any call to it fails the test
func (te *StubEngine) SubmitLocal(event interface{}) {
	require.Fail(te.t, "not implemented")
}

// Submit is implemented for a valid type assertion to Engine
// any call to it fails the test
func (te *StubEngine) Submit(originID flow.Identifier, event interface{}) {
	require.Fail(te.t, "not implemented")
}

// ProcessLocal is implemented for a valid type assertion to Engine
// any call to it fails the test
func (te *StubEngine) ProcessLocal(event interface{}) error {
	require.Fail(te.t, "not implemented")
	return fmt.Errorf(" unexpected method called")
}

// Process receives an originID and an event and casts them into the corresponding fields of the
// StubEngine. It then flags the received channel on reception of an event
func (te *StubEngine) Process(originID flow.Identifier, event interface{}) error {
	te.originID = originID
	te.event = event
	te.received <- struct{}{}
	return nil
}

// TestLibP2PNodesTestSuite runs all the test methods in this test suit
func TestLibP2PNodeTestSuite(t *testing.T) {
	suite.Run(t, new(StubEngineTestSuite))
}

// TestLibP2PNode_P2P tests end-to-end a P2P message sending and receiving between two nodes
func (s *StubEngineTestSuite) TestLibP2PNodeP2P() {

	golog.SetAllLoggers(gologging.INFO)
	// cancelling the context of test suite
	const count = 2

	nets := make([]*libp2p2.Network, 0)
	mws := make([]*libp2p2.Middleware, 0)
	ids := make([]flow.Identifier, 0)

	for i := 0; i < count; i++ {
		// defining id of node
		var nodeID [32]byte
		nodeID[0] = byte(i + 1)
		ID := flow.Identifier(nodeID)
		ids = append(ids, ID)

		// creating middleware of nodes
		mw, err := libp2p2.NewMiddleware(zerolog.Logger{}, json.NewCodec(), count-1, "0.0.0.0:0", ids[i])
		require.NoError(s.Suite.T(), err)
		mws = append(mws, mw)
	}
	for i := 0; i < count; i++ {
		ip, port := mws[(i+1)%count].GetIPPort()
		// mocks an identity
		targetID := flow.Identity{
			NodeID:  ids[(i+1)%count],
			Address: fmt.Sprintf("%s:%s", ip, port),
			Role:    flow.RoleCollection,
		}
		state := &protocol.State{}
		snapshot := &protocol.Snapshot{}
		state.On("Final").Return(snapshot).Once()
		snapshot.On("Identities", mockery.Anything).Return(flow.IdentityList{targetID}, nil).Once()
		// creating network of node-1
		net, err := libp2p2.NewNetwork(zerolog.Logger{}, json.NewCodec(), state, Local{me: ids[i]}, mws[i])
		require.NoError(s.Suite.T(), err)

		nets = append(nets, net)

		done := net.Ready()
		<-done
		time.Sleep(1 * time.Second)
	}

	// Step 4: Waits for nodes to heartbeat each other
	time.Sleep(10 * time.Second)

	// test engine1
	te1 := &StubEngine{
		t: s.Suite.T(),
	}
	c1, err := nets[0].Register(1, te1)
	require.NoError(s.Suite.T(), err)

	// test engine 2
	te2 := &StubEngine{
		t:        s.Suite.T(),
		received: make(chan struct{}),
	}

	_, err = nets[1].Register(1, te2)
	require.NoError(s.Suite.T(), err)

	// Send the message to node 2 using the conduit of node 1
	event := &libp2p.Echo{
		Text: "hello",
	}
	require.NoError(s.Suite.T(), c1.Submit(event, ids[1]))

	select {
	case <-te2.received:
		// evaluates reception of message at the other side
		// does not evaluate the content
		require.NotNil(s.Suite.T(), te2.originID)
		require.NotNil(s.Suite.T(), te2.event)
		assert.Equal(s.Suite.T(), ids[0], te2.originID)

		// evaluates proper reception of event
		// casts the received event at the receiver side
		rcvEvent, ok := te2.event.(*libp2p.Echo)
		// evaluates correctness of casting
		require.True(s.Suite.T(), ok)
		// evaluates content of received message
		assert.Equal(s.Suite.T(), event, rcvEvent)

	case <-time.After(10 * time.Second):
		assert.Fail(s.Suite.T(), "peer 1 failed to send a message to peer 2")
	}
}
