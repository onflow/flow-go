package adversary

import (
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
	mockinsecure "github.com/onflow/flow-go/insecure/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/utils/unittest"
)

const attackerAddress = "localhost:8000"

func TestAttackerObserve_SingleMessage(t *testing.T) {
	testAttackerObserve(t, 1)
}

func TestAttackerObserve_MultipleConcurrentMessages(t *testing.T) {
	testAttackerObserve(t, 10)
}

// testAttackerObserve evaluates that upon receiving concurrent messages from corruptible conduits, the attacker
// decodes the messages into events and relays them to its registered orchestrator.
func testAttackerObserve(t *testing.T, concurrencyDegree int) {
	withAttacker(
		t,
		concurrencyDegree,
		func(t *testing.T, orchestrator *mockinsecure.AttackOrchestrator, client insecure.Attacker_ObserveClient, messages []*insecure.Message, events []*insecure.Event) {

			orchestratorWG := sync.WaitGroup{}
			orchestratorWG.Add(concurrencyDegree) // keeps track of total events that orchestrator receives

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

			// sends all messages concurrently to the attacker (imitating corruptible conduits sending
			// messages conccurently to attacker).
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
		CorruptedId: originId,
		Channel:     channel,
		Content:     content,
		Protocol:    protocol,
		TargetNum:   targets,
		TargetIds:   targetIds,
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

// withAttacker is a test helper that creates and starts an attacker server, prepares a set of message fixtures and runs the given scenario
// against the attacker and generated messages.
func withAttacker(
	t *testing.T,
	messageCount int, // number of messages received by attacker from corruptible conduits while playing scenario.
	scenario func(*testing.T, *mockinsecure.AttackOrchestrator, insecure.Attacker_ObserveClient, []*insecure.Message, []*insecure.Event)) {

	codec := cbor.NewCodec()
	orchestrator := &mockinsecure.AttackOrchestrator{}
	// mocks start up of orchestrator
	orchestrator.On("Start", mock.AnythingOfType("*irrecoverable.signalerCtx")).Return().Once()

	attacker, err := NewAttacker(unittest.Logger(), attackerAddress, codec, orchestrator)

	require.NoError(t, err)

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

	gRpcClient, err := grpc.Dial(attackerAddress, grpc.WithTransportCredentials(grpcinsecure.NewCredentials()))
	require.NoError(t, err)

	client := insecure.NewAttackerClient(gRpcClient)
	clientStream, err := client.Observe(ctx)
	require.NoError(t, err)

	// creates fixtures and runs the scenario
	msgs, events := messageFixtures(t, codec, messageCount)
	scenario(t, orchestrator, clientStream, msgs, events)

	// terminates resources
	cancel()
	unittest.RequireCloseBefore(t, attacker.Done(), 1*time.Second, "could not stop attacker on time")
}
