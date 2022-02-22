package corruptible

import (
	"context"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestRegisterAdapter(t *testing.T) {
	f := NewCorruptibleConduitFactory(unittest.IdentifierFixture(), cbor.NewCodec())

	adapter := &mocknetwork.Adapter{}

	// registering adapter must go successful
	require.NoError(t, f.RegisterAdapter(adapter))

	// second attempt on registering adapter must fail
	require.Error(t, f.RegisterAdapter(adapter))
}

// TestNewConduit_HappyPath checks when factory has an adapter registered, it can successfully
// create conduits.
func TestNewConduit_HappyPath(t *testing.T) {
	f := NewCorruptibleConduitFactory(unittest.IdentifierFixture(), cbor.NewCodec())
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
	f := NewCorruptibleConduitFactory(unittest.IdentifierFixture(), cbor.NewCodec())
	channel := network.Channel("test-channel")

	c, err := f.NewConduit(context.Background(), channel)
	require.Error(t, err)
	require.Nil(t, c)
}

// TestFactoryHandleIncomingEvent_AttackerObserve evaluates that the incoming messages to the conduit factory are routed to the
// registered attacker if one exists.
func TestFactoryHandleIncomingEvent_AttackerObserve(t *testing.T) {
	codec := cbor.NewCodec()
	f := NewCorruptibleConduitFactory(unittest.IdentifierFixture(), codec)
	attacker := &mockAttacker{incomingBuffer: make(chan *insecure.Message)}
	f.attacker = attacker

	event := &message.TestMessage{Text: "this is a test message"}
	targetIds := unittest.IdentifierListFixture(10)
	channel := network.Channel("test-channel")

	go func() {
		err := f.HandleIncomingEvent(context.Background(), event, channel, insecure.Protocol_MULTICAST, uint32(3), targetIds...)
		require.NoError(t, err)
	}()

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
	f := NewCorruptibleConduitFactory(unittest.IdentifierFixture(), codec)

	adapter := &mocknetwork.Adapter{}
	err := f.RegisterAdapter(adapter)
	require.NoError(t, err)

	event := &message.TestMessage{Text: "this is a test message"}
	targetId := unittest.IdentifierFixture()
	channel := network.Channel("test-channel")

	adapter.On("UnicastOnChannel", channel, event, targetId).Return(nil).Once()

	err = f.HandleIncomingEvent(context.Background(), event, channel, insecure.Protocol_UNICAST, uint32(0), targetId)
	require.NoError(t, err)

	testifymock.AssertExpectationsForObjects(t, adapter)
}

// TestFactoryHandleIncomingEvent_PublishOverNetwork evaluates that the incoming publish events to the conduit factory are routed to the
// network adapter when no attacker registered to the factory.
func TestFactoryHandleIncomingEvent_PublishOverNetwork(t *testing.T) {
	codec := cbor.NewCodec()
	// corruptible conduit factory with no attacker registered.
	f := NewCorruptibleConduitFactory(unittest.IdentifierFixture(), codec)

	adapter := &mocknetwork.Adapter{}
	err := f.RegisterAdapter(adapter)
	require.NoError(t, err)

	event := &message.TestMessage{Text: "this is a test message"}
	channel := network.Channel("test-channel")

	adapter.On("PublishOnChannel", channel, event).Return(nil).Once()

	err = f.HandleIncomingEvent(context.Background(), event, channel, insecure.Protocol_PUBLISH, uint32(0))
	require.NoError(t, err)

	testifymock.AssertExpectationsForObjects(t, adapter)
}

type mockAttacker struct {
	incomingBuffer chan *insecure.Message
}

func (m *mockAttacker) Observe(_ context.Context, in *insecure.Message, _ ...grpc.CallOption) (*empty.Empty, error) {
	m.incomingBuffer <- in
	return &empty.Empty{}, nil
}
