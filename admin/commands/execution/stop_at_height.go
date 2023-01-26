package execution

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/engine/execution/ingestion"
)

var _ commands.AdminCommand = (*StopENAtHeightCommand)(nil)

// StopENAtHeightCommand will send a signal to engine to stop/crash EN
// at given height
type StopENAtHeightCommand struct {
	stopControl *ingestion.StopControl
}

// NewStopENAtHeightCommand creates a new StopENAtHeightCommand object
func NewStopENAtHeightCommand(sah *ingestion.StopControl) *StopENAtHeightCommand {
	return &StopENAtHeightCommand{
		stopControl: sah,
	}
}

type StopENAtHeightReq struct {
	height uint64
	crash  bool
}

// Handler method sets the stop height parameters.
// Errors only if setting of stop height parameters fails.
// Returns "ok" if successful.
func (s *StopENAtHeightCommand) Handler(_ context.Context, req *admin.CommandRequest) (interface{}, error) {
	sah := req.ValidatorData.(StopENAtHeightReq)

	oldHeight, oldCrash, err := s.stopControl.SetStopHeight(sah.height, sah.crash)

	if err != nil {
		return nil, err
	}

	log.Info().Msgf("admintool: EN will stop at height %d and crash: %t, previous values: %d %t", sah.height, sah.crash, oldHeight, oldCrash)

	return "ok", nil
}

// Validator checks the inputs for StopENAtHeight command.
// It expects the following fields in the Data field of the req object:
//   - height in a numeric format
//   - crash, a boolean
//
// Additionally, height must be a positive integer. If a float value is provided, only the integer part is used.
// The following sentinel errors are expected during normal operations:
// * `admin.InvalidAdminReqError` if any required field is missing or in a wrong format
func (s *StopENAtHeightCommand) Validator(req *admin.CommandRequest) error {

	input, ok := req.Data.(map[string]interface{})
	if !ok {
		return admin.NewInvalidAdminReqFormatError("expected map[string]any")
	}
	result, ok := input["height"]
	if !ok {
		return admin.NewInvalidAdminReqErrorf("missing required field: 'height'")
	}
	height, ok := result.(float64)
	if !ok || height <= 0 {
		return admin.NewInvalidAdminReqParameterError("height", "must be number >=0", result)
	}

	result, ok = input["crash"]
	if !ok {
		return admin.NewInvalidAdminReqErrorf("missing required field: 'crash'")
	}
	crash, ok := result.(bool)
	if !ok {
		return admin.NewInvalidAdminReqParameterError("crash", "must be bool", result)
	}

	req.ValidatorData = StopENAtHeightReq{
		height: uint64(height),
		crash:  crash,
	}

	return nil
}
