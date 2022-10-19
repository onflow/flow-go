package corruptnet

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/onflow/flow-go/network/channels"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/testutil"
	"github.com/onflow/flow-go/insecure"
	mockinsecure "github.com/onflow/flow-go/insecure/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestHandleOutgoingEvent_AttackerRegistered checks that egress messages (from a corrupt node to the corrupt network) are routed to a registered attacker.
// The attacker is mocked out in this test.
func TestHandleOutgoingEvent_AttackerRegistered(t *testing.T) {
	codec := unittest.NetworkCodec()
	corruptIdentity := unittest.IdentityFixture(unittest.WithAddress(insecure.DefaultAddress))
	flowNetwork := mocknetwork.NewNetwork(t)
	ccf := mockinsecure.NewCorruptConduitFactory(t)
	ccf.On("RegisterEgressController", mock.Anything).Return(nil)

	corruptNetwork, err := NewCorruptNetwork(
		unittest.Logger(),
		flow.BftTestnet,
		insecure.DefaultAddress,
		testutil.LocalFixture(t, corruptIdentity),
		codec,
		flowNetwork,
		ccf)
	require.NoError(t, err)

	attacker := newMockAttacker()

	attackerRegistered := sync.WaitGroup{}
	attackerRegistered.Add(1)
	go func() {
		attackerRegistered.Done()

		err := corruptNetwork.ConnectAttacker(&empty.Empty{}, attacker) // blocking call
		require.NoError(t, err)
	}()
	unittest.RequireReturnsBefore(t, attackerRegistered.Wait, 100*time.Millisecond, "could not register attacker on time")

	targetIds := unittest.IdentifierListFixture(10)
	msg := &message.TestMessage{Text: "this is a test msg"}
	channel := channels.TestNetworkChannel

	go func() {
		err := corruptNetwork.HandleOutgoingEvent(msg, channel, insecure.Protocol_MULTICAST, uint32(3), targetIds...)
		require.NoError(t, err)
	}()

	// For this test we use a mock attacker, that puts the incoming messages into a channel. Then in this test we keep reading from that channel till
	// either a message arrives or a timeout. Reading a message from that channel means attackers Observe has been called.
	var receivedMsg *insecure.Message
	unittest.RequireReturnsBefore(t, func() {
		receivedMsg = <-attacker.incomingBuffer
	}, 100*time.Millisecond, "mock attack could not receive incoming message on time")

	// checks content of the received message matches what has been sent.
	require.ElementsMatch(t, receivedMsg.Egress.TargetIDs, flow.IdsToBytes(targetIds))
	require.Equal(t, receivedMsg.Egress.TargetNum, uint32(3))
	require.Equal(t, receivedMsg.Egress.Protocol, insecure.Protocol_MULTICAST)
	require.Equal(t, receivedMsg.Egress.ChannelID, string(channel))

	decodedEvent, err := codec.Decode(receivedMsg.Egress.Payload)
	require.NoError(t, err)
	require.Equal(t, msg, decodedEvent)
	mock.AssertExpectationsForObjects(t, ccf)
}

// TestHandleOutgoingEvent_NoAttacker_UnicastOverNetwork checks that outgoing unicast events to the corrupt network
// are routed to the network adapter when no attacker is registered to the network.
func TestHandleOutgoingEvent_NoAttacker_UnicastOverNetwork(t *testing.T) {
	runCorruptNetworkTest(t, unittest.Logger(),
		func(
			corruptId flow.Identity, // identity of ccf
			corruptNetwork *Network,
			adapter *mocknetwork.Adapter, // mock adapter that ccf uses to communicate with authorized flow nodes.
			stream insecure.CorruptNetwork_ProcessAttackerMessageClient, // gRPC interface that attacker network uses to send messages to this ccf.
		) {
			msg := &message.TestMessage{Text: "this is a test msg"}
			channel := channels.TestNetworkChannel

			targetId := unittest.IdentifierFixture()

			adapter.On("UnicastOnChannel", channel, msg, targetId).Return(nil).Once()

			// simulate sending message by conduit
			err := corruptNetwork.HandleOutgoingEvent(msg, channel, insecure.Protocol_UNICAST, uint32(0), targetId)
			require.NoError(t, err)
		})
}

// TestHandleOutgoingEvent_NoAttacker_PublishOverNetwork checks that the outgoing publish events to the corrupt network
// are routed to the network adapter when no attacker registered to the network.
func TestHandleOutgoingEvent_NoAttacker_PublishOverNetwork(t *testing.T) {
	runCorruptNetworkTest(t, unittest.Logger(),
		func(
			corruptId flow.Identity, // identity of ccf
			corruptNetwork *Network,
			adapter *mocknetwork.Adapter, // mock adapter that ccf uses to communicate with authorized flow nodes.
			stream insecure.CorruptNetwork_ProcessAttackerMessageClient, // gRPC interface that attacker network uses to send messages to this ccf.
		) {
			msg := &message.TestMessage{Text: "this is a test msg"}
			channel := channels.TestNetworkChannel

			targetIds := unittest.IdentifierListFixture(10)
			params := []interface{}{channel, msg}
			for _, id := range targetIds {
				params = append(params, id)
			}

			adapter.On("PublishOnChannel", params...).Return(nil).Once()

			// simulate sending message by conduit
			err := corruptNetwork.HandleOutgoingEvent(msg, channel, insecure.Protocol_PUBLISH, uint32(0), targetIds...)
			require.NoError(t, err)
		})
}

// TestHandleOutgoingEvent_NoAttacker_MulticastOverNetwork checks that the outgoing multicast events to the corrupt network
// are routed to the network adapter when no attacker registered to the network.
func TestHandleOutgoingEvent_NoAttacker_MulticastOverNetwork(t *testing.T) {
	runCorruptNetworkTest(t, unittest.Logger(),
		func(
			corruptId flow.Identity, // identity of ccf
			corruptNetwork *Network,
			adapter *mocknetwork.Adapter, // mock adapter that ccf uses to communicate with authorized flow nodes.
			stream insecure.CorruptNetwork_ProcessAttackerMessageClient, // gRPC interface that attacker network uses to send messages to this ccf.
		) {
			msg := &message.TestMessage{Text: "this is a test msg"}
			channel := channels.TestNetworkChannel

			targetIds := unittest.IdentifierListFixture(10)

			params := []interface{}{channel, msg, uint(3)}
			for _, id := range targetIds {
				params = append(params, id)
			}
			adapter.On("MulticastOnChannel", params...).Return(nil).Once()

			// simulate sending message by conduit
			err := corruptNetwork.HandleOutgoingEvent(msg, channel, insecure.Protocol_MULTICAST, uint32(3), targetIds...)
			require.NoError(t, err)
		})
}

// TestProcessAttackerMessage checks that a corrupt network relays the messages to its underlying flow network.
func TestProcessAttackerMessage_MessageSentOnFlowNetwork(t *testing.T) {
	runCorruptNetworkTest(t, unittest.Logger(),
		func(
			corruptId flow.Identity, // identity of ccf
			corruptNetwork *Network,
			adapter *mocknetwork.Adapter, // mock adapter that ccf uses to communicate with authorized flow nodes.
			stream insecure.CorruptNetwork_ProcessAttackerMessageClient, // gRPC interface that attacker network uses to send messages to this ccf.
		) {
			// creates a corrupt event that attacker is sending on the flow network through the
			// corrupt conduit factory.
			msg, event, _ := insecure.EgressMessageFixture(t, unittest.NetworkCodec(), insecure.Protocol_MULTICAST, &message.TestMessage{
				Text: fmt.Sprintf("this is a test message: %d", rand.Int()),
			})

			params := []interface{}{channels.Channel(msg.Egress.ChannelID), event.FlowProtocolEvent, uint(3)}
			targetIds, err := flow.ByteSlicesToIds(msg.Egress.TargetIDs)
			require.NoError(t, err)

			for _, id := range targetIds {
				params = append(params, id)
			}
			corruptEventDispatchedOnFlowNetWg := sync.WaitGroup{}
			corruptEventDispatchedOnFlowNetWg.Add(1)
			adapter.On("MulticastOnChannel", params...).Run(func(args mock.Arguments) {
				corruptEventDispatchedOnFlowNetWg.Done()
			}).Return(nil).Once()

			// imitates a gRPC call from orchestrator to ccf through attacker network
			require.NoError(t, stream.Send(msg))

			unittest.RequireReturnsBefore(
				t,
				corruptEventDispatchedOnFlowNetWg.Wait,
				100*time.Millisecond,
				"attacker's message was not dispatched on flow network on time")
		})
}

// TestProcessAttackerMessage_ResultApproval_Dictated checks that when a corrupt network receives a result approval with an
// empty signature field, it fills its related fields with its own credentials (e.g., signature), and passes it through the Flow network.
func TestProcessAttackerMessage_ResultApproval_Dictated(t *testing.T) {
	runCorruptNetworkTest(t, unittest.Logger(),
		func(
			corruptId flow.Identity, // identity of ccf
			corruptNetwork *Network,
			adapter *mocknetwork.Adapter, // mock adapter that ccf uses to communicate with authorized flow nodes.
			stream insecure.CorruptNetwork_ProcessAttackerMessageClient, // gRPC interface that attacker network uses to send messages to this ccf.
		) {
			// creates a corrupt result approval that attacker is sending on the flow network through the
			// corrupt network.
			// corrupt result approval dictated by attacker needs to only have the attestation field, as the rest will be
			// filled up by the CCF.
			dictatedAttestation := *unittest.AttestationFixture()
			msg, _, _ := insecure.EgressMessageFixture(t, unittest.NetworkCodec(), insecure.Protocol_PUBLISH, &flow.ResultApproval{
				Body: flow.ResultApprovalBody{
					Attestation: dictatedAttestation,
				},
			})

			params := []interface{}{channels.Channel(msg.Egress.ChannelID), mock.Anything}
			targetIds, err := flow.ByteSlicesToIds(msg.Egress.TargetIDs)
			require.NoError(t, err)
			for _, id := range targetIds {
				params = append(params, id)
			}

			corruptEventDispatchedOnFlowNetWg := sync.WaitGroup{}
			corruptEventDispatchedOnFlowNetWg.Add(1)
			adapter.On("PublishOnChannel", params...).Run(func(args mock.Arguments) {
				approval, ok := args[1].(*flow.ResultApproval)
				require.True(t, ok)

				// attestation part of the approval must be the same as attacker dictates.
				require.Equal(t, dictatedAttestation, approval.Body.Attestation)

				// corrupt node should set the approver as its own id
				require.Equal(t, corruptId.NodeID, approval.Body.ApproverID)

				// approval should have a valid attestation signature from corrupt node
				id := approval.Body.Attestation.ID()
				valid, err := corruptId.StakingPubKey.Verify(approval.Body.AttestationSignature, id[:], corruptNetwork.approvalHasher)

				require.NoError(t, err)
				require.True(t, valid)

				// for now, we require a non-empty SPOCK
				// TODO: check correctness of spock
				require.NotEmpty(t, approval.Body.Spock)

				// approval body should have a valid signature from corrupt node
				bodyId := approval.Body.ID()
				valid, err = corruptId.StakingPubKey.Verify(approval.VerifierSignature, bodyId[:], corruptNetwork.approvalHasher)
				require.NoError(t, err)
				require.True(t, valid)

				corruptEventDispatchedOnFlowNetWg.Done()
				//}).Return(nil).Once()
			}).Return(nil).Once()
			// imitates a gRPC call from orchestrator to ccf through attacker network
			require.NoError(t, stream.Send(msg))

			unittest.RequireReturnsBefore(
				t,
				corruptEventDispatchedOnFlowNetWg.Wait,
				100*time.Millisecond,
				"attacker's message was not dispatched on flow network on time")
		})
}

// TestProcessAttackerMessage_ResultApproval_PassThrough checks that when a corrupt network
// receives a completely filled result approval,
// it fills its related fields with its own credentials (e.g., signature), and passes it through the Flow network.
func TestProcessAttackerMessage_ResultApproval_PassThrough(t *testing.T) {
	runCorruptNetworkTest(t, unittest.Logger(),
		func(
			corruptId flow.Identity, // identity of ccf
			corruptNetwork *Network,
			adapter *mocknetwork.Adapter, // mock flow network that ccf uses to communicate with authorized flow nodes.
			stream insecure.CorruptNetwork_ProcessAttackerMessageClient, // gRPC interface that attacker network uses to send messages to this ccf.
		) {

			passThroughApproval := unittest.ResultApprovalFixture()
			msg, _, _ := insecure.EgressMessageFixture(t, unittest.NetworkCodec(), insecure.Protocol_PUBLISH, passThroughApproval)

			params := []interface{}{channels.Channel(msg.Egress.ChannelID), mock.Anything}
			targetIds, err := flow.ByteSlicesToIds(msg.Egress.TargetIDs)
			require.NoError(t, err)
			for _, id := range targetIds {
				params = append(params, id)
			}

			corruptEventDispatchedOnFlowNetWg := sync.WaitGroup{}
			corruptEventDispatchedOnFlowNetWg.Add(1)
			adapter.On("PublishOnChannel", params...).Run(func(args mock.Arguments) {
				approval, ok := args[1].(*flow.ResultApproval)
				require.True(t, ok)

				// attestation part of the approval must be the same as attacker dictates.
				require.Equal(t, passThroughApproval, approval)

				corruptEventDispatchedOnFlowNetWg.Done()
			}).Return(nil).Once()

			// imitates a gRPC call from orchestrator to ccf through attacker network
			require.NoError(t, stream.Send(msg))

			unittest.RequireReturnsBefore(
				t,
				corruptEventDispatchedOnFlowNetWg.Wait,
				100*time.Millisecond,
				"attacker's message was not dispatched on flow network on time")
		})
}

// TestProcessAttackerMessage_ExecutionReceipt_Dictated checks that when a corrupt network receives an execution receipt with
// empty signature field, it fills its related fields with its own credentials (e.g., signature), and passes it through the Flow network.
func TestProcessAttackerMessage_ExecutionReceipt_Dictated(t *testing.T) {
	runCorruptNetworkTest(t, unittest.Logger(),
		func(
			corruptId flow.Identity, // identity of ccf
			corruptNetwork *Network,
			adapter *mocknetwork.Adapter, // mock flow network that ccf uses to communicate with authorized flow nodes.
			stream insecure.CorruptNetwork_ProcessAttackerMessageClient, // gRPC interface that attacker network uses to send messages to this ccf.
		) {
			// creates a corrupt execution receipt that attacker is sending on the flow network through the
			// corrupt conduit factory.
			// corrupt execution receipt dictated by attacker needs to only have the result field, as the rest will be
			// filled up by the CCF.
			dictatedResult := *unittest.ExecutionResultFixture()
			msg, _, _ := insecure.EgressMessageFixture(t, unittest.NetworkCodec(), insecure.Protocol_PUBLISH, &flow.ExecutionReceipt{
				ExecutionResult: dictatedResult,
			})

			params := []interface{}{channels.Channel(msg.Egress.ChannelID), mock.Anything}
			targetIds, err := flow.ByteSlicesToIds(msg.Egress.TargetIDs)
			require.NoError(t, err)
			for _, id := range targetIds {
				params = append(params, id)
			}

			corruptEventDispatchedOnFlowNetWg := sync.WaitGroup{}
			corruptEventDispatchedOnFlowNetWg.Add(1)
			adapter.On("PublishOnChannel", params...).Run(func(args mock.Arguments) {
				receipt, ok := args[1].(*flow.ExecutionReceipt)
				require.True(t, ok)

				// result part of the receipt must be the same as attacker dictates.
				require.Equal(t, dictatedResult, receipt.ExecutionResult)

				// corrupt node should set itself as the executor
				require.Equal(t, corruptId.NodeID, receipt.ExecutorID)

				// receipt should have a valid signature from corrupt node
				id := receipt.ID()
				valid, err := corruptId.StakingPubKey.Verify(receipt.ExecutorSignature, id[:], corruptNetwork.receiptHasher)
				require.NoError(t, err)
				require.True(t, valid)

				// TODO: check correctness of spock

				corruptEventDispatchedOnFlowNetWg.Done()
			}).Return(nil).Once()

			// imitates a gRPC call from orchestrator to ccf through attacker network
			require.NoError(t, stream.Send(msg))

			unittest.RequireReturnsBefore(
				t,
				corruptEventDispatchedOnFlowNetWg.Wait,
				100*time.Millisecond,
				"attacker's message was not dispatched on flow network on time")
		})
}

// TestProcessAttackerMessage_ExecutionReceipt_PassThrough checks that when a corrupt network
// receives a completely filled execution receipt, it treats it as a pass-through event and passes it as it is on the Flow network.
func TestProcessAttackerMessage_ExecutionReceipt_PassThrough(t *testing.T) {
	runCorruptNetworkTest(t, unittest.Logger(),
		func(
			corruptId flow.Identity, // identity of ccf
			corruptNetwork *Network,
			adapter *mocknetwork.Adapter, // mock flow network that ccf uses to communicate with authorized flow nodes.
			stream insecure.CorruptNetwork_ProcessAttackerMessageClient, // gRPC interface that attacker network uses to send messages to the corrupt network.
		) {

			passThroughReceipt := unittest.ExecutionReceiptFixture()
			msg, _, _ := insecure.EgressMessageFixture(t, unittest.NetworkCodec(), insecure.Protocol_PUBLISH, passThroughReceipt)

			params := []interface{}{channels.Channel(msg.Egress.ChannelID), mock.Anything}
			targetIds, err := flow.ByteSlicesToIds(msg.Egress.TargetIDs)
			require.NoError(t, err)
			for _, id := range targetIds {
				params = append(params, id)
			}

			corruptEventDispatchedOnFlowNetWg := sync.WaitGroup{}
			corruptEventDispatchedOnFlowNetWg.Add(1)
			adapter.On("PublishOnChannel", params...).Run(func(args mock.Arguments) {
				receipt, ok := args[1].(*flow.ExecutionReceipt)
				require.True(t, ok)

				// receipt should be completely intact.
				require.Equal(t, passThroughReceipt, receipt)

				corruptEventDispatchedOnFlowNetWg.Done()
			}).Return(nil).Once()

			// imitates a gRPC call from orchestrator to ccf through attacker network
			require.NoError(t, stream.Send(msg))

			unittest.RequireReturnsBefore(
				t,
				corruptEventDispatchedOnFlowNetWg.Wait,
				100*time.Millisecond,
				"attacker's message was not dispatched on flow network on time")
		})
}
