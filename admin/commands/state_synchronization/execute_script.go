package state_synchronization

import (
	"context"
	"encoding/json"
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
	scriptExecutor execution.ScriptExecutor
}

func (e *ExecuteScriptCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	d := req.ValidatorData.(*scriptData)

	result, _, err := e.scriptExecutor.ExecuteAtBlockHeight(context.Background(), d.script, d.arguments, d.height)
	if err != nil {
		return nil, err
	}

	return string(result), nil
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
		return admin.NewInvalidAdminReqFormatError("missing required field 'height")
	}

	scriptRaw, ok := input["script"]
	if !ok {
		return admin.NewInvalidAdminReqFormatError("missing required field 'script")
	}

	argsRaw, ok := input["args"]
	if !ok {
		return admin.NewInvalidAdminReqFormatError("missing required field 'args")
	}

	heightStr, ok := heightRaw.(string)
	if !ok {
		return admin.NewInvalidAdminReqFormatError("'height' must be string")
	}

	height, err := strconv.ParseUint(heightStr, 10, 64)
	if err != nil {
		return admin.NewInvalidAdminReqFormatError("'height' must be valid uint64 value", err)
	}

	scriptStr, ok := scriptRaw.(string)
	if !ok {
		return admin.NewInvalidAdminReqFormatError("'script' must be string")
	}

	argsStr, ok := argsRaw.(string)
	if !ok {
		return admin.NewInvalidAdminReqFormatError("'args' must be string")
	}

	args := make([][]byte, 0)
	err = json.Unmarshal([]byte(argsStr), &args)
	if err != nil {
		return admin.NewInvalidAdminReqFormatError("'args' not valid JSON", err)
	}

	req.ValidatorData = &scriptData{
		height:    height,
		script:    []byte(scriptStr),
		arguments: args,
	}

	return nil
}

func NewExecuteScriptCommand(scripts execution.ScriptExecutor) commands.AdminCommand {
	return &ExecuteScriptCommand{
		scripts,
	}
}
