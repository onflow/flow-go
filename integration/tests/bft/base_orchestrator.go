package bft

import (
	"fmt"
	"testing"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/utils/logging"
)

type OnEgressEvent func(event *insecure.EgressEvent) error
type OnIngressEvent func(event *insecure.IngressEvent) error

// BaseOrchestrator represents a simple `insecure.AttackOrchestrator` that tracks messages. This attack orchestrator
// simply passes through messages without changes to the orchestrator network. This orchestrator reduces boilerplate
// of BFT tests by implementing common logic. Boilerplate can further be reduced by using the OnEgressEvent and OnIngressEvent
// fields to define callbacks that will be invoked on each event received, this removes the need to redefine the logger and send
// message logic. Each func can be completely overriden if special functionality is needed.
type BaseOrchestrator struct {
	T                   *testing.T
	Logger              zerolog.Logger
	OrchestratorNetwork insecure.OrchestratorNetwork
	// OnEgressEvent callbacks invoked on each egress event received.
	OnEgressEvent []OnEgressEvent
	// OnIngressEvent callbacks invoked on each ingress event received.
	OnIngressEvent []OnIngressEvent
}

var _ insecure.AttackOrchestrator = &BaseOrchestrator{}

// HandleEgressEvent implements logic of processing the outgoing (egress) events received from a corrupted node.
func (b *BaseOrchestrator) HandleEgressEvent(event *insecure.EgressEvent) error {
	lg := b.Logger.With().
		Hex("corrupt_origin_id", logging.ID(event.CorruptOriginId)).
		Str("channel", event.Channel.String()).
		Str("protocol", event.Protocol.String()).
		Uint32("target_num", event.TargetNum).
		Strs("target_ids", logging.IDs(event.TargetIds)).
		Str("flow_protocol_event", logging.Type(event.FlowProtocolEvent)).Logger()

	for _, f := range b.OnEgressEvent {
		err := f(event)
		if err != nil {
			lg.Error().Err(err).Msg("could not pass through egress event")
			return err
		}
	}

	err := b.OrchestratorNetwork.SendEgress(event)
	if err != nil {
		lg.Error().Err(err).Msg("could not pass through egress event")
		return err
	}

	lg.Info().Str("event_id", event.FlowProtocolEventID.String()).Msg("egress event passed through successfully")
	return nil
}

// HandleIngressEvent implements logic of processing the incoming (ingress) events to a corrupt node.
// This handler will simply pass the message through to the corrupted node unmodified.
func (b *BaseOrchestrator) HandleIngressEvent(event *insecure.IngressEvent) error {
	lg := b.Logger.With().
		Hex("origin_id", logging.ID(event.OriginID)).
		Str("channel", event.Channel.String()).
		Str("corrupt_target_id", fmt.Sprintf("%v", event.CorruptTargetID)).
		Str("flow_protocol_event", fmt.Sprintf("%T", event.FlowProtocolEvent)).Logger()

	for _, f := range b.OnIngressEvent {
		err := f(event)
		if err != nil {
			lg.Error().Err(err).Msg("could not pass through ingress event")
			return err
		}
	}

	err := b.OrchestratorNetwork.SendIngress(event)
	if err != nil {
		lg.Error().Err(err).Msg("could not pass through ingress event")
		return err
	}

	lg.Info().Str("event_id", event.FlowProtocolEventID.String()).Msg("ingress event passed through successfully")
	return nil
}

func (b *BaseOrchestrator) Register(orchestratorNetwork insecure.OrchestratorNetwork) {
	b.OrchestratorNetwork = orchestratorNetwork
}
