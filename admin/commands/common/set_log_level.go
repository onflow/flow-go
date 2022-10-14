package common

import (
	"context"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
)

var _ commands.AdminCommand = (*SetLogLevelCommand)(nil)

type SetLogLevelCommand struct{}

func (s *SetLogLevelCommand) Handler(_ context.Context, req *admin.CommandRequest) (interface{}, error) {
	level := req.ValidatorData.(zerolog.Level)
	zerolog.SetGlobalLevel(level)

	log.Info().Msgf("changed log level to %v", level)
	return "ok", nil
}

// Validator validates the request.
// Returns admin.InvalidAdminReqError for invalid/malformed requests.
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
