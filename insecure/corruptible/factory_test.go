package corruptible

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow-go/engine/testutil"
	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestRegisterAdapter_FailDoubleRegistration checks that CorruptibleConduitFactory can be registered with only one adapter.
func TestRegisterAdapter_FailDoubleRegistration(t *testing.T) {
	me := testutil.LocalFixture(t, unittest.IdentityFixture())
	f := NewCorruptibleConduitFactory(
		unittest.Logger(),
		flow.BftTestnet,
		me,
		cbor.NewCodec(),
		"localhost:0")

	adapter := &mocknetwork.Adapter{}

	// registering adapter must go successful
	require.NoError(t, f.RegisterAdapter(adapter))

	// second attempt on registering adapter must fail
	require.Error(t, f.RegisterAdapter(adapter))
}

// TestNewConduit_HappyPath checks when factory has an adapter registered, it can successfully
// create conduits.
func TestNewConduit_HappyPath(t *testing.T) {
	me := testutil.LocalFixture(t, unittest.IdentityFixture())
	f := NewCorruptibleConduitFactory(
		unittest.Logger(),
		flow.BftTestnet,
		me,
		cbor.NewCodec(),
		"localhost:0")
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
	me := testutil.LocalFixture(t, unittest.IdentityFixture())
	f := NewCorruptibleConduitFactory(
		unittest.Logger(),
		flow.BftTestnet,
		me,
		cbor.NewCodec(),
		"localhost:0")
	channel := network.Channel("test-channel")

	c, err := f.NewConduit(context.Background(), channel)
	require.Error(t, err)
	require.Nil(t, c)
}

// TestFactoryHandleIncomingEvent_AttackerObserve evaluates that the incoming messages to the conduit factory are routed to the
// registered attacker if one exists.
func TestFactoryHandleIncomingEvent_AttackerObserve(t *testing.T) {
	cboreCodec := cbor.NewCodec()
	me := testutil.LocalFixture(t, unittest.IdentityFixture())
	f := NewCorruptibleConduitFactory(
		unittest.Logger(),
		flow.BftTestnet,
		me,
		cboreCodec,
		"localhost:0")
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
	require.Equal(t, receivedMsg.TargetNum, uint32(3))
	require.Equal(t, receivedMsg.Protocol, insecure.Protocol_MULTICAST)
	require.Equal(t, receivedMsg.ChannelID, string(channel))

	decodedEvent, err := cboreCodec.Decode(receivedMsg.Payload)
	require.NoError(t, err)
	require.Equal(t, event, decodedEvent)
}

// TestFactoryHandleIncomingEvent_UnicastOverNetwork evaluates that the incoming unicast events to the conduit factory are routed to the
// network adapter when no attacker registered to the factory.
func TestFactoryHandleIncomingEvent_UnicastOverNetwork(t *testing.T) {
	cborCodec := cbor.NewCodec()
	me := testutil.LocalFixture(t, unittest.IdentityFixture())
	// corruptible conduit factory with no attacker registered.
	f := NewCorruptibleConduitFactory(
		unittest.Logger(),
		flow.BftTestnet,
		me,
		cborCodec,
		"localhost:0")

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
	me := testutil.LocalFixture(t, unittest.IdentityFixture())
	// corruptible conduit factory with no attacker registered.
	f := NewCorruptibleConduitFactory(
		unittest.Logger(),
		flow.BftTestnet,
		me,
		codec,
		"localhost:0")

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
	me := testutil.LocalFixture(t, unittest.IdentityFixture())
	// corruptible conduit factory with no attacker registered.
	f := NewCorruptibleConduitFactory(
		unittest.Logger(),
		flow.BftTestnet,
		me,
		codec,
		"localhost:0")

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

// TestProcessAttackerMessage evaluates that corrupted conduit factory (ccf)
// relays the messages coming from the attack network to its underlying flow network.
func TestProcessAttackerMessage(t *testing.T) {
	withCorruptibleConduitFactory(t,
		func(
			corruptedId flow.Identity, // identity of ccf
			factory *ConduitFactory, // the ccf itself
			flowNetwork *mocknetwork.Adapter, // mock flow network that ccf uses to communicate with authorized flow nodes.
			stream insecure.CorruptibleConduitFactory_ProcessAttackerMessageClient, // gRPC interface that attack network uses to send messages to this ccf.
			msg *insecure.Message, // msg to be sent from orchestrator to the ccf.
			event interface{}, // corresponding event of the message which is expected to be dispatched on flow network by ccf.
		) {
			params := []interface{}{network.Channel(msg.ChannelID), event, uint(3)}
			targetIds, err := flow.ByteSlicesToIds(msg.TargetIDs)
			require.NoError(t, err)

			for _, id := range targetIds {
				params = append(params, id)
			}
			corruptedEventDispatchedOnFlowNetWg := sync.WaitGroup{}
			corruptedEventDispatchedOnFlowNetWg.Add(1)
			flowNetwork.On("MulticastOnChannel", params...).Run(func(args testifymock.Arguments) {
				corruptedEventDispatchedOnFlowNetWg.Done()
			}).Return(nil).Once()

			// imitates a gRPC call from orchestrator to ccf through attack network
			require.NoError(t, stream.Send(msg))

			unittest.RequireReturnsBefore(
				t,
				corruptedEventDispatchedOnFlowNetWg.Wait,
				1*time.Second,
				"attacker's message was not dispatched on flow network on time")
		})
}

// TestEngineClosingChannel evaluates that factory closes the channel whenever the corresponding engine of that channel attempts
// on closing it.
func TestEngineClosingChannel(t *testing.T) {
	codec := cbor.NewCodec()
	me := testutil.LocalFixture(t, unittest.IdentityFixture())
	// corruptible conduit factory with no attacker registered.
	f := NewCorruptibleConduitFactory(
		unittest.Logger(),
		flow.BftTestnet,
		me,
		codec,
		"localhost:0")

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

// withCorruptibleConduitFactory creates and starts a corruptible conduit factory, runs the "run" function and then
// terminates the factory.
func withCorruptibleConduitFactory(t *testing.T,
	run func(
		flow.Identity, // identity of ccf
		*ConduitFactory, // the ccf itself
		*mocknetwork.Adapter, // mock flow network that ccf uses to communicate with authorized flow nodes.
		insecure.CorruptibleConduitFactory_ProcessAttackerMessageClient, // gRPC interface that attack network uses to send messages to this ccf.
		*insecure.Message, // msg to be sent from orchestrator to the ccf.
		interface{}, // corresponding event of the message which is expected to be dispatched on flow network by ccf.
	)) {

	corruptedIdentity := unittest.IdentityFixture(unittest.WithAddress("localhost:0"))

	// life-cycle management of corruptible conduit factory.
	ctx, cancel := context.WithCancel(context.Background())
	ccfCtx, errChan := irrecoverable.WithSignaler(ctx)
	go func() {
		select {
		case err := <-errChan:
			t.Error("mock corruptible conduit factory startup encountered fatal error", err)
		case <-ctx.Done():
			return
		}
	}()

	me := testutil.LocalFixture(t, unittest.IdentityFixture())
	ccf := NewCorruptibleConduitFactory(
		unittest.Logger(),
		flow.BftTestnet,
		me,
		cbor.NewCodec(),
		"localhost:0",
	)

	// starts corruptible conduit factory
	ccf.Start(ccfCtx)
	unittest.RequireCloseBefore(t, ccf.Ready(), 1*time.Second, "could not start corruptible conduit factory on time")

	// extracting port that ccf gRPC server is running on
	_, ccfPortStr, err := net.SplitHostPort(ccf.ServerAddress())
	require.NoError(t, err)

	// registers a mock adapter to the corruptible conduit factory
	adapter := &mocknetwork.Adapter{}
	err = ccf.RegisterAdapter(adapter)
	require.NoError(t, err)

	// imitating an attacker dial to corruptible conduit factory (ccf) and opening a stream to it
	// on which the attacker dictates ccf to relay messages on the actual flow network
	gRpcClient, err := grpc.Dial(
		fmt.Sprintf("localhost:%s", ccfPortStr),
		grpc.WithTransportCredentials(grpcinsecure.NewCredentials()))
	require.NoError(t, err)

	client := insecure.NewCorruptibleConduitFactoryClient(gRpcClient)
	stream, err := client.ProcessAttackerMessage(context.Background())
	require.NoError(t, err)

	// creates a corrupted event that attacker is sending on the flow network through the
	// corrupted conduit factory.
	msg, event, _ := insecure.MessageFixture(t, cbor.NewCodec(), insecure.Protocol_MULTICAST)

	run(*corruptedIdentity, ccf, adapter, stream, msg, event.FlowProtocolEvent)

	// terminates attackNetwork
	cancel()
	unittest.RequireCloseBefore(t, ccf.Done(), 1*time.Second, "could not stop corruptible conduit on time")
}
