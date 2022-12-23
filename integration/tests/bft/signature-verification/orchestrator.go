package signature_verification

import (
	"fmt"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"golang.org/x/exp/rand"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/utils/logging"
	"github.com/onflow/flow-go/utils/unittest"
)

const (
	// numOfAuthorizedEvents the number of authorized events that will be created when the test orchestrator is initialized.
	// The numOfAuthorizedEvents allows us to wait for a certain number of authorized messages to be received, this should
	// give the network enough time to process the unauthorized messages. This ensures us that the unauthorized messages
	// were indeed dropped and not unprocessed.
	numOfAuthorizedEvents = 100

	// numOfUnauthorizedEvents the number of unauthorized events to send by the test orchestrator.
	numOfUnauthorizedEvents = 10
)

// SignatureValidationAttackOrchestrator represents an simple insecure.AttackOrchestrator that tracks any unsigned messages received by victim nodes as well as normal expected messages.
type SignatureValidationAttackOrchestrator struct {
	sync.Mutex
	t                          *testing.T
	logger                     zerolog.Logger
	orchestratorNetwork        insecure.OrchestratorNetwork
	codec                      network.Codec
	unauthorizedEventsReceived *atomic.Int64
	authorizedEventsReceived   *atomic.Int64
	unauthorizedEvents         map[flow.Identifier]*insecure.EgressEvent
	authorizedEvents           map[flow.Identifier]*insecure.EgressEvent
	authorizedEventReceivedWg  sync.WaitGroup
	attackerVNNoMsgSigning     flow.Identifier
	attackerVNWithMsgSigning   flow.Identifier
	victimEN                   flow.Identifier
}

var _ insecure.AttackOrchestrator = &SignatureValidationAttackOrchestrator{}

func NewOrchestrator(t *testing.T, logger zerolog.Logger, attackerVNNoMsgSigning, attackerVNWithMsgSigning, victimEN flow.Identifier) *SignatureValidationAttackOrchestrator {
	orchestrator := &SignatureValidationAttackOrchestrator{
		t:                          t,
		logger:                     logger.With().Str("component", "bft-test-orchestrator").Logger(),
		codec:                      unittest.NetworkCodec(),
		unauthorizedEventsReceived: atomic.NewInt64(0),
		authorizedEventsReceived:   atomic.NewInt64(0),
		unauthorizedEvents:         make(map[flow.Identifier]*insecure.EgressEvent),
		authorizedEvents:           make(map[flow.Identifier]*insecure.EgressEvent),
		authorizedEventReceivedWg:  sync.WaitGroup{},
		attackerVNNoMsgSigning:     attackerVNNoMsgSigning,
		attackerVNWithMsgSigning:   attackerVNWithMsgSigning,
		victimEN:                   victimEN,
	}

	return orchestrator
}

// HandleEgressEvent implements logic of processing the outgoing (egress) events received from a corrupted node.
func (s *SignatureValidationAttackOrchestrator) HandleEgressEvent(event *insecure.EgressEvent) error {
	lg := s.logger.With().
		Hex("corrupt_origin_id", logging.ID(event.CorruptOriginId)).
		Str("channel", event.Channel.String()).
		Str("protocol", event.Protocol.String()).
		Uint32("target_num", event.TargetNum).
		Strs("target_ids", logging.IDs(event.TargetIds)).
		Str("flow_protocol_event", logging.Type(event.FlowProtocolEvent)).Logger()

	err := s.orchestratorNetwork.SendEgress(event)
	if err != nil {
		lg.Error().Err(err).Msg("could not pass through egress event")
		return err
	}

	lg.Info().Str("event_id", event.FlowProtocolEventID.String()).Msg("egress event passed through successfully")
	return nil
}

// HandleIngressEvent implements logic of processing the incoming (ingress) events to a corrupt node.
// This handler will track any unauthorized messages that are expected to be blocked by libp2p for message
// signature validation failure. It also tracks all the authorized messages that are expected to be delivered to the node.
func (s *SignatureValidationAttackOrchestrator) HandleIngressEvent(event *insecure.IngressEvent) error {
	lg := s.logger.With().
		Hex("origin_id", logging.ID(event.OriginID)).
		Str("channel", event.Channel.String()).
		Str("corrupt_target_id", fmt.Sprintf("%v", event.CorruptTargetID)).
		Str("flow_protocol_event", fmt.Sprintf("%T", event.FlowProtocolEvent)).Logger()

	// Track any unauthorized events that are received by the corrupted node that has disabled message signing.
	// These messages should have been rejected by libp2p.
	if _, ok := s.unauthorizedEvents[event.FlowProtocolEventID]; ok {
		s.unauthorizedEventsReceived.Inc()
		lg.Warn().Str("event_id", event.FlowProtocolEventID.String()).Msg("unauthorized ingress event received")
	}

	// track all authorized events sent during test
	if _, ok := s.authorizedEvents[event.FlowProtocolEventID]; ok {
		s.authorizedEventsReceived.Inc()
		s.authorizedEventReceivedWg.Done()
	}

	err := s.orchestratorNetwork.SendIngress(event)
	if err != nil {
		lg.Error().Err(err).Msg("could not pass through ingress event")
		return err
	}

	lg.Info().Str("event_id", event.FlowProtocolEventID.String()).Msg("ingress event passed through successfully")
	return nil
}

func (s *SignatureValidationAttackOrchestrator) Register(orchestratorNetwork insecure.OrchestratorNetwork) {
	s.orchestratorNetwork = orchestratorNetwork
}

// sendUnauthorizedMsgs publishes a number of unauthorized messages without signatures from the corrupt AN with message signing enabled to the victim AN.
func (s *SignatureValidationAttackOrchestrator) sendUnauthorizedMsgs(t *testing.T) {
	for i := 0; i < numOfUnauthorizedEvents; i++ {
		event := s.getReqChunksEvent(s.attackerVNNoMsgSigning, s.victimEN)
		err := s.orchestratorNetwork.SendEgress(event)
		require.NoError(t, err)
		s.unauthorizedEvents[event.FlowProtocolEventID] = event
	}
}

// sendAuthorizedMsgs publishes a number of authorized messages from the corrupt AN with message signing enabled to the victim AN.
func (s *SignatureValidationAttackOrchestrator) sendAuthorizedMsgs(t *testing.T) {
	for i := 0; i < numOfAuthorizedEvents; i++ {
		event := s.getReqChunksEvent(s.attackerVNWithMsgSigning, s.victimEN)
		err := s.orchestratorNetwork.SendEgress(event)
		require.NoError(t, err)
		s.authorizedEvents[event.FlowProtocolEventID] = event
		s.authorizedEventReceivedWg.Add(1)
	}
}

// getReqCollEvent returns sync req event with the provided node ID as the originID.
func (s *SignatureValidationAttackOrchestrator) getReqChunksEvent(originID, targetID flow.Identifier) *insecure.EgressEvent {
	channel := channels.RequestChunks
	chunkDataReq := &messages.ChunkDataRequest{
		ChunkID: unittest.IdentifierFixture(),
		Nonce:   rand.Uint64(),
	}
	eventID := unittest.GetFlowProtocolEventID(s.t, channel, chunkDataReq, p2p.EventId)
	return &insecure.EgressEvent{
		CorruptOriginId:     originID,
		Channel:             channel,
		Protocol:            insecure.Protocol_PUBLISH,
		TargetNum:           0,
		TargetIds:           flow.IdentifierList{targetID},
		FlowProtocolEvent:   chunkDataReq,
		FlowProtocolEventID: eventID,
	}
}
