package passthrough

import (
	"fmt"
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
	typeExecutionResult   = "type-execution-result"
	typeChunkDataRequest  = "type-chunk-data-request"
	typeChunkDataResponse = "type-chunk-data-response"
	typeResultApproval    = "type-result-approval"
)

// DummyOrchestrator represents a simple orchestrator that passes through all incoming events.
type DummyOrchestrator struct {
	logger        zerolog.Logger
	attackNetwork insecure.AttackNetwork
	eventTracker  map[string]flow.IdentifierList
}

func NewDummyOrchestrator(logger zerolog.Logger) *DummyOrchestrator {
	return &DummyOrchestrator{
		logger: logger.With().Str("component", "dummy-orchestrator").Logger(),
		eventTracker: map[string]flow.IdentifierList{
			typeExecutionReceipt:  {},
			typeExecutionResult:   {},
			typeChunkDataRequest:  {},
			typeChunkDataResponse: {},
			typeResultApproval:    {},
		},
	}
}

// HandleEventFromCorruptedNode implements logic of processing the events received from a corrupted node.
//
// In Corruptible Conduit Framework for BFT testing, corrupted nodes relay their outgoing events to
// the attacker instead of dispatching them to the network.
//
// In this dummy orchestrator, the incoming event is passed through without any changes.
func (d *DummyOrchestrator) HandleEventFromCorruptedNode(event *insecure.Event) error {
	lg := d.logger.With().
		Hex("corrupted_id", logging.ID(event.CorruptedNodeId)).
		Str("channel", event.Channel.String()).
		Str("protocol", event.Protocol.String()).
		Uint32("target_num", event.TargetNum).
		Str("target_ids", fmt.Sprintf("%v", event.TargetIds)).
		Str("flow_protocol_event", fmt.Sprintf("%T", event.FlowProtocolEvent)).Logger()

	switch e := event.FlowProtocolEvent.(type) {
	case *flow.ExecutionReceipt:
		d.eventTracker[typeExecutionReceipt] = append(d.eventTracker[typeExecutionReceipt], e.ID())
	case *messages.ChunkDataRequest:
		d.eventTracker[typeChunkDataRequest] = append(d.eventTracker[typeChunkDataRequest], e.ChunkID)
	case *messages.ChunkDataResponse:
		d.eventTracker[typeChunkDataResponse] = append(d.eventTracker[typeChunkDataResponse], e.ChunkDataPack.ChunkID)
	case *flow.ResultApproval:
		d.eventTracker[typeResultApproval] = append(d.eventTracker[typeResultApproval], e.ID())
	}

	err := d.attackNetwork.Send(event)
	if err != nil {
		lg.Error().Err(err).Msg("could not pass through incoming event")
		return err
	}
	lg.Info().Msg("incoming event passed through successfully")
	return nil

}

func (d *DummyOrchestrator) WithAttackNetwork(attackNetwork insecure.AttackNetwork) {
	d.attackNetwork = attackNetwork
}

func (d *DummyOrchestrator) mustSeenFlowProtocolEvent(t *testing.T, eventType string, ids ...flow.Identifier) {
	events, ok := d.eventTracker[eventType]
	require.Truef(t, ok, "unknown type: %s", eventType)

	for _, id := range ids {
		require.Contains(t, events, id)
	}
}
