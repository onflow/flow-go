package common

import (
	"context"
	"errors"
	"fmt"

	golog "github.com/ipfs/go-log/v2"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
)

var _ commands.AdminCommand = (*SetGologLevelCommand)(nil)

type SetGologLevelCommand struct{}

func (s *SetGologLevelCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	level := req.ValidatorData.(golog.LogLevel)
	golog.SetAllLoggers(level)

	log.Info().Msgf("changed log level to %v", level)
	return "ok", nil
}

func (s *SetGologLevelCommand) Validator(req *admin.CommandRequest) error {
	level, ok := req.Data.(string)
	if !ok {
		return errors.New("the input must be a string")
	}
	logLevel, err := golog.LevelFromString(level)
	if err != nil {
		return fmt.Errorf("failed to parse level: %w", err)
	}
	req.ValidatorData = logLevel
	return nil
}
