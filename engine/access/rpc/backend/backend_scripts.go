package backend

import (
	"context"
	"crypto/md5" //nolint:gosec
	"errors"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/common/rpc"
	fvmerrors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// uniqueScriptLoggingTimeWindow is the duration for checking the uniqueness of scripts sent for execution
const uniqueScriptLoggingTimeWindow = 10 * time.Minute

type backendScripts struct {
	log               zerolog.Logger
	headers           storage.Headers
	executionReceipts storage.ExecutionReceipts
	state             protocol.State
	connFactory       connection.ConnectionFactory
	metrics           module.BackendScriptsMetrics
	loggedScripts     *lru.Cache[[md5.Size]byte, time.Time]
	nodeCommunicator  Communicator
	scriptExecutor    execution.ScriptExecutor
	scriptExecMode    IndexQueryMode
}

// scriptExecutionRequest encapsulates the data needed to execute a script to make it easier
// to pass around between the various methods involved in script execution
type scriptExecutionRequest struct {
	blockID            flow.Identifier
	height             uint64
	script             []byte
	arguments          [][]byte
	insecureScriptHash [md5.Size]byte
}

func newScriptExecutionRequest(blockID flow.Identifier, height uint64, script []byte, arguments [][]byte) *scriptExecutionRequest {
	return &scriptExecutionRequest{
		blockID:   blockID,
		height:    height,
		script:    script,
		arguments: arguments,

		// encode to MD5 as low compute/memory lookup key
		// CAUTION: cryptographically insecure md5 is used here, but only to de-duplicate logs.
		// *DO NOT* use this hash for any protocol-related or cryptographic functions.
		insecureScriptHash: md5.Sum(script), //nolint:gosec
	}
}

// ExecuteScriptAtLatestBlock executes provided script at the latest sealed block.
func (b *backendScripts) ExecuteScriptAtLatestBlock(
	ctx context.Context,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {
	latestHeader, err := b.state.Sealed().Head()
	if err != nil {
		// the latest sealed header MUST be available
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, err
	}

	return b.executeScript(ctx, newScriptExecutionRequest(latestHeader.ID(), latestHeader.Height, script, arguments))
}

// ExecuteScriptAtBlockID executes provided script at the provided block ID.
func (b *backendScripts) ExecuteScriptAtBlockID(
	ctx context.Context,
	blockID flow.Identifier,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {
	header, err := b.headers.ByBlockID(blockID)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	return b.executeScript(ctx, newScriptExecutionRequest(blockID, header.Height, script, arguments))
}

// ExecuteScriptAtBlockHeight executes provided script at the provided block height.
func (b *backendScripts) ExecuteScriptAtBlockHeight(
	ctx context.Context,
	blockHeight uint64,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {
	header, err := b.headers.ByHeight(blockHeight)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	return b.executeScript(ctx, newScriptExecutionRequest(header.ID(), blockHeight, script, arguments))
}

// executeScript executes the provided script using either the local execution state or the execution
// nodes depending on the node's configuration and the availability of the data.
func (b *backendScripts) executeScript(
	ctx context.Context,
	scriptRequest *scriptExecutionRequest,
) ([]byte, error) {
	switch b.scriptExecMode {
	case IndexQueryModeExecutionNodesOnly:
		result, _, err := b.executeScriptOnAvailableExecutionNodes(ctx, scriptRequest)
		return result, err

	case IndexQueryModeLocalOnly:
		result, _, err := b.executeScriptLocally(ctx, scriptRequest)
		return result, err

	case IndexQueryModeFailover:
		localResult, localDuration, localErr := b.executeScriptLocally(ctx, scriptRequest)
		if localErr == nil || isInvalidArgumentError(localErr) || status.Code(localErr) == codes.Canceled {
			return localResult, localErr
		}
		// Note: scripts that timeout are retried on the execution nodes since ANs may have performance
		// issues for some scripts.
		execResult, execDuration, execErr := b.executeScriptOnAvailableExecutionNodes(ctx, scriptRequest)

		resultComparer := newScriptResultComparison(b.log, b.metrics, scriptRequest)
		_ = resultComparer.compare(
			newScriptResult(execResult, execDuration, execErr),
			newScriptResult(localResult, localDuration, localErr),
		)

		return execResult, execErr

	case IndexQueryModeCompare:
		execResult, execDuration, execErr := b.executeScriptOnAvailableExecutionNodes(ctx, scriptRequest)
		// we can only compare the results if there were either no errors or a cadence error
		// since we cannot distinguish the EN error as caused by the block being pruned or some other reason,
		// which may produce a valid RN output but an error for the EN
		if execErr != nil && !isInvalidArgumentError(execErr) {
			return nil, execErr
		}
		localResult, localDuration, localErr := b.executeScriptLocally(ctx, scriptRequest)

		resultComparer := newScriptResultComparison(b.log, b.metrics, scriptRequest)
		_ = resultComparer.compare(
			newScriptResult(execResult, execDuration, execErr),
			newScriptResult(localResult, localDuration, localErr),
		)

		// always return EN results
		return execResult, execErr

	default:
		return nil, status.Errorf(codes.Internal, "unknown script execution mode: %v", b.scriptExecMode)
	}
}

// executeScriptLocally executes the provided script using the local execution state.
func (b *backendScripts) executeScriptLocally(
	ctx context.Context,
	r *scriptExecutionRequest,
) ([]byte, time.Duration, error) {
	execStartTime := time.Now()

	result, err := b.scriptExecutor.ExecuteAtBlockHeight(ctx, r.script, r.arguments, r.height)

	execEndTime := time.Now()
	execDuration := execEndTime.Sub(execStartTime)

	lg := b.log.With().
		Str("script_executor_addr", "localhost").
		Hex("block_id", logging.ID(r.blockID)).
		Uint64("height", r.height).
		Hex("script_hash", r.insecureScriptHash[:]).
		Dur("execution_dur_ms", execDuration).
		Logger()

	if err != nil {
		convertedErr := convertScriptExecutionError(err, r.height)

		switch status.Code(convertedErr) {
		case codes.InvalidArgument, codes.Canceled, codes.DeadlineExceeded:
			lg.Debug().Err(err).
				Str("script", string(r.script)).
				Msg("script failed to execute locally")

		default:
			lg.Error().Err(err).Msg("script execution failed")
			b.metrics.ScriptExecutionErrorLocal()
		}

		return nil, execDuration, convertedErr
	}

	if b.log.GetLevel() == zerolog.DebugLevel && b.shouldLogScript(execEndTime, r.insecureScriptHash) {
		lg.Debug().
			Str("script", string(r.script)).
			Msg("Successfully executed script")
		b.loggedScripts.Add(r.insecureScriptHash, execEndTime)
	}

	// log execution time
	b.metrics.ScriptExecuted(execDuration, len(r.script))

	return result, execDuration, nil
}

// executeScriptOnAvailableExecutionNodes executes the provided script using available execution nodes.
func (b *backendScripts) executeScriptOnAvailableExecutionNodes(
	ctx context.Context,
	r *scriptExecutionRequest,
) ([]byte, time.Duration, error) {
	// find few execution nodes which have executed the block earlier and provided an execution receipt for it
	executors, err := executionNodesForBlockID(ctx, r.blockID, b.executionReceipts, b.state, b.log)
	if err != nil {
		return nil, 0, status.Errorf(codes.Internal, "failed to find script executors at blockId %v: %v", r.blockID.String(), err)
	}

	lg := b.log.With().
		Hex("block_id", logging.ID(r.blockID)).
		Hex("script_hash", r.insecureScriptHash[:]).
		Logger()

	var result []byte
	var execDuration time.Duration
	errToReturn := b.nodeCommunicator.CallAvailableNode(
		executors,
		func(node *flow.IdentitySkeleton) error {
			execStartTime := time.Now()

			result, err = b.tryExecuteScriptOnExecutionNode(ctx, node.Address, r)

			executionTime := time.Now()
			execDuration = executionTime.Sub(execStartTime)

			if err != nil {
				return err
			}

			if b.log.GetLevel() == zerolog.DebugLevel {
				if b.shouldLogScript(executionTime, r.insecureScriptHash) {
					lg.Debug().
						Str("script_executor_addr", node.Address).
						Str("script", string(r.script)).
						Dur("execution_dur_ms", execDuration).
						Msg("Successfully executed script")
					b.loggedScripts.Add(r.insecureScriptHash, executionTime)
				}
			}

			// log execution time
			b.metrics.ScriptExecuted(time.Since(execStartTime), len(r.script))

			return nil
		},
		func(node *flow.IdentitySkeleton, err error) bool {
			if status.Code(err) == codes.InvalidArgument {
				lg.Debug().Err(err).
					Str("script_executor_addr", node.Address).
					Str("script", string(r.script)).
					Msg("script failed to execute on the execution node")
				return true
			}
			return false
		},
	)

	if errToReturn != nil {
		if status.Code(errToReturn) != codes.InvalidArgument {
			b.metrics.ScriptExecutionErrorOnExecutionNode()
			b.log.Error().Err(errToReturn).Msg("script execution failed for execution node internal reasons")
		}
		return nil, execDuration, rpc.ConvertError(errToReturn, "failed to execute script on execution nodes", codes.Internal)
	}

	return result, execDuration, nil
}

// tryExecuteScriptOnExecutionNode attempts to execute the script on the given execution node.
func (b *backendScripts) tryExecuteScriptOnExecutionNode(
	ctx context.Context,
	executorAddress string,
	r *scriptExecutionRequest,
) ([]byte, error) {
	execRPCClient, closer, err := b.connFactory.GetExecutionAPIClient(executorAddress)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create client for execution node %s: %v",
			executorAddress, err)
	}
	defer closer.Close()

	execResp, err := execRPCClient.ExecuteScriptAtBlockID(ctx, &execproto.ExecuteScriptAtBlockIDRequest{
		BlockId:   r.blockID[:],
		Script:    r.script,
		Arguments: r.arguments,
	})
	if err != nil {
		return nil, status.Errorf(status.Code(err), "failed to execute the script on the execution node %s: %v", executorAddress, err)
	}
	return execResp.GetValue(), nil
}

// isInvalidArgumentError checks if the error is from an invalid argument
func isInvalidArgumentError(scriptExecutionErr error) bool {
	return status.Code(scriptExecutionErr) == codes.InvalidArgument
}

// shouldLogScript checks if the script hash is unique in the time window
func (b *backendScripts) shouldLogScript(execTime time.Time, scriptHash [md5.Size]byte) bool {
	timestamp, seen := b.loggedScripts.Get(scriptHash)
	if seen {
		return execTime.Sub(timestamp) >= uniqueScriptLoggingTimeWindow
	}
	return true
}

// convertScriptExecutionError converts the script execution error to a gRPC error
func convertScriptExecutionError(err error, height uint64) error {
	if err == nil {
		return nil
	}

	var coded fvmerrors.CodedError
	if fvmerrors.As(err, &coded) {
		// general FVM/ledger errors
		if coded.Code().IsFailure() {
			return rpc.ConvertError(err, "failed to execute script", codes.Internal)
		}

		switch coded.Code() {
		case fvmerrors.ErrCodeScriptExecutionCancelledError:
			return status.Errorf(codes.Canceled, "script execution canceled: %v", err)

		case fvmerrors.ErrCodeScriptExecutionTimedOutError:
			return status.Errorf(codes.DeadlineExceeded, "script execution timed out: %v", err)

		default:
			// runtime errors
			return status.Errorf(codes.InvalidArgument, "failed to execute script: %v", err)
		}
	}

	return convertIndexError(err, height, "failed to execute script")
}

// convertIndexError converts errors related to index to a gRPC error
func convertIndexError(err error, height uint64, defaultMsg string) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, execution.ErrDataNotAvailable) {
		return status.Errorf(codes.OutOfRange, "data for block height %d is not available", height)
	}

	if errors.Is(err, storage.ErrNotFound) {
		return status.Errorf(codes.NotFound, "data not found: %v", err)
	}

	return rpc.ConvertError(err, defaultMsg, codes.Internal)
}
