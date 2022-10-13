package corruptnet

// This test file covers corrupt network tests that are not ingress or egress specific, including error conditions.

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/network/mocknetwork"

	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestEngineClosingChannel evaluates that corrupt network closes the channel whenever the corresponding
// engine of that channel attempts on closing it.
func TestEngineClosingChannel(t *testing.T) {
	runCorruptNetworkTest(t, unittest.Logger(),
		func(
			corruptedId flow.Identity, // identity of ccf
			corruptNetwork *Network,
			adapter *mocknetwork.Adapter, // mock adapter that ccf uses to communicate with authorized flow nodes.
			stream insecure.CorruptNetwork_ProcessAttackerMessageClient, // gRPC interface that orchestrator network uses to send messages to this ccf.
		) {
			channel := channels.TestNetworkChannel

			// on invoking adapter.UnRegisterChannel(channel), it must return a nil, which means
			// that the channel has been unregistered by the adapter successfully.
			adapter.On("UnRegisterChannel", channel).Return(nil).Once()

			err := corruptNetwork.EngineClosingChannel(channel)
			require.NoError(t, err)
		})
}

// TestProcessAttackerMessage_EmptyEgressIngressMessage checks that corrupt network returns an error
// and exits if both egress and ingress messages are nil.
func TestProcessAttackerMessage_EmptyEgressIngressMessage_Exit(t *testing.T) {
	f := func(t *testing.T) {
		processAttackerMessage_EmptyEgressIngressMessage_Exit(t)
	}
	unittest.CrashTest(t, f, "both ingress and egress messages can't be nil")
}

func processAttackerMessage_EmptyEgressIngressMessage_Exit(t *testing.T) {
	logger, _ := unittest.HookedLogger()

	runCorruptNetworkTest(t, logger,
		func(
			corruptedId flow.Identity, // identity of ccf
			corruptNetwork *Network,
			adapter *mocknetwork.Adapter, // mock adapter that ccf uses to communicate with authorized flow nodes.
			stream insecure.CorruptNetwork_ProcessAttackerMessageClient, // gRPC interface that orchestrator network uses to send messages to this ccf.
		) {
			corruptedEventDispatchedOnFlowNetWg := sync.WaitGroup{}
			corruptedEventDispatchedOnFlowNetWg.Add(1)
			adapter.On("MulticastOnChannel", mock.Anything).Run(func(args mock.Arguments) {
				corruptedEventDispatchedOnFlowNetWg.Done()
			}).Return(nil).Once()

			// create empty wrapper message with no ingress or egress message - this should never happen
			msg := &insecure.Message{}
			require.Nil(t, msg.Ingress)
			require.Nil(t, msg.Egress)

			// imitates a gRPC call from orchestrator to ccf through orchestrator network
			require.NoError(t, stream.Send(msg))

			unittest.RequireReturnsBefore(
				t,
				corruptedEventDispatchedOnFlowNetWg.Wait,
				1*time.Second,
				"attacker's message was not dispatched on flow network on time")
		})
}

// TestProcessAttackerMessage_NotEmptyEgressIngressMessage_Exit checks that corrupt network returns an error
// and exits if both egress and ingress messages are NOT nil.
func TestProcessAttackerMessage_NotEmptyEgressIngressMessage_Exit(t *testing.T) {
	f := func(t *testing.T) {
		processAttackerMessage_NotEmptyEgressIngressMessage_Exit(t)
	}
	unittest.CrashTest(t, f, "both ingress and egress messages can't be set")
}

func processAttackerMessage_NotEmptyEgressIngressMessage_Exit(t *testing.T) {
	logger, _ := unittest.HookedLogger()

	runCorruptNetworkTest(t, logger,
		func(
			corruptedId flow.Identity, // identity of ccf
			corruptNetwork *Network,
			adapter *mocknetwork.Adapter, // mock adapter that ccf uses to communicate with authorized flow nodes.
			stream insecure.CorruptNetwork_ProcessAttackerMessageClient, // gRPC interface that orchestrator network uses to send messages to this ccf.
		) {
			// creates a corrupted event that attacker is sending on the flow network through the
			// corrupted conduit factory.
			doubleMsg, egressEvent, _ := insecure.EgressMessageFixture(t, unittest.NetworkCodec(), insecure.Protocol_MULTICAST, &message.TestMessage{
				Text: fmt.Sprintf("this is a test message: %d", rand.Int()),
			})

			ingressMsg := insecure.IngressMessageFixture(t, unittest.NetworkCodec(), insecure.Protocol_MULTICAST, &message.TestMessage{
				Text: fmt.Sprintf("this is a test message: %d", rand.Int()),
			})

			params := []interface{}{channels.Channel(doubleMsg.Egress.ChannelID), egressEvent.FlowProtocolEvent, uint(3)}
			targetIds, err := flow.ByteSlicesToIds(doubleMsg.Egress.TargetIDs)
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
			require.NotNil(t, doubleMsg.Egress)
			doubleMsg.Ingress = ingressMsg.Ingress
			require.NotNil(t, doubleMsg.Ingress)

			// imitates a gRPC call from orchestrator to ccf through orchestrator network
			require.NoError(t, stream.Send(doubleMsg))

			unittest.RequireReturnsBefore(
				t,
				corruptedEventDispatchedOnFlowNetWg.Wait,
				1*time.Second,
				"attacker's message was not dispatched on flow network on time")
		})
}
