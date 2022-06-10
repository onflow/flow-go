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
	"google.golang.org/grpc"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow-go/insecure"
	mockinsecure "github.com/onflow/flow-go/insecure/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/utils/unittest"
)

const serverAddress = "localhost:0"

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
	messages, events, identities := insecure.MessageFixtures(t, cbor.NewCodec(), insecure.Protocol_MULTICAST, concurrencyDegree)

	withAttackNetworkClient(
		t,
		identities,
		func(t *testing.T, orchestrator *mockinsecure.AttackOrchestrator, ccfs []*mockCorruptibleConduitFactory) {
			// mocks orchestrator to receive each event exactly once.
			orchestratorWG := mockOrchestratorHandlingEvent(t, orchestrator, events)

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

// withAttackNetworkClient creates an attack network with a mock orchestrator, starts the attack network, creates a streaming gRPC client to it, and
// executes the injected run function on the orchestrator and gRPC client of attack network. Finally, it terminates the gRPC client and the
// attack network.
func withAttackNetworkClient(
	t *testing.T,
	corruptedIds flow.IdentityList,
	run func(*testing.T, *mockinsecure.AttackOrchestrator, insecure.Attacker_ObserveClient)) {

	withCorruptedConnections(t, corruptedIds,
		func(attackNetwork *AttackNetwork,
			_ map[flow.Identifier]*mockinsecure.CorruptedNodeConnection,
			orchestrator *mockinsecure.AttackOrchestrator) {

			withMockCorruptibleConduitFactories(
				t,
				corruptedIds.NodeIDs(),
				func(signalerContext irrecoverable.SignalerContext,
					ccfs []*mockCorruptibleConduitFactory,
					ccfPorts map[flow.Identifier]string) {

					connector := NewCorruptedConnector(unittest.Logger(),
						func(receivedMsg *insecure.Message) {
							attackNetwork.Observe(receivedMsg)
						},
						corruptedIds,
						ccfPorts)

				})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			gRpcClient, err := grpc.Dial(attackNetwork.ServerAddress(), grpc.WithTransportCredentials(grpcinsecure.NewCredentials()))
			require.NoError(t, err)

			client := insecure.NewAttackerClient(gRpcClient)
			clientStream, err := client.Observe(ctx)
			require.NoError(t, err)

			// creates fixtures and runs the scenario
			run(t, orchestrator, clientStream)
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
	_, events, corruptedIds := insecure.MessageFixtures(t, cbor.NewCodec(), protocol, concurrencyDegree)

	withCorruptedConnections(t,
		corruptedIds,
		func(attackNetwork *AttackNetwork,
			connections map[flow.Identifier]*mockinsecure.CorruptedNodeConnection,
			_ *mockinsecure.AttackOrchestrator) {

			connectionRcvWG := &sync.WaitGroup{}
			connectionRcvWG.Add(concurrencyDegree)

			for _, corruptedId := range corruptedIds {
				corruptedId := corruptedId
				connection, ok := connections[corruptedId.NodeID]
				require.True(t, ok)

				// we test the communication between the attack network and orchestrator while mocking the connections
				// between the attack network and corrupted nodes.
				connection.On("SendMessage", mock.Anything).Run(func(args mock.Arguments) {
					msg, ok := args[0].(*insecure.Message)
					require.True(t, ok)

					matchEventForMessage(t, events, msg, corruptedId.NodeID)

					connectionRcvWG.Done()
				}).Return(nil)
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
			unittest.RequireReturnsBefore(t, connectionRcvWG.Wait, 1*time.Second, "connections could not receive messages on time")
		})
}

// mackEventForMessage fails the test if given message is not meant to be sent on behalf of the corrupted id, or it does not correspond to any
// of the given events.
func matchEventForMessage(t *testing.T, events []*insecure.Event, message *insecure.Message, corruptedId flow.Identifier) {
	codec := cbor.NewCodec()

	require.Equal(t, corruptedId[:], message.OriginID[:])

	for _, event := range events {
		if event.CorruptedNodeId == corruptedId {
			require.Equal(t, event.Channel.String(), message.ChannelID)
			require.Equal(t, event.Protocol, message.Protocol)
			require.Equal(t, flow.IdsToBytes(event.TargetIds), message.TargetIDs)
			require.Equal(t, event.TargetNum, message.TargetNum)

			content, err := codec.Decode(message.Payload)
			require.NoError(t, err)

			require.Equal(t, event.FlowProtocolEvent, content)

			return
		}
	}

	require.Fail(t, fmt.Sprintf("could not find any matching event for the message: %v", message))
}

// withCorruptedConnections creates an attack network with a mock orchestrator.
// It then starts the attack network, executes the given run function on the attack network and its orchestrator,
// and finally terminates the attack network.
func withCorruptedConnections(t *testing.T,
	corruptedIds flow.IdentityList,
	run func(*AttackNetwork, map[flow.Identifier]*mockinsecure.CorruptedNodeConnection, *mockinsecure.AttackOrchestrator)) {
	codec := cbor.NewCodec()
	orchestrator := &mockinsecure.AttackOrchestrator{}
	connector := &mockinsecure.CorruptedNodeConnector{}

	attackNetwork, err := NewAttackNetwork(
		unittest.Logger(),
		codec,
		orchestrator,
		connector,
		corruptedIds)
	require.NoError(t, err)
	connector.On("WithAttackerAddress", mock.Anything).Return().Once()

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

	// mocks registering attackNetwork as the attack network functionality for orchestrator.
	orchestrator.On("WithAttackNetwork", attackNetwork).Return().Once()
	connections := mockConnectorForConnect(t, connector, corruptedIds)

	// starts attackNetwork
	attackNetwork.Start(attackCtx)
	unittest.RequireCloseBefore(t, attackNetwork.Ready(), 1*time.Second, "could not start attackNetwork on time")

	run(attackNetwork, connections, orchestrator)

	// terminates attackNetwork
	cancel()
	unittest.RequireCloseBefore(t, attackNetwork.Done(), 1*time.Second, "could not stop attackNetwork on time")
}

// mockOrchestratorHandlingEvent mocks the given orchestrator to receive each of the given events exactly once. The returned wait group is
// released when individual events are seen by orchestrator exactly once.
func mockOrchestratorHandlingEvent(t *testing.T, orchestrator *mockinsecure.AttackOrchestrator, events []*insecure.Event) *sync.WaitGroup {
	orchestratorWG := &sync.WaitGroup{}
	orchestratorWG.Add(len(events)) // keeps track of total events that orchestrator receives

	mu := sync.Mutex{}
	seen := make(map[*insecure.Event]struct{}) // keeps track of unique events received by orchestrator
	orchestrator.On("HandleEventFromCorruptedNode", mock.Anything).Run(func(args mock.Arguments) {
		mu.Lock()
		defer mu.Unlock()

		e, ok := args[0].(*insecure.Event)
		require.True(t, ok)

		// event should not be seen before.
		_, ok = seen[e]
		require.False(t, ok)

		// received event by orchestrator must be an expected one.
		require.Contains(t, events, e)
		seen[e] = struct{}{}
		orchestratorWG.Done()

	}).Return(nil)

	return orchestratorWG
}

func mockConnectorForConnect(t *testing.T, connector *mockinsecure.CorruptedNodeConnector, corruptedIds flow.IdentityList) map[flow.Identifier]*mockinsecure.CorruptedNodeConnection {
	connections := make(map[flow.Identifier]*mockinsecure.CorruptedNodeConnection)
	connector.On("Connect", mock.Anything, mock.Anything).
		Return(
			func(ctx context.Context, id flow.Identifier) insecure.CorruptedNodeConnection {
				_, ok := corruptedIds.ByNodeID(id)
				require.True(t, ok)
				connection := &mockinsecure.CorruptedNodeConnection{}
				// mocks closing connections at the termination time of the attack.
				connection.On("CloseConnection").Return(nil)
				connections[id] = connection
				return connection
			},
			func(ctx context.Context, id flow.Identifier) error {
				_, ok := corruptedIds.ByNodeID(id)
				require.True(t, ok)
				return nil
			})

	return connections
}

// withMockCorruptibleConduitFactories creates and starts mock Corruptible Conduit Factories (CCF)s.
// These mock CCFs only run the gRPC part of an actual CCF. Once all CCFs are up and running, the injected "run" function is executed.
func withMockCorruptibleConduitFactories(
	t *testing.T,
	corruptedIds flow.IdentifierList,
	run func(irrecoverable.SignalerContext,
		[]*mockCorruptibleConduitFactory,
		map[flow.Identifier]string)) {

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
