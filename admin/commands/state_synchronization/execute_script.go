package state_synchronization

import (
	"context"
	"fmt"
	"strconv"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/module/execution"
)

var _ commands.AdminCommand = (*ReadExecutionDataCommand)(nil)

type scriptData struct {
	height    uint64
	script    []byte
	arguments [][]byte
}

type ExecuteScriptCommand struct {
	scriptExecutor *execution.Scripts
}

func (e *ExecuteScriptCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	d := req.ValidatorData.(*scriptData)

	result, err := e.scriptExecutor.ExecuteAtBlockHeight(context.Background(), d.script, d.arguments, d.height)
	if err != nil {
		return nil, err
	}

	return fmt.Sprintf("%s", result), nil
}

// Validator validates the request.
// Returns admin.InvalidAdminReqError for invalid/malformed requests.
func (e *ExecuteScriptCommand) Validator(req *admin.CommandRequest) error {
	input, ok := req.Data.(map[string]interface{})
	if !ok {
		return admin.NewInvalidAdminReqFormatError("expected map[string]any")
	}

	heightRaw, ok := input["height"]
	if !ok {
		return admin.NewInvalidAdminReqErrorf("missing required field 'height")
	}

	scriptRaw, ok := input["script"]
	if !ok {
		return admin.NewInvalidAdminReqErrorf("missing required field 'script")
	}

	argumentsRaw, ok := input["arguments"]
	if !ok {
		return admin.NewInvalidAdminReqErrorf("missing required field 'arguments")
	}

	heightStr, ok := heightRaw.(string)
	if !ok {
		return admin.NewInvalidAdminReqErrorf("'height' must be string")
	}

	height, err := strconv.ParseUint(heightStr, 10, 64)
	if err != nil {
		return admin.NewInvalidAdminReqErrorf("'height' must be valid uint64 value", err)
	}

	data := &scriptData{
		height: height,
	}

	return nil
}

func NewExecuteScriptCommand(scripts *execution.Scripts) commands.AdminCommand {
	return &ExecuteScriptCommand{
		scripts,
	}
}
