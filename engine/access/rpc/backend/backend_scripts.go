package backend

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec
	"time"

	lru "github.com/hashicorp/golang-lru"
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
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
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
	loggedScripts     *lru.Cache
	nodeCommunicator  Communicator
	indexer           indexer.IndexReporter
	scriptExecutor    *execution.Scripts
	scriptExecMode    ScriptExecutionMode
}

func (b *backendScripts) ExecuteScriptAtLatestBlock(
	ctx context.Context,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {
	latestHeader, err := b.state.Sealed().Head()
	if err != nil {
		// the latest sealed header MUST be available
		return nil, status.Errorf(codes.Internal, "failed to get latest sealed header: %v", err)
	}

	return b.executeScript(ctx, latestHeader.ID(), latestHeader.Height, script, arguments)
}

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

	return b.executeScript(ctx, blockID, header.Height, script, arguments)
}

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

	return b.executeScript(ctx, header.ID(), blockHeight, script, arguments)
}

func isCadenceScriptError(scriptExecutionErr error) bool {
	return scriptExecutionErr == nil || status.Code(scriptExecutionErr) == codes.InvalidArgument
}

// executeScriptOnExecutionNode forwards the request to the execution node using the execution node
// grpc client and converts the response back to the access node api response format
func (b *backendScripts) executeScript(
	ctx context.Context,
	blockID flow.Identifier,
	height uint64,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {
	// encode to MD5 as low compute/memory lookup key
	// CAUTION: cryptographically insecure md5 is used here, but only to de-duplicate logs.
	// *DO NOT* use this hash for any protocol-related or cryptographic functions.
	insecureScriptHash := md5.Sum(script) //nolint:gosec

	if b.scriptExecMode == ScriptExecutionModeExecutionNodesOnly {
		return b.executeScriptOnAvailableExecutionNodes(ctx, blockID, script, arguments, insecureScriptHash)
	}

	localResult, localErr := b.executeScriptLocally(ctx, blockID, height, script, arguments, insecureScriptHash)
	switch b.scriptExecMode {
	case ScriptExecutionModeFailover:
		if localErr == nil || isCadenceScriptError(localErr) {
			return localResult, localErr
		}
		return b.executeScriptOnAvailableExecutionNodes(ctx, blockID, script, arguments, insecureScriptHash)

	case ScriptExecutionModeCompare:
		execResult, execErr := b.executeScriptOnAvailableExecutionNodes(ctx, blockID, script, arguments, insecureScriptHash)
		// we can only compare the results if there were either no errors or a cadence error
		// since we cannot distinguish the EN error as caused by the block being pruned or some other reason,
		// which may produce a valid RN output but an error for the EN
		if execErr != nil && !isCadenceScriptError(execErr) {
			return nil, execErr
		}

		b.compareScriptExecutionResults(execResult, execErr, localResult, localErr, blockID, script, insecureScriptHash)

		// always return EN results
		return execResult, execErr

	default:
		return localResult, localErr
	}
}

func (b *backendScripts) compareScriptExecutionResults(
	execNodeResult []byte,
	execErr error,
	localResult []byte,
	localErr error,
	blockID flow.Identifier,
	script []byte,
	insecureScriptHash [md5.Size]byte,
) {
	// check errors first
	if execErr != nil {
		if execErr == localErr {
			b.metrics.ScriptExecutionErrorMatch()
		} else {
			b.metrics.ScriptExecutionErrorMismatch()
			b.logScriptExecutionComparison(blockID, script, insecureScriptHash, execNodeResult, localResult, execErr, localErr,
				"cadence errors from local execution do not match and EN")
		}
		return
	}

	if bytes.Equal(execNodeResult, localResult) {
		b.metrics.ScriptExecutionResultMatch()
	} else {
		b.metrics.ScriptExecutionResultMismatch()
		b.logScriptExecutionComparison(blockID, script, insecureScriptHash, execNodeResult, localResult, execErr, localErr,
			"script execution results from local execution do not match EN")
	}
}

func (b *backendScripts) logScriptExecutionComparison(
	blockID flow.Identifier,
	script []byte,
	insecureScriptHash [md5.Size]byte,
	execNodeResult []byte,
	localResult []byte,
	executionError error,
	localError error,
	msg string,
) {
	// over-log for ease of debug
	if executionError != nil || localError != nil {
		b.log.Debug().
			Hex("block_id", blockID[:]).
			Str("script", string(script)).
			Hex("script_hash", insecureScriptHash[:]).
			AnErr("execution_node_error", executionError).
			AnErr("local_error", localError).
			Msg(msg)
	} else {
		b.log.Debug().
			Hex("block_id", blockID[:]).
			Str("script", string(script)).
			Hex("script_hash", insecureScriptHash[:]).
			Hex("execution_node_result", execNodeResult).
			Hex("local_result", localResult).
			Msg(msg)
	}
}

func (b *backendScripts) executeScriptLocally(
	ctx context.Context,
	blockID flow.Identifier,
	height uint64,
	script []byte,
	arguments [][]byte,
	insecureScriptHash [md5.Size]byte,
) ([]byte, error) {
	// make sure data is available for the requested block
	if height < b.indexer.LowestIndexedHeight() || height > b.indexer.HighestIndexedHeight() {
		return nil, status.Errorf(codes.OutOfRange, "block height %d is not indexed. have range [%d, %d]",
			height,
			b.indexer.LowestIndexedHeight(),
			b.indexer.HighestIndexedHeight(),
		)
	}

	lg := b.log.With().
		Str("script_executor_addr", "localhost").
		Hex("block_id", logging.ID(blockID)).
		Hex("script_hash", insecureScriptHash[:]).
		Logger()

	execStartTime := time.Now()

	result, err := b.scriptExecutor.ExecuteAtBlockHeight(ctx, script, arguments, height)

	execEndTime := time.Now()
	execDuration := execEndTime.Sub(execStartTime)

	if err != nil {
		convertedErr := convertScriptExecutionError(err)

		if status.Code(convertedErr) == codes.InvalidArgument {
			lg.Debug().Err(err).
				Str("script", string(script)).
				Dur("execution_dur_ms", execDuration).
				Msg("script failed to execute locally")
			return nil, convertedErr
		}

		// TODO: metrics
		lg.Error().Err(err).Msg("script execution failed for internal reasons")
		return nil, convertedErr
	}

	if b.log.GetLevel() == zerolog.DebugLevel {
		if b.shouldLogScript(execEndTime, insecureScriptHash) {
			lg.Debug().
				Str("script", string(script)).
				Dur("execution_dur_ms", execDuration).
				Msg("Successfully executed script")
			b.loggedScripts.Add(insecureScriptHash, execEndTime)
		}
	}

	// log execution time
	b.metrics.ScriptExecuted(execEndTime.Sub(execStartTime), len(script))

	return result, nil
}

func (b *backendScripts) executeScriptOnAvailableExecutionNodes(
	ctx context.Context,
	blockID flow.Identifier,
	script []byte,
	arguments [][]byte,
	insecureScriptHash [md5.Size]byte,
) ([]byte, error) {
	// find few execution nodes which have executed the block earlier and provided an execution receipt for it
	executors, err := executionNodesForBlockID(ctx, blockID, b.executionReceipts, b.state, b.log)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to find script executors at blockId %v: %v", blockID.String(), err)
	}

	lg := b.log.With().
		Hex("block_id", logging.ID(blockID)).
		Hex("script_hash", insecureScriptHash[:]).
		Logger()

	var result []byte
	errToReturn := b.nodeCommunicator.CallAvailableNode(
		executors,
		func(node *flow.Identity) error {
			execStartTime := time.Now()
			result, err = b.tryExecuteScriptOnExecutionNode(ctx, node.Address, blockID, script, arguments)
			if err == nil {
				if b.log.GetLevel() == zerolog.DebugLevel {
					executionTime := time.Now()
					if b.shouldLogScript(executionTime, insecureScriptHash) {
						lg.Debug().
							Str("script_executor_addr", node.Address).
							Str("script", string(script)).
							Msg("Successfully executed script")
						b.loggedScripts.Add(insecureScriptHash, executionTime)
					}
				}

				// log execution time
				b.metrics.ScriptExecuted(
					time.Since(execStartTime),
					len(script),
				)

				return nil
			}

			return err
		},
		func(node *flow.Identity, err error) bool {
			if status.Code(err) == codes.InvalidArgument {
				lg.Debug().Err(err).
					Str("script_executor_addr", node.Address).
					Str("script", string(script)).
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
		return nil, rpc.ConvertError(errToReturn, "failed to execute script on execution nodes", codes.Internal)
	}

	return result, nil
}

func (b *backendScripts) tryExecuteScriptOnExecutionNode(
	ctx context.Context,
	executorAddress string,
	blockID flow.Identifier,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {
	execRPCClient, closer, err := b.connFactory.GetExecutionAPIClient(executorAddress)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create client for execution node %s: %v",
			executorAddress, err)
	}
	defer closer.Close()

	execResp, err := execRPCClient.ExecuteScriptAtBlockID(ctx, &execproto.ExecuteScriptAtBlockIDRequest{
		BlockId:   blockID[:],
		Script:    script,
		Arguments: arguments,
	})
	if err != nil {
		return nil, status.Errorf(status.Code(err), "failed to execute the script on the execution node %s: %v", executorAddress, err)
	}
	return execResp.GetValue(), nil
}

// shouldLogScript checks if the script hash is unique in the time window
func (b *backendScripts) shouldLogScript(execTime time.Time, scriptHash [md5.Size]byte) bool {
	rawTimestamp, seen := b.loggedScripts.Get(scriptHash)
	if !seen || rawTimestamp == nil {
		return true
	}

	// safe cast
	timestamp := rawTimestamp.(time.Time)
	return execTime.Sub(timestamp) >= uniqueScriptLoggingTimeWindow
}

func convertScriptExecutionError(err error) error {
	if err == nil {
		return nil
	}

	var coded fvmerrors.CodedError
	if fvmerrors.As(err, &coded) {
		// general FVM/ledger errors
		if coded.Code().IsFailure() {
			return rpc.ConvertError(err, "failed to execute script", codes.Internal)
		}

		// runtime errors
		return status.Errorf(codes.InvalidArgument, "failed to execute script: %v", err)
	}

	return rpc.ConvertError(err, "failed to execute script", codes.Internal)
}
