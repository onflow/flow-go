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
	TypeExecutionReceipt  = "type-execution-receipt"
	TypeChunkDataRequest  = "type-chunk-data-request"
	TypeChunkDataResponse = "type-chunk-data-response"
	TypeResultApproval    = "type-result-approval"
)

// dummyOrchestrator represents a simple orchestrator that passes through all incoming events.
type dummyOrchestrator struct {
	sync.Mutex
	logger              zerolog.Logger
	orchestratorNetwork insecure.AttackerNetwork
	eventTracker        map[string]flow.IdentifierList
}

var _ insecure.AttackerOrchestrator = &dummyOrchestrator{}

func NewDummyOrchestrator(logger zerolog.Logger) *dummyOrchestrator {
	return &dummyOrchestrator{
		logger: logger.With().Str("component", "dummy-orchestrator").Logger(),
		eventTracker: map[string]flow.IdentifierList{
			TypeExecutionReceipt:  {},
			TypeChunkDataRequest:  {},
			TypeChunkDataResponse: {},
			TypeResultApproval:    {},
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
		d.eventTracker[TypeExecutionReceipt] = append(d.eventTracker[TypeExecutionReceipt], e.ID())
	case *messages.ChunkDataRequest:
		d.eventTracker[TypeChunkDataRequest] = append(d.eventTracker[TypeChunkDataRequest], e.ChunkID)
	case *messages.ChunkDataResponse:
		d.eventTracker[TypeChunkDataResponse] = append(d.eventTracker[TypeChunkDataResponse], e.ChunkDataPack.ChunkID)
	case *flow.ResultApproval:
		d.eventTracker[TypeResultApproval] = append(d.eventTracker[TypeResultApproval], e.ID())
	}

	err := d.orchestratorNetwork.SendEgress(event)
	if err != nil {
		// since this is used for testing, if we encounter any RPC send error, crash the orchestrator.
		lg.Fatal().Err(err).Msg("could not pass through egress event")
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

	err := d.orchestratorNetwork.SendIngress(event)

	if err != nil {
		// since this is used for testing, if we encounter any RPC send error, crash the orchestrator.
		lg.Fatal().Err(err).Msg("could not pass through ingress event")
		return err
	}
	lg.Info().Msg("ingress event passed through successfully")
	return nil
}

func (d *dummyOrchestrator) Register(orchestratorNetwork insecure.AttackerNetwork) {
	d.orchestratorNetwork = orchestratorNetwork
}

// MustSeenFlowProtocolEvent checks the dummy orchestrator has passed through the flow protocol events with given ids. It fails
// if any entity is gone missing from sight of the orchestrator.
func (d *dummyOrchestrator) MustSeenFlowProtocolEvent(t *testing.T, eventType string, ids ...flow.Identifier) {
	events, ok := d.eventTracker[eventType]
	require.Truef(t, ok, "unknown type: %s", eventType)

	for _, id := range ids {
		require.Contains(t, events, id)
	}
}
