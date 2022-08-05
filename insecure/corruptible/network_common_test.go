package corruptible

import (
	"fmt"
	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"math/rand"
	"sync"
	"testing"
	"time"
)

// This test file covers corruptible network tests that are not ingress or egress specific, including error conditions.

// TestEngineClosingChannel evaluates that corruptible network closes the channel whenever the corresponding
// engine of that channel attempts on closing it.
func TestEngineClosingChannel(t *testing.T) {
	corruptibleNetwork, adapter := corruptibleNetworkFixture(t, unittest.Logger())
	channel := channels.TestNetworkChannel

	// on invoking adapter.UnRegisterChannel(channel), it must return a nil, which means
	// that the channel has been unregistered by the adapter successfully.
	adapter.On("UnRegisterChannel", channel).Return(nil).Once()

	err := corruptibleNetwork.EngineClosingChannel(channel)
	require.NoError(t, err)

	// adapter's UnRegisterChannel method must be called once.
	mock.AssertExpectationsForObjects(t, adapter)
}

// TestProcessAttackerMessage_EmptyEgressIngressMessage checks that corruptible network returns an error
// and exits if both egress and ingress messages are nil.
func TestProcessAttackerMessage_EmptyEgressIngressMessage_Exit(t *testing.T) {
	f := func(t *testing.T) {
		processAttackerMessage_EmptyEgressIngressMessage_Exit(t)
	}
	unittest.CrashTest(t, f, "both ingress and egress messages can't be nil", "TestProcessAttackerMessage_EmptyEgressIngressMessage_Exit")
}

func processAttackerMessage_EmptyEgressIngressMessage_Exit(t *testing.T) {
	logger, _ := unittest.HookedLogger()

	withCorruptibleNetwork(t, logger,
		func(
			corruptedId flow.Identity, // identity of ccf
			corruptibleNetwork *Network,
			adapter *mocknetwork.Adapter, // mock adapter that ccf uses to communicate with authorized flow nodes.
			stream insecure.CorruptibleConduitFactory_ProcessAttackerMessageClient, // gRPC interface that attack network uses to send messages to this ccf.
		) {
			// creates a corrupted event that attacker is sending on the flow network through the
			// corrupted conduit factory.
			msg, event, _ := insecure.EgressMessageFixture(t, unittest.NetworkCodec(), insecure.Protocol_MULTICAST, &message.TestMessage{
				Text: fmt.Sprintf("this is a test message: %d", rand.Int()),
			})

			params := []interface{}{channels.Channel(msg.Egress.ChannelID), event.FlowProtocolEvent, uint(3)}
			targetIds, err := flow.ByteSlicesToIds(msg.Egress.TargetIDs)
			require.NoError(t, err)

			for _, id := range targetIds {
				params = append(params, id)
			}
			corruptedEventDispatchedOnFlowNetWg := sync.WaitGroup{}
			corruptedEventDispatchedOnFlowNetWg.Add(1)
			adapter.On("MulticastOnChannel", params...).Run(func(args mock.Arguments) {
				corruptedEventDispatchedOnFlowNetWg.Done()
			}).Return(nil).Once()

			// set both message types to nil to force the error and exit - this should never happen
			msg.Ingress = nil
			msg.Egress = nil

			// imitates a gRPC call from orchestrator to ccf through attack network
			require.NoError(t, stream.Send(msg))

			unittest.RequireReturnsBefore(
				t,
				corruptedEventDispatchedOnFlowNetWg.Wait,
				1*time.Second,
				"attacker's message was not dispatched on flow network on time")
		})
}

// TestProcessAttackerMessage_NotEmptyEgressIngressMessage_Exit checks that corruptible network returns an error
// and exits if both egress and ingress messages are NOT nil.
func TestProcessAttackerMessage_NotEmptyEgressIngressMessage_Exit(t *testing.T) {
	f := func(t *testing.T) {
		processAttackerMessage_NotEmptyEgressIngressMessage_Exit(t)
	}
	unittest.CrashTest(t, f, "both ingress and egress messages can't be set", "TestProcessAttackerMessage_NotEmptyEgressIngressMessage_Exit")
}

func processAttackerMessage_NotEmptyEgressIngressMessage_Exit(t *testing.T) {
	logger, _ := unittest.HookedLogger()

	withCorruptibleNetwork(t, logger,
		func(
			corruptedId flow.Identity, // identity of ccf
			corruptibleNetwork *Network,
			adapter *mocknetwork.Adapter, // mock adapter that ccf uses to communicate with authorized flow nodes.
			stream insecure.CorruptibleConduitFactory_ProcessAttackerMessageClient, // gRPC interface that attack network uses to send messages to this ccf.
		) {
			// creates a corrupted event that attacker is sending on the flow network through the
			// corrupted conduit factory.
			msg, event, _ := insecure.EgressMessageFixture(t, unittest.NetworkCodec(), insecure.Protocol_MULTICAST, &message.TestMessage{
				Text: fmt.Sprintf("this is a test message: %d", rand.Int()),
			})

			msg2, _, _ := insecure.IngressMessageFixture(t, unittest.NetworkCodec(), insecure.Protocol_MULTICAST, &message.TestMessage{
				Text: fmt.Sprintf("this is a test message: %d", rand.Int()),
			})

			params := []interface{}{channels.Channel(msg.Egress.ChannelID), event.FlowProtocolEvent, uint(3)}
			targetIds, err := flow.ByteSlicesToIds(msg.Egress.TargetIDs)
			require.NoError(t, err)

			for _, id := range targetIds {
				params = append(params, id)
			}
			corruptedEventDispatchedOnFlowNetWg := sync.WaitGroup{}
			corruptedEventDispatchedOnFlowNetWg.Add(1)
			adapter.On("MulticastOnChannel", params...).Run(func(args mock.Arguments) {
				corruptedEventDispatchedOnFlowNetWg.Done()
			}).Return(nil).Once()

			// set both message types to not nil to force the error and exit - this should never happen
			require.NotNil(t, msg.Egress)
			msg.Ingress = msg2.Ingress
			require.NotNil(t, msg.Ingress)

			// imitates a gRPC call from orchestrator to ccf through attack network
			require.NoError(t, stream.Send(msg))

			unittest.RequireReturnsBefore(
				t,
				corruptedEventDispatchedOnFlowNetWg.Wait,
				1*time.Second,
				"attacker's message was not dispatched on flow network on time")
		})
}
