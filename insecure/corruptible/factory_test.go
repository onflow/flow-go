package corruptible

import (
	"context"
	"testing"
	"time"

	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestRegisterAdapter_FailDoubleRegistration checks that CorruptibleConduitFactory can be registered with only one adapter.
func TestRegisterAdapter_FailDoubleRegistration(t *testing.T) {
	f := NewCorruptibleConduitFactory(unittest.Logger(), flow.BftTestnet, unittest.IdentifierFixture(), cbor.NewCodec())

	adapter := &mocknetwork.Adapter{}

	// registering adapter must go successful
	require.NoError(t, f.RegisterAdapter(adapter))

	// second attempt on registering adapter must fail
	require.Error(t, f.RegisterAdapter(adapter))
}

// TestNewConduit_HappyPath checks when factory has an adapter registered, it can successfully
// create conduits.
func TestNewConduit_HappyPath(t *testing.T) {
	f := NewCorruptibleConduitFactory(unittest.Logger(), flow.BftTestnet, unittest.IdentifierFixture(), cbor.NewCodec())
	channel := network.Channel("test-channel")

	adapter := &mocknetwork.Adapter{}

	require.NoError(t, f.RegisterAdapter(adapter))

	c, err := f.NewConduit(context.Background(), channel)
	require.NoError(t, err)
	require.NotNil(t, c)
}

// TestNewConduit_MissingAdapter checks when factory does not have an adapter registered,
// any attempts on creating a conduit fails with an error.
func TestNewConduit_MissingAdapter(t *testing.T) {
	f := NewCorruptibleConduitFactory(unittest.Logger(), flow.BftTestnet, unittest.IdentifierFixture(), cbor.NewCodec())
	channel := network.Channel("test-channel")

	c, err := f.NewConduit(context.Background(), channel)
	require.Error(t, err)
	require.Nil(t, c)
}

// TestFactoryHandleIncomingEvent_AttackerObserve evaluates that the incoming messages to the conduit factory are routed to the
// registered attacker if one exists.
func TestFactoryHandleIncomingEvent_AttackerObserve(t *testing.T) {
	codec := cbor.NewCodec()
	f := NewCorruptibleConduitFactory(unittest.Logger(), flow.BftTestnet, unittest.IdentifierFixture(), codec)
	attacker := newMockAttackerObserverClient()
	f.attackerObserveClient = attacker

	event := &message.TestMessage{Text: "this is a test message"}
	targetIds := unittest.IdentifierListFixture(10)
	channel := network.Channel("test-channel")

	go func() {
		err := f.HandleIncomingEvent(event, channel, insecure.Protocol_MULTICAST, uint32(3), targetIds...)
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
	require.Equal(t, receivedMsg.Targets, uint32(3))
	require.Equal(t, receivedMsg.Protocol, insecure.Protocol_MULTICAST)
	require.Equal(t, receivedMsg.ChannelID, string(channel))

	decodedEvent, err := codec.Decode(receivedMsg.Payload)
	require.NoError(t, err)
	require.Equal(t, event, decodedEvent)
}

// TestFactoryHandleIncomingEvent_UnicastOverNetwork evaluates that the incoming unicast events to the conduit factory are routed to the
// network adapter when no attacker registered to the factory.
func TestFactoryHandleIncomingEvent_UnicastOverNetwork(t *testing.T) {
	codec := cbor.NewCodec()
	// corruptible conduit factory with no attacker registered.
	f := NewCorruptibleConduitFactory(unittest.Logger(), flow.BftTestnet, unittest.IdentifierFixture(), codec)

	adapter := &mocknetwork.Adapter{}
	err := f.RegisterAdapter(adapter)
	require.NoError(t, err)

	event := &message.TestMessage{Text: "this is a test message"}
	targetId := unittest.IdentifierFixture()
	channel := network.Channel("test-channel")

	adapter.On("UnicastOnChannel", channel, event, targetId).Return(nil).Once()

	err = f.HandleIncomingEvent(event, channel, insecure.Protocol_UNICAST, uint32(0), targetId)
	require.NoError(t, err)

	testifymock.AssertExpectationsForObjects(t, adapter)
}

// TestFactoryHandleIncomingEvent_PublishOverNetwork evaluates that the incoming publish events to the conduit factory are routed to the
// network adapter when no attacker registered to the factory.
func TestFactoryHandleIncomingEvent_PublishOverNetwork(t *testing.T) {
	codec := cbor.NewCodec()
	// corruptible conduit factory with no attacker registered.
	f := NewCorruptibleConduitFactory(unittest.Logger(), flow.BftTestnet, unittest.IdentifierFixture(), codec)

	adapter := &mocknetwork.Adapter{}
	err := f.RegisterAdapter(adapter)
	require.NoError(t, err)

	event := &message.TestMessage{Text: "this is a test message"}
	channel := network.Channel("test-channel")

	adapter.On("PublishOnChannel", channel, event).Return(nil).Once()

	err = f.HandleIncomingEvent(event, channel, insecure.Protocol_PUBLISH, uint32(0))
	require.NoError(t, err)

	testifymock.AssertExpectationsForObjects(t, adapter)
}

// TestFactoryHandleIncomingEvent_MulticastOverNetwork evaluates that the incoming multicast events to the conduit factory are routed to the
// network adapter when no attacker registered to the factory.
func TestFactoryHandleIncomingEvent_MulticastOverNetwork(t *testing.T) {
	codec := cbor.NewCodec()
	// corruptible conduit factory with no attacker registered.
	f := NewCorruptibleConduitFactory(unittest.Logger(), flow.BftTestnet, unittest.IdentifierFixture(), codec)

	adapter := &mocknetwork.Adapter{}
	err := f.RegisterAdapter(adapter)
	require.NoError(t, err)

	event := &message.TestMessage{Text: "this is a test message"}
	channel := network.Channel("test-channel")
	targetIds := unittest.IdentifierListFixture(10)

	params := []interface{}{channel, event, uint(3)}
	for _, id := range targetIds {
		params = append(params, id)
	}

	adapter.On("MulticastOnChannel", params...).Return(nil).Once()

	err = f.HandleIncomingEvent(event, channel, insecure.Protocol_MULTICAST, uint32(3), targetIds...)
	require.NoError(t, err)

	testifymock.AssertExpectationsForObjects(t, adapter)
}

// TestProcessAttackerMessage evaluates that conduit factory relays the messages coming from the attacker to its underlying network adapter.
func TestProcessAttackerMessage(t *testing.T) {
	codec := cbor.NewCodec()
	// corruptible conduit factory with no attacker registered.
	f := NewCorruptibleConduitFactory(unittest.Logger(), flow.BftTestnet, unittest.IdentifierFixture(), codec)

	adapter := &mocknetwork.Adapter{}
	err := f.RegisterAdapter(adapter)
	require.NoError(t, err)

	event := &message.TestMessage{Text: "this is a test message"}
	channel := network.Channel("test-channel")
	targetIds := unittest.IdentifierListFixture(10)

	params := []interface{}{channel, event, uint(3)}
	for _, id := range targetIds {
		params = append(params, id)
	}

	adapter.On("MulticastOnChannel", params...).Return(nil).Once()

	// imitates an RPC call from the attacker
	msg, err := f.eventToMessage(event, channel, insecure.Protocol_MULTICAST, uint32(3), targetIds...)
	require.NoError(t, err)

	err = f.processAttackerMessage(msg)
	require.NoError(t, err)

	testifymock.AssertExpectationsForObjects(t, adapter)
}

// TestEngineClosingChannel evaluates that factory closes the channel whenever the corresponding engine of that channel attempts
// on closing it.
func TestEngineClosingChannel(t *testing.T) {
	codec := cbor.NewCodec()
	// corruptible conduit factory with no attacker registered.
	f := NewCorruptibleConduitFactory(unittest.Logger(), flow.BftTestnet, unittest.IdentifierFixture(), codec)

	adapter := &mocknetwork.Adapter{}
	err := f.RegisterAdapter(adapter)
	require.NoError(t, err)

	channel := network.Channel("test-channel")

	// on invoking adapter.UnRegisterChannel(channel), it must return a nil, which means
	// that the channel has been unregistered by the adapter successfully.
	adapter.On("UnRegisterChannel", channel).Return(nil).Once()

	err = f.EngineClosingChannel(channel)
	require.NoError(t, err)

	// adapter's UnRegisterChannel method must be called once.
	testifymock.AssertExpectationsForObjects(t, adapter)
}
