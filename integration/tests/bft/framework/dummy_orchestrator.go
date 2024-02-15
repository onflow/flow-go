package framework

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/integration/tests/bft"
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

// orchestrator represents a simple orchestrator that passes through all incoming events.
type orchestrator struct {
	*bft.BaseOrchestrator
	sync.Mutex
	egressEventTracker  map[string]flow.IdentifierList
	ingressEventTracker map[string]flow.IdentifierList
}

var _ insecure.AttackOrchestrator = &orchestrator{}

func NewDummyOrchestrator(t *testing.T, Logger zerolog.Logger) *orchestrator {
	o := &orchestrator{
		BaseOrchestrator: &bft.BaseOrchestrator{
			T:      t,
			Logger: Logger.With().Str("component", "dummy-orchestrator").Logger(),
		},
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

	o.OnEgressEvent = append(o.OnEgressEvent, o.trackEgressEvents)

	return o
}

// trackEgressEvent tracks egress events by event type, this func is used as a callback in the BaseOrchestrator OnEgressEvent callback list.
func (o *orchestrator) trackEgressEvents(event *insecure.EgressEvent) error {
	switch e := event.FlowProtocolEvent.(type) {
	case *flow.ExecutionReceipt:
		o.egressEventTracker[typeExecutionReceipt] = append(o.egressEventTracker[typeExecutionReceipt], e.ID())
	case *messages.ChunkDataRequest:
		o.egressEventTracker[typeChunkDataRequest] = append(o.egressEventTracker[typeChunkDataRequest], e.ChunkID)
	case *messages.ChunkDataResponse:
		o.egressEventTracker[typeChunkDataResponse] = append(o.egressEventTracker[typeChunkDataResponse], e.ChunkDataPack.ChunkID)
	case *flow.ResultApproval:
		o.egressEventTracker[typeResultApproval] = append(o.egressEventTracker[typeResultApproval], e.ID())
	}
	return nil
}

// HandleIngressEvent implements logic of processing the incoming (ingress) events to a corrupt node.
func (o *orchestrator) HandleIngressEvent(event *insecure.IngressEvent) error {
	lg := o.Logger.With().
		Hex("origin_id", logging.ID(event.OriginID)).
		Str("channel", event.Channel.String()).
		Str("corrupt_target_id", fmt.Sprintf("%v", event.CorruptTargetID)).
		Str("flow_protocol_event", fmt.Sprintf("%T", event.FlowProtocolEvent)).Logger()

	switch e := event.FlowProtocolEvent.(type) {
	case *messages.ChunkDataRequest:
		o.ingressEventTracker[typeChunkDataRequest] = append(o.ingressEventTracker[typeChunkDataRequest], e.ChunkID)
	case *messages.ChunkDataResponse:
		o.ingressEventTracker[typeChunkDataResponse] = append(o.ingressEventTracker[typeChunkDataResponse], e.ChunkDataPack.ChunkID)
	}
	err := o.OrchestratorNetwork.SendIngress(event)

	if err != nil {
		if errors.Is(err, io.EOF) {
			// log a warning and continue for EOF errors
			lg.Err(err).Msg("could not pass through ingress event")
			return nil
		}
		// since this is used for testing, if we encounter any RPC send error, crash the orchestrator.
		lg.Error().Err(err).Msg("could not pass through ingress event")
		return err
	}
	lg.Info().Msg("ingress event passed through successfully")
	return nil
}

// mustSeenEgressFlowProtocolEvent checks the dummy orchestrator has passed through the egress flow protocol events with given ids.
// It fails if any entity is gone missing from sight of the orchestrator.
func (o *orchestrator) mustSeenEgressFlowProtocolEvent(t *testing.T, eventType string, ids ...flow.Identifier) {
	events, ok := o.egressEventTracker[eventType]
	require.Truef(t, ok, "unknown type: %s", eventType)

	for _, id := range ids {
		require.Contains(t, events, id)
	}
}

// mustSeenIngressFlowProtocolEvent checks the dummy orchestrator has passed through the ingress flow protocol events with given ids.
// It fails if any entity is gone missing from sight of the orchestrator.
func (o *orchestrator) mustSeenIngressFlowProtocolEvent(t *testing.T, eventType string, ids ...flow.Identifier) {
	events, ok := o.ingressEventTracker[eventType]
	require.Truef(t, ok, "unknown type: %s", eventType)

	for _, id := range ids {
		require.Contains(t, events, id)
	}
}
