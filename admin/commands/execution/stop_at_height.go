package execution

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/engine/execution/ingestion/stop"
)

var _ commands.AdminCommand = (*StopAtHeightCommand)(nil)

// StopAtHeightCommand will send a signal to engine to stop/crash EN
// at given height
type StopAtHeightCommand struct {
	stopControl *stop.StopControl
}

// NewStopAtHeightCommand creates a new StopAtHeightCommand object
func NewStopAtHeightCommand(sah *stop.StopControl) *StopAtHeightCommand {
	return &StopAtHeightCommand{
		stopControl: sah,
	}
}

type StopAtHeightReq struct {
	height uint64
	crash  bool
}

// Handler method sets the stop height parameters.
// Errors only if setting of stop height parameters fails.
// Returns "ok" if successful.
func (s *StopAtHeightCommand) Handler(_ context.Context, req *admin.CommandRequest) (interface{}, error) {
	sah := req.ValidatorData.(StopAtHeightReq)

	oldParams := s.stopControl.GetStopParameters()
	newParams := stop.StopParameters{
		StopBeforeHeight: sah.height,
		ShouldCrash:      sah.crash,
	}

	err := s.stopControl.SetStopParameters(newParams)

	if err != nil {
		return nil, err
	}

	log.Info().
		Interface("newParams", newParams).
		Interface("oldParams", oldParams).
		Msgf("admintool: New En stop parameters set")

	return "ok", nil
}

// Validator checks the inputs for StopAtHeight command.
// It expects the following fields in the Data field of the req object:
//   - height in a numeric format
//   - crash, a boolean
//
// Additionally, height must be a positive integer. If a float value is provided, only the integer part is used.
// The following sentinel errors are expected during normal operations:
// * `admin.InvalidAdminReqError` if any required field is missing or in a wrong format
func (s *StopAtHeightCommand) Validator(req *admin.CommandRequest) error {

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

	req.ValidatorData = StopAtHeightReq{
		height: uint64(height),
		crash:  crash,
	}

	return nil
}
