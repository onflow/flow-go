package corruptible

import (
	"context"
	"fmt"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/onflow/flow-go/engine/testutil"
	"github.com/onflow/flow-go/insecure"
	mockinsecure "github.com/onflow/flow-go/insecure/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"
	"math/rand"
	"net"
	"sync"
	"testing"
	"time"
)

// TestHandleOutgoingEvent_AttackerObserve evaluates that the incoming messages to the corrupted network are routed to the
// registered attacker if one exists.
func TestHandleOutgoingEvent_AttackerObserve(t *testing.T) {
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

// TestHandleOutgoingEvent_NoAttacker_UnicastOverNetwork checks that outgoing unicast events to the corrupted network
// are routed to the network adapter when no attacker is registered to the network.
func TestHandleOutgoingEvent_NoAttacker_UnicastOverNetwork(t *testing.T) {
	corruptibleNetwork, adapter := getCorruptibleNetworkNoAttacker(t, nil)
	event, channel := getMessageAndChannel()
	targetId := unittest.IdentifierFixture()

	adapter.On("UnicastOnChannel", channel, event, targetId).Return(nil).Once()

	// simulate sending message by conduit
	err := corruptibleNetwork.HandleOutgoingEvent(event, channel, insecure.Protocol_UNICAST, uint32(0), targetId)
	require.NoError(t, err)

	// check that correct Adapter method called
	mock.AssertExpectationsForObjects(t, adapter)
}

// TestHandleOutgoingEvent_NoAttacker_PublishOverNetwork checks that the outgoing publish events to the corrupted network
// are routed to the network adapter when no attacker registered to the network.
func TestHandleOutgoingEvent_NoAttacker_PublishOverNetwork(t *testing.T) {
	corruptibleNetwork, adapter := getCorruptibleNetworkNoAttacker(t, nil)
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

	// check that correct Adapter method called
	mock.AssertExpectationsForObjects(t, adapter)
}

// TestHandleOutgoingEvent_NoAttacker_MulticastOverNetwork checks that the outgoing multicast events to the corrupted network
// are routed to the network adapter when no attacker registered to the network.
func TestHandleOutgoingEvent_NoAttacker_MulticastOverNetwork(t *testing.T) {
	corruptibleNetwork, adapter := getCorruptibleNetworkNoAttacker(t, nil)
	event, channel := getMessageAndChannel()

	targetIds := unittest.IdentifierListFixture(10)

	params := []interface{}{channel, event, uint(3)}
	for _, id := range targetIds {
		params = append(params, id)
	}
	adapter.On("MulticastOnChannel", params...).Return(nil).Once()

	// simulate sending message by conduit
	err := corruptibleNetwork.HandleOutgoingEvent(event, channel, insecure.Protocol_MULTICAST, uint32(3), targetIds...)
	require.NoError(t, err)

	// check that correct Adapter method called
	mock.AssertExpectationsForObjects(t, adapter)
}

// TestProcessAttackerMessage evaluates that corrupted network relays the messages to its underlying flow network.
func TestProcessAttackerMessage(t *testing.T) {
	withCorruptibleNetwork(t,
		func(
			corruptedId flow.Identity, // identity of ccf
			corruptibleNetwork *Network,
			adapter *mocknetwork.Adapter, // mock adapter that ccf uses to communicate with authorized flow nodes.
			stream insecure.CorruptibleConduitFactory_ProcessAttackerMessageClient, // gRPC interface that attack network uses to send messages to this ccf.
		) {
			// creates a corrupted event that attacker is sending on the flow network through the
			// corrupted conduit factory.
			msg, event, _ := insecure.MessageFixture(t, cbor.NewCodec(), insecure.Protocol_MULTICAST, &message.TestMessage{
				Text: fmt.Sprintf("this is a test message: %d", rand.Int()),
			})

			params := []interface{}{network.Channel(msg.ChannelID), event.FlowProtocolEvent, uint(3)}
			targetIds, err := flow.ByteSlicesToIds(msg.TargetIDs)
			require.NoError(t, err)

			for _, id := range targetIds {
				params = append(params, id)
			}
			corruptedEventDispatchedOnFlowNetWg := sync.WaitGroup{}
			corruptedEventDispatchedOnFlowNetWg.Add(1)
			adapter.On("MulticastOnChannel", params...).Run(func(args mock.Arguments) {
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

// TestProcessAttackerMessage_ResultApproval_Dictated checks that when corruptible network receives a result approval with
// empty signature field,
// it fills its related fields with its own credentials (e.g., signature), and passes it through the Flow network.
func TestProcessAttackerMessage_ResultApproval_Dictated(t *testing.T) {
	withCorruptibleNetwork(t,
		func(
			corruptedId flow.Identity, // identity of ccf
			corruptibleNetwork *Network,
			adapter *mocknetwork.Adapter, // mock adapter that ccf uses to communicate with authorized flow nodes.
			stream insecure.CorruptibleConduitFactory_ProcessAttackerMessageClient, // gRPC interface that attack network uses to send messages to this ccf.
		) {
			// creates a corrupted result approval that attacker is sending on the flow network through the
			// corrupted network.
			// corrupted result approval dictated by attacker needs to only have the attestation field, as the rest will be
			// filled up by the CCF.
			dictatedAttestation := *unittest.AttestationFixture()
			msg, _, _ := insecure.MessageFixture(t, cbor.NewCodec(), insecure.Protocol_PUBLISH, &flow.ResultApproval{
				Body: flow.ResultApprovalBody{
					Attestation: dictatedAttestation,
				},
			})

			params := []interface{}{network.Channel(msg.ChannelID), mock.Anything}
			targetIds, err := flow.ByteSlicesToIds(msg.TargetIDs)
			require.NoError(t, err)
			for _, id := range targetIds {
				params = append(params, id)
			}

			corruptedEventDispatchedOnFlowNetWg := sync.WaitGroup{}
			corruptedEventDispatchedOnFlowNetWg.Add(1)
			adapter.On("PublishOnChannel", params...).Run(func(args mock.Arguments) {
				approval, ok := args[1].(*flow.ResultApproval)
				require.True(t, ok)

				// attestation part of the approval must be the same as attacker dictates.
				require.Equal(t, dictatedAttestation, approval.Body.Attestation)

				// corrupted node should set the approver as its own id
				require.Equal(t, corruptedId.NodeID, approval.Body.ApproverID)

				// approval should have a valid attestation signature from corrupted node
				id := approval.Body.Attestation.ID()
				valid, err := corruptedId.StakingPubKey.Verify(approval.Body.AttestationSignature, id[:], corruptibleNetwork.approvalHasher)

				require.NoError(t, err)
				require.True(t, valid)

				// for now, we require a non-empty SPOCK
				// TODO: check correctness of spock
				require.NotEmpty(t, approval.Body.Spock)

				// approval body should have a valid signature from corrupted node
				bodyId := approval.Body.ID()
				valid, err = corruptedId.StakingPubKey.Verify(approval.VerifierSignature, bodyId[:], corruptibleNetwork.approvalHasher)
				require.NoError(t, err)
				require.True(t, valid)

				corruptedEventDispatchedOnFlowNetWg.Done()
			}).Return(nil).Once()

			// imitates a gRPC call from orchestrator to ccf through attack network
			require.NoError(t, stream.Send(msg))

			unittest.RequireReturnsBefore(
				t,
				corruptedEventDispatchedOnFlowNetWg.Wait,
				//1*time.Second,
				1000*time.Second,
				"attacker's message was not dispatched on flow network on time")
		})
}

// ******************** HELPERS ****************************

func getCorruptibleNetworkNoAttacker(t *testing.T, corruptedID *flow.Identity) (*Network, *mocknetwork.Adapter) {
	// create corruptible network with no attacker registered
	codec := cbor.NewCodec()

	corruptedIdentity := corruptedID
	if corruptedID == nil {
		corruptedIdentity = unittest.IdentityFixture(unittest.WithAddress("localhost:0"))
	}

	//corruptedIdentity := unittest.IdentityFixture(unittest.WithAddress("localhost:0"))
	flowNetwork := &mocknetwork.Network{}
	flowNetwork.On("Start", mock.Anything).Return()
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

// withCorruptibleNetwork creates and starts a corruptible network, runs the "run" function and then
// terminates the network.
func withCorruptibleNetwork(t *testing.T,
	run func(
		flow.Identity, // identity of ccf
		*Network, // corruptible network
		*mocknetwork.Adapter, // mock adapter that corrupted network uses to communicate with authorized flow nodes.
		insecure.CorruptibleConduitFactory_ProcessAttackerMessageClient, // gRPC interface that attack network uses to send messages to this ccf.
	)) {

	corruptedIdentity := unittest.IdentityFixture(unittest.WithAddress("localhost:0"))

	// life-cycle management of corruptible network
	ctx, cancel := context.WithCancel(context.Background())
	ccfCtx, errChan := irrecoverable.WithSignaler(ctx)
	go func() {
		select {
		case err := <-errChan:
			t.Error("corruptible network startup encountered fatal error", err)
		case <-ctx.Done():
			return
		}
	}()

	corruptibleNetwork, adapter := getCorruptibleNetworkNoAttacker(t, corruptedIdentity)

	// start corruptible network
	corruptibleNetwork.Start(ccfCtx)
	unittest.RequireCloseBefore(t, corruptibleNetwork.Ready(), 1*time.Second, "could not start corruptible network on time")

	// extracting port that ccf gRPC server is running on
	_, ccfPortStr, err := net.SplitHostPort(corruptibleNetwork.ServerAddress())
	require.NoError(t, err)

	// imitating an attacker dial to corruptible network and opening a stream to it
	// on which the attacker dictates to relay messages on the actual flow network
	gRpcClient, err := grpc.Dial(
		fmt.Sprintf("localhost:%s", ccfPortStr),
		grpc.WithTransportCredentials(grpcinsecure.NewCredentials()))
	require.NoError(t, err)

	client := insecure.NewCorruptibleConduitFactoryClient(gRpcClient)
	stream, err := client.ProcessAttackerMessage(context.Background())
	require.NoError(t, err)

	run(*corruptedIdentity, corruptibleNetwork, adapter, stream)

	// terminates attackNetwork
	cancel()
	unittest.RequireCloseBefore(t, corruptibleNetwork.Done(), 1*time.Second, "could not stop corruptible conduit on time")
}
