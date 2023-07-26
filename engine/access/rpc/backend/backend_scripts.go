package backend

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec
	"io"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/onflow/flow/protobuf/go/flow/access"

	"github.com/hashicorp/go-multierror"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// uniqueScriptLoggingTimeWindow is the duration for checking the uniqueness of scripts sent for execution
const uniqueScriptLoggingTimeWindow = 10 * time.Minute

type backendScripts struct {
	headers              storage.Headers
	executionReceipts    storage.ExecutionReceipts
	state                protocol.State
	connFactory          ConnectionFactory
	log                  zerolog.Logger
	metrics              module.BackendScriptsMetrics
	loggedScripts        *lru.Cache
	archiveAddressList   []string
	archivePorts         []uint
	scriptExecValidation bool
}

func (b *backendScripts) ExecuteScriptAtLatestBlock(
	ctx context.Context,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {

	// get the latest sealed header
	latestHeader, err := b.state.Sealed().Head()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get latest sealed header: %v", err)
	}

	// get the block id of the latest sealed header
	latestBlockID := latestHeader.ID()

	// execute script on the execution node at that block id
	return b.executeScriptOnExecutor(ctx, latestBlockID, script, arguments)
}

func (b *backendScripts) ExecuteScriptAtBlockID(
	ctx context.Context,
	blockID flow.Identifier,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {
	// execute script on the execution node at that block id
	return b.executeScriptOnExecutor(ctx, blockID, script, arguments)
}

func (b *backendScripts) ExecuteScriptAtBlockHeight(
	ctx context.Context,
	blockHeight uint64,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {
	// get header at given height
	header, err := b.headers.ByHeight(blockHeight)
	if err != nil {
		err = rpc.ConvertStorageError(err)
		return nil, err
	}

	blockID := header.ID()

	// execute script on the execution node at that block id
	return b.executeScriptOnExecutor(ctx, blockID, script, arguments)
}

func (b *backendScripts) findScriptExecutors(
	ctx context.Context,
	blockID flow.Identifier,
) ([]string, error) {
	executors, err := executionNodesForBlockID(ctx, blockID, b.executionReceipts, b.state, b.log)
	if err != nil {
		return nil, err
	}
	executorAddrs := make([]string, 0, len(executors))
	for _, executor := range executors {
		executorAddrs = append(executorAddrs, executor.Address)
	}
	return executorAddrs, nil
}

// executeScriptOnExecutionNode forwards the request to the execution node using the execution node
// grpc client and converts the response back to the access node api response format
func (b *backendScripts) executeScriptOnExecutor(
	ctx context.Context,
	blockID flow.Identifier,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {
	// encode to MD5 as low compute/memory lookup key
	// CAUTION: cryptographically insecure md5 is used here, but only to de-duplicate logs.
	// *DO NOT* use this hash for any protocol-related or cryptographic functions.
	insecureScriptHash := md5.Sum(script) //nolint:gosec
	// try execution on Archive nodes first
	archiveResult, err := b.executeScriptOnAvailableArchiveNodes(ctx, blockID, script, arguments, insecureScriptHash)
	// try execution nodes if the script wasn't executed
	b.log.Info().Bool("validation mode", b.scriptExecValidation).Msg("running script execution strategy")
	if b.scriptExecValidation {
		execNodeResult, errExec := b.executeScriptOnAvailableExecutionNodes(
			ctx, blockID, script, arguments, insecureScriptHash)
		b.log.Debug().Hex("archive result", archiveResult).Msg("got archive node response for comparison")
		b.log.Debug().Hex("exec node result", execNodeResult).Msg("got execution node response for comparison")
		if bytes.Equal(execNodeResult, archiveResult) && errExec == err {
			b.logScriptExecutionComparison(blockID, insecureScriptHash,
				"script execution results on Archive node and EN are equal")
		} else {
			b.logScriptExecutionComparison(blockID, insecureScriptHash,
				"script execution results on Archive node and EN are not equal")
		}
		// return EN results by default
		return execNodeResult, errExec
	} else {
		// execute on execution nodes if it's not a script error
		if err != nil && status.Code(err) != codes.InvalidArgument {
			execNodeResult, err := b.executeScriptOnAvailableExecutionNodes(
				ctx, blockID, script, arguments, insecureScriptHash)
			return execNodeResult, err
		}
		return archiveResult, err
	}
}

func (b *backendScripts) logScriptExecutionComparison(
	blockID flow.Identifier,
	insecureScriptHash [16]byte,
	msg string,
) {
	b.log.Info().Hex("block_id", blockID[:]).Hex("script_hash", insecureScriptHash[:]).Msg(msg)
}

// executeScriptOnAvailableArchiveNodes executes the given script for a blockID on all archive nodes available
func (b *backendScripts) executeScriptOnAvailableArchiveNodes(
	ctx context.Context,
	blockID flow.Identifier,
	script []byte,
	arguments [][]byte,
	insecureScriptHash [16]byte,
) ([]byte, error) {
	var errors *multierror.Error
	if len(b.archiveAddressList) > 0 {
		startTime := time.Now()
		for idx, rnAddr := range b.archiveAddressList {
			rnPort := b.archivePorts[idx]
			result, err := b.tryExecuteScriptOnArchiveNode(ctx, rnAddr, rnPort, blockID, script, arguments)
			if err == nil {
				// log execution time
				b.metrics.ScriptExecuted(
					time.Since(startTime),
					len(script),
				)
				return result, nil
			} else {
				errCode := status.Code(err)
				switch errCode {
				case codes.InvalidArgument:
					// failure due to cadence script, no need to query further
					b.log.Debug().Err(err).
						Str("script_executor_addr", rnAddr).
						Hex("block_id", blockID[:]).
						Hex("script_hash", insecureScriptHash[:]).
						Str("script", string(script)).
						Msg("script failed to execute on the execution node")
					return nil, err
				case codes.NotFound:
					// failures due to unavailable blocks are explicitly marked Not found
					b.metrics.ScriptExecutionErrorOnArchiveNode()
					b.log.Error().Err(err).Msg("script execution failed for archive node")
				default:
					continue
				}
				errors = multierror.Append(errors, err)
			}
		}
	}
	// don't need to distinguish error codes at this point
	return nil, rpc.ConvertMultiError(errors, "failed to execute script on archive nodes", codes.Internal)
}

func (b *backendScripts) executeScriptOnAvailableExecutionNodes(
	ctx context.Context,
	blockID flow.Identifier,
	script []byte,
	arguments [][]byte,
	insecureScriptHash [16]byte,
) ([]byte, error) {
	// find few execution nodes which have executed the block earlier and provided an execution receipt for it
	scriptExecutors, err := b.findScriptExecutors(ctx, blockID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to find script executors at blockId %v: %v",
			blockID.String(), err)
	}
	var errors *multierror.Error
	for _, executorAddress := range scriptExecutors {
		execStartTime := time.Now() // record start time
		result, err := b.tryExecuteScriptOnExecutionNode(ctx, executorAddress, blockID, script, arguments)
		if err == nil {
			if b.log.GetLevel() == zerolog.DebugLevel {
				executionTime := time.Now()
				if b.shouldLogScript(executionTime, insecureScriptHash) {
					b.log.Debug().
						Str("script_executor_addr", executorAddress).
						Hex("block_id", blockID[:]).
						Hex("script_hash", insecureScriptHash[:]).
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
			return result, nil
		}
		// return if it's just a script failure as opposed to an EN/RN failure and skip trying other ENs/RNs
		if status.Code(err) == codes.InvalidArgument {
			b.log.Debug().Err(err).
				Str("script_executor_addr", executorAddress).
				Hex("block_id", blockID[:]).
				Hex("script_hash", insecureScriptHash[:]).
				Str("script", string(script)).
				Msg("script failed to execute on the execution node")
			return nil, err
		}
		errors = multierror.Append(errors, err)
	}

	errToReturn := errors.ErrorOrNil()
	if errToReturn != nil {
		b.metrics.ScriptExecutionErrorOnExecutionNode()
		b.log.Error().Err(err).Msg("script execution failed for execution node internal reasons")
	}

	return nil, rpc.ConvertMultiError(errors, "failed to execute script on execution nodes", codes.Internal)
}

// shouldLogScript checks if the script hash is unique in the time window
func (b *backendScripts) shouldLogScript(execTime time.Time, scriptHash [16]byte) bool {
	rawTimestamp, seen := b.loggedScripts.Get(scriptHash)
	if !seen || rawTimestamp == nil {
		return true
	} else {
		// safe cast
		timestamp := rawTimestamp.(time.Time)
		return execTime.Sub(timestamp) >= uniqueScriptLoggingTimeWindow
	}
}

func (b *backendScripts) tryExecuteScriptOnExecutionNode(
	ctx context.Context,
	executorAddress string,
	blockID flow.Identifier,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {
	req := &execproto.ExecuteScriptAtBlockIDRequest{
		BlockId:   blockID[:],
		Script:    script,
		Arguments: arguments,
	}
	execRPCClient, closer, err := b.connFactory.GetExecutionAPIClient(executorAddress)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create client for execution node %s: %v",
			executorAddress, err)
	}
	defer func(closer io.Closer) {
		err := closer.Close()
		if err != nil {
			b.log.Error().Err(err).Msg("failed to close execution client")
		}
	}(closer)

	execResp, err := execRPCClient.ExecuteScriptAtBlockID(ctx, req)
	if err != nil {
		if status.Code(err) == codes.Unavailable {
			b.connFactory.InvalidateExecutionAPIClient(executorAddress)
		}
		return nil, status.Errorf(status.Code(err), "failed to execute the script on the execution node %s: %v",
			executorAddress, err)
	}
	return execResp.GetValue(), nil
}

func (b *backendScripts) tryExecuteScriptOnArchiveNode(
	ctx context.Context,
	executorAddress string,
	port uint,
	blockID flow.Identifier,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {
	req := &access.ExecuteScriptAtBlockIDRequest{
		BlockId:   blockID[:],
		Script:    script,
		Arguments: arguments,
	}

	archiveClient, closer, err := b.connFactory.GetAccessAPIClientWithPort(executorAddress, port)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create client for archive node %s: %v",
			executorAddress, err)
	}
	defer func(closer io.Closer) {
		err := closer.Close()
		if err != nil {
			b.log.Error().Err(err).Msg("failed to close archive client")
		}
	}(closer)
	resp, err := archiveClient.ExecuteScriptAtBlockID(ctx, req)
	if err != nil {
		if status.Code(err) == codes.Unavailable {
			b.connFactory.InvalidateAccessAPIClient(executorAddress)
		}
		return nil, status.Errorf(status.Code(err), "failed to execute the script on archive node %s: %v",
			executorAddress, err)
	}
	return resp.GetValue(), nil
}
