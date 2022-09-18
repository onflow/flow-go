package execution

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/engine/execution/ingestion"
	"github.com/rs/zerolog/log"
)

var _ commands.AdminCommand = (*StopAtHeightCommand)(nil)

// StopAtHeightCommand will send a signal to engine to stop/crash EN
// at given height
type StopAtHeightCommand struct {
	stopAtHeight *ingestion.StopAtHeight
}

func NewStopAtHeightCommand(sah *ingestion.StopAtHeight) *StopAtHeightCommand {
	return &StopAtHeightCommand{
		stopAtHeight: sah,
	}
}

type StopAtHeightReq struct {
	height uint64
	crash  bool
}

func (s *StopAtHeightCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	sah := req.ValidatorData.(StopAtHeightReq)

	_, oldHeight, oldCrash := s.stopAtHeight.Set(sah.height, sah.crash)

	log.Info().Msgf("admintool: EN will stop at height %d and crash: %t, previous values: %d %t", sah.height, sah.crash, oldHeight, oldCrash)

	return "ok", nil
}

func (s *StopAtHeightCommand) Validator(req *admin.CommandRequest) error {

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

	result, ok = input["crash"]
	errInvalidCrashValue := fmt.Errorf("invalid value for \"crash\": expected a boolean, but got: %v", result)
	if !ok {
		return errInvalidCrashValue
	}
	crash, ok := result.(bool)
	if !ok {
		return errInvalidCrashValue
	}

	req.ValidatorData = StopAtHeightReq{
		height: uint64(height),
		crash:  crash,
	}

	return nil
}
