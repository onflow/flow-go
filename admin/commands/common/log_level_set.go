package common

import (
	"context"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/utils/logging"
)

var _ commands.AdminCommand = (*SetLogLevelCommand)(nil)

// SetLogLevelCommand sets the global default log level. Components with per-component overrides
// (from the CLI flag or a prior set-component-log-level command) are unaffected.
type SetLogLevelCommand struct {
	registry *logging.LogRegistry
}

// NewSetLogLevelCommand constructs a SetLogLevelCommand.
func NewSetLogLevelCommand(registry *logging.LogRegistry) *SetLogLevelCommand {
	return &SetLogLevelCommand{registry: registry}
}

// Handler sets the global default level via the registry.
//
// No error returns are expected during normal operation.
func (s *SetLogLevelCommand) Handler(_ context.Context, req *admin.CommandRequest) (interface{}, error) {
	level := req.ValidatorData.(zerolog.Level)
	s.registry.SetDefaultLevel(level)
	log.Info().Msgf("changed default log level to %v", level)
	return "ok", nil
}

// Validator validates the request.
//
// Returns [admin.InvalidAdminReqError] for invalid or malformed requests.
func (s *SetLogLevelCommand) Validator(req *admin.CommandRequest) error {
	level, ok := req.Data.(string)
	if !ok {
		return admin.NewInvalidAdminReqFormatError("the input must be a string")
	}
	logLevel, err := zerolog.ParseLevel(level)
	if err != nil {
		return admin.NewInvalidAdminReqErrorf("failed to parse level: %w", err)
	}
	req.ValidatorData = logLevel
	return nil
}
