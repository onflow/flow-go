package execution

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/engine/execution/ingestion"
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

// Handler method sets the stop height parameters.
// Errors only if setting of stop height parameters fails.
// Returns "ok" if successful.
func (s *StopAtHeightCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	sah := req.ValidatorData.(StopAtHeightReq)

	_, oldHeight, oldCrash, err := s.stopAtHeight.Set(sah.height, sah.crash)

	if err != nil {
		return nil, err
	}

	log.Info().Msgf("admintool: EN will stop at height %d and crash: %t, previous values: %d %t", sah.height, sah.crash, oldHeight, oldCrash)

	return "ok", nil
}

// Validator checks the inputs for StopAtHeight command.
// It expects the following fields in the Data field of the req object:
//   - height in a numeric format
//   - crash, a boolean
//
// Additionally, height must be a positive integer. If a float value is provided, only the integer part is used.
// The following sentinel errors are expected during normal operations:
// * `commands.ErrValidatorReqDataFormat` if `req` is not a key-value map
// * `InvalidAdminParameterError` if any required field is missing or in a wrong format
func (s *StopAtHeightCommand) Validator(req *admin.CommandRequest) error {

	input, ok := req.Data.(map[string]interface{})
	if !ok {
		return admin.ErrValidatorReqDataFormat
	}
	result, ok := input["height"]
	errInvalidHeightValue := admin.NewInvalidAdminParameterError("height", "expected a positive integer", result)

	if !ok {
		return errInvalidHeightValue
	}

	height, ok := result.(float64)
	if !ok || height <= 0 {
		return errInvalidHeightValue
	}

	result, ok = input["crash"]
	errInvalidCrashValue := admin.NewInvalidAdminParameterError("crash", "expected a boolean", result)
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
