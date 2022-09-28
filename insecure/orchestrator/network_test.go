package orchestrator

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/insecure"
	mockinsecure "github.com/onflow/flow-go/insecure/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestOrchestratorNetworkObserve_SingleMessage(t *testing.T) {
	testOrchestratorNetworkObserve(t, 1)
}

func TestOrchestratorNetworkObserve_MultipleConcurrentMessages(t *testing.T) {
	testOrchestratorNetworkObserve(t, 10)
}

// testOrchestratorNetworkObserve evaluates that upon receiving concurrent messages from corruptible conduits, the orchestrator network
// decodes the messages into events and relays them to its registered orchestrator.
func testOrchestratorNetworkObserve(t *testing.T, concurrencyDegree int) {
	// creates event fixtures and their corresponding messages.
	messages, egressEvents, corruptedIds := insecure.EgressMessageFixtures(t, unittest.NetworkCodec(), insecure.Protocol_MULTICAST, concurrencyDegree)

	withMockOrchestrator(
		t,
		corruptedIds,
		func(orchestratorNetwork *Network, orchestrator *mockinsecure.AttackOrchestrator, ccfs []*mockCorruptNetwork) {
			// mocks orchestrator to receive each event exactly once.
			orchestratorWG := mockOrchestratorHandlingEgressEvent(t, orchestrator, egressEvents)

			// sends all messages concurrently to the orchestrator network (imitating corruptible conduits sending
			// messages concurrently to orchestrator network).
			orchestratorNetworkSendWG := sync.WaitGroup{}
			orchestratorNetworkSendWG.Add(concurrencyDegree)

			for i, msg := range messages {
				msg := msg
				ccf := ccfs[i]

				go func() {
					err := ccf.attackerObserveStream.Send(msg) // pretends the ith ccf is sending message to attacker for observe
					require.NoError(t, err)
					orchestratorNetworkSendWG.Done()
				}()
			}

			// all messages should be sent to orchestrator network in a timely fashion.
			unittest.RequireReturnsBefore(t, orchestratorNetworkSendWG.Wait, 1*time.Second, "could not send all messages to attack orchestratorNetwork on time")
			// all events should be relayed to the orchestrator by the orchestrator network in a timely fashion.
			unittest.RequireReturnsBefore(t, orchestratorWG.Wait, 1*time.Second, "orchestrator could not receive messages on time")
		})
}

func TestOrchestratorNetworkUnicast_SingleMessage(t *testing.T) {
	testOrchestratorNetwork(t, insecure.Protocol_UNICAST, 1)
}

func TestOrchestratorNetworkUnicast_ConcurrentMessages(t *testing.T) {
	testOrchestratorNetwork(t, insecure.Protocol_UNICAST, 10)
}

func TestOrchestratorNetworkMulticast_SingleMessage(t *testing.T) {
	testOrchestratorNetwork(t, insecure.Protocol_MULTICAST, 1)
}

func TestOrchestratorNetworkMulticast_ConcurrentMessages(t *testing.T) {
	testOrchestratorNetwork(t, insecure.Protocol_MULTICAST, 10)
}

func TestOrchestratorNetworkPublish_SingleMessage(t *testing.T) {
	testOrchestratorNetwork(t, insecure.Protocol_PUBLISH, 1)
}

func TestOrchestratorNetworkPublish_ConcurrentMessages(t *testing.T) {
	testOrchestratorNetwork(t, insecure.Protocol_PUBLISH, 10)
}

// testOrchestratorNetwork evaluates that the orchestrator can successfully route an event to a corrupted node through the orchestrator network.
// By a corrupted node here, we mean a node that runs with a corruptible conduit factory.
func testOrchestratorNetwork(t *testing.T, protocol insecure.Protocol, concurrencyDegree int) {
	// creates event fixtures and their corresponding messages.
	_, egressEvents, corruptedIds := insecure.EgressMessageFixtures(t, unittest.NetworkCodec(), protocol, concurrencyDegree)

	withMockOrchestrator(t,
		corruptedIds,
		func(orchestratorNetwork *Network, _ *mockinsecure.AttackOrchestrator, ccfs []*mockCorruptNetwork) {
			attackerMsgReceived := &sync.WaitGroup{}
			attackerMsgReceived.Add(concurrencyDegree)

			for i, corruptedId := range corruptedIds {
				corruptedId := corruptedId
				ccf := ccfs[i]

				// testing message delivery from orchestrator network to ccfs
				go func() {
					msg := <-ccf.attackerMsg
					matchEventForMessage(t, egressEvents, msg, corruptedId.NodeID)
					attackerMsgReceived.Done()
				}()
			}

			orchestratorNetworkSendWG := &sync.WaitGroup{}
			orchestratorNetworkSendWG.Add(concurrencyDegree)

			for _, event := range egressEvents {
				event := event
				go func() {
					err := orchestratorNetwork.SendEgress(event)
					require.NoError(t, err)

					orchestratorNetworkSendWG.Done()
				}()
			}

			// all events should be sent to orchestratorNetwork in a timely fashion.
			unittest.RequireReturnsBefore(t, orchestratorNetworkSendWG.Wait, 1*time.Second, "could not send all events to orchestratorNetwork on time")
			// all events should be relayed to the connections by the orchestratorNetwork in a timely fashion.
			unittest.RequireReturnsBefore(t, attackerMsgReceived.Wait, 1*time.Second, "connections could not receive messages on time")
		})
}

// matchEgressEventForMessage fails the test if given message is not meant to be sent on behalf of the corrupted id, or it does not correspond to any
// of the given events.
func matchEventForMessage(t *testing.T, egressEvents []*insecure.EgressEvent, message *insecure.Message, corruptedId flow.Identifier) {
	codec := unittest.NetworkCodec()

	require.Equal(t, corruptedId[:], message.Egress.CorruptOriginID[:])

	for _, egressEvent := range egressEvents {
		if egressEvent.CorruptOriginId == corruptedId {
			require.Equal(t, egressEvent.Channel.String(), message.Egress.ChannelID)
			require.Equal(t, egressEvent.Protocol, message.Egress.Protocol)
			require.Equal(t, flow.IdsToBytes(egressEvent.TargetIds), message.Egress.TargetIDs)
			require.Equal(t, egressEvent.TargetNum, message.Egress.TargetNum)

			content, err := codec.Decode(message.Egress.Payload)
			require.NoError(t, err)

			require.Equal(t, egressEvent.FlowProtocolEvent, content)

			return
		}
	}

	require.Fail(t, fmt.Sprintf("could not find any matching egressEvent for the message: %v", message))
}

// withMockOrchestrator creates a Corrupted Conduit Factory (CCF) for each given corrupted identity.
// It then creates an orchestrator network, establishes a connection to each CCF, and then registers a mock orchestrator
// on top of the orchestrator network.
// Once the orchestrator network, CCFs, and mock orchestrator are all ready, it executes the injected "run" function.
func withMockOrchestrator(t *testing.T,
	corruptedIds flow.IdentityList,
	run func(*Network, *mockinsecure.AttackOrchestrator, []*mockCorruptNetwork)) {

	withMockCorruptibleConduitFactories(t,
		corruptedIds.NodeIDs(),
		func(signalerContext irrecoverable.SignalerContext, ccfs []*mockCorruptNetwork, ccfPorts map[flow.Identifier]string) {

			orchestrator := &mockinsecure.AttackOrchestrator{}
			connector := NewCorruptedConnector(unittest.Logger(), corruptedIds, ccfPorts)

			orchestratorNetwork, err := NewOrchestratorNetwork(
				unittest.Logger(),
				unittest.NetworkCodec(),
				orchestrator,
				connector,
				corruptedIds)
			require.NoError(t, err)

			// mocks registering orchestratorNetwork as the orchestrator network functionality for orchestrator.
			orchestrator.On("Register", orchestratorNetwork).Return().Once()

			// life-cycle management of orchestratorNetwork.
			ctx, cancel := context.WithCancel(context.Background())
			attackCtx, errChan := irrecoverable.WithSignaler(ctx)
			go func() {
				select {
				case err := <-errChan:
					t.Error("orchestratorNetwork startup encountered fatal error", err)
				case <-ctx.Done():
					return
				}
			}()

			// starts orchestratorNetwork
			orchestratorNetwork.Start(attackCtx)
			unittest.RequireCloseBefore(t, orchestratorNetwork.Ready(), 1*time.Second, "could not start orchestratorNetwork on time")

			attackerRegisteredOnAllCCFs := &sync.WaitGroup{}
			attackerRegisteredOnAllCCFs.Add(len(ccfs))
			for _, ccf := range ccfs {
				ccf := ccf

				go func() {
					<-ccf.attackerRegMsg
					attackerRegisteredOnAllCCFs.Done()
				}()
			}

			unittest.RequireReturnsBefore(t, attackerRegisteredOnAllCCFs.Wait, 1*time.Second, "could not register attacker on all ccfs on time")

			run(orchestratorNetwork, orchestrator, ccfs)

			// terminates orchestratorNetwork
			cancel()
			unittest.RequireCloseBefore(t, orchestratorNetwork.Done(), 1*time.Second, "could not stop orchestratorNetwork on time")
		})
}

// mockOrchestratorHandlingEgressEvent mocks the given orchestrator to receive each of the given egress events exactly once. The returned wait group is
// released when individual events are seen by orchestrator exactly once.
func mockOrchestratorHandlingEgressEvent(t *testing.T, orchestrator *mockinsecure.AttackOrchestrator, egressEvents []*insecure.EgressEvent) *sync.WaitGroup {
	orchestratorWG := &sync.WaitGroup{}
	orchestratorWG.Add(len(egressEvents)) // keeps track of total egress events that orchestrator receives

	mu := sync.Mutex{}
	seen := make(map[*insecure.EgressEvent]struct{}) // keeps track of unique egress events received by orchestrator
	orchestrator.On("HandleEgressEvent", mock.Anything).Run(func(args mock.Arguments) {
		mu.Lock()
		defer mu.Unlock()

		e, ok := args[0].(*insecure.EgressEvent)
		require.True(t, ok)

		// event should not be seen before.
		_, ok = seen[e]
		require.False(t, ok)

		// received event by orchestrator must be an expected one.
		require.Contains(t, egressEvents, e)
		seen[e] = struct{}{}
		orchestratorWG.Done()

	}).Return(nil)

	return orchestratorWG
}

// withMockCorruptibleConduitFactories creates and starts mock Corruptible Conduit Factories (CCF)s for each given corrupted identity.
// These mock CCFs only run the gRPC part of an actual CCF. Once all CCFs are up and running, the injected "run" function is executed.
func withMockCorruptibleConduitFactories(
	t *testing.T,
	corruptedIds flow.IdentifierList,
	run func(irrecoverable.SignalerContext, []*mockCorruptNetwork, map[flow.Identifier]string)) {

	count := len(corruptedIds)

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

	ccfs := make([]*mockCorruptNetwork, count)
	ccfPorts := make(map[flow.Identifier]string)
	for i := 0; i < count; i++ {
		// factory
		ccf := newMockCorruptNetwork()
		ccf.Start(ccfCtx)
		unittest.RequireCloseBefore(t, ccf.Ready(), 1*time.Second, "could not start corruptible conduit factory on time")
		ccfs[i] = ccf

		// port mapping
		_, ccfPortStr, err := net.SplitHostPort(ccf.ServerAddress())
		require.NoError(t, err)

		ccfId := corruptedIds[i]
		ccfPorts[ccfId] = ccfPortStr
	}

	run(ccfCtx, ccfs, ccfPorts)

	// terminates orchestratorNetwork
	cancel()

	// stop all ccfs
	for i := 0; i < count; i++ {
		unittest.RequireCloseBefore(t, ccfs[i].Done(), 1*time.Second, "could not stop corruptible conduit factory on time")
	}
}
