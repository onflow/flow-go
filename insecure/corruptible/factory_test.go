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

// TestProcessAttackerMessage evaluates that corrupted conduit factory (ccf)
// relays the messages coming from the attack network to its underlying flow network.
//func TestProcessAttackerMessage(t *testing.T) {
//	withCorruptibleNetwork(t,
//		func(
//			corruptedId flow.Identity, // identity of ccf
//			factory *ConduitFactory, // the ccf itself
//			flowNetwork *mocknetwork.Adapter, // mock flow network that ccf uses to communicate with authorized flow nodes.
//			stream insecure.CorruptibleConduitFactory_ProcessAttackerMessageClient, // gRPC interface that attack network uses to send messages to this ccf.
//		) {
//			// creates a corrupted event that attacker is sending on the flow network through the
//			// corrupted conduit factory.
//			msg, event, _ := insecure.MessageFixture(t, cbor.NewCodec(), insecure.Protocol_MULTICAST, &message.TestMessage{
//				Text: fmt.Sprintf("this is a test message: %d", rand.Int()),
//			})
//
//			params := []interface{}{network.Channel(msg.ChannelID), event.FlowProtocolEvent, uint(3)}
//			targetIds, err := flow.ByteSlicesToIds(msg.TargetIDs)
//			require.NoError(t, err)
//
//			for _, id := range targetIds {
//				params = append(params, id)
//			}
//			corruptedEventDispatchedOnFlowNetWg := sync.WaitGroup{}
//			corruptedEventDispatchedOnFlowNetWg.Add(1)
//			flowNetwork.On("MulticastOnChannel", params...).Run(func(args testifymock.Arguments) {
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

// TestProcessAttackerMessage_ResultApproval_Dictated evaluates that when corrupted conduit factory (ccf) receives a result approval with
// empty signature field,
// it fills its related fields with its own credentials (e.g., signature), and passes it through the Flow network.
//func TestProcessAttackerMessage_ResultApproval_Dictated(t *testing.T) {
//	withCorruptibleNetwork(t,
//		func(
//			corruptedId flow.Identity,                                              // identity of ccf
//			factory *ConduitFactory,                                                // the ccf itself
//			flowNetwork *mocknetwork.Adapter,                                       // mock flow network that ccf uses to communicate with authorized flow nodes.
//			stream insecure.CorruptibleConduitFactory_ProcessAttackerMessageClient, // gRPC interface that attack network uses to send messages to this ccf.
//		) {
//			// creates a corrupted result approval that attacker is sending on the flow network through the
//			// corrupted conduit factory.
//			// corrupted result approval dictated by attacker needs to only have the attestation field, as the rest will be
//			// filled up by the CCF.
//			dictatedAttestation := *unittest.AttestationFixture()
//			msg, _, _ := insecure.MessageFixture(t, cbor.NewCodec(), insecure.Protocol_PUBLISH, &flow.ResultApproval{
//				Body: flow.ResultApprovalBody{
//					Attestation: dictatedAttestation,
//				},
//			})
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
//				approval, ok := args[1].(*flow.ResultApproval)
//				require.True(t, ok)
//
//				// attestation part of the approval must be the same as attacker dictates.
//				require.Equal(t, dictatedAttestation, approval.Body.Attestation)
//
//				// corrupted node should set the approver as its own id
//				require.Equal(t, corruptedId.NodeID, approval.Body.ApproverID)
//
//				// approval should have a valid attestation signature from corrupted node
//				id := approval.Body.Attestation.ID()
//				valid, err := corruptedId.StakingPubKey.Verify(approval.Body.AttestationSignature, id[:], factory.approvalHasher)
//				require.NoError(t, err)
//				require.True(t, valid)
//
//				// for now, we require a non-empty SPOCK
//				// TODO: check correctness of spock
//				require.NotEmpty(t, approval.Body.Spock)
//
//				// approval body should have a valid signature from corrupted node
//				bodyId := approval.Body.ID()
//				valid, err = corruptedId.StakingPubKey.Verify(approval.VerifierSignature, bodyId[:], factory.approvalHasher)
//				require.NoError(t, err)
//				require.True(t, valid)
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

// TestProcessAttackerMessage_ResultApproval_PassThrough evaluates that when corrupted conduit factory (
// ccf) receives a completely filled result approval,
// it fills its related fields with its own credentials (e.g., signature), and passes it through the Flow network.
//func TestProcessAttackerMessage_ResultApproval_PassThrough(t *testing.T) {
//	withCorruptibleNetwork(t,
//		func(
//			corruptedId flow.Identity, // identity of ccf
//			factory *ConduitFactory, // the ccf itself
//			flowNetwork *mocknetwork.Adapter, // mock flow network that ccf uses to communicate with authorized flow nodes.
//			stream insecure.CorruptibleConduitFactory_ProcessAttackerMessageClient, // gRPC interface that attack network uses to send messages to this ccf.
//		) {
//
//			passThroughApproval := unittest.ResultApprovalFixture()
//			msg, _, _ := insecure.MessageFixture(t, cbor.NewCodec(), insecure.Protocol_PUBLISH, passThroughApproval)
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
//				approval, ok := args[1].(*flow.ResultApproval)
//				require.True(t, ok)
//
//				// attestation part of the approval must be the same as attacker dictates.
//				require.Equal(t, passThroughApproval, approval)
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

// TestProcessAttackerMessage_ExecutionReceipt_Dictated evaluates that when corrupted conduit factory (ccf) receives an execution receipt with
// empty signature field, it fills its related fields with its own credentials (e.g., signature), and passes it through the Flow network.
//func TestProcessAttackerMessage_ExecutionReceipt_Dictated(t *testing.T) {
//	withCorruptibleNetwork(t,
//		func(
//			corruptedId flow.Identity, // identity of ccf
//			factory *ConduitFactory, // the ccf itself
//			flowNetwork *mocknetwork.Adapter, // mock flow network that ccf uses to communicate with authorized flow nodes.
//			stream insecure.CorruptibleConduitFactory_ProcessAttackerMessageClient, // gRPC interface that attack network uses to send messages to this ccf.
//		) {
//			// creates a corrupted execution receipt that attacker is sending on the flow network through the
//			// corrupted conduit factory.
//			// corrupted execution receipt dictated by attacker needs to only have the result field, as the rest will be
//			// filled up by the CCF.
//			dictatedResult := *unittest.ExecutionResultFixture()
//			msg, _, _ := insecure.MessageFixture(t, cbor.NewCodec(), insecure.Protocol_PUBLISH, &flow.ExecutionReceipt{
//				ExecutionResult: dictatedResult,
//			})
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
//				// result part of the receipt must be the same as attacker dictates.
//				require.Equal(t, dictatedResult, receipt.ExecutionResult)
//
//				// corrupted node should set itself as the executor
//				require.Equal(t, corruptedId.NodeID, receipt.ExecutorID)
//
//				// receipt should have a valid signature from corrupted node
//				id := receipt.ID()
//				valid, err := corruptedId.StakingPubKey.Verify(receipt.ExecutorSignature, id[:], factory.receiptHasher)
//				require.NoError(t, err)
//				require.True(t, valid)
//
//				// TODO: check correctness of spock
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
