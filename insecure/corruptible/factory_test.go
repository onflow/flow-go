package corruptible

import (
	"context"
	mockinsecure "github.com/onflow/flow-go/insecure/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
	"testing"
)

// TestNewConduit_HappyPath checks when factory has an adapter registered and an egress controller,
// it can successfully create conduits.
func TestNewConduit_HappyPath(t *testing.T) {
	ccf := NewCorruptibleConduitFactory(unittest.Logger(), flow.BftTestnet)
	channel := network.Channel("test-channel")
	require.NoError(t, ccf.RegisterEgressController(&mockinsecure.EgressController{}))
	require.NoError(t, ccf.RegisterAdapter(&mocknetwork.Adapter{}))

	c, err := ccf.NewConduit(context.Background(), channel)
	require.NoError(t, err)
	require.NotNil(t, c)
}

// TestRegisterAdapter_FailDoubleRegistration checks that CorruptibleConduitFactory can be registered with only one adapter.
func TestRegisterAdapter_FailDoubleRegistration(t *testing.T) {
	ccf := NewCorruptibleConduitFactory(unittest.Logger(), flow.BftTestnet)
	adapter := &mocknetwork.Adapter{}

	// registering adapter should be successful
	require.NoError(t, ccf.RegisterAdapter(adapter))

	// second attempt at registering adapter should fail
	require.ErrorContains(t, ccf.RegisterAdapter(adapter), "network adapter, one already exists")
}

// TestRegisterEgressController_FailDoubleRegistration checks that CorruptibleConduitFactory can be registered with only one egress controller.
func TestRegisterEgressController_FailDoubleRegistration(t *testing.T) {
	ccf := NewCorruptibleConduitFactory(unittest.Logger(), flow.BftTestnet)
	egressController := &mockinsecure.EgressController{}

	// registering egress controller should be successful
	require.NoError(t, ccf.RegisterEgressController(egressController))

	// second attempt at registering egress controller should fail
	require.ErrorContains(t, ccf.RegisterEgressController(egressController), "egress controller, one already exists")

}

// TestNewConduit_MissingAdapter checks when factory does not have an adapter registered (but does have egress controller),
// any attempts on creating a conduit fails with an error.
func TestNewConduit_MissingAdapter(t *testing.T) {
	ccf := NewCorruptibleConduitFactory(unittest.Logger(), flow.BftTestnet)
	channel := network.Channel("test-channel")
	require.NoError(t, ccf.RegisterEgressController(&mockinsecure.EgressController{}))

	c, err := ccf.NewConduit(context.Background(), channel)
	require.ErrorContains(t, err, "missing a registered network adapter")
	require.Nil(t, c)
}

// TestNewConduit_MissingEgressController checks that test fails when factory doesn't have egress controller,
// but does have adapter.
func TestNewConduit_MissingEgressController(t *testing.T) {
	ccf := NewCorruptibleConduitFactory(unittest.Logger(), flow.BftTestnet)
	channel := network.Channel("test-channel")
	require.NoError(t, ccf.RegisterAdapter(&mocknetwork.Adapter{}))

	c, err := ccf.NewConduit(context.Background(), channel)
	require.ErrorContains(t, err, "missing a registered egress controller")
	require.Nil(t, c)
}

// TestProcessAttackerMessage_ExecutionReceipt_PassThrough evaluates that when corrupted conduit factory (
// ccf) receives a completely filled execution receipt, it treats it as a pass-through event and passes it as it is on the Flow network.
//func TestProcessAttackerMessage_ExecutionReceipt_PassThrough(t *testing.T) {
//	withCorruptibleNetwork(t,
//		func(
//			corruptedId flow.Identity,                                              // identity of ccf
//			factory *ConduitFactory,                                                // the ccf itself
//			flowNetwork *mocknetwork.Adapter,                                       // mock flow network that ccf uses to communicate with authorized flow nodes.
//			stream insecure.CorruptibleConduitFactory_ProcessAttackerMessageClient, // gRPC interface that attack network uses to send messages to this ccf.
//		) {
//
//			passThroughReceipt := unittest.ExecutionReceiptFixture()
//			msg, _, _ := insecure.MessageFixture(t, cbor.NewCodec(), insecure.Protocol_PUBLISH, passThroughReceipt)
//
//			params := []interface{}{network.Channel(msg.ChannelID), testifymock.Anything}
//			targetIds, err := flow.ByteSlicesToIds(msg.TargetIDs)
//			require.NoError(t, err)
//			for _, id := range targetIds {
//				params = append(params, id)
//			}
//
//			corruptedEventDispatchedOnFlowNetWg := sync.WaitGroup{}
//			corruptedEventDispatchedOnFlowNetWg.Add(1)
//			flowNetwork.On("PublishOnChannel", params...).Run(func(args testifymock.Arguments) {
//				receipt, ok := args[1].(*flow.ExecutionReceipt)
//				require.True(t, ok)
//
//				// receipt should be completely intact.
//				require.Equal(t, passThroughReceipt, receipt)
//
//				corruptedEventDispatchedOnFlowNetWg.Done()
//			}).Return(nil).Once()
//
//			// imitates a gRPC call from orchestrator to ccf through attack network
//			require.NoError(t, stream.Send(msg))
//
//			unittest.RequireReturnsBefore(
//				t,
//				corruptedEventDispatchedOnFlowNetWg.Wait,
//				1*time.Second,
//				"attacker's message was not dispatched on flow network on time")
//		})
//}

// TestEngineClosingChannel evaluates that factory closes the channel whenever the corresponding engine of that channel attempts
// on closing it.
//func TestEngineClosingChannel(t *testing.T) {
//	codec := cbor.NewCodec()
//	me := testutil.LocalFixture(t, unittest.IdentityFixture())
//	// corruptible conduit factory with no attacker registered.
//	f := NewCorruptibleConduitFactory(
//		unittest.Logger(),
//		flow.BftTestnet,
//		me,
//		codec,
//		"localhost:0")
//
//	adapter := &mocknetwork.Adapter{}
//	err := f.RegisterAdapter(adapter)
//	require.NoError(t, err)
//
//	channel := network.Channel("test-channel")
//
//	// on invoking adapter.UnRegisterChannel(channel), it must return a nil, which means
//	// that the channel has been unregistered by the adapter successfully.
//	adapter.On("UnRegisterChannel", channel).Return(nil).Once()
//
//	err = f.EngineClosingChannel(channel)
//	require.NoError(t, err)
//
//	// adapter's UnRegisterChannel method must be called once.
//	testifymock.AssertExpectationsForObjects(t, adapter)
//}
