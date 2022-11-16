package topicvalidator

import (
	"fmt"
	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/utils/logging"
	"github.com/rs/zerolog"
	"sync"
)

const (
	typeExecutionReceipt  = "type-execution-receipt"
	typeChunkDataRequest  = "type-chunk-data-request"
	typeChunkDataResponse = "type-chunk-data-response"
	typeResultApproval    = "type-result-approval"
)

// testOrchestrator represents a simple testOrchestrator that passes through all incoming events.
type testOrchestrator struct {
	sync.Mutex
	logger              zerolog.Logger
	orchestratorNetwork insecure.OrchestratorNetwork
	eventTracker        map[string]flow.IdentifierList
}

var _ insecure.AttackOrchestrator = &testOrchestrator{}

func NewOrchestrator(logger zerolog.Logger) *testOrchestrator {
	return &testOrchestrator{
		logger: logger.With().Str("component", "dummy-testOrchestrator").Logger(),
		eventTracker: map[string]flow.IdentifierList{
			typeExecutionReceipt:  {},
			typeChunkDataRequest:  {},
			typeChunkDataResponse: {},
			typeResultApproval:    {},
		},
	}
}

// HandleEgressEvent implements logic of processing the outgoing (egress) events received from a corrupted node.
//
// In Corruptible Conduit Framework for BFT testing, corrupted nodes relay their outgoing events to
// the attacker instead of dispatching them to the network.
//
// In this dummy testOrchestrator, the incoming event is passed through without any changes.
// Passing through means that the testOrchestrator returns the events as they are to the original corrupted nodes so they
// dispatch them on the Flow network.
func (o *testOrchestrator) HandleEgressEvent(event *insecure.EgressEvent) error {
	lg := o.logger.With().
		Hex("corrupt_origin_id", logging.ID(event.CorruptOriginId)).
		Str("channel", event.Channel.String()).
		Str("protocol", event.Protocol.String()).
		Uint32("target_num", event.TargetNum).
		Str("target_ids", fmt.Sprintf("%v", event.TargetIds)).
		Str("flow_protocol_event", fmt.Sprintf("%T", event.FlowProtocolEvent)).Logger()

	switch e := event.FlowProtocolEvent.(type) {
	case *flow.ExecutionReceipt:
		o.eventTracker[typeExecutionReceipt] = append(o.eventTracker[typeExecutionReceipt], e.ID())
	case *messages.ChunkDataRequest:
		o.eventTracker[typeChunkDataRequest] = append(o.eventTracker[typeChunkDataRequest], e.ChunkID)
	case *messages.ChunkDataResponse:
		o.eventTracker[typeChunkDataResponse] = append(o.eventTracker[typeChunkDataResponse], e.ChunkDataPack.ChunkID)
	case *flow.ResultApproval:
		o.eventTracker[typeResultApproval] = append(o.eventTracker[typeResultApproval], e.ID())
	}

	err := o.orchestratorNetwork.SendEgress(event)
	if err != nil {
		// since this is used for testing, if we encounter any RPC send error, crash the testOrchestrator.
		lg.Fatal().Err(err).Msg("could not pass through egress event")
		return err
	}
	lg.Info().Msg("egress event passed through successfully")
	return nil
}

// HandleIngressEvent implements logic of processing the incoming (ingress) events to a corrupt node.
func (o *testOrchestrator) HandleIngressEvent(event *insecure.IngressEvent) error {
	lg := o.logger.With().
		Hex("origin_id", logging.ID(event.OriginID)).
		Str("channel", event.Channel.String()).
		Str("corrupt_target_id", fmt.Sprintf("%v", event.CorruptTargetID)).
		Str("flow_protocol_event", fmt.Sprintf("%T", event.FlowProtocolEvent)).Logger()

	err := o.orchestratorNetwork.SendIngress(event)

	if err != nil {
		// since this is used for testing, if we encounter any RPC send error, crash the testOrchestrator.
		lg.Fatal().Err(err).Msg("could not pass through ingress event")
		return err
	}
	lg.Info().Msg("ingress event passed through successfully")
	return nil
}

func (o *testOrchestrator) Register(orchestratorNetwork insecure.OrchestratorNetwork) {
	o.orchestratorNetwork = orchestratorNetwork
}
