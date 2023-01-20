package blocklist

import (
	"fmt"
	"github.com/onflow/flow-go/integration/tests/bft"
	"github.com/onflow/flow-go/utils/logging"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

// AdminBlockListAttackOrchestrator represents a simple `insecure.AttackOrchestrator` that tracks messages received before and after the senderVN is blocked by the receiverEN via the admin blocklist command.
type AdminBlockListAttackOrchestrator struct {
	sync.Mutex
	t                          *testing.T
	logger                     zerolog.Logger
	orchestratorNetwork        insecure.OrchestratorNetwork
	codec                      network.Codec
	unauthorizedEventsReceived *atomic.Int64
	authorizedEventsReceived   *atomic.Int64
	unauthorizedEvents         map[flow.Identifier]*insecure.EgressEvent
	authorizedEvents           map[flow.Identifier]*insecure.EgressEvent
	authorizedEventsReceivedWg sync.WaitGroup
	senderVN                   flow.Identifier
	receiverEN                 flow.Identifier
}

var _ insecure.AttackOrchestrator = &AdminBlockListAttackOrchestrator{}

func NewOrchestrator(t *testing.T, logger zerolog.Logger, senderVN, receiverEN flow.Identifier) *AdminBlockListAttackOrchestrator {
	orchestrator := &AdminBlockListAttackOrchestrator{
		t:                          t,
		logger:                     logger.With().Str("component", "bft-test-orchestrator").Logger(),
		codec:                      unittest.NetworkCodec(),
		unauthorizedEventsReceived: atomic.NewInt64(0),
		authorizedEventsReceived:   atomic.NewInt64(0),
		unauthorizedEvents:         make(map[flow.Identifier]*insecure.EgressEvent),
		authorizedEvents:           make(map[flow.Identifier]*insecure.EgressEvent),
		authorizedEventsReceivedWg: sync.WaitGroup{},
		senderVN:                   senderVN,
		receiverEN:                 receiverEN,
	}

	return orchestrator
}

// HandleEgressEvent implements logic of processing the outgoing (egress) events received from a corrupted node. This attack orchestrator
// simply passes through messages without changes to the orchestrator network.
func (a *AdminBlockListAttackOrchestrator) HandleEgressEvent(event *insecure.EgressEvent) error {
	lg := a.logger.With().
		Hex("corrupt_origin_id", logging.ID(event.CorruptOriginId)).
		Str("channel", event.Channel.String()).
		Str("protocol", event.Protocol.String()).
		Uint32("target_num", event.TargetNum).
		Strs("target_ids", logging.IDs(event.TargetIds)).
		Str("flow_protocol_event", logging.Type(event.FlowProtocolEvent)).Logger()

	err := a.orchestratorNetwork.SendEgress(event)
	if err != nil {
		lg.Error().Err(err).Msg("could not pass through egress event")
		return err
	}

	lg.Info().Str("event_id", event.FlowProtocolEventID.String()).Msg("egress event passed through successfully")
	return nil
}

// HandleIngressEvent implements logic of processing the incoming (ingress) events to a corrupt node.
// This handler will track authorized messages that are expected to be received by the receiverEN before we block the sender.
// It also tracks unauthorized messages received if any that are expected to be blocked after the senderVN is blocked via the admin blocklist command.
func (a *AdminBlockListAttackOrchestrator) HandleIngressEvent(event *insecure.IngressEvent) error {
	lg := a.logger.With().
		Hex("origin_id", logging.ID(event.OriginID)).
		Str("channel", event.Channel.String()).
		Str("corrupt_target_id", fmt.Sprintf("%v", event.CorruptTargetID)).
		Str("flow_protocol_event", fmt.Sprintf("%T", event.FlowProtocolEvent)).Logger()

	// Track any unauthorized events that are received, these events are sent after the admin blocklist command
	// is used to block the sender node.
	if _, ok := a.unauthorizedEvents[event.FlowProtocolEventID]; ok {
		a.unauthorizedEventsReceived.Inc()
		lg.Warn().Str("event_id", event.FlowProtocolEventID.String()).Msg("unauthorized ingress event received")
	}

	// track all authorized events sent before the sender node is blocked.
	if _, ok := a.authorizedEvents[event.FlowProtocolEventID]; ok {
		// ensure event received intact no changes have been made to the underlying message
		//a.assertEventsEqual(expectedEvent, event)
		a.authorizedEventsReceived.Inc()
		a.authorizedEventsReceivedWg.Done()
	}

	err := a.orchestratorNetwork.SendIngress(event)
	if err != nil {
		lg.Error().Err(err).Msg("could not pass through ingress event")
		return err
	}

	lg.Info().Str("event_id", event.FlowProtocolEventID.String()).Msg("ingress event passed through successfully")
	return nil
}

func (a *AdminBlockListAttackOrchestrator) Register(orchestratorNetwork insecure.OrchestratorNetwork) {
	a.orchestratorNetwork = orchestratorNetwork
}

// sendAuthorizedMsgs publishes a number of authorized messages from the senderVN. Authorized messages are messages
// that are sent before the senderVN is blocked.
func (a *AdminBlockListAttackOrchestrator) sendAuthorizedMsgs(t *testing.T) {
	for i := 0; i < 10; i++ {
		event := bft.RequestChunkDataPackFixture(a.t, a.senderVN, a.receiverEN, insecure.Protocol_PUBLISH)
		err := a.orchestratorNetwork.SendEgress(event)
		require.NoError(t, err)
		a.authorizedEvents[event.FlowProtocolEventID] = event
		a.authorizedEventsReceivedWg.Add(1)
	}
}

// sendUnauthorizedMsgs publishes a number of unauthorized messages. Unauthorized messages are messages that are sent
// after the senderVN is blocked via the admin blocklist command. These messages are not expected to be received.
func (a *AdminBlockListAttackOrchestrator) sendUnauthorizedMsgs(t *testing.T) {
	for i := 0; i < 10; i++ {
		event := bft.RequestChunkDataPackFixture(a.t, a.senderVN, a.receiverEN, insecure.Protocol_PUBLISH)
		err := a.orchestratorNetwork.SendEgress(event)
		require.NoError(t, err)
		a.unauthorizedEvents[event.FlowProtocolEventID] = event
	}
}
