package cohort2

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/go-log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/internal/testutils"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/p2pnet"
	"github.com/onflow/flow-go/utils/unittest"
)

// EchoEngineTestSuite tests the correctness of the entire pipeline of network -> libp2p
// protocol stack. It creates two instances of a stubengine, connects them through network, and sends a
// single message from one engine to the other one through different scenarios.
type EchoEngineTestSuite struct {
	suite.Suite
	testutils.ConduitWrapper                   // used as a wrapper around conduit methods
	networks                 []*p2pnet.Network // used to keep track of the networks
	libp2pNodes              []p2p.LibP2PNode  // used to keep track of the libp2p nodes
	ids                      flow.IdentityList // used to keep track of the identifiers associated with networks
	cancel                   context.CancelFunc
}

// Some tests are skipped in short mode to speedup the build.

// TestEchoEngineTestSuite runs all the test methods in this test suit
func TestEchoEngineTestSuite(t *testing.T) {
	suite.Run(t, new(EchoEngineTestSuite))
}

func (suite *EchoEngineTestSuite) SetupTest() {
	const count = 2
	log.SetAllLoggers(log.LevelError)

	ctx, cancel := context.WithCancel(context.Background())
	suite.cancel = cancel

	signalerCtx := irrecoverable.NewMockSignalerContext(suite.T(), ctx)

	// both nodes should be of the same role to get connected on epidemic dissemination
	var nodes []p2p.LibP2PNode
	sporkId := unittest.IdentifierFixture()

	suite.ids, nodes = testutils.LibP2PNodeForNetworkFixture(suite.T(), sporkId, count)
	suite.libp2pNodes = nodes
	suite.networks, _ = testutils.NetworksFixture(suite.T(), sporkId, suite.ids, nodes)
	// starts the nodes and networks
	testutils.StartNodes(signalerCtx, suite.T(), nodes)
	for _, net := range suite.networks {
		testutils.StartNetworks(signalerCtx, suite.T(), []network.EngineRegistry{net})
		unittest.RequireComponentsReadyBefore(suite.T(), 1*time.Second, net)
	}
}

// TearDownTest closes the networks within a specified timeout
func (suite *EchoEngineTestSuite) TearDownTest() {
	suite.cancel()
	testutils.StopComponents(suite.T(), suite.networks, 3*time.Second)
	testutils.StopComponents(suite.T(), suite.libp2pNodes, 3*time.Second)
}

// TestUnknownChannel evaluates that registering an engine with an unknown channel returns an error.
// All channels should be registered as topics in engine.topicMap.
func (suite *EchoEngineTestSuite) TestUnknownChannel() {
	e := NewEchoEngine(suite.T(), suite.networks[0], 1, channels.TestNetworkChannel, false, suite.Unicast)
	_, err := suite.networks[0].Register("unknown-channel-id", e)
	require.Error(suite.T(), err)
}

// TestClusterChannel evaluates that registering a cluster channel  is done without any error.
func (suite *EchoEngineTestSuite) TestClusterChannel() {
	e := NewEchoEngine(suite.T(), suite.networks[0], 1, channels.TestNetworkChannel, false, suite.Unicast)
	// creates a cluster channel
	clusterChannel := channels.SyncCluster(flow.Testnet)
	// registers engine with cluster channel
	_, err := suite.networks[0].Register(clusterChannel, e)
	// registering cluster channel should not cause an error
	require.NoError(suite.T(), err)
}

// TestDuplicateChannel evaluates that registering an engine with duplicate channel returns an error.
func (suite *EchoEngineTestSuite) TestDuplicateChannel() {
	// creates an echo engine, which registers it on test network channel
	e := NewEchoEngine(suite.T(), suite.networks[0], 1, channels.TestNetworkChannel, false, suite.Unicast)

	// attempts to register the same engine again on test network channel which
	// should cause an error
	_, err := suite.networks[0].Register(channels.TestNetworkChannel, e)
	require.Error(suite.T(), err)
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
	unittest.SkipUnless(suite.T(), unittest.TEST_LONG_RUNNING, "covered by TestDuplicateMessageParallel_Publish")
	suite.duplicateMessageSequential(suite.Publish)
}

// TestDuplicateMessageSequential_Unicast evaluates the correctness of network layer on deduplicating
// the received messages over Unicast method of nodes' Conduits.
// Messages are delivered to the receiver in a sequential manner.
func (suite *EchoEngineTestSuite) TestDuplicateMessageSequential_Unicast() {
	unittest.SkipUnless(suite.T(), unittest.TEST_LONG_RUNNING, "covered by TestDuplicateMessageParallel_Unicast")
	suite.duplicateMessageSequential(suite.Unicast)
}

// TestDuplicateMessageSequential_Multicast evaluates the correctness of network layer on deduplicating
// the received messages over Multicast method of nodes' Conduits.
// Messages are delivered to the receiver in a sequential manner.
func (suite *EchoEngineTestSuite) TestDuplicateMessageSequential_Multicast() {
	unittest.SkipUnless(suite.T(), unittest.TEST_LONG_RUNNING, "covered by TestDuplicateMessageParallel_Multicast")
	suite.duplicateMessageSequential(suite.Multicast)
}

// TestDuplicateMessageParallel_Publish evaluates the correctness of network layer
// on deduplicating the received messages via Publish method of nodes' Conduits.
// Messages are delivered to the receiver in parallel via the Publish method of Conduits.
func (suite *EchoEngineTestSuite) TestDuplicateMessageParallel_Publish() {
	unittest.SkipUnless(suite.T(), unittest.TEST_LONG_RUNNING, "covered by TestDuplicateMessageParallel_Multicast")
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
func (suite *EchoEngineTestSuite) duplicateMessageSequential(send testutils.ConduitSendWrapperFunc) {
	sndID := 0
	rcvID := 1
	// registers engines in the network
	// sender's engine
	sender := NewEchoEngine(suite.Suite.T(), suite.networks[sndID], 10, channels.TestNetworkChannel, false, send)

	// receiver's engine
	receiver := NewEchoEngine(suite.Suite.T(), suite.networks[rcvID], 10, channels.TestNetworkChannel, false, send)

	// allow nodes to heartbeat and discover each other if using PubSub
	testutils.OptionalSleep(send)

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
func (suite *EchoEngineTestSuite) duplicateMessageParallel(send testutils.ConduitSendWrapperFunc) {
	sndID := 0
	rcvID := 1
	// registers engines in the network
	// sender's engine
	sender := NewEchoEngine(suite.Suite.T(), suite.networks[sndID], 10, channels.TestNetworkChannel, false, send)

	// receiver's engine
	receiver := NewEchoEngine(suite.Suite.T(), suite.networks[rcvID], 10, channels.TestNetworkChannel, false, send)

	// allow nodes to heartbeat and discover each other
	testutils.OptionalSleep(send)

	// Sends a message from sender to receiver
	event := &message.TestMessage{
		Text: "hello",
	}

	// sends the same message 10 times
	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			require.NoError(suite.Suite.T(), send(event, sender.con, suite.ids[rcvID].NodeID))
			wg.Done()
		}()
	}
	unittest.RequireReturnsBefore(suite.T(), wg.Wait, 3*time.Second, "could not send message on time")

	require.Eventually(suite.T(), func() bool {
		return len(receiver.seen) > 0
	}, 3*time.Second, 500*time.Millisecond)

	// receiver should only see the message once, and the rest should be dropped due to
	// duplication
	receiver.RLock()
	require.Equal(suite.Suite.T(), 1, receiver.seen[event.Text])
	require.Len(suite.Suite.T(), receiver.seen, 1)
	receiver.RUnlock()
}

// duplicateMessageDifferentChan is a helper function that sends the same message from two distinct
// sender engines to the two distinct receiver engines via the send wrapper function of Conduits.
func (suite *EchoEngineTestSuite) duplicateMessageDifferentChan(send testutils.ConduitSendWrapperFunc) {
	const (
		sndNode = iota
		rcvNode
	)
	const (
		channel1 = channels.TestNetworkChannel
		channel2 = channels.TestMetricsChannel
	)
	// registers engines in the network
	// first type
	// sender'suite engine
	sender1 := NewEchoEngine(suite.Suite.T(), suite.networks[sndNode], 10, channel1, false, send)

	// receiver's engine
	receiver1 := NewEchoEngine(suite.Suite.T(), suite.networks[rcvNode], 10, channel1, false, send)

	// second type
	// registers engines in the network
	// sender'suite engine
	sender2 := NewEchoEngine(suite.Suite.T(), suite.networks[sndNode], 10, channel2, false, send)

	// receiver's engine
	receiver2 := NewEchoEngine(suite.Suite.T(), suite.networks[rcvNode], 10, channel2, false, send)

	// allow nodes to heartbeat and discover each other
	testutils.OptionalSleep(send)

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
	unittest.RequireReturnsBefore(suite.T(), wg.Wait, 3*time.Second, "could not handle sending unicasts on time")
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
func (suite *EchoEngineTestSuite) singleMessage(echo bool, send testutils.ConduitSendWrapperFunc) {
	sndID := 0
	rcvID := 1

	// registers engines in the network
	// sender's engine
	sender := NewEchoEngine(suite.Suite.T(), suite.networks[sndID], 10, channels.TestNetworkChannel, echo, send)

	// receiver's engine
	receiver := NewEchoEngine(suite.Suite.T(), suite.networks[rcvID], 10, channels.TestNetworkChannel, echo, send)

	// allow nodes to heartbeat and discover each other
	testutils.OptionalSleep(send)

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

		assertMessageReceived(suite.T(), receiver, event, channels.TestNetworkChannel)

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
			assertMessageReceived(suite.T(), sender, echoEvent, channels.TestNetworkChannel)

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
func (suite *EchoEngineTestSuite) multiMessageSync(echo bool, count int, send testutils.ConduitSendWrapperFunc) {
	sndID := 0
	rcvID := 1
	// registers engines in the network
	// sender's engine
	sender := NewEchoEngine(suite.Suite.T(), suite.networks[sndID], 10, channels.TestNetworkChannel, echo, send)

	// receiver's engine
	receiver := NewEchoEngine(suite.Suite.T(), suite.networks[rcvID], 10, channels.TestNetworkChannel, echo, send)

	// allow nodes to heartbeat and discover each other
	testutils.OptionalSleep(send)

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

			assertMessageReceived(suite.T(), receiver, event, channels.TestNetworkChannel)

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
				assertMessageReceived(suite.T(), sender, echoEvent, channels.TestNetworkChannel)
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
func (suite *EchoEngineTestSuite) multiMessageAsync(echo bool, count int, send testutils.ConduitSendWrapperFunc) {
	sndID := 0
	rcvID := 1

	// registers engines in the network
	// sender's engine
	sender := NewEchoEngine(suite.Suite.T(), suite.networks[sndID], 10, channels.TestNetworkChannel, echo, send)

	// receiver's engine
	receiver := NewEchoEngine(suite.Suite.T(), suite.networks[rcvID], 10, channels.TestNetworkChannel, echo, send)

	// allow nodes to heartbeat and discover each other
	testutils.OptionalSleep(send)

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
				assert.Equal(suite.T(), channels.TestNetworkChannel, <-receiver.channel)
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
					assert.Equal(suite.T(), channels.TestNetworkChannel, <-sender.channel)
				}, 100*time.Millisecond)

			case <-time.After(10 * time.Second):
				assert.Fail(suite.Suite.T(), "receiver failed to send an echo message back to sender")
			}
		}
	}
}

// assertMessageReceived asserts that the given message was received on the given channel
// for the given engine
func assertMessageReceived(t *testing.T, e *EchoEngine, m *message.TestMessage, c channels.Channel) {
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
