package libp2p

import (
	"context"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network/codec/json"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/middleware"
)

// StubEngine is a simple engine that is used for testing the correctness of
// driving the engines with libp2p
type StubEngine struct {
	t        *testing.T
	net      Network         // used to communicate with the network layer
	originID flow.Identifier // used to keep track of the source originID of the events
	event    interface{}     // used to keep track of the events that the node receives
	received chan struct{}   // used as an indicator on reception of messages for testing
}

type StubEngineTestSuite struct {
	LibP2PNodeTestSuite
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

// SetupTests initiates the test setups prior to each test
func (s *StubEngineTestSuite) SetupTest() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
}

// TestLibP2PNode_P2P tests end-to-end a P2P message sending and receiving between two nodes
func (s *StubEngineTestSuite) TestLibP2PNodeP2P() {
	// cancelling the context of test suite
	defer s.cancel()

	targetID1 := flow.Identifier(byte(0))
	mw1, err := middleware.New(zerolog.Logger{}, json.NewCodec(), uint(0), "0.0.0.0:0", targetID1)
	require.NoError(s.Suite.T(), err)

	_, err = NewNetwork(zerolog.Logger{}, json.NewCodec(), nil, nil, mw1)
	require.NoError(s.Suite.T(), err)

	//// Peer 1 will be sending a message to Peer 2
	//peer1 := nodes[0]
	//peer2 := nodes[1]
	//
	//// Get actual ip and port numbers on which the node starts
	//// for node 2
	//ip2, port2 := peer2.GetIPPort()
	//id2 := NodeAddress{
	//	name: peer2.name,
	//	ip:   ip2,
	//	port: port2,
	//}
	//
	//// Add the second node as a peer to the first node
	//require.NoError(s.Suite.T(), peer1.AddPeers(s.ctx, []NodeAddress{id2}))
	//
	//// Create and register engines for each of the nodes
	//// test engine1
	//te1 := &StubEngine{
	//	t: s.Suite.T(),
	//}
	//conduit1, err := peer1.Register(1, te1)
	//require.NoError(s.Suite.T(), err)
	//
	//// test engine 2
	//te2 := &StubEngine{
	//	t:        s.Suite.T(),
	//	received: make(chan struct{}),
	//}
	//_, err = peer2.Register(1, te2)
	//require.NoError(s.Suite.T(), err)
	//
	//// Generates node2 Flow Identifier
	//// Create target byte array from the node name "node2" -> []byte
	//var target [32]byte
	//copy(target[:], id2.name)
	//targetID := flow.Identifier(target)
	//
	//// Send the message to peer 2 using the conduit of peer 1
	//require.NoError(s.Suite.T(), conduit1.Submit([]byte("hello"), targetID))
	//
	//select {
	//case <-te2.received:
	//	// Asserts that the message was received by peer 2
	//	require.NotNil(s.Suite.T(), te2.originID)
	//	require.NotNil(s.Suite.T(), te2.event)
	//	senderID := bytes.Trim(te2.originID[:], "\x00")
	//	senderIDStr := string(senderID)
	//	assert.Equal(s.Suite.T(), peer1.name, senderIDStr)
	//	assert.Equal(s.Suite.T(), "hello", fmt.Sprintf("%s", te2.event))
	//case <-time.After(3 * time.Second):
	//	assert.Fail(s.Suite.T(), "peer 1 failed to send a message to peer 2")
	//}
}
