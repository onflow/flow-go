package requirement

import (
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/integration/tests/bft"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/utils/unittest"
)

const (
	// numOfAuthorizedEvents the number of authorized events that will be created when the test orchestrator is initialized.
	// The numOfAuthorizedEvents allows us to wait for a certain number of authorized messages to be received, this should
	// give the network enough time to process the unauthorized messages. This ensures us that the unauthorized messages
	// were indeed dropped and not unprocessed.
	// This threshold must be set to a low value to make the test conclude faster
	// by waiting for fewer events, which is beneficial when running the test
	// on an asynchronous network where event delivery can be unpredictable.
	numOfAuthorizedEvents = 5

	// numOfUnauthorizedEvents the number of unauthorized events to send by the test orchestrator.
	// This threshold must be set to a low value to make the test conclude faster
	// by waiting for fewer events, which is beneficial when running the test
	// on an asynchronous network where event delivery can be unpredictable.
	numOfUnauthorizedEvents = 5
)

// Orchestrator represents a simple `insecure.AttackOrchestrator` that tracks any unsigned messages received by victim nodes as well as the typically expected messages.
type Orchestrator struct {
	*bft.BaseOrchestrator
	codec                      network.Codec
	unauthorizedEventsReceived *atomic.Int64
	authorizedEventsReceived   *atomic.Int64
	unauthorizedEvents         map[flow.Identifier]*insecure.EgressEvent
	authorizedEvents           map[flow.Identifier]*insecure.EgressEvent
	authorizedEventReceivedWg  sync.WaitGroup
	attackerVNNoMsgSigning     flow.Identifier
	attackerVNWithMsgSigning   flow.Identifier
	victimENID                 flow.Identifier
}

var _ insecure.AttackOrchestrator = &Orchestrator{}

func NewOrchestrator(t *testing.T, logger zerolog.Logger, attackerVNNoMsgSigning, attackerVNWithMsgSigning, victimEN flow.Identifier) *Orchestrator {
	orchestrator := &Orchestrator{
		BaseOrchestrator: &bft.BaseOrchestrator{
			T:      t,
			Logger: logger.With().Str("component", "bft-test-orchestrator").Logger(),
		},
		codec:                      unittest.NetworkCodec(),
		unauthorizedEventsReceived: atomic.NewInt64(0),
		authorizedEventsReceived:   atomic.NewInt64(0),
		unauthorizedEvents:         make(map[flow.Identifier]*insecure.EgressEvent),
		authorizedEvents:           make(map[flow.Identifier]*insecure.EgressEvent),
		authorizedEventReceivedWg:  sync.WaitGroup{},
		attackerVNNoMsgSigning:     attackerVNNoMsgSigning,
		attackerVNWithMsgSigning:   attackerVNWithMsgSigning,
		victimENID:                 victimEN,
	}

	orchestrator.OnIngressEvent = append(orchestrator.OnIngressEvent, orchestrator.trackIngressEvents)

	return orchestrator
}

// trackIngressEvents callback that will track any unauthorized messages that are expected to be blocked by libp2p for message
// signature validation failure. It also tracks all the authorized messages that are expected to be delivered to the node.
func (s *Orchestrator) trackIngressEvents(event *insecure.IngressEvent) error {
	// Track any unauthorized events that are received by the corrupted node that has disabled message signing.
	// These messages should have been rejected by libp2p.
	if _, ok := s.unauthorizedEvents[event.FlowProtocolEventID]; ok {
		s.unauthorizedEventsReceived.Inc()
		s.Logger.Warn().Str("event_id", event.FlowProtocolEventID.String()).Msg("unauthorized ingress event received")
	}
	// track all authorized events sent during test
	if expectedEvent, ok := s.authorizedEvents[event.FlowProtocolEventID]; ok {
		// ensure event received intact no changes have been made to the underlying message
		s.assertEventsEqual(expectedEvent, event)
		s.authorizedEventsReceived.Inc()
		s.authorizedEventReceivedWg.Done()
	}
	return nil
}

// sendUnauthorizedMsgs publishes a number of unauthorized messages without signatures from one corrupt VN to another (victim) corrupt EN.
// The sender is corrupt since the attacker needs to take control over what it sends. Moreover, the receiver is also corrupt as the testing
// framework needs to have an eye on what it receives (i.e., ingress traffic).
func (s *Orchestrator) sendUnauthorizedMsgs(t *testing.T) {
	for i := 0; i < numOfUnauthorizedEvents; i++ {
		event := bft.RequestChunkDataPackEgressFixture(s.T, s.attackerVNNoMsgSigning, s.victimENID, insecure.Protocol_PUBLISH)
		err := s.OrchestratorNetwork.SendEgress(event)
		require.NoError(t, err)
		s.unauthorizedEvents[event.FlowProtocolEventID] = event
	}
}

// sendAuthorizedMsgs publishes a number of authorized messages from one corrupt VN with message signing enabled to another (victim) corrupt EN.
// This func allows us to ensure that unauthorized messages have been processed.
func (s *Orchestrator) sendAuthorizedMsgs(t *testing.T) {
	for i := 0; i < numOfAuthorizedEvents; i++ {
		event := bft.RequestChunkDataPackEgressFixture(s.T, s.attackerVNWithMsgSigning, s.victimENID, insecure.Protocol_PUBLISH)
		err := s.OrchestratorNetwork.SendEgress(event)
		require.NoError(t, err)
		s.authorizedEvents[event.FlowProtocolEventID] = event
		s.authorizedEventReceivedWg.Add(1)
	}
}

// assertEventsEqual checks that an all the fields in an egress event are equal to the given ingress event, this asserts
// that the event was not tampered with as it passes through from attacker -> victim node.
func (s *Orchestrator) assertEventsEqual(egressEvent *insecure.EgressEvent, ingressEvent *insecure.IngressEvent) {
	// ensure event received intact no changes have been made to the underlying message
	require.Equal(s.T, egressEvent.FlowProtocolEventID, ingressEvent.FlowProtocolEventID)
	require.Equal(s.T, egressEvent.Channel, ingressEvent.Channel)
	require.Equal(s.T, egressEvent.TargetIds[0], ingressEvent.CorruptTargetID)
	require.Equal(s.T, egressEvent.FlowProtocolEvent.(*messages.ChunkDataRequest), ingressEvent.FlowProtocolEvent.(*messages.ChunkDataRequest))
}
