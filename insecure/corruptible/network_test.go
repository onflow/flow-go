package corruptible

import (
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/onflow/flow-go/engine/testutil"
	"github.com/onflow/flow-go/insecure"
	mockinsecure "github.com/onflow/flow-go/insecure/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"
)

// TestNetworkHandleOutgoingEvent_AttackerObserve evaluates that the incoming messages to the corrupted network are routed to the
// registered attacker if one exists.
func TestNetworkHandleOutgoingEvent_AttackerObserve(t *testing.T) {
	codec := cbor.NewCodec()
	corruptedIdentity := unittest.IdentityFixture(unittest.WithAddress("localhost:0"))
	flowNetwork := &mocknetwork.Network{}
	ccf := &mockinsecure.CorruptibleConduitFactory{}
	ccf.On("RegisterEgressController", mock.Anything).Return(nil)

	corruptibleNetwork, err := NewCorruptibleNetwork(
		unittest.Logger(),
		flow.BftTestnet,
		"localhost:0",
		testutil.LocalFixture(t, corruptedIdentity),
		codec,
		flowNetwork,
		ccf)
	require.NoError(t, err)

	attacker := newMockAttackerObserverClient()

	attackerRegistered := sync.WaitGroup{}
	attackerRegistered.Add(1)
	go func() {
		attackerRegistered.Done()

		err := corruptibleNetwork.ConnectAttacker(&empty.Empty{}, attacker) // blocking call
		require.NoError(t, err)
	}()
	unittest.RequireReturnsBefore(t, attackerRegistered.Wait, 1*time.Second, "could not register attacker on time")

	targetIds := unittest.IdentifierListFixture(10)
	event, channel := getMessageAndChannel()

	go func() {
		err := corruptibleNetwork.HandleOutgoingEvent(event, channel, insecure.Protocol_MULTICAST, uint32(3), targetIds...)
		require.NoError(t, err)
	}()

	// For this test we use a mock attacker, that puts the incoming messages into a channel. Then in this test we keep reading from that channel till
	// either a message arrives or a timeout. Reading a message from that channel means attackers Observe has been called.
	var receivedMsg *insecure.Message
	unittest.RequireReturnsBefore(t, func() {
		receivedMsg = <-attacker.incomingBuffer
	}, 100*time.Millisecond, "mock attack could not receive incoming message on time")

	// checks content of the received message matches what has been sent.
	require.ElementsMatch(t, receivedMsg.TargetIDs, flow.IdsToBytes(targetIds))
	require.Equal(t, receivedMsg.TargetNum, uint32(3))
	require.Equal(t, receivedMsg.Protocol, insecure.Protocol_MULTICAST)
	require.Equal(t, receivedMsg.ChannelID, string(channel))

	decodedEvent, err := codec.Decode(receivedMsg.Payload)
	require.NoError(t, err)
	require.Equal(t, event, decodedEvent)
}

// TestNetworkHandleOutgoingEvent_NoAttacker_UnicastOverNetwork checks that outgoing unicast events to the corrupted network
// are routed to the network adapter when no attacker is registered to the network.
func TestNetworkHandleOutgoingEvent_NoAttacker_UnicastOverNetwork(t *testing.T) {
	corruptibleNetwork, adapter := getCorruptibleNetworkNoAttacker(t)

	event, channel := getMessageAndChannel()
	targetId := unittest.IdentifierFixture()

	adapter.On("UnicastOnChannel", channel, event, targetId).Return(nil).Once()

	// simulate sending message by conduit
	err := corruptibleNetwork.HandleOutgoingEvent(event, channel, insecure.Protocol_UNICAST, uint32(0), targetId)
	require.NoError(t, err)

	// check that correct Adapter method called
	mock.AssertExpectationsForObjects(t, adapter)
}

// TestNetworkHandleOutgoingEvent_NoAttacker_PublishOverNetwork checks that the outgoing publish events to the corrupted network
// are routed to the network adapter when no attacker registered to the network.
func TestNetworkHandleOutgoingEvent_NoAttacker_PublishOverNetwork(t *testing.T) {
	corruptibleNetwork, adapter := getCorruptibleNetworkNoAttacker(t)

	event, channel := getMessageAndChannel()

	targetIds := unittest.IdentifierListFixture(10)
	params := []interface{}{channel, event}
	for _, id := range targetIds {
		params = append(params, id)
	}

	adapter.On("PublishOnChannel", params...).Return(nil).Once()

	// simulate sending message by conduit
	err := corruptibleNetwork.HandleOutgoingEvent(event, channel, insecure.Protocol_PUBLISH, uint32(0), targetIds...)
	require.NoError(t, err)
	mock.AssertExpectationsForObjects(t, adapter)
}

// HELPERS

func getCorruptibleNetworkNoAttacker(t *testing.T) (*Network, *mocknetwork.Adapter) {
	// create corruptible network with no attacker registered
	codec := cbor.NewCodec()
	corruptedIdentity := unittest.IdentityFixture(unittest.WithAddress("localhost:0"))
	flowNetwork := &mocknetwork.Network{}
	ccf := NewCorruptibleConduitFactory(unittest.Logger(), flow.BftTestnet)

	// set up adapter, so we can check if that it called the expected method
	adapter := &mocknetwork.Adapter{}
	err := ccf.RegisterAdapter(adapter)
	require.NoError(t, err)

	corruptibleNetwork, err := NewCorruptibleNetwork(
		unittest.Logger(),
		flow.BftTestnet,
		"localhost:0",
		testutil.LocalFixture(t, corruptedIdentity),
		codec,
		flowNetwork,
		ccf)
	require.NoError(t, err)
	// return adapter so callers can set up test specific expectations
	return corruptibleNetwork, adapter
}

func getMessageAndChannel() (*message.TestMessage, network.Channel) {
	message := &message.TestMessage{Text: "this is a test message"}
	channel := network.Channel("test-channel")

	return message, channel
}
