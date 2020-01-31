package test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	golog "github.com/ipfs/go-log"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	gologging "github.com/whyrusleeping/go-logging"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/libp2p/message"
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
	ids  flow.IdentityList    // used to keep track of the identifiers associated with networks
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

// TestSingleMessage tests sending a single message from sender to receiver
func (s *StubEngineTestSuite) TestSingleMessage() {
	// set to false for no echo expectation
	s.singleMessage(false)
}

// TestSingleMessage tests sending a single message from sender to receiver
// it also evaluates the correct reception of an echo message back
func (s *StubEngineTestSuite) TestSingleEcho() {
	// set to true for an echo expectation
	s.singleMessage(true)
}

// TestMultiMsgSync tests sending multiple messages from sender to receiver
// sender and receiver are synced over reception
func (s *StubEngineTestSuite) TestMultiMsgSync() {
	// set to false for no echo expectation
	s.multiMessageSync(false, 10)
}

// TestEchoMultiMsgSync tests sending multiple messages from sender to receiver
// it also evaluates the correct reception of an echo message back for each send
// sender and receiver are synced over reception
func (s *StubEngineTestSuite) TestEchoMultiMsgSync() {
	// set to true for an echo expectation
	s.multiMessageSync(true, 10)
}

// TestMultiMsgAsync tests sending multiple messages from sender to receiver
// sender and receiver are not synchronized
func (s *StubEngineTestSuite) TestMultiMsgAsync() {
	// set to false for no echo expectation
	s.multiMessageAsync(false, 10)
}

// TestEchoMultiMsgAsync tests sending multiple messages from sender to receiver
// it also evaluates the correct reception of an echo message back for each send
// sender and receiver are not synchronized
func (s *StubEngineTestSuite) TestEchoMultiMsgAsync() {
	// set to true for an echo expectation
	s.multiMessageAsync(true, 10)
}

// SingleMessage sends a single message from one network instance to the other one
// it evaluates the correctness of implementation against correct delivery of the message.
// in case echo is true, it also evaluates correct reception of the echo message from the receiver side
func (s *StubEngineTestSuite) singleMessage(echo bool) {
	sndID := 0
	rcvID := 1
	// test engine1
	sender := NewEchoEngine(s.Suite.T(), s.nets[sndID], 1, 1)

	// test engine 2
	receiver := NewEchoEngine(s.Suite.T(), s.nets[rcvID], 1, 1)

	// Send the message to node 2 using the conduit of node 1
	event := &message.Echo{
		Text: "hello",
	}
	require.NoError(s.Suite.T(), sender.con.Submit(event, s.ids[rcvID].NodeID))

	// evaluates reception of echo request
	select {
	case <-receiver.received:
		// evaluates reception of message at the other side
		// does not evaluate the content
		require.NotNil(s.Suite.T(), receiver.originID)
		require.NotNil(s.Suite.T(), receiver.event)
		assert.Equal(s.Suite.T(), s.ids[sndID].NodeID, receiver.originID)

		// evaluates proper reception of event
		// casts the received event at the receiver side
		rcvEvent, ok := (<-receiver.event).(*message.Echo)
		// evaluates correctness of casting
		require.True(s.Suite.T(), ok)
		// evaluates content of received message
		assert.Equal(s.Suite.T(), event, rcvEvent)

	case <-time.After(10 * time.Second):
		assert.Fail(s.Suite.T(), "sender failed to send a message to receiver")
	}

	// evaluates echo back
	if echo {
		// evaluates reception of echo response
		select {
		case <-sender.received:
			// evaluates reception of message at the other side
			// does not evaluate the content
			require.NotNil(s.Suite.T(), sender.originID)
			require.NotNil(s.Suite.T(), sender.event)
			assert.Equal(s.Suite.T(), s.ids[rcvID].NodeID, sender.originID)

			// evaluates proper reception of event
			// casts the received event at the receiver side
			rcvEvent, ok := (<-sender.event).(*message.Echo)
			// evaluates correctness of casting
			require.True(s.Suite.T(), ok)
			// evaluates content of received message
			echoEvent := &message.Echo{
				Text: fmt.Sprintf("%s: %s", receiver.echomsg, event.Text),
			}
			assert.Equal(s.Suite.T(), echoEvent, rcvEvent)

		case <-time.After(10 * time.Second):
			assert.Fail(s.Suite.T(), "receiver failed to send an echo message back to sender")
		}
	}
}

// multiMessageSync sends a multiple messages from one network instance to the other one
// it evaluates the correctness of implementation against correct delivery of the messages.
// sender and receiver are sync over reception, i.e., sender sends one message at a time and
// waits for its reception
// count defines number of messages
func (s *StubEngineTestSuite) multiMessageSync(echo bool, count int) {
	sndID := 0
	rcvID := 1
	// test engine1
	sender := NewEchoEngine(s.Suite.T(), s.nets[sndID], count, 1)

	// test engine 2
	receiver := NewEchoEngine(s.Suite.T(), s.nets[rcvID], count, 1)

	for i := 0; i < count; i++ {
		// Send the message to receiver
		event := &message.Echo{
			Text: fmt.Sprintf("hello%d", i),
		}
		require.NoError(s.Suite.T(), sender.con.Submit(event, s.ids[rcvID].NodeID))

		select {
		case <-receiver.received:
			// evaluates reception of message at the other side
			// does not evaluate the content
			require.NotNil(s.Suite.T(), receiver.originID)
			require.NotNil(s.Suite.T(), receiver.event)
			assert.Equal(s.Suite.T(), s.ids[sndID].NodeID, receiver.originID)

			// evaluates proper reception of event
			// casts the received event at the receiver side
			rcvEvent, ok := (<-receiver.event).(*message.Echo)
			// evaluates correctness of casting
			require.True(s.Suite.T(), ok)
			// evaluates content of received message
			assert.Equal(s.Suite.T(), event, rcvEvent)

		case <-time.After(2 * time.Second):
			assert.Fail(s.Suite.T(), "sender failed to send a message to receiver")
		}

		// evaluates echo back
		if echo {
			// evaluates reception of echo response
			select {
			case <-sender.received:
				// evaluates reception of message at the other side
				// does not evaluate the content
				require.NotNil(s.Suite.T(), sender.originID)
				require.NotNil(s.Suite.T(), sender.event)
				assert.Equal(s.Suite.T(), s.ids[rcvID].NodeID, sender.originID)

				// evaluates proper reception of event
				// casts the received event at the receiver side
				rcvEvent, ok := (<-sender.event).(*message.Echo)
				// evaluates correctness of casting
				require.True(s.Suite.T(), ok)
				// evaluates content of received message
				echoEvent := &message.Echo{
					Text: fmt.Sprintf("%s: %s", receiver.echomsg, event.Text),
				}
				assert.Equal(s.Suite.T(), echoEvent, rcvEvent)

			case <-time.After(10 * time.Second):
				assert.Fail(s.Suite.T(), "receiver failed to send an echo message back to sender")
			}
		}

	}

}

// multiMessageAsync sends a multiple messages from one network instance to the other one
// it evaluates the correctness of implementation against correct delivery of the messages.
// sender and receiver are async, i.e., sender sends all its message at blast
// count defines number of messages
func (s *StubEngineTestSuite) multiMessageAsync(echo bool, count int) {
	sndID := 0
	rcvID := 1
	// test engine1
	sender := NewEchoEngine(s.Suite.T(), s.nets[sndID], count, 1)

	// test engine 2
	receiver := NewEchoEngine(s.Suite.T(), s.nets[rcvID], count, 1)

	// keeps track of async received messages at receiver side
	received := make(map[string]struct{})

	// keeps track of async received echo messages at sender side
	// echorcv := make(map[string]struct{})

	for i := 0; i < count; i++ {
		// Send the message to node 2 using the conduit of node 1
		event := &message.Echo{
			Text: fmt.Sprintf("hello%d", i),
		}
		require.NoError(s.Suite.T(), sender.con.Submit(event, s.ids[1].NodeID))
	}

	for i := 0; i < count; i++ {
		select {
		case <-receiver.received:
			// evaluates reception of message at the other side
			// does not evaluate the content
			require.NotNil(s.Suite.T(), receiver.originID)
			require.NotNil(s.Suite.T(), receiver.event)
			assert.Equal(s.Suite.T(), s.ids[0].NodeID, receiver.originID)

			// evaluates proper reception of event
			// casts the received event at the receiver side
			rcvEvent, ok := (<-receiver.event).(*message.Echo)
			// evaluates correctness of casting
			require.True(s.Suite.T(), ok)

			// evaluates content of received message
			// the content should not yet received and be unique
			_, rcv := received[rcvEvent.Text]
			assert.False(s.Suite.T(), rcv)
			// marking event as received
			received[rcvEvent.Text] = struct{}{}

		case <-time.After(2 * time.Second):
			assert.Fail(s.Suite.T(), "sender failed to send a message to receiver")
		}
	}

	for i := 0; i < count; i++ {
		// evaluates echo back
		if echo {
			// evaluates reception of echo response
			select {
			case <-sender.received:
				// evaluates reception of message at the other side
				// does not evaluate the content
				require.NotNil(s.Suite.T(), sender.originID)
				require.NotNil(s.Suite.T(), sender.event)
				assert.Equal(s.Suite.T(), s.ids[rcvID].NodeID, sender.originID)

				// evaluates proper reception of event
				// casts the received event at the receiver side
				rcvEvent, ok := (<-sender.event).(*message.Echo)
				// evaluates correctness of casting
				require.True(s.Suite.T(), ok)
				// evaluates content of received echo message
				// the content should not yet received and be unique
				_, rcv := received[rcvEvent.Text]
				assert.False(s.Suite.T(), rcv)
				// echo messages should start with prefix msg of receiver that echos back
				assert.True(s.Suite.T(), strings.HasPrefix(rcvEvent.Text, receiver.echomsg))
				// marking echo event as received
				received[rcvEvent.Text] = struct{}{}

			case <-time.After(10 * time.Second):
				assert.Fail(s.Suite.T(), "receiver failed to send an echo message back to sender")
			}
		}
	}

}

// create ids creates and initializes count-many flow identifiers instances
func (s *StubEngineTestSuite) createIDs(count int) []*flow.Identity {
	identities := make([]*flow.Identity, 0)
	for i := 0; i < count; i++ {
		// defining id of node
		var nodeID [32]byte
		nodeID[0] = byte(i + 1)
		identity := &flow.Identity{
			NodeID: nodeID,
		}
		identities = append(identities, identity)
	}
	return identities
}

// create middleware receives an ids slice and creates and initializes a middleware instances for each id
func (s *StubEngineTestSuite) createMiddleware(identities []*flow.Identity) []*libp2p.Middleware {
	count := len(identities)
	mws := make([]*libp2p.Middleware, 0)
	for i := 0; i < count; i++ {
		// creating middleware of nodes
		mw, err := libp2p.NewMiddleware(zerolog.Logger{}, json.NewCodec(), "0.0.0.0:0", identities[i].NodeID)
		require.NoError(s.Suite.T(), err)

		mws = append(mws, mw)
	}
	return mws
}

// createNetworks receives a slice of middlewares their associated flow identifiers,
// and for each middleware creates a network instance on top
// it returns the slice of created middlewares
func (s *StubEngineTestSuite) createNetworks(mws []*libp2p.Middleware, ids flow.IdentityList) []*libp2p.Network {
	count := len(mws)
	nets := make([]*libp2p.Network, 0)

	// creates and mocks the state
	state := &protocol.State{}
	snapshot := &protocol.Snapshot{}

	idLst := &flow.IdentityList{}
	for i := 0; i < count; i++ {
		state.On("Final").Return(snapshot)
		rFun := func() flow.IdentityList {
			fmt.Println(" callleeed ")
			return *idLst
		}
		snapshot.On("Identities").Return(rFun(), nil)
	}

	for i := 0; i < count; i++ {
		// creates and mocks me
		// creating network of node-1
		me := &mock.Local{}
		me.On("NodeID").Return(ids[i].NodeID)
		net, err := libp2p.NewNetwork(zerolog.Logger{}, json.NewCodec(), state, me, mws[i])
		require.NoError(s.Suite.T(), err)

		nets = append(nets, net)

		// starts the middlewares
		done := net.Ready()
		<-done
		// time.Sleep(1 * time.Second)
	}

	for i, m := range mws {
		// retrieves IP and port of the middleware
		ip, port := m.GetIPPort()

		// mocks an identity for the middleware
		id := &flow.Identity{
			NodeID:  ids[i].NodeID,
			Address: fmt.Sprintf("%s:%s", ip, port),
			Role:    flow.RoleCollection,
			Stake:   0,
		}
		*idLst = append(*idLst, id)
	}

	return nets
}
