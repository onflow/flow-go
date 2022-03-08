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

func TestAttackerStart(t *testing.T) {
	count := 1
	withScenario(t, count,
		func(t *testing.T,
			orchestrator *mockinsecure.AttackOrchestrator,
			client insecure.Attacker_ObserveClient,
			messages []*insecure.Message,
			events []*insecure.CorruptedNodeEvent) {

			orchestratorWG := sync.WaitGroup{}
			orchestratorWG.Add(count)

			mu := sync.Mutex{}
			seen := make(map[*insecure.CorruptedNodeEvent]struct{}) // keeps track of seen events by orchestrator
			orchestrator.On("HandleEventFromCorruptedNode", mock.Anything).Run(func(args mock.Arguments) {
				mu.Lock()
				defer mu.Unlock()

				e, ok := args[0].(*insecure.CorruptedNodeEvent)
				require.True(t, ok)

				// event should not be seen before.
				_, ok = seen[e]
				require.False(t, ok)

				// received event by orchestrator must be an expected one.
				require.Contains(t, events, e)
				seen[e] = struct{}{}
				orchestratorWG.Done()

			}).Return(nil)

			attackerSendWG := sync.WaitGroup{}
			for _, msg := range messages {
				msg := msg

				go func() {
					err := client.Send(msg)
					require.NoError(t, err)
					attackerSendWG.Done()
				}()
			}

			unittest.RequireReturnsBefore(t, attackerSendWG.Wait, 1*time.Second, "could not send all messages to attacker on time")
			unittest.RequireReturnsBefore(t, orchestratorWG.Wait, 10*time.Second, "could not receive message")
		})
}

func messageFixture(t *testing.T, codec network.Codec) (*insecure.Message, *insecure.CorruptedNodeEvent) {
	// fixture for content of message
	originId := unittest.IdentifierFixture()
	targetIds := unittest.IdentifierListFixture(10)
	targets := uint32(3)
	protocol := insecure.Protocol_MULTICAST
	channel := network.Channel("test-channel")
	event := &message.TestMessage{
		Text: fmt.Sprintf("this is a test message: %d", rand.Int()),
	}

	// encodes event to create payload
	payload, err := codec.Encode(event)
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
	e := &insecure.CorruptedNodeEvent{
		CorruptedId: originId,
		Channel:     channel,
		Event:       event,
		Protocol:    protocol,
		TargetNum:   targets,
		TargetIds:   targetIds,
	}

	return m, e
}

func messageFixtures(t *testing.T, codec network.Codec, count int) ([]*insecure.Message, []*insecure.CorruptedNodeEvent) {
	msgs := make([]*insecure.Message, count)
	events := make([]*insecure.CorruptedNodeEvent, count)

	for i := 0; i < count; i++ {
		m, e := messageFixture(t, codec)

		msgs = append(msgs, m)
		events = append(events, e)
	}

	return msgs, events
}

func withScenario(t *testing.T, eventCount int, scenario func(*testing.T, *mockinsecure.AttackOrchestrator, insecure.Attacker_ObserveClient,
	[]*insecure.Message,
	[]*insecure.CorruptedNodeEvent)) {
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

	gRpcClient, err := grpc.Dial(attackerAddress, grpc.WithInsecure())
	require.NoError(t, err)

	client := insecure.NewAttackerClient(gRpcClient)
	clientStream, err := client.Observe(ctx)
	require.NoError(t, err)

	// creates fixtures and runs the scenario
	msgs, events := messageFixtures(t, codec, eventCount)
	scenario(t, orchestrator, clientStream, msgs, events)

	// terminates resources
	cancel()
	attacker.Stop()
	unittest.RequireCloseBefore(t, attacker.Done(), 1*time.Second, "could not stop attacker on time")
}
