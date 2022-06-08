package common

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
)

var _ commands.AdminCommand = (*SetLogLevelCommand)(nil)

type SetLogLevelCommand struct{}

func (s *SetLogLevelCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	level := req.ValidatorData.(zerolog.Level)
	zerolog.SetGlobalLevel(level)

	log.Info().Msgf("changed log level to %v", level)
	return "ok", nil
}

func (s *SetLogLevelCommand) Validator(req *admin.CommandRequest) error {
	level, ok := req.Data.(string)
	if !ok {
		return errors.New("the input must be a string")
	}
	logLevel, err := zerolog.ParseLevel(level)
	if err != nil {
		return fmt.Errorf("failed to parse level: %w", err)
	}
	req.ValidatorData = logLevel
	return nil
}
