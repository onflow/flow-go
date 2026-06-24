package execution

import (
	"context"

	"github.com/rs/zerolog/log"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
)

var _ commands.AdminCommand = (*TriggerCheckpointCommand)(nil)

// TriggerCheckpointCommand will send a signal to compactor to trigger checkpoint
// once finishing writing the current WAL segment file.
// When running in remote ledger mode (ledgerServiceAddr is non-empty), this command
// returns an error directing users to the ledger service's admin endpoint.
type TriggerCheckpointCommand struct {
	trigger                *atomic.Bool
	ledgerServiceAddr      string // non-empty when using remote ledger service
	ledgerServiceAdminAddr string // admin HTTP address for remote ledger service
}

// NewTriggerCheckpointCommand creates a new TriggerCheckpointCommand.
// Parameters:
//   - trigger: atomic bool to signal the compactor (used only in local ledger mode)
//   - ledgerServiceAddr: gRPC address of the remote ledger service (empty string for local mode)
//   - ledgerServiceAdminAddr: admin HTTP address of the remote ledger service (for error messages)
func NewTriggerCheckpointCommand(trigger *atomic.Bool, ledgerServiceAddr, ledgerServiceAdminAddr string) *TriggerCheckpointCommand {
	return &TriggerCheckpointCommand{
		trigger:                trigger,
		ledgerServiceAddr:      ledgerServiceAddr,
		ledgerServiceAdminAddr: ledgerServiceAdminAddr,
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
	// When using remote ledger service, checkpointing is handled by the ledger service
	if s.ledgerServiceAddr != "" {
		if s.ledgerServiceAdminAddr == "" {
			return admin.NewInvalidAdminReqErrorf(
				"trigger-checkpoint is not available when using remote ledger service (connected to %s). "+
					"Please use the ledger service's admin endpoint instead. "+
					"The admin address was not configured - check if the ledger service was started with --admin-addr",
				s.ledgerServiceAddr,
			)
		}
		return admin.NewInvalidAdminReqErrorf(
			"trigger-checkpoint is not available when using remote ledger service (connected to %s). "+
				"Please use the ledger service's admin endpoint instead: "+
				"curl -X POST http://%s/admin/run_command -H 'Content-Type: application/json' -d '{\"commandName\": \"trigger-checkpoint\", \"data\": {}}'",
			s.ledgerServiceAddr,
			s.ledgerServiceAdminAddr,
		)
	}
	return nil
}
