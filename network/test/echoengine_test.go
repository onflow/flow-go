package test

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/go-log"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/utils/unittest"
)

// EchoEngineTestSuite tests the correctness of the entire pipeline of network -> middleware -> libp2p
// protocol stack. It creates two instances of a stubengine, connects them through network, and sends a
// single message from one engine to the other one through different scenarios.
type EchoEngineTestSuite struct {
	suite.Suite
	ConduitWrapper                   // used as a wrapper around conduit methods
	nets           []*p2p.Network    // used to keep track of the networks
	ids            flow.IdentityList // used to keep track of the identifiers associated with networks
}

// Some tests are skipped to speedup the build.
// However, they can be enabled if the environment variable "AllNetworkTest" is set with any value

// TestStubEngineTestSuite runs all the test methods in this test suit
func TestStubEngineTestSuite(t *testing.T) {
	suite.Run(t, new(EchoEngineTestSuite))
}

func (suite *EchoEngineTestSuite) SetupTest() {
	const count = 2
	logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
	log.SetAllLoggers(log.LevelError)
	// both nodes should be of the same role to get connected on epidemic dissemination
	suite.ids, _, suite.nets, _ = GenerateIDsMiddlewaresNetworks(suite.T(), count, logger, 100, nil, !DryRun)
}

// TearDownTest closes the networks within a specified timeout
func (suite *EchoEngineTestSuite) TearDownTest() {
	stopNetworks(suite.T(), suite.nets, 3*time.Second)
}

// TestUnknownChannel evaluates that registering an engine with an unknown channel returns an error.
// All channels should be registered as topics in engine.topicMap.
func (suite *EchoEngineTestSuite) TestUnknownChannel() {
	e := NewEchoEngine(suite.T(), suite.nets[0], 1, engine.TestNetwork, false, suite.Unicast)
	_, err := suite.nets[0].Register("unknown-channel-id", e)
	require.Error(suite.T(), err)
}

// TestClusterChannel evaluates that registering a cluster channel  is done without any error.
func (suite *EchoEngineTestSuite) TestClusterChannel() {
	e := NewEchoEngine(suite.T(), suite.nets[0], 1, engine.TestNetwork, false, suite.Unicast)
	// creates a cluster channel
	clusterChannel := engine.ChannelSyncCluster(flow.Testnet)
	// registers engine with cluster channel
	_, err := suite.nets[0].Register(clusterChannel, e)
	// registering cluster channel should not cause an error
	require.NoError(suite.T(), err)
}

// TestDuplicateChannel evaluates that registering an engine with duplicate channel returns an error.
func (suite *EchoEngineTestSuite) TestDuplicateChannel() {
	// creates an echo engine, which registers it on test network channel
	e := NewEchoEngine(suite.T(), suite.nets[0], 1, engine.TestNetwork, false, suite.Unicast)

	// attempts to register the same engine again on test network channel which
	// should cause an error
	_, err := suite.nets[0].Register(engine.TestNetwork, e)
	require.Error(suite.T(), err)
}

// TestSingleMessage_Publish tests sending a single message from sender to receiver using
// the Publish method of Conduit.
func (suite *EchoEngineTestSuite) TestSingleMessage_Publish() {
	suite.skipTest("covered by TestEchoMultiMsgAsync_Publish")
	// set to false for no echo expectation
	suite.singleMessage(false, suite.Publish)
}

// TestSingleMessage_Unicast tests sending a single message from sender to receiver using
// the Unicast method of Conduit.
func (suite *EchoEngineTestSuite) TestSingleMessage_Unicast() {
	suite.skipTest("covered by TestEchoMultiMsgAsync_Unicast")
	// set to false for no echo expectation
	suite.singleMessage(false, suite.Unicast)
}

// TestSingleMessage_Multicast tests sending a single message from sender to receiver using
// the Multicast method of Conduit.
func (suite *EchoEngineTestSuite) TestSingleMessage_Multicast() {
	suite.skipTest("covered by TestEchoMultiMsgAsync_Multicast")
	// set to false for no echo expectation
	suite.singleMessage(false, suite.Multicast)
}

// TestSingleEcho_Publish tests sending a single message from sender to receiver using
// the Publish method of its Conduit.
// It also evaluates the correct reception of an echo message back.
func (suite *EchoEngineTestSuite) TestSingleEcho_Publish() {
	suite.skipTest("covered by TestEchoMultiMsgAsync_Publish")
	// set to true for an echo expectation
	suite.singleMessage(true, suite.Publish)
}

// TestSingleEcho_Unicast tests sending a single message from sender to receiver using
// the Unicast method of its Conduit.
// It also evaluates the correct reception of an echo message back.
func (suite *EchoEngineTestSuite) TestSingleEcho_Unicast() {
	suite.skipTest("covered by TestEchoMultiMsgAsync_Unicast")
	// set to true for an echo expectation
	suite.singleMessage(true, suite.Unicast)
}

// TestSingleEcho_Multicast tests sending a single message from sender to receiver using
// the Multicast method of its Conduit.
// It also evaluates the correct reception of an echo message back.
func (suite *EchoEngineTestSuite) TestSingleEcho_Multicast() {
	suite.skipTest("covered by TestEchoMultiMsgAsync_Multicast")
	// set to true for an echo expectation
	suite.singleMessage(true, suite.Multicast)
}

// TestMultiMsgSync_Publish tests sending multiple messages from sender to receiver
// using the Publish method of its Conduit.
// Sender and receiver are synced over reception.
func (suite *EchoEngineTestSuite) TestMultiMsgSync_Publish() {
	suite.skipTest("covered by TestEchoMultiMsgAsync_Publish")
	// set to false for no echo expectation
	suite.multiMessageSync(false, 10, suite.Publish)
}

// TestMultiMsgSync_Unicast tests sending multiple messages from sender to receiver
// using the Unicast method of its Conduit.
// Sender and receiver are synced over reception.
func (suite *EchoEngineTestSuite) TestMultiMsgSync_Unicast() {
	suite.skipTest("covered by TestEchoMultiMsgAsync_Unicast")
	// set to false for no echo expectation
	suite.multiMessageSync(false, 10, suite.Unicast)
}

// TestMultiMsgSync_Multicast tests sending multiple messages from sender to receiver
// using the Multicast method of its Conduit.
// Sender and receiver are synced over reception.
func (suite *EchoEngineTestSuite) TestMultiMsgSync_Multicast() {
	suite.skipTest("covered by TestEchoMultiMsgAsync_Multicast")
	// set to false for no echo expectation
	suite.multiMessageSync(false, 10, suite.Multicast)
}

// TestEchoMultiMsgSync_Publish tests sending multiple messages from sender to receiver
// using the Publish method of its Conduit.
// It also evaluates the correct reception of an echo message back for each send
// sender and receiver are synced over reception.
func (suite *EchoEngineTestSuite) TestEchoMultiMsgSync_Publish() {
	suite.skipTest("covered by TestEchoMultiMsgAsync_Publish")
	// set to true for an echo expectation
	suite.multiMessageSync(true, 10, suite.Publish)
}

// TestEchoMultiMsgSync_Multicast tests sending multiple messages from sender to receiver
// using the Multicast method of its Conduit.
// It also evaluates the correct reception of an echo message back for each send
// sender and receiver are synced over reception.
func (suite *EchoEngineTestSuite) TestEchoMultiMsgSync_Multicast() {
	suite.skipTest("covered by TestEchoMultiMsgAsync_Multicast")
	// set to true for an echo expectation
	suite.multiMessageSync(true, 10, suite.Multicast)
}

// TestMultiMsgAsync_Publish tests sending multiple messages from sender to receiver
// using the Publish method of their Conduit.
// Sender and receiver are not synchronized
func (suite *EchoEngineTestSuite) TestMultiMsgAsync_Publish() {
	suite.skipTest("covered by TestEchoMultiMsgAsync_Publish")
	// set to false for no echo expectation
	suite.multiMessageAsync(false, 10, suite.Publish)
}

// TestMultiMsgAsync_Unicast tests sending multiple messages from sender to receiver
// using the Unicast method of their Conduit.
// Sender and receiver are not synchronized
func (suite *EchoEngineTestSuite) TestMultiMsgAsync_Unicast() {
	suite.skipTest("covered by TestEchoMultiMsgAsync_Unicast")
	// set to false for no echo expectation
	suite.multiMessageAsync(false, 10, suite.Unicast)
}

// TestMultiMsgAsync_Multicast tests sending multiple messages from sender to receiver
// using the Multicast method of their Conduit.
// Sender and receiver are not synchronized.
func (suite *EchoEngineTestSuite) TestMultiMsgAsync_Multicast() {
	suite.skipTest("covered by TestEchoMultiMsgAsync_Multicast")
	// set to false for no echo expectation
	suite.multiMessageAsync(false, 10, suite.Multicast)
}

// TestEchoMultiMsgAsync_Publish tests sending multiple messages from sender to receiver
// using the Publish method of their Conduit.
// It also evaluates the correct reception of an echo message back for each send.
// Sender and receiver are not synchronized
func (suite *EchoEngineTestSuite) TestEchoMultiMsgAsync_Publish() {
	// set to true for an echo expectation
	suite.multiMessageAsync(true, 10, suite.Publish)
}

// TestEchoMultiMsgAsync_Unicast tests sending multiple messages from sender to receiver
// using the Unicast method of their Conduit.
// It also evaluates the correct reception of an echo message back for each send.
// Sender and receiver are not synchronized
func (suite *EchoEngineTestSuite) TestEchoMultiMsgAsync_Unicast() {
	// set to true for an echo expectation
	suite.multiMessageAsync(true, 10, suite.Unicast)
}

// TestEchoMultiMsgAsync_Multicast tests sending multiple messages from sender to receiver
// using the Multicast method of their Conduit.
// It also evaluates the correct reception of an echo message back for each send.
// Sender and receiver are not synchronized
func (suite *EchoEngineTestSuite) TestEchoMultiMsgAsync_Multicast() {
	// set to true for an echo expectation
	suite.multiMessageAsync(true, 10, suite.Multicast)
}

// TestDuplicateMessageSequential_Publish evaluates the correctness of network layer on deduplicating
// the received messages over Publish method of nodes' Conduits.
// Messages are delivered to the receiver in a sequential manner.
func (suite *EchoEngineTestSuite) TestDuplicateMessageSequential_Publish() {
	suite.skipTest("covered by TestDuplicateMessageParallel_Publish")
	suite.duplicateMessageSequential(suite.Publish)
}

// TestDuplicateMessageSequential_Unicast evaluates the correctness of network layer on deduplicating
// the received messages over Unicast method of nodes' Conduits.
// Messages are delivered to the receiver in a sequential manner.
func (suite *EchoEngineTestSuite) TestDuplicateMessageSequential_Unicast() {
	suite.skipTest("covered by TestDuplicateMessageParallel_Unicast")
	suite.duplicateMessageSequential(suite.Unicast)
}

// TestDuplicateMessageSequential_Multicast evaluates the correctness of network layer on deduplicating
// the received messages over Multicast method of nodes' Conduits.
// Messages are delivered to the receiver in a sequential manner.
func (suite *EchoEngineTestSuite) TestDuplicateMessageSequential_Multicast() {
	suite.skipTest("covered by TestDuplicateMessageParallel_Multicast")
	suite.duplicateMessageSequential(suite.Multicast)
}

// TestDuplicateMessageParallel_Publish evaluates the correctness of network layer
// on deduplicating the received messages via Publish method of nodes' Conduits.
// Messages are delivered to the receiver in parallel via the Publish method of Conduits.
func (suite *EchoEngineTestSuite) TestDuplicateMessageParallel_Publish() {
	suite.duplicateMessageParallel(suite.Publish)
}

// TestDuplicateMessageParallel_Unicast evaluates the correctness of network layer
// on deduplicating the received messages via Unicast method of nodes' Conduits.
// Messages are delivered to the receiver in parallel via the Unicast method of Conduits.
func (suite *EchoEngineTestSuite) TestDuplicateMessageParallel_Unicast() {
	suite.duplicateMessageParallel(suite.Unicast)
}

// TestDuplicateMessageParallel_Multicast evaluates the correctness of network layer
// on deduplicating the received messages via Multicast method of nodes' Conduits.
// Messages are delivered to the receiver in parallel via the Multicast method of Conduits.
func (suite *EchoEngineTestSuite) TestDuplicateMessageParallel_Multicast() {
	suite.duplicateMessageParallel(suite.Multicast)
}

// TestDuplicateMessageDifferentChan_Publish evaluates the correctness of network layer
// on deduplicating the received messages against different engine ids. In specific, the
// desire behavior is that the deduplication should happen based on both eventID and channel.
// Messages are sent via the Publish methods of the Conduits.
func (suite *EchoEngineTestSuite) TestDuplicateMessageDifferentChan_Publish() {
	suite.duplicateMessageDifferentChan(suite.Publish)
}

// TestDuplicateMessageDifferentChan_Unicast evaluates the correctness of network layer
// on deduplicating the received messages against different engine ids. In specific, the
// desire behavior is that the deduplication should happen based on both eventID and channel.
// Messages are sent via the Unicast methods of the Conduits.
func (suite *EchoEngineTestSuite) TestDuplicateMessageDifferentChan_Unicast() {
	suite.duplicateMessageDifferentChan(suite.Unicast)
}

// TestDuplicateMessageDifferentChan_Multicast evaluates the correctness of network layer
// on deduplicating the received messages against different engine ids. In specific, the
// desire behavior is that the deduplication should happen based on both eventID and channel.
// Messages are sent via the Multicast methods of the Conduits.
func (suite *EchoEngineTestSuite) TestDuplicateMessageDifferentChan_Multicast() {
	suite.duplicateMessageDifferentChan(suite.Multicast)
}

// duplicateMessageSequential is a helper function that sends duplicate messages sequentially
// from a receiver to the sender via the injected send wrapper function of conduit.
func (suite *EchoEngineTestSuite) duplicateMessageSequential(send ConduitSendWrapperFunc) {
	sndID := 0
	rcvID := 1
	// registers engines in the network
	// sender's engine
	sender := NewEchoEngine(suite.Suite.T(), suite.nets[sndID], 10, engine.TestNetwork, false, send)

	// receiver's engine
	receiver := NewEchoEngine(suite.Suite.T(), suite.nets[rcvID], 10, engine.TestNetwork, false, send)

	// allow nodes to heartbeat and discover each other if using PubSub
	optionalSleep(send)

	// Sends a message from sender to receiver
	event := &message.TestMessage{
		Text: "hello",
	}

	// sends the same message 10 times
	for i := 0; i < 10; i++ {
		require.NoError(suite.Suite.T(), send(event, sender.con, suite.ids[rcvID].NodeID))
	}

	time.Sleep(1 * time.Second)

	// receiver should only see the message once, and the rest should be dropped due to
	// duplication
	receiver.RLock()
	require.Equal(suite.Suite.T(), 1, receiver.seen[event.Text])
	require.Len(suite.Suite.T(), receiver.seen, 1)
	receiver.RUnlock()
}

// duplicateMessageParallel is a helper function that sends duplicate messages concurrent;u
// from a receiver to the sender via the injected send wrapper function of conduit.
func (suite *EchoEngineTestSuite) duplicateMessageParallel(send ConduitSendWrapperFunc) {
	sndID := 0
	rcvID := 1
	// registers engines in the network
	// sender's engine
	sender := NewEchoEngine(suite.Suite.T(), suite.nets[sndID], 10, engine.TestNetwork, false, send)

	// receiver's engine
	receiver := NewEchoEngine(suite.Suite.T(), suite.nets[rcvID], 10, engine.TestNetwork, false, send)

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
			require.NoError(suite.Suite.T(), send(event, sender.con, suite.ids[rcvID].NodeID))
		}()
	}
	wg.Wait()
	time.Sleep(1 * time.Second)

	// receiver should only see the message once, and the rest should be dropped due to
	// duplication
	receiver.RLock()
	require.Equal(suite.Suite.T(), 1, receiver.seen[event.Text])
	require.Len(suite.Suite.T(), receiver.seen, 1)
	receiver.RUnlock()
}

// duplicateMessageDifferentChan is a helper function that sends the same message from two distinct
// sender engines to the two distinct receiver engines via the send wrapper function of Conduits.
func (suite *EchoEngineTestSuite) duplicateMessageDifferentChan(send ConduitSendWrapperFunc) {
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
	// sender'suite engine
	sender1 := NewEchoEngine(suite.Suite.T(), suite.nets[sndNode], 10, channel1, false, send)

	// receiver's engine
	receiver1 := NewEchoEngine(suite.Suite.T(), suite.nets[rcvNode], 10, channel1, false, send)

	// second type
	// registers engines in the network
	// sender'suite engine
	sender2 := NewEchoEngine(suite.Suite.T(), suite.nets[sndNode], 10, channel2, false, send)

	// receiver's engine
	receiver2 := NewEchoEngine(suite.Suite.T(), suite.nets[rcvNode], 10, channel2, false, send)

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
			require.NoError(suite.Suite.T(), send(event, sender1.con, suite.ids[rcvNode].NodeID))

			// sender2 to receiver2 on channel2
			require.NoError(suite.Suite.T(), send(event, sender2.con, suite.ids[rcvNode].NodeID))
		}()
	}
	unittest.RequireReturnsBefore(suite.T(), wg.Wait, 1*time.Second, "could not handle sending unicasts on time")
	time.Sleep(1 * time.Second)

	// each receiver should only see the message once, and the rest should be dropped due to
	// duplication
	receiver1.RLock()
	receiver2.RLock()
	require.Equal(suite.Suite.T(), 1, receiver1.seen[event.Text])
	require.Equal(suite.Suite.T(), 1, receiver2.seen[event.Text])

	require.Len(suite.Suite.T(), receiver1.seen, 1)
	require.Len(suite.Suite.T(), receiver2.seen, 1)
	receiver1.RUnlock()
	receiver2.RUnlock()
}

// singleMessage sends a single message from one network instance to the other one
// it evaluates the correctness of implementation against correct delivery of the message.
// in case echo is true, it also evaluates correct reception of the echo message from the receiver side
func (suite *EchoEngineTestSuite) singleMessage(echo bool, send ConduitSendWrapperFunc) {
	sndID := 0
	rcvID := 1

	// registers engines in the network
	// sender's engine
	sender := NewEchoEngine(suite.Suite.T(), suite.nets[sndID], 10, engine.TestNetwork, echo, send)

	// receiver's engine
	receiver := NewEchoEngine(suite.Suite.T(), suite.nets[rcvID], 10, engine.TestNetwork, echo, send)

	// allow nodes to heartbeat and discover each other
	optionalSleep(send)

	// Sends a message from sender to receiver
	event := &message.TestMessage{
		Text: "hello",
	}
	require.NoError(suite.Suite.T(), send(event, sender.con, suite.ids[rcvID].NodeID))

	// evaluates reception of echo request
	select {
	case <-receiver.received:
		// evaluates reception of message at the other side
		// does not evaluate the content
		receiver.RLock()
		require.NotNil(suite.Suite.T(), receiver.originID)
		require.NotNil(suite.Suite.T(), receiver.event)
		assert.Equal(suite.Suite.T(), suite.ids[sndID].NodeID, receiver.originID)
		receiver.RUnlock()

		assertMessageReceived(suite.T(), receiver, event, engine.TestNetwork)

	case <-time.After(10 * time.Second):
		assert.Fail(suite.Suite.T(), "sender failed to send a message to receiver")
	}

	// evaluates echo back
	if echo {
		// evaluates reception of echo response
		select {
		case <-sender.received:
			// evaluates reception of message at the other side
			// does not evaluate the content
			sender.RLock()
			require.NotNil(suite.Suite.T(), sender.originID)
			require.NotNil(suite.Suite.T(), sender.event)
			assert.Equal(suite.Suite.T(), suite.ids[rcvID].NodeID, sender.originID)
			sender.RUnlock()

			echoEvent := &message.TestMessage{
				Text: fmt.Sprintf("%s: %s", receiver.echomsg, event.Text),
			}
			assertMessageReceived(suite.T(), sender, echoEvent, engine.TestNetwork)

		case <-time.After(10 * time.Second):
			assert.Fail(suite.Suite.T(), "receiver failed to send an echo message back to sender")
		}
	}
}

// multiMessageSync sends a multiple messages from one network instance to the other one
// it evaluates the correctness of implementation against correct delivery of the messages.
// sender and receiver are sync over reception, i.e., sender sends one message at a time and
// waits for its reception
// count defines number of messages
func (suite *EchoEngineTestSuite) multiMessageSync(echo bool, count int, send ConduitSendWrapperFunc) {
	sndID := 0
	rcvID := 1
	// registers engines in the network
	// sender's engine
	sender := NewEchoEngine(suite.Suite.T(), suite.nets[sndID], 10, engine.TestNetwork, echo, send)

	// receiver's engine
	receiver := NewEchoEngine(suite.Suite.T(), suite.nets[rcvID], 10, engine.TestNetwork, echo, send)

	// allow nodes to heartbeat and discover each other
	optionalSleep(send)

	for i := 0; i < count; i++ {
		// Send the message to receiver
		event := &message.TestMessage{
			Text: fmt.Sprintf("hello%d", i),
		}
		// sends a message from sender to receiver using send wrapper function
		require.NoError(suite.Suite.T(), send(event, sender.con, suite.ids[rcvID].NodeID))

		select {
		case <-receiver.received:
			// evaluates reception of message at the other side
			// does not evaluate the content
			receiver.RLock()
			require.NotNil(suite.Suite.T(), receiver.originID)
			require.NotNil(suite.Suite.T(), receiver.event)
			assert.Equal(suite.Suite.T(), suite.ids[sndID].NodeID, receiver.originID)
			receiver.RUnlock()

			assertMessageReceived(suite.T(), receiver, event, engine.TestNetwork)

		case <-time.After(2 * time.Second):
			assert.Fail(suite.Suite.T(), "sender failed to send a message to receiver")
		}

		// evaluates echo back
		if echo {
			// evaluates reception of echo response
			select {
			case <-sender.received:
				// evaluates reception of message at the other side
				// does not evaluate the content
				sender.RLock()
				require.NotNil(suite.Suite.T(), sender.originID)
				require.NotNil(suite.Suite.T(), sender.event)
				assert.Equal(suite.Suite.T(), suite.ids[rcvID].NodeID, sender.originID)

				receiver.RLock()
				echoEvent := &message.TestMessage{
					Text: fmt.Sprintf("%s: %s", receiver.echomsg, event.Text),
				}
				assertMessageReceived(suite.T(), sender, echoEvent, engine.TestNetwork)
				receiver.RUnlock()
				sender.RUnlock()

			case <-time.After(10 * time.Second):
				assert.Fail(suite.Suite.T(), "receiver failed to send an echo message back to sender")
			}
		}

	}

}

// multiMessageAsync sends a multiple messages from one network instance to the other one
// it evaluates the correctness of implementation against correct delivery of the messages.
// sender and receiver are async, i.e., sender sends all its message at blast
// count defines number of messages
func (suite *EchoEngineTestSuite) multiMessageAsync(echo bool, count int, send ConduitSendWrapperFunc) {
	sndID := 0
	rcvID := 1

	// registers engines in the network
	// sender's engine
	sender := NewEchoEngine(suite.Suite.T(), suite.nets[sndID], 10, engine.TestNetwork, echo, send)

	// receiver's engine
	receiver := NewEchoEngine(suite.Suite.T(), suite.nets[rcvID], 10, engine.TestNetwork, echo, send)

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
		require.NoError(suite.Suite.T(), send(event, sender.con, suite.ids[1].NodeID))
	}

	for i := 0; i < count; i++ {
		select {
		case <-receiver.received:
			// evaluates reception of message at the other side
			// does not evaluate the content
			receiver.RLock()
			require.NotNil(suite.Suite.T(), receiver.originID)
			require.NotNil(suite.Suite.T(), receiver.event)
			assert.Equal(suite.Suite.T(), suite.ids[0].NodeID, receiver.originID)
			receiver.RUnlock()

			// wrap blocking channel reads with a timeout
			unittest.AssertReturnsBefore(suite.T(), func() {
				// evaluates proper reception of event
				// casts the received event at the receiver side
				rcvEvent, ok := (<-receiver.event).(*message.TestMessage)
				// evaluates correctness of casting
				require.True(suite.T(), ok)

				// evaluates content of received message
				// the content should not yet received and be unique
				_, rcv := received[rcvEvent.Text]
				assert.False(suite.T(), rcv)
				// marking event as received
				received[rcvEvent.Text] = struct{}{}

				// evaluates channel that message was received on
				assert.Equal(suite.T(), engine.TestNetwork, <-receiver.channel)
			}, 100*time.Millisecond)

		case <-time.After(2 * time.Second):
			assert.Fail(suite.Suite.T(), "sender failed to send a message to receiver")
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
				sender.RLock()
				require.NotNil(suite.Suite.T(), sender.originID)
				require.NotNil(suite.Suite.T(), sender.event)
				assert.Equal(suite.Suite.T(), suite.ids[rcvID].NodeID, sender.originID)
				sender.RUnlock()

				// wrap blocking channel reads with a timeout
				unittest.AssertReturnsBefore(suite.T(), func() {
					// evaluates proper reception of event
					// casts the received event at the receiver side
					rcvEvent, ok := (<-sender.event).(*message.TestMessage)
					// evaluates correctness of casting
					require.True(suite.T(), ok)
					// evaluates content of received echo message
					// the content should not yet received and be unique
					_, rcv := received[rcvEvent.Text]
					assert.False(suite.T(), rcv)
					// echo messages should start with prefix msg of receiver that echos back
					assert.True(suite.T(), strings.HasPrefix(rcvEvent.Text, receiver.echomsg))
					// marking echo event as received
					received[rcvEvent.Text] = struct{}{}

					// evaluates channel that message was received on
					assert.Equal(suite.T(), engine.TestNetwork, <-sender.channel)
				}, 100*time.Millisecond)

			case <-time.After(10 * time.Second):
				assert.Fail(suite.Suite.T(), "receiver failed to send an echo message back to sender")
			}
		}
	}
}

func (suite *EchoEngineTestSuite) skipTest(reason string) {
	if _, found := os.LookupEnv("AllNetworkTest"); !found {
		suite.T().Skip(reason)
	}
}

// assertMessageReceived asserts that the given message was received on the given channel
// for the given engine
func assertMessageReceived(t *testing.T, e *EchoEngine, m *message.TestMessage, c network.Channel) {
	// wrap blocking channel reads with a timeout
	unittest.AssertReturnsBefore(t, func() {
		// evaluates proper reception of event
		// casts the received event at the receiver side
		rcvEvent, ok := (<-e.event).(*message.TestMessage)
		// evaluates correctness of casting
		require.True(t, ok)
		// evaluates content of received message
		assert.Equal(t, m, rcvEvent)

		// evaluates channel that message was received on
		assert.Equal(t, c, <-e.channel)
	}, 100*time.Millisecond)
}
