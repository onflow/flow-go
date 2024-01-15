package collection

import (
	"context"

	"github.com/rs/zerolog/log"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
)

var _ commands.AdminCommand = (*TriggerCheckpointCommand)(nil)

// TriggerCheckpointCommand will send a signal to compactor to trigger checkpoint
// once finishing writing the current WAL segment file
type TriggerCheckpointCommand struct {
	trigger *atomic.Bool
}

func NewAddRateLimitedAddress(trigger *atomic.Bool) *TriggerCheckpointCommand {
	return &TriggerCheckpointCommand{
		trigger: trigger,
	}
}

func (s *TriggerCheckpointCommand) Handler(_ context.Context, _ *admin.CommandRequest) (interface{}, error) {
	if s.trigger.CompareAndSwap(false, true) {
		log.Info().Msgf("admintool: trigger checkpoint as soon as finishing writing the current segment file. you can find log about 'compactor' to check the checkpointing progress")
	} else {
		log.Info().Msgf("admintool: checkpoint is already set to be triggered")
	}

	return "ok", nil
}

func (s *TriggerCheckpointCommand) Validator(_ *admin.CommandRequest) error {
	return nil
}
