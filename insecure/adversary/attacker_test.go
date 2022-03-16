package adversary_test

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/insecure/adversary"
	mockinsecure "github.com/onflow/flow-go/insecure/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestAttackerObserve_SingleMessage(t *testing.T) {
	testAttackerObserve(t, 1)
}

func TestAttackerObserve_MultipleConcurrentMessages(t *testing.T) {
	testAttackerObserve(t, 10)
}

// testAttackerObserve evaluates that upon receiving concurrent messages from corruptible conduits, the attacker
// decodes the messages into events and relays them to its registered orchestrator.
func testAttackerObserve(t *testing.T, concurrencyDegree int) {
	// creates event fixtures and their corresponding messages.
	messages, events := messageFixtures(t, cbor.NewCodec(), concurrencyDegree)

	withAttackerClient(
		t,
		func(t *testing.T, orchestrator *mockinsecure.AttackOrchestrator, client insecure.Attacker_ObserveClient) {
			// mocks orchestrator to receive each event exactly once.
			orchestratorWG := mockOrchestratorHandlingEvent(t, orchestrator, events)

			// sends all messages concurrently to the attacker (imitating corruptible conduits sending
			// messages concurrently to attacker).
			attackerSendWG := sync.WaitGroup{}
			attackerSendWG.Add(concurrencyDegree)

			for _, msg := range messages {
				msg := msg

				go func() {
					err := client.Send(msg)
					require.NoError(t, err)
					attackerSendWG.Done()
				}()
			}

			// all messages should be sent to attacker in a timely fashion.
			unittest.RequireReturnsBefore(t, attackerSendWG.Wait, 1*time.Second, "could not send all messages to attacker on time")
			// all events should be relayed to the orchestrator by the attacker in a timely fashion.
			unittest.RequireReturnsBefore(t, orchestratorWG.Wait, 1*time.Second, "orchestrator could not receive messages on time")
		})
}

// messageFixture creates and returns a randomly generated gRPC message that is sent between a corruptible conduit and the attacker.
// It also generates and returns the corresponding application-layer event of that message, which is sent between the attacker and the
// orchestrator.
func messageFixture(t *testing.T, codec network.Codec) (*insecure.Message, *insecure.Event) {
	// fixture for content of message
	originId := unittest.IdentifierFixture()
	targetIds := unittest.IdentifierListFixture(10)
	targets := uint32(3)
	protocol := insecure.Protocol_MULTICAST
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
		Targets:   targets,
		TargetIDs: flow.IdsToBytes(targetIds),
		Payload:   payload,
		Protocol:  protocol,
	}

	// creates corresponding event of that message that
	// is sent by attacker to orchestrator.
	e := &insecure.Event{
		CorruptedId:       originId,
		Channel:           channel,
		FlowProtocolEvent: content,
		Protocol:          protocol,
		TargetNum:         targets,
		TargetIds:         targetIds,
	}

	return m, e
}

// messageFixtures creates and returns randomly generated gRCP messages and their corresponding protocol-level events.
// The messages are sent between a corruptible conduit and the attacker.
// The events are the corresponding protocol-level representation of messages.
func messageFixtures(t *testing.T, codec network.Codec, count int) ([]*insecure.Message, []*insecure.Event) {
	msgs := make([]*insecure.Message, count)
	events := make([]*insecure.Event, count)

	for i := 0; i < count; i++ {
		m, e := messageFixture(t, codec)

		msgs[i] = m
		events[i] = e
	}

	return msgs, events
}

// withAttackerClient creates an attacker with a mock orchestrator, starts the attacker, creates a streaming gRPC client to it, and
// executes the injected run function on the orchestrator and gRPC client of attacker. Finally, it terminates the gRPC client and the
// attacker.
func withAttackerClient(
	t *testing.T,
	run func(*testing.T, *mockinsecure.AttackOrchestrator, insecure.Attacker_ObserveClient)) {

	withAttacker(t, func(t *testing.T, attacker *adversary.Attacker, orchestrator *mockinsecure.AttackOrchestrator) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		gRpcClient, err := grpc.Dial(attacker.Address().String(), grpc.WithTransportCredentials(grpcinsecure.NewCredentials()))
		require.NoError(t, err)

		client := insecure.NewAttackerClient(gRpcClient)
		clientStream, err := client.Observe(ctx)
		require.NoError(t, err)

		// creates fixtures and runs the scenario
		run(t, orchestrator, clientStream)
	})
}

// withAttacker creates an attacker with a mock orchestrator.
// It then starts the attacker, executes the given run function on the attacker and its orchestrator, and finally terminates the attacker.
func withAttacker(t *testing.T, run func(t *testing.T, attacker *adversary.Attacker, orchestrator *mockinsecure.AttackOrchestrator)) {
	codec := cbor.NewCodec()
	orchestrator := &mockinsecure.AttackOrchestrator{}
	// mocks start up of orchestrator
	orchestrator.On("Start", mock.AnythingOfType("*irrecoverable.signalerCtx")).Return().Once()

	attacker, err := adversary.NewAttacker(unittest.Logger(), "localhost:0", codec, orchestrator)
	require.NoError(t, err)

	// life-cycle management of attacker.
	ctx, cancel := context.WithCancel(context.Background())
	attackCtx, errChan := irrecoverable.WithSignaler(ctx)
	go func() {
		select {
		case err := <-errChan:
			t.Error("attacker startup encountered fatal error", err)
		case <-ctx.Done():
			return
		}
	}()

	// starts attacker
	attacker.Start(attackCtx)
	unittest.RequireCloseBefore(t, attacker.Ready(), 1*time.Second, "could not start attacker on time")

	assert.Eventually(t, func() bool {
		return attacker.Address() != nil
	}, 5*time.Second, 10*time.Millisecond)

	run(t, attacker, orchestrator)

	// terminates attacker
	cancel()
	unittest.RequireCloseBefore(t, attacker.Done(), 1*time.Second, "could not stop attacker on time")
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
