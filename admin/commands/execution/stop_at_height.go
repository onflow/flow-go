package execution

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
)

var _ commands.AdminCommand = (*StopAtHeightCommand)(nil)

// StopAtHeightCommand will send a signal to engine to stop/crash EN
// at given height
type StopAtHeightCommand struct {
	height *atomic.Uint64
	crash  *atomic.Bool
}

func NewStopAtHeightCommand(height *atomic.Uint64, crash *atomic.Bool) *StopAtHeightCommand {
	return &StopAtHeightCommand{
		height: height,
		crash:  crash,
	}
}

type StopAtHeightReq struct {
	height uint64
	crash  bool
}

func (s *StopAtHeightCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	sah := req.ValidatorData.(StopAtHeightReq)

	oldHeight := s.height.Swap(sah.height)
	oldCrash := s.crash.Swap(sah.crash)

	log.Info().Msgf("admintool: EN will stop at height %d and crash: %t, previous values: %d %t", sah.height, sah.crash, oldHeight, oldCrash)

	return "ok", nil
}

func (s *StopAtHeightCommand) Validator(req *admin.CommandRequest) error {

	sahr := StopAtHeightReq{}

	input, ok := req.Data.(map[string]interface{})
	if !ok {
		return commands.ErrValidatorReqDataFormat
	}
	result, ok := input["height"]
	errInvalidHeightValue := fmt.Errorf("invalid value for \"height\": expected a positive integer, but got: %v", result)

	if !ok {
		return errInvalidHeightValue
	}

	height, ok := result.(float64)
	if !ok || height <= 0 {
		return errInvalidHeightValue
	}
	sahr.height = uint64(height)

	result, ok = input["crash"]
	errInvalidCrashValue := fmt.Errorf("invalid value for \"crash\": expected a boolean, but got: %v", result)
	if !ok {
		return errInvalidCrashValue
	}
	crash, ok := result.(bool)
	if !ok {
		return errInvalidCrashValue
	}

	sahr.crash = crash

	req.ValidatorData = sahr

	return nil
}
