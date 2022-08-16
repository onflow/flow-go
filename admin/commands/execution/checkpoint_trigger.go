package execution

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
)

var _ commands.AdminCommand = (*TriggerCheckpointCommand)(nil)

// TriggerCheckpointCommand will send a signal to compactor to trigger checkpoint
// once finishing writing the current WAL segment file
type TriggerCheckpointCommand struct {
	trigger chan<- interface{}
}

func NewTriggerCheckpointCommand(trigger chan<- interface{}) *TriggerCheckpointCommand {
	return &TriggerCheckpointCommand{
		trigger: trigger,
	}
}

func (s *TriggerCheckpointCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	select {
	case s.trigger <- struct{}{}:
	default:
	}

	log.Info().Msgf("admintool: trigger checkpoint as soonas finishing writing the current segment file. you can find log about 'compactor' to check the checkpointing progress")

	return "ok", nil
}

func (s *TriggerCheckpointCommand) Validator(req *admin.CommandRequest) error {
	return nil
}
