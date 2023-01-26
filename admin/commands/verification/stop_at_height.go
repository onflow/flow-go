package verification

import (
	"context"
	"github.com/onflow/flow-go/engine/verification"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
)

var _ commands.AdminCommand = (*StopVNAtHeightCommand)(nil)

// StopVNAtHeightCommand will send a signal to engine to stop/crash EN
// at given height
type StopVNAtHeightCommand struct {
	stopControl *verification.StopControl
}

// NewStopVNAtHeightCommand creates a new StopVNAtHeightCommand object
func NewStopVNAtHeightCommand(sah *verification.StopControl) *StopVNAtHeightCommand {
	return &StopVNAtHeightCommand{
		stopControl: sah,
	}
}

type StopVNAtHeightReq struct {
	height uint64
}

// Handler method sets the stop height parameters.
// Errors only if setting of stop height parameters fails.
// Returns "ok" if successful.
func (s *StopVNAtHeightCommand) Handler(_ context.Context, req *admin.CommandRequest) (interface{}, error) {
	sah := req.ValidatorData.(StopVNAtHeightReq)

	oldHeight, err := s.stopControl.SetStopHeight(sah.height)

	if err != nil {
		return nil, err
	}

	log.Info().Msgf("admintool: VN will stop at height %d previously set to: %d", sah.height, oldHeight)

	return "ok", nil
}

// Validator checks the inputs for StopVNAtHeight command.
// It expects the following fields in the Data field of the req object:
//   - height in a numeric format
//
// Additionally, height must be a positive integer. If a float value is provided, only the integer part is used.
// The following sentinel errors are expected during normal operations:
// * `admin.InvalidAdminReqError` if any required field is missing or in a wrong format
func (s *StopVNAtHeightCommand) Validator(req *admin.CommandRequest) error {

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

	req.ValidatorData = StopVNAtHeightReq{
		height: uint64(height),
	}

	return nil
}
