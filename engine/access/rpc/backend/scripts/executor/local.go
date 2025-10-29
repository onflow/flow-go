package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/common/rpc"
	fvmerrors "github.com/onflow/flow-go/fvm/errors"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
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
//   - [version.ErrOutOfRange] - if block height is higher that last handled block height.
//   - [execution.ErrIncompatibleNodeVersion] - if the block height is not compatible with the node version.
//   - [storage.ErrNotFound] - if block or registerSnapshot value at height was not found or snapshot at executionResultID was not found.
//   - [indexer.ErrIndexNotInitialized] - if the storage is still bootstrapping.
//   - [storage.ErrHeightNotIndexed] - if the requested height is outside the range of indexed blocks.
//   - [codes.Canceled] - if the script execution was canceled.
//   - [codes.DeadlineExceeded] - if the script execution timed out.
//   - [codes.ResourceExhausted] - if computation or memory limits were exceeded.
//   - [codes.InvalidArgument] - if the script execution failed due to invalid arguments or runtime errors.
//   - [codes.FailedPrecondition] - if data for block is not available.
//   - [codes.OutOfRange] - if data for block height is not available.
//   - [codes.NotFound] - if data not found.
//   - [codes.Internal] - for internal failures or index conversion errors.
func (l *LocalScriptExecutor) Execute(ctx context.Context, r *Request, executionResultInfo *optimistic_sync.ExecutionResultInfo,
) ([]byte, *accessmodel.ExecutorMetadata, error) {
	execStartTime := time.Now()

	executionResultID := executionResultInfo.ExecutionResultID
	snapshot, err := l.executionStateCache.Snapshot(executionResultID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get snapshot for execution result %s: %w", executionResultID, err)
	}

	registers, err := snapshot.Registers()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get registers storage from snapshot: %w", err)
	}
	result, err := l.scriptExecutor.ExecuteAtBlockHeight(
		ctx,
		r.script,
		r.arguments,
		r.height,
		registers,
	)

	execEndTime := time.Now()
	execDuration := execEndTime.Sub(execStartTime)

	log := l.log.With().
		Str("script_executor_addr", "localhost").
		Hex("block_id", logging.ID(r.blockID)).
		Uint64("height", r.height).
		Hex("script_hash", r.insecureScriptHash[:]).
		Logger()

	if err != nil {
		convertedErr := convertScriptExecutionError(err, r.height)

		switch status.Code(convertedErr) {
		case codes.InvalidArgument, codes.Canceled, codes.DeadlineExceeded:
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

// convertScriptExecutionError converts the script execution error to a gRPC error.
//
// Expected error returns during normal operation:
//   - [codes.Canceled] - if the script execution was canceled.
//   - [codes.DeadlineExceeded] - if the script execution timed out.
//   - [codes.ResourceExhausted] - if computation or memory limits were exceeded.
//   - [codes.InvalidArgument] - if the script execution failed due to invalid arguments or runtime errors.
//   - [codes.FailedPrecondition] - if data for block is not available.
//   - [codes.OutOfRange] - if data for block height is not available.
//   - [codes.NotFound] - if data not found.
//   - [codes.Internal] - for internal failures or index conversion errors.
func convertScriptExecutionError(err error, height uint64) error {
	if err == nil {
		return nil
	}

	var failure fvmerrors.CodedFailure
	if fvmerrors.As(err, &failure) {
		return rpc.ConvertError(err, "failed to execute script", codes.Internal)
	}

	// general FVM/ledger errors
	var coded fvmerrors.CodedError
	if fvmerrors.As(err, &coded) {
		switch coded.Code() {
		case fvmerrors.ErrCodeScriptExecutionCancelledError:
			return status.Errorf(codes.Canceled, "script execution canceled: %v", err)

		case fvmerrors.ErrCodeScriptExecutionTimedOutError:
			return status.Errorf(codes.DeadlineExceeded, "script execution timed out: %v", err)

		case fvmerrors.ErrCodeComputationLimitExceededError:
			return status.Errorf(codes.ResourceExhausted, "script execution computation limit exceeded: %v", err)

		case fvmerrors.ErrCodeMemoryLimitExceededError:
			return status.Errorf(codes.ResourceExhausted, "script execution memory limit exceeded: %v", err)

		default:
			// runtime errors
			return status.Errorf(codes.InvalidArgument, "failed to execute script: %v", err)
		}
	}

	return rpc.ConvertIndexError(err, height, "failed to execute script")
}
