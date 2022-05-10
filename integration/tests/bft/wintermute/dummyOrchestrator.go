package wintermute

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/utils/logging"
)

// DummyOrchestrator represents a simple orchestrator that passes through all incoming events.
type DummyOrchestrator struct {
	logger        zerolog.Logger
	attackNetwork insecure.AttackNetwork
}

func NewDummyOrchestrator(logger zerolog.Logger) *DummyOrchestrator {
	return &DummyOrchestrator{logger: logger.With().Str("component", "dummy-orchestrator").Logger()}
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
