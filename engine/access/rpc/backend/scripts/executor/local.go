package executor

import (
	"context"
	"crypto/md5" //nolint:gosec
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

// TODO(Uliana): add godoc to whole file
type LocalScriptExecutor struct {
	log     zerolog.Logger
	metrics module.BackendScriptsMetrics

	scriptExecutor      execution.ScriptExecutor
	scriptCache         *LoggedScriptCache
	executionStateCache optimistic_sync.ExecutionStateCache
}

var _ ScriptExecutor = (*LocalScriptExecutor)(nil)

func NewLocalScriptExecutor(
	log zerolog.Logger,
	metrics module.BackendScriptsMetrics,
	scriptExecutor execution.ScriptExecutor,
	scriptCache *LoggedScriptCache,
	executionStateCache optimistic_sync.ExecutionStateCache,
) *LocalScriptExecutor {
	return &LocalScriptExecutor{
		log:                 zerolog.New(log).With().Str("script_executor", "local").Logger(),
		metrics:             metrics,
		scriptCache:         scriptCache,
		scriptExecutor:      scriptExecutor,
		executionStateCache: executionStateCache,
	}
}

// Execute
// Expected errors during normal operation:
//   - storage.ErrNotFound - result is not available, not ready for querying, or does not descend from the latest sealed result.
func (l *LocalScriptExecutor) Execute(
	ctx context.Context,
	r *Request,
) ([]byte, *accessmodel.ExecutorMetadata, error) {
	execStartTime := time.Now()

	executionResultID := r.execResultInfo.ExecutionResultID
	snapshot, err := l.executionStateCache.Snapshot(executionResultID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get snapshot for execution result %s: %w", executionResultID, err)
	}

	result, err := l.scriptExecutor.ExecuteAtBlockHeight(
		ctx,
		r.script,
		r.arguments,
		r.height,
		snapshot.Registers(),
	)

	execEndTime := time.Now()
	execDuration := execEndTime.Sub(execStartTime)

	// encode to MD5 as low compute/memory lookup key
	// CAUTION: cryptographically insecure md5 is used here, but only to de-duplicate logs.
	// *DO NOT* use this hash for any protocol-related or cryptographic functions.
	insecureScriptHash := md5.Sum(r.script) //nolint:gosec

	log := l.log.With().
		Str("script_executor_addr", "localhost").
		Hex("block_id", logging.ID(r.blockID)).
		Uint64("height", r.height).
		Hex("script_hash", insecureScriptHash[:]).
		Logger()

	if err != nil {
		convertedErr := convertScriptExecutionError(err, r.height)

		switch status.Code(convertedErr) {
		case codes.InvalidArgument, codes.Canceled, codes.DeadlineExceeded:
			l.scriptCache.LogFailedScript(r.blockID, insecureScriptHash, execEndTime, "localhost", r.script)

		default:
			log.Debug().Err(err).Msg("script execution failed")
			l.metrics.ScriptExecutionErrorLocal() //TODO: this should be called in above cases as well?
		}

		return nil, nil, convertedErr
	}

	l.scriptCache.LogExecutedScript(r.blockID, insecureScriptHash, execEndTime, "localhost", r.script, execDuration)
	l.metrics.ScriptExecuted(execDuration, len(r.script))

	metadata := &accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultID,
		ExecutorIDs:       r.execResultInfo.ExecutionNodes.NodeIDs(),
	}

	return result, metadata, nil
}

// convertScriptExecutionError converts the script execution error to a gRPC error
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
