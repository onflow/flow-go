package state_synchronization

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/engine/common/version"
	fvmerrors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	"github.com/onflow/flow-go/storage"
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
//   - [admin.DataNotFoundError] - if data required to process the request is not available.
//   - [admin.OutOfRangeError] - if data required to process the request is outside the available range.
//   - [admin.RequestCanceledError] - if script execution canceled.
//   - [admin.RequestTimedOutError] - if script execution timed out.
//   - [admin.ResourceExhausted] - if script execution computation limit exceeded or memory limit exceeded.
func (e *ExecuteScriptCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	d := req.ValidatorData.(*scriptData)

	registerSnapshotReader, err := e.registersAsyncStore.RegisterSnapshotReader()
	if err != nil {
		err = admin.RequireErrorIs(ctx, err, indexer.ErrIndexNotInitialized)
		err = fmt.Errorf("failed to get register snapshot reader: %w", err)
		return nil, admin.NewPreconditionFailedError(err)
	}

	result, err := e.scriptExecutor.ExecuteAtBlockHeight(context.Background(), d.script, d.arguments, d.height, registerSnapshotReader)
	if err != nil {
		err = fmt.Errorf("failed to execute script: %w", err)

		switch {
		case errors.Is(err, version.ErrOutOfRange):
			return nil, admin.NewOutOfRangeError(err)
		case errors.Is(err, execution.ErrIncompatibleNodeVersion),
			errors.Is(err, storage.ErrNotFound),
			errors.Is(err, storage.ErrHeightNotIndexed):
			return nil, admin.NewDataNotFoundError("script", err)
		case fvmerrors.IsScriptExecutionCancelledError(err):
			return nil, admin.NewRequestCanceledError(err)
		case fvmerrors.IsScriptExecutionTimedOutError(err):
			return nil, admin.NewRequestTimedOutError(err)
		case fvmerrors.IsComputationLimitExceededError(err):
			return nil, admin.NewResourceExhausted(err)
		case fvmerrors.IsMemoryLimitExceededError(err):
			return nil, admin.NewResourceExhausted(err)
		default:
			return nil, admin.RequireNoError(ctx, err)
		}
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
