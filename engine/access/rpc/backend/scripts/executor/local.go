package executor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
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
		log:                 log.With().Str("script_executor", "local").Logger(),
		metrics:             metrics,
		scriptLogger:        scriptLogger,
		scriptExecutor:      scriptExecutor,
		executionStateCache: executionStateCache,
	}
}

// Execute executes the provided script at the requested block.
//
// Expected error returns during normal operation:
//   - [access.InvalidRequestError] - if the script execution failed due to invalid arguments or runtime errors.
//   - [access.ResourceExhausted] - if computation or memory limits were exceeded.
//   - [access.DataNotFoundError] - if data was not found.
//   - [access.OutOfRangeError] - if the data for the requested height is outside the node's available range.
//   - [access.PreconditionFailedError] - if the registers storage is still bootstrapping.
//   - [access.RequestCanceledError] - if the script execution was canceled.
//   - [access.RequestTimedOutError] - if the script execution timed out.
func (l *LocalScriptExecutor) Execute(ctx context.Context, r *Request, executionResultInfo *optimistic_sync.ExecutionResultInfo,
) ([]byte, *accessmodel.ExecutorMetadata, error) {
	execStartTime := time.Now()

	executionResultID := executionResultInfo.ExecutionResultID
	snapshot, err := l.executionStateCache.Snapshot(executionResultID)
	if err != nil {
		err = access.RequireErrorIs(ctx, err, storage.ErrNotFound)
		err = fmt.Errorf("could not find snapshot for execution result %s: %w", executionResultInfo.ExecutionResultID, err)
		return nil, nil, access.NewDataNotFoundError("execution state snapshot", err)
	}

	registers, err := snapshot.Registers()
	if err != nil {
		err = access.RequireErrorIs(ctx, err, indexer.ErrIndexNotInitialized)
		err = fmt.Errorf("failed to get registers storage from snapshot: %w", err)
		return nil, nil, access.NewPreconditionFailedError(err)
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
	execDuration := time.Since(execStartTime)

	if err != nil {
		convertedErr := convertScriptExecutionError(err)

		switch {
		case access.IsInvalidRequestError(convertedErr), access.IsRequestCanceledError(convertedErr), access.IsRequestTimedOutError(convertedErr):
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

// convertScriptExecutionError converts script executor and FVM errors to corresponding access-layer sentinel errors.
//
// Expected error returns during normal operation:
//   - [access.InvalidRequestError] - if the script execution failed due to invalid arguments or runtime errors.
//   - [access.ResourceExhausted] - if computation or memory limits were exceeded.
//   - [access.DataNotFoundError] - if data for the requested height was not found.
//   - [access.OutOfRangeError] - if the data for the requested height is outside the node's available range.
//   - [access.RequestCanceledError] - if the script execution was canceled.
//   - [access.RequestTimedOutError] - if the script execution timed out.
func convertScriptExecutionError(err error) error {
	switch {
	case errors.Is(err, version.ErrOutOfRange),
		errors.Is(err, storage.ErrHeightNotIndexed),
		errors.Is(err, execution.ErrIncompatibleNodeVersion):
		return access.NewOutOfRangeError(err)
	case errors.Is(err, storage.ErrNotFound):
		return access.NewDataNotFoundError("header", err)
	case errors.Is(err, context.Canceled):
		return access.NewRequestCanceledError(err)
	case errors.Is(err, context.DeadlineExceeded):
		return access.NewRequestTimedOutError(err)
	}

	// general FVM/ledger errors
	var coded fvmerrors.CodedError
	if fvmerrors.As(err, &coded) {
		switch coded.Code() {
		case fvmerrors.ErrCodeScriptExecutionCancelledError:
			return access.NewRequestCanceledError(err)
		case fvmerrors.ErrCodeScriptExecutionTimedOutError:
			return access.NewRequestTimedOutError(err)
		case fvmerrors.ErrCodeComputationLimitExceededError:
			err = fmt.Errorf("script execution computation limit exceeded: %w", err)
			return access.NewResourceExhausted(err)
		case fvmerrors.ErrCodeMemoryLimitExceededError:
			err = fmt.Errorf("script execution memory limit exceeded: %w", err)
			return access.NewResourceExhausted(err)
		default:
			// runtime errors
			return access.NewInvalidRequestError(err)
		}
	}

	return err
}
