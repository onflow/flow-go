package executor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/common/version"
	fvmerrors "github.com/onflow/flow-go/fvm/errors"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// LocalScriptExecutor is a script executor that runs scripts using only the local node's storage.
type LocalScriptExecutor struct {
	log     zerolog.Logger
	metrics module.BackendScriptsMetrics

	scriptExecutor      execution.ScriptExecutor
	scriptLogger        *ScriptLogger
	executionStateCache optimistic_sync.ExecutionStateCache
}

var _ ScriptExecutor = (*LocalScriptExecutor)(nil)

// NewLocalScriptExecutor creates a new [LocalScriptExecutor].
func NewLocalScriptExecutor(
	log zerolog.Logger,
	metrics module.BackendScriptsMetrics,
	scriptExecutor execution.ScriptExecutor,
	scriptLogger *ScriptLogger,
	executionStateCache optimistic_sync.ExecutionStateCache,
) *LocalScriptExecutor {
	return &LocalScriptExecutor{
		log:                 zerolog.New(log).With().Str("script_executor", "local").Logger(),
		metrics:             metrics,
		scriptLogger:        scriptLogger,
		scriptExecutor:      scriptExecutor,
		executionStateCache: executionStateCache,
	}
}

// Execute executes the provided script at the requested block.
//
// Expected error returns during normal operation:
//   - [InvalidArgumentError] - if the script execution failed due to invalid arguments or runtime errors.
//   - [ResourceExhausted] - if computation or memory limits were exceeded.
//   - [DataNotFoundError] - if block or registerSnapshot value at height was not found or snapshot at executionResultID was not found.
//   - [OutOfRangeError] - if the requested height is outside the available range, if the block height is not compatible with the node version,
//     if the requested height is outside the range of indexed blocks.
//   - [PreconditionFailedError] - if the registers storage is still bootstrapping.
//   - [ScriptExecutionCanceledError] - if the script execution was canceled.
//   - [ScriptExecutionTimedOutError] - if the script execution timed out.
//   - [InternalError] - for internal failures or index conversion errors.
func (l *LocalScriptExecutor) Execute(ctx context.Context, r *Request, executionResultInfo *optimistic_sync.ExecutionResultInfo,
) ([]byte, *accessmodel.ExecutorMetadata, error) {
	execStartTime := time.Now()

	executionResultID := executionResultInfo.ExecutionResultID
	snapshot, err := l.executionStateCache.Snapshot(executionResultID)
	if err != nil {
		err = RequireErrorIs(ctx, err, storage.ErrNotFound)
		err = fmt.Errorf("could not find snapshot for execution result %s: %w", executionResultInfo.ExecutionResultID, err)
		return nil, nil, NewDataNotFoundError("execution state snapshot", err)
	}

	registers, err := snapshot.Registers()
	if err != nil {
		err = RequireErrorIs(ctx, err, indexer.ErrIndexNotInitialized)
		err = fmt.Errorf("failed to get registers storage from snapshot: %w", err)
		return nil, nil, NewPreconditionFailedError(err)
	}

	log := l.log.With().
		Str("script_executor_addr", "localhost").
		Hex("block_id", logging.ID(r.blockID)).
		Uint64("height", r.height).
		Hex("script_hash", r.insecureScriptHash[:]).
		Logger()

	result, err := l.scriptExecutor.ExecuteAtBlockHeight(
		ctx,
		r.script,
		r.arguments,
		r.height,
		registers,
	)
	execEndTime := time.Now()
	execDuration := execEndTime.Sub(execStartTime)

	if err != nil {
		convertedErr := convertScriptExecutionError(err)

		switch {
		case IsInvalidArgumentError(convertedErr), IsScriptExecutionCanceledError(convertedErr), IsScriptExecutionTimedOutError(convertedErr):
			l.scriptLogger.LogFailedScript(r, "localhost")

		default:
			log.Debug().Err(err).Msg("script execution failed")
			l.metrics.ScriptExecutionErrorLocal() //TODO: this should be called in above cases as well?
		}

		return nil, nil, convertedErr
	}

	l.scriptLogger.LogExecutedScript(r, "localhost", execDuration)
	l.metrics.ScriptExecuted(execDuration, len(r.script))

	metadata := &accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultID,
		ExecutorIDs:       executionResultInfo.ExecutionNodes.NodeIDs(),
	}

	return result, metadata, nil
}

// convertScriptExecutionError converts errors to the script execution errors.
//
// Expected error returns during normal operation:
//   - [InvalidArgumentError] - if the script execution failed due to invalid arguments or runtime errors.
//   - [ResourceExhausted] - if computation or memory limits were exceeded.
//   - [DataNotFoundError] - if block or registerSnapshot value at height was not found or snapshot at executionResultID was not found.
//   - [OutOfRangeError] - if the requested height is outside the available range, if the block height is not compatible with the node version,
//     if the requested height is outside the range of indexed blocks.
//   - [ScriptExecutionCanceledError] - if the script execution was canceled.
//   - [ScriptExecutionTimedOutError] - if the script execution timed out.
//   - [InternalError] - for internal failures or index conversion errors.
func convertScriptExecutionError(err error) error {
	switch {
	case errors.Is(err, version.ErrOutOfRange),
		errors.Is(err, storage.ErrHeightNotIndexed),
		errors.Is(err, execution.ErrIncompatibleNodeVersion):
		return NewOutOfRangeError(err)
	case errors.Is(err, storage.ErrNotFound):
		return NewDataNotFoundError("scriptExecutor", err)
	}

	var failure fvmerrors.CodedFailure
	if fvmerrors.As(err, &failure) {
		return NewInternalError(err)
	}

	// general FVM/ledger errors
	var coded fvmerrors.CodedError
	if fvmerrors.As(err, &coded) {
		switch coded.Code() {
		case fvmerrors.ErrCodeScriptExecutionCancelledError:
			return NewScriptExecutionCanceledError(err)
		case fvmerrors.ErrCodeScriptExecutionTimedOutError:
			return NewScriptExecutionTimedOutError(err)
		case fvmerrors.ErrCodeComputationLimitExceededError:
			err = fmt.Errorf("script execution computation limit exceeded: %w", err)
			return NewResourceExhausted(err)
		case fvmerrors.ErrCodeMemoryLimitExceededError:
			err = fmt.Errorf("script execution memory limit exceeded: %w", err)
			return NewResourceExhausted(err)
		default:
			// runtime errors
			return NewInvalidArgumentError(err)
		}
	}

	return NewInternalError(err)
}
