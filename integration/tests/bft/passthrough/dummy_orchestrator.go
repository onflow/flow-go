package passthrough

import (
	"fmt"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	typeExecutionReceipt  = "type-execution-receipt"
	typeChunkDataRequest  = "type-chunk-data-request"
	typeChunkDataResponse = "type-chunk-data-response"
	typeResultApproval    = "type-result-approval"
)

// dummyOrchestrator represents a simple orchestrator that passes through all incoming events.
type dummyOrchestrator struct {
	sync.Mutex
	logger              zerolog.Logger
	orchestratorNetwork insecure.OrchestratorNetwork
	egressEventTracker  map[string]flow.IdentifierList
	ingressEventTracker map[string]flow.IdentifierList
}

var _ insecure.AttackOrchestrator = &dummyOrchestrator{}

func NewDummyOrchestrator(logger zerolog.Logger) *dummyOrchestrator {
	return &dummyOrchestrator{
		logger: logger.With().Str("component", "dummy-orchestrator").Logger(),
		egressEventTracker: map[string]flow.IdentifierList{
			typeExecutionReceipt:  {},
			typeChunkDataRequest:  {},
			typeChunkDataResponse: {},
			typeResultApproval:    {},
		},
		ingressEventTracker: map[string]flow.IdentifierList{
			typeChunkDataRequest:  {},
			typeChunkDataResponse: {},
		},
	}
}

// HandleEgressEvent implements logic of processing the outgoing (egress) events received from a corrupted node.
//
// In Corruptible Conduit Framework for BFT testing, corrupted nodes relay their outgoing events to
// the attacker instead of dispatching them to the network.
//
// In this dummy orchestrator, the incoming event is passed through without any changes.
// Passing through means that the orchestrator returns the events as they are to the original corrupted nodes so they
// dispatch them on the Flow network.
func (d *dummyOrchestrator) HandleEgressEvent(event *insecure.EgressEvent) error {
	lg := d.logger.With().
		Hex("corrupt_origin_id", logging.ID(event.CorruptOriginId)).
		Str("channel", event.Channel.String()).
		Str("protocol", event.Protocol.String()).
		Uint32("target_num", event.TargetNum).
		Str("target_ids", fmt.Sprintf("%v", event.TargetIds)).
		Str("flow_protocol_event", fmt.Sprintf("%T", event.FlowProtocolEvent)).Logger()

	switch e := event.FlowProtocolEvent.(type) {
	case *flow.ExecutionReceipt:
		d.egressEventTracker[typeExecutionReceipt] = append(d.egressEventTracker[typeExecutionReceipt], e.ID())
	case *messages.ChunkDataRequest:
		d.egressEventTracker[typeChunkDataRequest] = append(d.egressEventTracker[typeChunkDataRequest], e.ChunkID)
	case *messages.ChunkDataResponse:
		d.egressEventTracker[typeChunkDataResponse] = append(d.egressEventTracker[typeChunkDataResponse], e.ChunkDataPack.ChunkID)
	case *flow.ResultApproval:
		d.egressEventTracker[typeResultApproval] = append(d.egressEventTracker[typeResultApproval], e.ID())
	}

	err := d.orchestratorNetwork.SendEgress(event)
	if err != nil {
		lg.Error().Err(err).Msg("could not pass through egress event")
		return err
	}
	lg.Info().Msg("egress event passed through successfully")
	return nil
}

// HandleIngressEvent implements logic of processing the incoming (ingress) events to a corrupt node.
func (d *dummyOrchestrator) HandleIngressEvent(event *insecure.IngressEvent) error {
	lg := d.logger.With().
		Hex("origin_id", logging.ID(event.OriginID)).
		Str("channel", event.Channel.String()).
		Str("corrupt_target_id", fmt.Sprintf("%v", event.CorruptTargetID)).
		Str("flow_protocol_event", fmt.Sprintf("%T", event.FlowProtocolEvent)).Logger()

	switch e := event.FlowProtocolEvent.(type) {
	case *messages.ChunkDataRequest:
		d.ingressEventTracker[typeChunkDataRequest] = append(d.ingressEventTracker[typeChunkDataRequest], e.ChunkID)
	case *messages.ChunkDataResponse:
		d.ingressEventTracker[typeChunkDataResponse] = append(d.ingressEventTracker[typeChunkDataResponse], e.ChunkDataPack.ChunkID)
	}

	err := d.orchestratorNetwork.SendIngress(event)

	if err != nil {
		// since this is used for testing, if we encounter any RPC send error, crash the orchestrator.
		lg.Error().Err(err).Msg("could not pass through ingress event")
		return err
	}
	lg.Info().Msg("ingress event passed through successfully")
	return nil
}

func (d *dummyOrchestrator) Register(orchestratorNetwork insecure.OrchestratorNetwork) {
	d.orchestratorNetwork = orchestratorNetwork
}

// mustSeenEgressFlowProtocolEvent checks the dummy orchestrator has passed through the egress flow protocol events with given ids.
// It fails if any entity is gone missing from sight of the orchestrator.
func (d *dummyOrchestrator) mustSeenEgressFlowProtocolEvent(t *testing.T, eventType string, ids ...flow.Identifier) {
	events, ok := d.egressEventTracker[eventType]
	require.Truef(t, ok, "unknown type: %s", eventType)

	for _, id := range ids {
		require.Contains(t, events, id)
	}
}

// mustSeenIngressFlowProtocolEvent checks the dummy orchestrator has passed through the ingress flow protocol events with given ids.
// It fails if any entity is gone missing from sight of the orchestrator.
func (d *dummyOrchestrator) mustSeenIngressFlowProtocolEvent(t *testing.T, eventType string, ids ...flow.Identifier) {
	events, ok := d.ingressEventTracker[eventType]
	require.Truef(t, ok, "unknown type: %s", eventType)

	for _, id := range ids {
		require.Contains(t, events, id)
	}
}
