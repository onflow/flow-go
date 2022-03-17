package attacknetwork_test

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/insecure/attacknetwork"
	mockinsecure "github.com/onflow/flow-go/insecure/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
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
	messages, events, identities := messageFixtures(t, cbor.NewCodec(), insecure.Protocol_MULTICAST, concurrencyDegree)

	withAttackNetworkClient(
		t,
		identities,
		func(t *testing.T, orchestrator *mockinsecure.AttackOrchestrator, client insecure.Attacker_ObserveClient) {
			// mocks orchestrator to receive each event exactly once.
			orchestratorWG := mockOrchestratorHandlingEvent(t, orchestrator, events)

			// sends all messages concurrently to the attack network (imitating corruptible conduits sending
			// messages concurrently to attack network).
			attackNetworkSendWG := sync.WaitGroup{}
			attackNetworkSendWG.Add(concurrencyDegree)

			for _, msg := range messages {
				msg := msg

				go func() {
					err := client.Send(msg)
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

// messageFixture creates and returns a randomly generated gRPC message that is sent between a corruptible conduit and the attack network.
// It also generates and returns the corresponding application-layer event of that message, which is sent between the attack network and the
// orchestrator.
func messageFixture(t *testing.T, codec network.Codec, protocol insecure.Protocol) (*insecure.Message, *insecure.Event, *flow.Identity) {
	// fixture for content of message
	originId := unittest.IdentifierFixture()

	var targetIds flow.IdentifierList
	targetNum := uint32(0)

	if protocol == insecure.Protocol_UNICAST {
		targetIds = unittest.IdentifierListFixture(1)
	} else {
		targetIds = unittest.IdentifierListFixture(10)
	}

	if protocol == insecure.Protocol_MULTICAST {
		targetNum = uint32(3)
	}

	channel := network.Channel("test-channel")
	content := &message.TestMessage{
		Text: fmt.Sprintf("this is a test message: %d", rand.Int()),
	}

	// encodes event to create payload
	payload, err := codec.Encode(content)
	require.NoError(t, err)

	// creates message that goes over gRPC.
	m := &insecure.Message{
		ChannelID: "test-channel",
		OriginID:  originId[:],
		TargetNum: targetNum,
		TargetIDs: flow.IdsToBytes(targetIds),
		Payload:   payload,
		Protocol:  protocol,
	}

	// creates corresponding event of that message that
	// is sent by attack network to orchestrator.
	e := &insecure.Event{
		CorruptedId:       originId,
		Channel:           channel,
		FlowProtocolEvent: content,
		Protocol:          protocol,
		TargetNum:         targetNum,
		TargetIds:         targetIds,
	}

	return m, e, unittest.IdentityFixture(unittest.WithNodeID(originId))
}

// messageFixtures creates and returns randomly generated gRCP messages and their corresponding protocol-level events.
// The messages are sent between a corruptible conduit and the attack network.
// The events are the corresponding protocol-level representation of messages.
func messageFixtures(t *testing.T, codec network.Codec, protocol insecure.Protocol, count int) ([]*insecure.Message, []*insecure.Event,
	flow.IdentityList) {
	msgs := make([]*insecure.Message, count)
	events := make([]*insecure.Event, count)
	identities := flow.IdentityList{}

	for i := 0; i < count; i++ {
		m, e, id := messageFixture(t, codec, protocol)

		msgs[i] = m
		events[i] = e
		// created identity must be unique
		require.NotContains(t, identities, id)
		identities = append(identities, id)
	}

	return msgs, events, identities
}

// withAttackNetworkClient creates an attack network with a mock orchestrator, starts the attack network, creates a streaming gRPC client to it, and
// executes the injected run function on the orchestrator and gRPC client of attack network. Finally, it terminates the gRPC client and the
// attack network.
func withAttackNetworkClient(
	t *testing.T,
	corruptedIds flow.IdentityList,
	run func(*testing.T, *mockinsecure.AttackOrchestrator, insecure.Attacker_ObserveClient)) {

	withAttackNetwork(t, corruptedIds,
		func(
			t *testing.T,
			attackNetwork *attacknetwork.AttackNetwork,
			_ map[flow.Identifier]*mockinsecure.CorruptedNodeConnection,
			orchestrator *mockinsecure.AttackOrchestrator) {

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			gRpcClient, err := grpc.Dial(attackNetwork.ServerAddress().String(), grpc.WithTransportCredentials(grpcinsecure.NewCredentials()))
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

func testAttackNetwork(t *testing.T, protocol insecure.Protocol, concurrencyDegree int) {
	// creates event fixtures and their corresponding messages.
	_, events, corruptedIds := messageFixtures(t, cbor.NewCodec(), protocol, concurrencyDegree)

	withAttackNetwork(t,
		corruptedIds,
		func(t *testing.T,
			attackNetwork *attacknetwork.AttackNetwork,
			connections map[flow.Identifier]*mockinsecure.CorruptedNodeConnection,
			_ *mockinsecure.AttackOrchestrator) {

			connectionRcvWG := &sync.WaitGroup{}
			connectionRcvWG.Add(concurrencyDegree)

			for _, corruptedId := range corruptedIds {
				corruptedId := corruptedId
				connection, ok := connections[corruptedId.NodeID]
				require.True(t, ok)

				connection.On("SendMessage", mock.Anything).Run(func(args mock.Arguments) {
					msg, ok := args[0].(*insecure.Message)
					require.True(t, ok)

					matchEventForMessage(t, events, msg)

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

func matchEventForMessage(t *testing.T, events []*insecure.Event, message *insecure.Message) {
	codec := cbor.NewCodec()

	for _, event := range events {
		if bytes.Equal(event.CorruptedId[:], message.OriginID) {
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

// withAttackNetwork creates an attack network with a mock orchestrator.
// It then starts the attack network, executes the given run function on the attack network and its orchestrator,
// and finally terminates the attack network.
func withAttackNetwork(t *testing.T,
	corruptedIds flow.IdentityList,
	run func(*testing.T, *attacknetwork.AttackNetwork, map[flow.Identifier]*mockinsecure.CorruptedNodeConnection, *mockinsecure.AttackOrchestrator)) {
	codec := cbor.NewCodec()
	orchestrator := &mockinsecure.AttackOrchestrator{}
	connector := &mockinsecure.CorruptedNodeConnector{}

	attackNetwork, err := attacknetwork.NewAttackNetwork(unittest.Logger(), serverAddress, codec, orchestrator, connector, corruptedIds)
	require.NoError(t, err)

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

	run(t, attackNetwork, connections, orchestrator)

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
