package state_synchronization

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/module/execution"
)

var _ commands.AdminCommand = (*ReadExecutionDataCommand)(nil)

// scriptData holds the parsed input data for ExecuteScriptCommand.
type scriptData struct {
	height    uint64
	script    []byte
	arguments [][]byte
}

// ExecuteScriptCommand is an admin command that executes a Cadence script.
type ExecuteScriptCommand struct {
	scriptExecutor      execution.ScriptExecutor
	registersAsyncStore *execution.RegistersAsyncStore
}

// Handler executes the Cadence script against the blockchain state at the
// specified block height.
//
// Expected error returns during normal operation:
//   - [version.ErrOutOfRange] - if incoming block height is higher that last handled block height.
//   - [execution.ErrIncompatibleNodeVersion] - if the block height is not compatible with the node version.
//   - [storage.ErrNotFound] - if data was not found.
//   - [storage.ErrHeightNotIndexed] - if the requested height is outside the range of indexed blocks.
//   - [fvmerrors.ErrCodeScriptExecutionCancelledError] - if script execution canceled.
//   - [fvmerrors.ErrCodeScriptExecutionTimedOutError] - if script execution timed out.
//   - [fvmerrors.ErrCodeComputationLimitExceededError] - if script execution computation limit exceeded.
//   - [fvmerrors.ErrCodeMemoryLimitExceededError] - if script execution memory limit exceeded.
//   - [indexer.ErrIndexNotInitialized] - if data for block is not available.
func (e *ExecuteScriptCommand) Handler(_ context.Context, req *admin.CommandRequest) (interface{}, error) {
	d := req.ValidatorData.(*scriptData)

	registerSnapshotReader, err := e.registersAsyncStore.RegisterSnapshotReader()
	if err != nil {
		return nil, fmt.Errorf("failed to get register snapshot reader: %w", err)
	}

	result, err := e.scriptExecutor.ExecuteAtBlockHeight(context.Background(), d.script, d.arguments, d.height, registerSnapshotReader)
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

func NewExecuteScriptCommand(scripts execution.ScriptExecutor, registersAsyncStore *execution.RegistersAsyncStore) commands.AdminCommand {
	return &ExecuteScriptCommand{
		scriptExecutor:      scripts,
		registersAsyncStore: registersAsyncStore,
	}
}
