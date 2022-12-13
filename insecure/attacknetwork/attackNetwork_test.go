package attacknetwork

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

func TestAttackNetworkObserve_SingleMessage(t *testing.T) {
	testAttackNetworkObserve(t, 1)
}

func TestAttackNetworkObserve_MultipleConcurrentMessages(t *testing.T) {
	testAttackNetworkObserve(t, 10)
}

// testAttackNetworkObserve evaluates that upon receiving concurrent messages from corruptible conduits, the attack network
// decodes the messages into events and relays them to its registered orchestrator.
func testAttackNetworkObserve(t *testing.T, concurrencyDegree int) {
	// creates event fixtures and their corresponding messages.
	messages, events, corruptedIds := insecure.EgressMessageFixtures(t, unittest.NetworkCodec(), insecure.Protocol_MULTICAST, concurrencyDegree)

	withMockOrchestrator(
		t,
		corruptedIds,
		func(network *AttackNetwork, orchestrator *mockinsecure.AttackOrchestrator, ccfs []*mockCorruptibleConduitFactory) {
			// mocks orchestrator to receive each event exactly once.
			orchestratorWG := mockOrchestratorHandlingEgressEvent(t, orchestrator, events)

			// sends all messages concurrently to the attack network (imitating corruptible conduits sending
			// messages concurrently to attack network).
			attackNetworkSendWG := sync.WaitGroup{}
			attackNetworkSendWG.Add(concurrencyDegree)

			for i, msg := range messages {
				msg := msg
				ccf := ccfs[i]

				go func() {
					err := ccf.attackerObserveStream.Send(msg) // pretends the ith ccf is sending message to attacker for observe
					require.NoError(t, err)
					attackNetworkSendWG.Done()
				}()
			}

			// all messages should be sent to attack network in a timely fashion.
			unittest.RequireReturnsBefore(t, attackNetworkSendWG.Wait, 1*time.Second, "could not send all messages to attack network on time")
			// all events should be relayed to the orchestrator by the attack network in a timely fashion.
			unittest.RequireReturnsBefore(t, orchestratorWG.Wait, 1*time.Second, "orchestrator could not receive messages on time")
		})
}

func TestAttackNetworkUnicast_SingleMessage(t *testing.T) {
	testAttackNetwork(t, insecure.Protocol_UNICAST, 1)
}

func TestAttackNetworkUnicast_ConcurrentMessages(t *testing.T) {
	testAttackNetwork(t, insecure.Protocol_UNICAST, 10)
}

func TestAttackNetworkMulticast_SingleMessage(t *testing.T) {
	testAttackNetwork(t, insecure.Protocol_MULTICAST, 1)
}

func TestAttackNetworkMulticast_ConcurrentMessages(t *testing.T) {
	testAttackNetwork(t, insecure.Protocol_MULTICAST, 10)
}

func TestAttackNetworkPublish_SingleMessage(t *testing.T) {
	testAttackNetwork(t, insecure.Protocol_PUBLISH, 1)
}

func TestAttackNetworkPublish_ConcurrentMessages(t *testing.T) {
	testAttackNetwork(t, insecure.Protocol_PUBLISH, 10)
}

// testAttackNetwork evaluates that the orchestrator can successfully route an event to a corrupted node through the attack network.
// By a corrupted node here, we mean a node that runs with a corruptible conduit factory.
func testAttackNetwork(t *testing.T, protocol insecure.Protocol, concurrencyDegree int) {
	// creates event fixtures and their corresponding messages.
	_, events, corruptedIds := insecure.EgressMessageFixtures(t, unittest.NetworkCodec(), protocol, concurrencyDegree)

	withMockOrchestrator(t,
		corruptedIds,
		func(attackNetwork *AttackNetwork, _ *mockinsecure.AttackOrchestrator, ccfs []*mockCorruptibleConduitFactory) {
			attackerMsgReceived := &sync.WaitGroup{}
			attackerMsgReceived.Add(concurrencyDegree)

			for i, corruptedId := range corruptedIds {
				corruptedId := corruptedId
				ccf := ccfs[i]

				// testing message delivery from attack network to ccfs
				go func() {
					msg := <-ccf.attackerMsg
					matchEgressEventForMessage(t, events, msg, corruptedId.NodeID)
					attackerMsgReceived.Done()
				}()
			}

			attackNetworkSendWG := &sync.WaitGroup{}
			attackNetworkSendWG.Add(concurrencyDegree)

			for _, event := range events {
				event := event
				go func() {
					err := attackNetwork.Send(event)
					require.NoError(t, err)

					attackNetworkSendWG.Done()
				}()
			}

			// all events should be sent to attackNetwork in a timely fashion.
			unittest.RequireReturnsBefore(t, attackNetworkSendWG.Wait, 1*time.Second, "could not send all events to attackNetwork on time")
			// all events should be relayed to the connections by the attackNetwork in a timely fashion.
			unittest.RequireReturnsBefore(t, attackerMsgReceived.Wait, 1*time.Second, "connections could not receive messages on time")
		})
}

// matchEgressEventForMessage fails the test if given message is not meant to be sent on behalf of the corrupted id, or it does not correspond to any
// of the given events.
func matchEgressEventForMessage(t *testing.T, events []*insecure.EgressEvent, message *insecure.Message, corruptedId flow.Identifier) {
	codec := unittest.NetworkCodec()

	require.Equal(t, corruptedId[:], message.Egress.OriginID[:])

	for _, event := range events {
		if event.CorruptedNodeId == corruptedId {
			require.Equal(t, event.Channel.String(), message.Egress.ChannelID)
			require.Equal(t, event.Protocol, message.Egress.Protocol)
			require.Equal(t, flow.IdsToBytes(event.TargetIds), message.Egress.TargetIDs)
			require.Equal(t, event.TargetNum, message.Egress.TargetNum)

			content, err := codec.Decode(message.Egress.Payload)
			require.NoError(t, err)

			require.Equal(t, event.FlowProtocolEvent, content)

			return
		}
	}

	require.Fail(t, fmt.Sprintf("could not find any matching event for the message: %v", message))
}

// withMockOrchestrator creates a Corrupted Conduit Factory (CCF) for each given corrupted identity.
// It then creates an attack network, establishes a connection to each CCF, and then registers a mock orchestrator
// on top of the attack network.
// Once the attack network, CCFs, and mock orchestrator are all ready, it executes the injected "run" function.
func withMockOrchestrator(t *testing.T,
	corruptedIds flow.IdentityList,
	run func(*AttackNetwork, *mockinsecure.AttackOrchestrator, []*mockCorruptibleConduitFactory)) {

	withMockCorruptibleConduitFactories(t,
		corruptedIds.NodeIDs(),
		func(signalerContext irrecoverable.SignalerContext, ccfs []*mockCorruptibleConduitFactory, ccfPorts map[flow.Identifier]string) {

			orchestrator := &mockinsecure.AttackOrchestrator{}
			connector := NewCorruptedConnector(unittest.Logger(), corruptedIds, ccfPorts)

			attackNetwork, err := NewAttackNetwork(
				unittest.Logger(),
				unittest.NetworkCodec(),
				orchestrator,
				connector,
				corruptedIds)
			require.NoError(t, err)

			// mocks registering attackNetwork as the attack network functionality for orchestrator.
			orchestrator.On("WithAttackNetwork", attackNetwork).Return().Once()

			// life-cycle management of attackNetwork.
			ctx, cancel := context.WithCancel(context.Background())
			attackCtx, errChan := irrecoverable.WithSignaler(ctx)
			go func() {
				select {
				case err := <-errChan:
					t.Error("attackNetwork startup encountered fatal error", err)
				case <-ctx.Done():
					return
				}
			}()

			// starts attackNetwork
			attackNetwork.Start(attackCtx)
			unittest.RequireCloseBefore(t, attackNetwork.Ready(), 1*time.Second, "could not start attackNetwork on time")

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

			run(attackNetwork, orchestrator, ccfs)

			// terminates attackNetwork
			cancel()
			unittest.RequireCloseBefore(t, attackNetwork.Done(), 1*time.Second, "could not stop attackNetwork on time")
		})
}

// mockOrchestratorHandlingEgressEvent mocks the given orchestrator to receive each of the given egress events exactly once. The returned wait group is
// released when individual events are seen by orchestrator exactly once.
func mockOrchestratorHandlingEgressEvent(t *testing.T, orchestrator *mockinsecure.AttackOrchestrator, egressEvents []*insecure.EgressEvent) *sync.WaitGroup {
	orchestratorWG := &sync.WaitGroup{}
	orchestratorWG.Add(len(egressEvents)) // keeps track of total egress events that orchestrator receives

	mu := sync.Mutex{}
	seen := make(map[*insecure.EgressEvent]struct{}) // keeps track of unique egress events received by orchestrator
	orchestrator.On("HandleEventFromCorruptedNode", mock.Anything).Run(func(args mock.Arguments) {
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
	run func(irrecoverable.SignalerContext, []*mockCorruptibleConduitFactory, map[flow.Identifier]string)) {

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

	ccfs := make([]*mockCorruptibleConduitFactory, count)
	ccfPorts := make(map[flow.Identifier]string)
	for i := 0; i < count; i++ {
		// factory
		ccf := newMockCorruptibleConduitFactory()
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

	// terminates attackNetwork
	cancel()

	// stop all ccfs
	for i := 0; i < count; i++ {
		unittest.RequireCloseBefore(t, ccfs[i].Done(), 1*time.Second, "could not stop corruptible conduit factory on time")
	}
}
