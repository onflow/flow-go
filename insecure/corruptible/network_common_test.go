package corruptible

import (
	"fmt"
	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"math/rand"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"
)

// This test file covers corruptible network tests that are not ingress or egress specific, including error conditions.

// TestEngineClosingChannel evaluates that corruptible network closes the channel whenever the corresponding
// engine of that channel attempts on closing it.
func TestEngineClosingChannel(t *testing.T) {
	corruptibleNetwork, adapter := corruptibleNetworkFixture(t, unittest.Logger())
	channel := channels.Channel("test-channel")

	// on invoking adapter.UnRegisterChannel(channel), it must return a nil, which means
	// that the channel has been unregistered by the adapter successfully.
	adapter.On("UnRegisterChannel", channel).Return(nil).Once()

	err := corruptibleNetwork.EngineClosingChannel(channel)
	require.NoError(t, err)

	// adapter's UnRegisterChannel method must be called once.
	mock.AssertExpectationsForObjects(t, adapter)
}

// TestProcessAttackerMessage_EmptyEgressIngressMessage checks that corruptible network returns an error
// if both egress and ingress messages are nil.
func TestProcessAttackerMessage_EmptyEgressIngressMessage(t *testing.T) {
	//var loggerHook unittest.LoggerHook
	logger, loggerHook := unittest.HookedLogger()

	if os.Getenv("BE_CRASHER") == "1" {
		//os.Exit(1)
		withCorruptibleNetwork(t, logger,
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

				msg.Ingress = nil
				msg.Egress = nil

				// imitates a gRPC call from orchestrator to ccf through attack network
				//err = stream.Send(msg)
				t.Logf("TestProcessAttackerMessage_EmptyEgressIngressMessage>1")
				require.NoError(t, stream.Send(msg))
				t.Logf("TestProcessAttackerMessage_EmptyEgressIngressMessage>2")

				//msg.Ingress = nil
				//msg.Egress = nil
				//
				//require.NoError(t, stream.Send(msg))
				//
				//msg.Ingress = nil
				//msg.Egress = nil
				//
				//require.NoError(t, stream.Send(msg))
				//require.NoError(t, stream.Send(msg))
				//require.NoError(t, stream.Send(msg))
				//require.NoError(t, stream.Context().Err())

				unittest.RequireReturnsBefore(
					t,
					corruptedEventDispatchedOnFlowNetWg.Wait,
					1*time.Second,
					"attacker's message was not dispatched on flow network on time")
			})
		return
	}

	// Start the actual test in a different subprocess
	cmd := exec.Command(os.Args[0], "-test.run=TestProcessAttackerMessage_EmptyEgressIngressMessage")
	cmd.Env = append(os.Environ(), "BE_CRASHER=1")
	//stdout, err := cmd.StdoutPipe()
	//require.NoError(t, err)
	//stderr, err := cmd.StderrPipe()
	//require.NoError(t, err)

	// expect error from run
	err := cmd.Run()
	require.Error(t, err)
	require.Contains(t, "exit status 1", err.Error())
	//if err := cmd.Start(); err != nil {
	//	t.Fatal(err)
	//}

	// Check that the log fatal message is what we expected
	//gotBytes, err := ioutil.ReadAll(stderr)
	//require.NoError(t, err)
	//got := string(gotBytes)
	//require.Equal(t, "", got)

	//stdoutBytes, err := ioutil.ReadAll(stdout)
	//require.NoError(t, err)
	//stdoutStr := string(stdoutBytes)
	//require.Contains(t, stdoutStr, "both ingress and egress messages can't be nil")

	require.Contains(t, loggerHook.Logs(), "both ingress and egress messages can't be nil")

	// Check that the program exited
	//err = cmd.Wait()
	//require.Contains(t, "exit status 1", err.Error())
	//if e, ok := err.(*exec.ExitError); !ok || e.Success() {
	//	t.Fatalf("Process ran with err %v, want exit status 1", err)
	//}
}
