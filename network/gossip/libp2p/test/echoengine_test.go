package test

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	golog "github.com/ipfs/go-log"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	gologging "github.com/whyrusleeping/go-logging"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/libp2p/message"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p"
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
	s.ids = CreateIDs(count)

	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	mws, err := CreateMiddleware(logger, s.ids)
	require.NoError(s.Suite.T(), err)
	s.mws = mws

	nets, err := CreateNetworks(logger, s.mws, s.ids, 100, false)
	require.NoError(s.Suite.T(), err)
	s.nets = nets
}

// TearDownTest closes the networks within a specified timeout
func (s *StubEngineTestSuite) TearDownTest() {
	for _, net := range s.nets {
		select {
		// closes the network
		case <-net.Done():
			continue
		case <-time.After(3 * time.Second):
			s.Suite.Fail("could not stop the network")
		}
	}
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

// TestDuplicateMessageSequential evaluates the correctness of network layer
// on deduplicating the received messages. Messages are delivered to the receiver
// in a sequential manner.
func (s *StubEngineTestSuite) TestDuplicateMessageSequential() {
	sndID := 0
	rcvID := 1
	// registers engines in the network
	// sender's engine
	sender := NewEchoEngine(s.Suite.T(), s.nets[sndID], 10, engine.ExecutionComputer, false)

	// receiver's engine
	receiver := NewEchoEngine(s.Suite.T(), s.nets[rcvID], 10, engine.ExecutionComputer, false)

	// Sends a message from sender to receiver
	event := &message.Echo{
		Text: fmt.Sprintf("hello"),
	}

	// sends the same message 10 times
	for i := 0; i < 10; i++ {
		require.NoError(s.Suite.T(), sender.con.Submit(event, s.ids[rcvID].NodeID))
	}

	time.Sleep(1 * time.Second)

	// receiver should only see the message once, and the rest should be dropped due to
	// duplication
	require.Equal(s.Suite.T(), 1, receiver.seen[event.Text])
	require.Len(s.Suite.T(), receiver.seen, 1)
}

// TestDuplicateMessageParallel evaluates the correctness of network layer
// on deduplicating the received messages. Messages are delivered to the receiver
// in parallel.
func (s *StubEngineTestSuite) TestDuplicateMessageParallel() {
	sndID := 0
	rcvID := 1
	// registers engines in the network
	// sender's engine
	sender := NewEchoEngine(s.Suite.T(), s.nets[sndID], 10, engine.ExecutionComputer, false)

	// receiver's engine
	receiver := NewEchoEngine(s.Suite.T(), s.nets[rcvID], 10, engine.ExecutionComputer, false)

	// Sends a message from sender to receiver
	event := &message.Echo{
		Text: fmt.Sprintf("hello"),
	}

	// sends the same message 10 times
	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			require.NoError(s.Suite.T(), sender.con.Submit(event, s.ids[rcvID].NodeID))
		}()
	}
	wg.Wait()
	time.Sleep(1 * time.Second)

	// receiver should only see the message once, and the rest should be dropped due to
	// duplication
	require.Equal(s.Suite.T(), 1, receiver.seen[event.Text])
	require.Len(s.Suite.T(), receiver.seen, 1)
}

// TestDuplicateMessageDifferentChan evaluates the correctness of network layer
// on deduplicating the received messages against different engine ids. In specific, the
// desire behavior is that the deduplication should happen based on both eventID and channelID
func (s *StubEngineTestSuite) TestDuplicateMessageDifferentChan() {
	s.T().Skip()
	const (
		sndNode = iota
		rcvNode
	)
	const (
		channel1 = engine.ExecutionReceiptProvider
		channel2 = engine.ExecutionComputer
	)
	// registers engines in the network
	// first type
	// sender's engine
	sender1 := NewEchoEngine(s.Suite.T(), s.nets[sndNode], 10, channel1, false)

	// receiver's engine
	receiver1 := NewEchoEngine(s.Suite.T(), s.nets[rcvNode], 10, channel1, false)

	// second type
	// registers engines in the network
	// sender's engine
	sender2 := NewEchoEngine(s.Suite.T(), s.nets[sndNode], 10, channel2, false)

	// receiver's engine
	receiver2 := NewEchoEngine(s.Suite.T(), s.nets[rcvNode], 10, channel2, false)

	// Sends a message from sender to receiver
	event := &message.Echo{
		Text: fmt.Sprintf("hello"),
	}

	// sends the same message 10 times on both channels
	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// sender1 to receiver1 on channel1
			require.NoError(s.Suite.T(), sender1.con.Submit(event, s.ids[rcvNode].NodeID))

			// sender2 to receiver2 on channel2
			require.NoError(s.Suite.T(), sender2.con.Submit(event, s.ids[rcvNode].NodeID))
		}()
	}
	wg.Wait()
	time.Sleep(1 * time.Second)

	// each receiver should only see the message once, and the rest should be dropped due to
	// duplication
	require.Equal(s.Suite.T(), 1, receiver1.seen[event.Text])
	require.Equal(s.Suite.T(), 1, receiver2.seen[event.Text])

	require.Len(s.Suite.T(), receiver1.seen, 1)
	require.Len(s.Suite.T(), receiver2.seen, 1)
}

// singleMessage sends a single message from one network instance to the other one
// it evaluates the correctness of implementation against correct delivery of the message.
// in case echo is true, it also evaluates correct reception of the echo message from the receiver side
func (s *StubEngineTestSuite) singleMessage(echo bool) {
	sndID := 0
	rcvID := 1

	// registers engines in the network
	// sender's engine
	sender := NewEchoEngine(s.Suite.T(), s.nets[sndID], 10, engine.ExecutionComputer, echo)

	// receiver's engine
	receiver := NewEchoEngine(s.Suite.T(), s.nets[rcvID], 10, engine.ExecutionComputer, echo)

	// Sends a message from sender to receiver
	event := &message.Echo{
		Text: fmt.Sprintf("hello"),
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
	// registers engines in the network
	// sender's engine
	sender := NewEchoEngine(s.Suite.T(), s.nets[sndID], 10, engine.ExecutionComputer, echo)

	// receiver's engine
	receiver := NewEchoEngine(s.Suite.T(), s.nets[rcvID], 10, engine.ExecutionComputer, echo)

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

	// registers engines in the network
	// sender's engine
	sender := NewEchoEngine(s.Suite.T(), s.nets[sndID], 10, engine.ExecutionComputer, echo)

	// receiver's engine
	receiver := NewEchoEngine(s.Suite.T(), s.nets[rcvID], 10, engine.ExecutionComputer, echo)

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
