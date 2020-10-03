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

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/network/gossip/libp2p"
)

// EchoEngineTestSuite tests the correctness of the entire pipeline of network -> middleware -> libp2p
// protocol stack. It creates two instances of a stubengine, connects them through network, and sends a
// single message from one engine to the other one through different scenarios.
type EchoEngineTestSuite struct {
	suite.Suite
	ConduitWrapper                      // used as a wrapper around conduit methods
	nets           []*libp2p.Network    // used to keep track of the networks
	mws            []*libp2p.Middleware // used to keep track of the middlewares associated with networks
	ids            flow.IdentityList    // used to keep track of the identifiers associated with networks
}

// Some tests are skipped to speedup the build.
// However, they can be enabled if the environment variable "AllNetworkTest" is set with any value

// TestStubEngineTestSuite runs all the test methods in this test suit
func TestStubEngineTestSuite(t *testing.T) {
	suite.Run(t, new(EchoEngineTestSuite))
}

func (s *EchoEngineTestSuite) SetupTest() {
	const count = 2
	golog.SetAllLoggers(golog.LevelInfo)
	s.ids = CreateIDs(count)

	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	mws, err := createMiddleware(logger, s.ids)
	require.NoError(s.Suite.T(), err)
	s.mws = mws

	nets, err := createNetworks(logger, s.mws, s.ids, 100, false)
	require.NoError(s.Suite.T(), err)
	s.nets = nets
}

// TearDownTest closes the networks within a specified timeout
func (s *EchoEngineTestSuite) TearDownTest() {
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

// TestSingleMessage_Submit tests sending a single message from sender to receiver using
// the Submit method of Conduit.
func (s *EchoEngineTestSuite) TestSingleMessage_Submit() {
	s.skipTest("covered by TestEchoMultiMsgAsync_Submit")
	// set to false for no echo expectation
	s.singleMessage(false, s.Submit)
}

// TestSingleMessage_Publish tests sending a single message from sender to receiver using
// the Publish method of Conduit.
func (s *EchoEngineTestSuite) TestSingleMessage_Publish() {
	s.skipTest("covered by TestEchoMultiMsgAsync_Publish")
	// set to false for no echo expectation
	s.singleMessage(false, s.Publish)
}

// TestSingleMessage_Unicast tests sending a single message from sender to receiver using
// the Unicast method of Conduit.
func (s *EchoEngineTestSuite) TestSingleMessage_Unicast() {
	s.skipTest("covered by TestEchoMultiMsgAsync_Unicast")
	// set to false for no echo expectation
	s.singleMessage(false, s.Unicast)
}

// TestSingleMessage_Multicast tests sending a single message from sender to receiver using
// the Multicast method of Conduit.
func (s *EchoEngineTestSuite) TestSingleMessage_Multicast() {
	s.skipTest("covered by TestEchoMultiMsgAsync_Multicast")
	// set to false for no echo expectation
	s.singleMessage(false, s.Multicast)
}

// TestSingleEcho_Submit tests sending a single message from sender to receiver using
// the Submit method of its Conduit.
// It also evaluates the correct reception of an echo message back.
func (s *EchoEngineTestSuite) TestSingleEcho_Submit() {
	s.skipTest("covered by TestEchoMultiMsgAsync_Submit")
	// set to true for an echo expectation
	s.singleMessage(true, s.Submit)
}

// TestSingleEcho_Publish tests sending a single message from sender to receiver using
// the Publish method of its Conduit.
// It also evaluates the correct reception of an echo message back.
func (s *EchoEngineTestSuite) TestSingleEcho_Publish() {
	s.skipTest("covered by TestEchoMultiMsgAsync_Publish")
	// set to true for an echo expectation
	s.singleMessage(true, s.Publish)
}

// TestSingleEcho_Unicast tests sending a single message from sender to receiver using
// the Unicast method of its Conduit.
// It also evaluates the correct reception of an echo message back.
func (s *EchoEngineTestSuite) TestSingleEcho_Unicast() {
	s.skipTest("covered by TestEchoMultiMsgAsync_Unicast")
	// set to true for an echo expectation
	s.singleMessage(true, s.Unicast)
}

// TestSingleEcho_Multicast tests sending a single message from sender to receiver using
// the Multicast method of its Conduit.
// It also evaluates the correct reception of an echo message back.
func (s *EchoEngineTestSuite) TestSingleEcho_Multicast() {
	s.skipTest("covered by TestEchoMultiMsgAsync_Multicast")
	// set to true for an echo expectation
	s.singleMessage(true, s.Multicast)
}

// TestMultiMsgSync_Submit tests sending multiple messages from sender to receiver
// using the Submit method of its Conduit.
// Sender and receiver are synced over reception.
func (s *EchoEngineTestSuite) TestMultiMsgSync_Submit() {
	s.skipTest("covered by TestEchoMultiMsgAsync_Submit")
	// set to false for no echo expectation
	s.multiMessageSync(false, 10, s.Submit)
}

// TestMultiMsgSync_Publish tests sending multiple messages from sender to receiver
// using the Publish method of its Conduit.
// Sender and receiver are synced over reception.
func (s *EchoEngineTestSuite) TestMultiMsgSync_Publish() {
	s.skipTest("covered by TestEchoMultiMsgAsync_Publish")
	// set to false for no echo expectation
	s.multiMessageSync(false, 10, s.Publish)
}

// TestMultiMsgSync_Unicast tests sending multiple messages from sender to receiver
// using the Unicast method of its Conduit.
// Sender and receiver are synced over reception.
func (s *EchoEngineTestSuite) TestMultiMsgSync_Unicast() {
	s.skipTest("covered by TestEchoMultiMsgAsync_Unicast")
	// set to false for no echo expectation
	s.multiMessageSync(false, 10, s.Unicast)
}

// TestMultiMsgSync_Multicast tests sending multiple messages from sender to receiver
// using the Multicast method of its Conduit.
// Sender and receiver are synced over reception.
func (s *EchoEngineTestSuite) TestMultiMsgSync_Multicast() {
	s.skipTest("covered by TestEchoMultiMsgAsync_Multicast")
	// set to false for no echo expectation
	s.multiMessageSync(false, 10, s.Multicast)
}

// TestEchoMultiMsgSync_Submit tests sending multiple messages from sender to receiver
// using the Submit method of its Conduit.
// It also evaluates the correct reception of an echo message back for each send
// sender and receiver are synced over reception.
func (s *EchoEngineTestSuite) TestEchoMultiMsgSync_Submit() {
	s.skipTest("covered by TestEchoMultiMsgAsync_Submit")
	// set to true for an echo expectation
	s.multiMessageSync(true, 10, s.Submit)
}

// TestEchoMultiMsgSync_Publish tests sending multiple messages from sender to receiver
// using the Publish method of its Conduit.
// It also evaluates the correct reception of an echo message back for each send
// sender and receiver are synced over reception.
func (s *EchoEngineTestSuite) TestEchoMultiMsgSync_Publish() {
	s.skipTest("covered by TestEchoMultiMsgAsync_Publish")
	// set to true for an echo expectation
	s.multiMessageSync(true, 10, s.Publish)
}

// TestEchoMultiMsgSync_Unicast tests sending multiple messages from sender to receiver
// using the Unicast method of its Conduit.
// It also evaluates the correct reception of an echo message back for each send
// sender and receiver are synced over reception.
func (s *EchoEngineTestSuite) TestEchoMultiMsgSync_Unicast() {
	s.skipTest("covered by TestEchoMultiMsgAsync_Unicast")
	// set to true for an echo expectation
	s.multiMessageSync(true, 10, s.Submit)
}

// TestEchoMultiMsgSync_Multicast tests sending multiple messages from sender to receiver
// using the Multicast method of its Conduit.
// It also evaluates the correct reception of an echo message back for each send
// sender and receiver are synced over reception.
func (s *EchoEngineTestSuite) TestEchoMultiMsgSync_Multicast() {
	s.skipTest("covered by TestEchoMultiMsgAsync_Multicast")
	// set to true for an echo expectation
	s.multiMessageSync(true, 10, s.Multicast)
}

// TestMultiMsgAsync_Submit tests sending multiple messages from sender to receiver
// using the Submit method of their Conduit.
// Sender and receiver are not synchronized.
func (s *EchoEngineTestSuite) TestMultiMsgAsync_Submit() {
	s.skipTest("covered by TestEchoMultiMsgAsync_Submit")
	// set to false for no echo expectation
	s.multiMessageAsync(false, 10, s.Submit)
}

// TestMultiMsgAsync_Publish tests sending multiple messages from sender to receiver
// using the Publish method of their Conduit.
// Sender and receiver are not synchronized
func (s *EchoEngineTestSuite) TestMultiMsgAsync_Publish() {
	s.skipTest("covered by TestEchoMultiMsgAsync_Publish")
	// set to false for no echo expectation
	s.multiMessageAsync(false, 10, s.Publish)
}

// TestMultiMsgAsync_Unicast tests sending multiple messages from sender to receiver
// using the Unicast method of their Conduit.
// Sender and receiver are not synchronized
func (s *EchoEngineTestSuite) TestMultiMsgAsync_Unicast() {
	s.skipTest("covered by TestEchoMultiMsgAsync_Unicast")
	// set to false for no echo expectation
	s.multiMessageAsync(false, 10, s.Unicast)
}

// TestMultiMsgAsync_Multicast tests sending multiple messages from sender to receiver
// using the Multicast method of their Conduit.
// Sender and receiver are not synchronized.
func (s *EchoEngineTestSuite) TestMultiMsgAsync_Multicast() {
	s.skipTest("covered by TestEchoMultiMsgAsync_Multicast")
	// set to false for no echo expectation
	s.multiMessageAsync(false, 10, s.Multicast)
}

// TestEchoMultiMsgAsync_Submit tests sending multiple messages from sender to receiver
// using the Submit method of their Conduit.
// It also evaluates the correct reception of an echo message back for each send.
// Sender and receiver are not synchronized
func (s *EchoEngineTestSuite) TestEchoMultiMsgAsync_Submit() {
	// set to true for an echo expectation
	s.multiMessageAsync(true, 10, s.Submit)
}

// TestEchoMultiMsgAsync_Publish tests sending multiple messages from sender to receiver
// using the Publish method of their Conduit.
// It also evaluates the correct reception of an echo message back for each send.
// Sender and receiver are not synchronized
func (s *EchoEngineTestSuite) TestEchoMultiMsgAsync_Publish() {
	// set to true for an echo expectation
	s.multiMessageAsync(true, 10, s.Publish)
}

// TestEchoMultiMsgAsync_Unicast tests sending multiple messages from sender to receiver
// using the Unicast method of their Conduit.
// It also evaluates the correct reception of an echo message back for each send.
// Sender and receiver are not synchronized
func (s *EchoEngineTestSuite) TestEchoMultiMsgAsync_Unicast() {
	// set to true for an echo expectation
	s.multiMessageAsync(true, 10, s.Unicast)
}

// TestEchoMultiMsgAsync_Multicast tests sending multiple messages from sender to receiver
// using the Multicast method of their Conduit.
// It also evaluates the correct reception of an echo message back for each send.
// Sender and receiver are not synchronized
func (s *EchoEngineTestSuite) TestEchoMultiMsgAsync_Multicast() {
	// set to true for an echo expectation
	s.multiMessageAsync(true, 10, s.Multicast)
}

// TestDuplicateMessageSequential_Submit evaluates the correctness of network layer on deduplicating
// the received messages over Submit method of nodes' Conduits.
// Messages are delivered to the receiver in a sequential manner.
func (s *EchoEngineTestSuite) TestDuplicateMessageSequential_Submit() {
	s.skipTest("covered by TestDuplicateMessageParallel_Submit")
	s.duplicateMessageSequential(s.Submit)
}

// TestDuplicateMessageSequential_Publish evaluates the correctness of network layer on deduplicating
// the received messages over Publish method of nodes' Conduits.
// Messages are delivered to the receiver in a sequential manner.
func (s *EchoEngineTestSuite) TestDuplicateMessageSequential_Publish() {
	s.skipTest("covered by TestDuplicateMessageParallel_Publish")
	s.duplicateMessageSequential(s.Publish)
}

// TestDuplicateMessageSequential_Unicast evaluates the correctness of network layer on deduplicating
// the received messages over Unicast method of nodes' Conduits.
// Messages are delivered to the receiver in a sequential manner.
func (s *EchoEngineTestSuite) TestDuplicateMessageSequential_Unicast() {
	s.skipTest("covered by TestDuplicateMessageParallel_Unicast")
	s.duplicateMessageSequential(s.Unicast)
}

// TestDuplicateMessageSequential_Multicast evaluates the correctness of network layer on deduplicating
// the received messages over Multicast method of nodes' Conduits.
// Messages are delivered to the receiver in a sequential manner.
func (s *EchoEngineTestSuite) TestDuplicateMessageSequential_Multicast() {
	s.skipTest("covered by TestDuplicateMessageParallel_Multicast")
	s.duplicateMessageSequential(s.Multicast)
}

// TestDuplicateMessageParallel_Submit evaluates the correctness of network layer
// on deduplicating the received messages via Submit method of nodes' Conduits.
// Messages are delivered to the receiver in parallel via the Submit method of Conduits.
func (s *EchoEngineTestSuite) TestDuplicateMessageParallel_Submit() {
	s.duplicateMessageParallel(s.Submit)
}

// TestDuplicateMessageParallel_Publish evaluates the correctness of network layer
// on deduplicating the received messages via Publish method of nodes' Conduits.
// Messages are delivered to the receiver in parallel via the Publish method of Conduits.
func (s *EchoEngineTestSuite) TestDuplicateMessageParallel_Publish() {
	s.duplicateMessageParallel(s.Publish)
}

// TestDuplicateMessageParallel_Unicast evaluates the correctness of network layer
// on deduplicating the received messages via Unicast method of nodes' Conduits.
// Messages are delivered to the receiver in parallel via the Unicast method of Conduits.
func (s *EchoEngineTestSuite) TestDuplicateMessageParallel_Unicast() {
	s.duplicateMessageParallel(s.Unicast)
}

// TestDuplicateMessageParallel_Multicast evaluates the correctness of network layer
// on deduplicating the received messages via Multicast method of nodes' Conduits.
// Messages are delivered to the receiver in parallel via the Multicast method of Conduits.
func (s *EchoEngineTestSuite) TestDuplicateMessageParallel_Multicast() {
	s.duplicateMessageParallel(s.Multicast)
}

// TestDuplicateMessageDifferentChan_Submit evaluates the correctness of network layer
// on deduplicating the received messages via Submit method of Conduits against different engine ids. In specific, the
// desire behavior is that the deduplication should happen based on both eventID and channelID.
// Messages are sent via the Submit method of the Conduits.
func (s *EchoEngineTestSuite) TestDuplicateMessageDifferentChan_Submit() {
	s.duplicateMessageDifferentChan(s.Submit)
}

// TestDuplicateMessageDifferentChan_Publish evaluates the correctness of network layer
// on deduplicating the received messages against different engine ids. In specific, the
// desire behavior is that the deduplication should happen based on both eventID and channelID.
// Messages are sent via the Publish methods of the Conduits.
func (s *EchoEngineTestSuite) TestDuplicateMessageDifferentChan_Publish() {
	s.duplicateMessageDifferentChan(s.Publish)
}

// TestDuplicateMessageDifferentChan_Unicast evaluates the correctness of network layer
// on deduplicating the received messages against different engine ids. In specific, the
// desire behavior is that the deduplication should happen based on both eventID and channelID.
// Messages are sent via the Unicast methods of the Conduits.
func (s *EchoEngineTestSuite) TestDuplicateMessageDifferentChan_Unicast() {
	s.duplicateMessageDifferentChan(s.Unicast)
}

// TestDuplicateMessageDifferentChan_Multicast evaluates the correctness of network layer
// on deduplicating the received messages against different engine ids. In specific, the
// desire behavior is that the deduplication should happen based on both eventID and channelID.
// Messages are sent via the Multicast methods of the Conduits.
func (s *EchoEngineTestSuite) TestDuplicateMessageDifferentChan_Multicast() {
	s.duplicateMessageDifferentChan(s.Multicast)
}

// duplicateMessageSequential is a helper function that sends duplicate messages sequentially
// from a receiver to the sender via the injected send wrapper function of conduit.
func (s *EchoEngineTestSuite) duplicateMessageSequential(send ConduitSendWrapperFunc) {
	sndID := 0
	rcvID := 1
	// registers engines in the network
	// sender's engine
	sender := NewEchoEngine(s.Suite.T(), s.nets[sndID], 10, engine.TestNetwork, false, send)

	// receiver's engine
	receiver := NewEchoEngine(s.Suite.T(), s.nets[rcvID], 10, engine.TestNetwork, false, send)

	// allow nodes to heartbeat and discover each other if using PubSub
	optionalSleep(send)

	// Sends a message from sender to receiver
	event := &message.TestMessage{
		Text: "hello",
	}

	// sends the same message 10 times
	for i := 0; i < 10; i++ {
		require.NoError(s.Suite.T(), send(event, sender.con, s.ids[rcvID].NodeID))
	}

	time.Sleep(1 * time.Second)

	// receiver should only see the message once, and the rest should be dropped due to
	// duplication
	require.Equal(s.Suite.T(), 1, receiver.seen[event.Text])
	require.Len(s.Suite.T(), receiver.seen, 1)
}

// duplicateMessageParallel is a helper function that sends duplicate messages concurrent;u
// from a receiver to the sender via the injected send wrapper function of conduit.
func (s *EchoEngineTestSuite) duplicateMessageParallel(send ConduitSendWrapperFunc) {
	sndID := 0
	rcvID := 1
	// registers engines in the network
	// sender's engine
	sender := NewEchoEngine(s.Suite.T(), s.nets[sndID], 10, engine.TestNetwork, false, send)

	// receiver's engine
	receiver := NewEchoEngine(s.Suite.T(), s.nets[rcvID], 10, engine.TestNetwork, false, send)

	// allow nodes to heartbeat and discover each other
	optionalSleep(send)

	// Sends a message from sender to receiver
	event := &message.TestMessage{
		Text: "hello",
	}

	// sends the same message 10 times
	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			require.NoError(s.Suite.T(), send(event, sender.con, s.ids[rcvID].NodeID))
		}()
	}
	wg.Wait()
	time.Sleep(1 * time.Second)

	// receiver should only see the message once, and the rest should be dropped due to
	// duplication
	require.Equal(s.Suite.T(), 1, receiver.seen[event.Text])
	require.Len(s.Suite.T(), receiver.seen, 1)
}

// duplicateMessageDifferentChan is a helper function that sends the same message from two distinct
// sender engines to the two distinct receiver engines via the send wrapper function of Conduits.
func (s *EchoEngineTestSuite) duplicateMessageDifferentChan(send ConduitSendWrapperFunc) {
	const (
		sndNode = iota
		rcvNode
	)
	const (
		channel1 = engine.TestNetwork
		channel2 = engine.TestMetrics
	)
	// registers engines in the network
	// first type
	// sender's engine
	sender1 := NewEchoEngine(s.Suite.T(), s.nets[sndNode], 10, channel1, false, send)

	// receiver's engine
	receiver1 := NewEchoEngine(s.Suite.T(), s.nets[rcvNode], 10, channel1, false, send)

	// second type
	// registers engines in the network
	// sender's engine
	sender2 := NewEchoEngine(s.Suite.T(), s.nets[sndNode], 10, channel2, false, send)

	// receiver's engine
	receiver2 := NewEchoEngine(s.Suite.T(), s.nets[rcvNode], 10, channel2, false, send)

	// allow nodes to heartbeat and discover each other
	optionalSleep(send)

	// Sends a message from sender to receiver
	event := &message.TestMessage{
		Text: "hello",
	}

	// sends the same message 10 times on both channels
	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// sender1 to receiver1 on channel1
			require.NoError(s.Suite.T(), send(event, sender1.con, s.ids[rcvNode].NodeID))

			// sender2 to receiver2 on channel2
			require.NoError(s.Suite.T(), send(event, sender2.con, s.ids[rcvNode].NodeID))
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
func (s *EchoEngineTestSuite) singleMessage(echo bool, send ConduitSendWrapperFunc) {
	sndID := 0
	rcvID := 1

	// registers engines in the network
	// sender's engine
	sender := NewEchoEngine(s.Suite.T(), s.nets[sndID], 10, engine.TestNetwork, echo, send)

	// receiver's engine
	receiver := NewEchoEngine(s.Suite.T(), s.nets[rcvID], 10, engine.TestNetwork, echo, send)

	// allow nodes to heartbeat and discover each other
	optionalSleep(send)

	// Sends a message from sender to receiver
	event := &message.TestMessage{
		Text: "hello",
	}
	require.NoError(s.Suite.T(), send(event, sender.con, s.ids[rcvID].NodeID))

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
		rcvEvent, ok := (<-receiver.event).(*message.TestMessage)
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
			rcvEvent, ok := (<-sender.event).(*message.TestMessage)
			// evaluates correctness of casting
			require.True(s.Suite.T(), ok)
			// evaluates content of received message
			echoEvent := &message.TestMessage{
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
func (s *EchoEngineTestSuite) multiMessageSync(echo bool, count int, send ConduitSendWrapperFunc) {
	sndID := 0
	rcvID := 1
	// registers engines in the network
	// sender's engine
	sender := NewEchoEngine(s.Suite.T(), s.nets[sndID], 10, engine.TestNetwork, echo, send)

	// receiver's engine
	receiver := NewEchoEngine(s.Suite.T(), s.nets[rcvID], 10, engine.TestNetwork, echo, send)

	// allow nodes to heartbeat and discover each other
	optionalSleep(send)

	for i := 0; i < count; i++ {
		// Send the message to receiver
		event := &message.TestMessage{
			Text: fmt.Sprintf("hello%d", i),
		}
		// sends a message from sender to receiver using send wrapper function
		require.NoError(s.Suite.T(), send(event, sender.con, s.ids[rcvID].NodeID))

		select {
		case <-receiver.received:
			// evaluates reception of message at the other side
			// does not evaluate the content
			require.NotNil(s.Suite.T(), receiver.originID)
			require.NotNil(s.Suite.T(), receiver.event)
			assert.Equal(s.Suite.T(), s.ids[sndID].NodeID, receiver.originID)

			// evaluates proper reception of event
			// casts the received event at the receiver side
			rcvEvent, ok := (<-receiver.event).(*message.TestMessage)
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
				rcvEvent, ok := (<-sender.event).(*message.TestMessage)
				// evaluates correctness of casting
				require.True(s.Suite.T(), ok)
				// evaluates content of received message
				echoEvent := &message.TestMessage{
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
func (s *EchoEngineTestSuite) multiMessageAsync(echo bool, count int, send ConduitSendWrapperFunc) {
	sndID := 0
	rcvID := 1

	// registers engines in the network
	// sender's engine
	sender := NewEchoEngine(s.Suite.T(), s.nets[sndID], 10, engine.TestNetwork, echo, send)

	// receiver's engine
	receiver := NewEchoEngine(s.Suite.T(), s.nets[rcvID], 10, engine.TestNetwork, echo, send)

	// allow nodes to heartbeat and discover each other
	optionalSleep(send)

	// keeps track of async received messages at receiver side
	received := make(map[string]struct{})

	// keeps track of async received echo messages at sender side
	// echorcv := make(map[string]struct{})

	for i := 0; i < count; i++ {
		// Send the message to node 2 using the conduit of node 1
		event := &message.TestMessage{
			Text: fmt.Sprintf("hello%d", i),
		}
		require.NoError(s.Suite.T(), send(event, sender.con, s.ids[1].NodeID))
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
			rcvEvent, ok := (<-receiver.event).(*message.TestMessage)
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
				rcvEvent, ok := (<-sender.event).(*message.TestMessage)
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

func (s *EchoEngineTestSuite) skipTest(reason string) {
	if _, found := os.LookupEnv("AllNetworkTest"); !found {
		s.T().Skip(reason)
	}
}
