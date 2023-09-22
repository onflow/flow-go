package backend

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec
	"io"
	"time"

	"github.com/hashicorp/go-multierror"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/onflow/flow/protobuf/go/flow/access"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/rpc/connection"
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
	connFactory          connection.ConnectionFactory
	log                  zerolog.Logger
	metrics              module.BackendScriptsMetrics
	loggedScripts        *lru.Cache[[md5.Size]byte, time.Time]
	archiveAddressList   []string
	archivePorts         []uint
	scriptExecValidation bool
	nodeCommunicator     Communicator
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

func isCadenceScriptError(scriptExecutionErr error) bool {
	return scriptExecutionErr == nil || status.Code(scriptExecutionErr) == codes.InvalidArgument
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
	// try execution nodes if the script wasn't executed
	if b.scriptExecValidation {
		execNodeResult, execErr := b.executeScriptOnAvailableExecutionNodes(
			ctx, blockID, script, arguments, insecureScriptHash)
		// we can only compare the results if there are no errors from RN and EN
		// since we cannot distinguish the EN error as caused by the block being pruned or some other reason,
		// which may produce a valid RN output but an error for the EN
		if isCadenceScriptError(execErr) {
			archiveResult, archiveErr := b.executeScriptOnAvailableArchiveNodes(ctx, blockID, script, arguments,
				insecureScriptHash)
			// return EN results by default
			b.compareScriptExecutionResults(execNodeResult, execErr, archiveResult, archiveErr, blockID,
				script)
			return execNodeResult, execErr
		}
		return execNodeResult, execErr
	}
	archiveResult, archiveErr := b.executeScriptOnAvailableArchiveNodes(ctx, blockID, script, arguments,
		insecureScriptHash)
	// execute on execution nodes if it's not a script error
	if !isCadenceScriptError(archiveErr) {
		execNodeResult, err := b.executeScriptOnAvailableExecutionNodes(
			ctx, blockID, script, arguments, insecureScriptHash)
		return execNodeResult, err
	}
	return archiveResult, archiveErr
}

func (b *backendScripts) compareScriptExecutionResults(
	execNodeResult []byte,
	execErr error,
	archiveResult []byte,
	archiveErr error,
	blockID flow.Identifier,
	script []byte,
) {
	// check errors first
	if execErr != nil {
		if archiveErr != nil && execErr == archiveErr {
			b.metrics.ScriptExecutionErrorMatch()
		} else {
			b.metrics.ScriptExecutionErrorMismatch()
			b.logScriptExecutionComparison(blockID, script, execNodeResult, archiveResult, execErr, archiveErr,
				"cadence errors on Archive node and EN are not equal")
		}
		return
	}
	if bytes.Equal(execNodeResult, archiveResult) {
		b.metrics.ScriptExecutionResultMatch()
	} else {
		b.metrics.ScriptExecutionResultMismatch()
		b.logScriptExecutionComparison(blockID, script, execNodeResult, archiveResult, execErr, archiveErr,
			"script execution results on Archive node and EN are not equal")
	}
}

func (b *backendScripts) logScriptExecutionComparison(
	blockID flow.Identifier,
	script []byte,
	execNodeResult []byte,
	archiveResult []byte,
	executionError error,
	archiveError error,
	msg string,
) {
	// over-log for ease of debug
	if executionError != nil || archiveError != nil {
		b.log.Debug().Hex("block_id", blockID[:]).
			Str("script", string(script)).
			AnErr("execution_node_error", executionError).
			AnErr("archive_node_error", archiveError).
			Msg(msg)
	} else {
		b.log.Debug().Hex("block_id", blockID[:]).
			Str("script", string(script)).
			Hex("execution_node_result", execNodeResult).
			Hex("archive_node_result", archiveResult).
			Msg(msg)
	}
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
					// failures due to unavailable blocks  are explicitly marked Not found
					b.metrics.ScriptExecutionErrorOnArchiveNode()
					b.log.Error().Err(err).Msg("script execution failed for archive node")
					return nil, err
				default:
					errors = multierror.Append(errors, err)
					continue
				}
			}
		}
	}
	// don't need to distinguish error codes at this point
	if errors.ErrorOrNil() != nil {
		return nil, rpc.ConvertMultiError(errors, "failed to execute script on archive nodes", codes.Internal)
	}
	return nil, status.Errorf(codes.Unavailable, "no archive nodes in address list")
}

func (b *backendScripts) executeScriptOnAvailableExecutionNodes(
	ctx context.Context,
	blockID flow.Identifier,
	script []byte,
	arguments [][]byte,
	insecureScriptHash [16]byte,
) ([]byte, error) {
	// find few execution nodes which have executed the block earlier and provided an execution receipt for it
	executors, err := executionNodesForBlockID(ctx, blockID, b.executionReceipts, b.state, b.log)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to find script executors at blockId %v: %v", blockID.String(), err)
	}
	var result []byte
	hasInvalidArgument := false
	errToReturn := b.nodeCommunicator.CallAvailableNode(
		executors,
		func(node *flow.Identity) error {
			execStartTime := time.Now()
			result, err = b.tryExecuteScriptOnExecutionNode(ctx, node.Address, blockID, script, arguments)
			if err == nil {
				if b.log.GetLevel() == zerolog.DebugLevel {
					executionTime := time.Now()
					if b.shouldLogScript(executionTime, insecureScriptHash) {
						b.log.Debug().
							Str("script_executor_addr", node.Address).
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

				return nil
			}

			return err
		},
		func(node *flow.Identity, err error) bool {
			hasInvalidArgument = status.Code(err) == codes.InvalidArgument
			if hasInvalidArgument {
				b.log.Debug().Err(err).
					Str("script_executor_addr", node.Address).
					Hex("block_id", blockID[:]).
					Hex("script_hash", insecureScriptHash[:]).
					Str("script", string(script)).
					Msg("script failed to execute on the execution node")
			}
			return hasInvalidArgument
		},
	)

	if hasInvalidArgument {
		return nil, errToReturn
	}

	if errToReturn == nil {
		return result, nil
	} else {
		b.metrics.ScriptExecutionErrorOnExecutionNode()
		b.log.Error().Err(err).Msg("script execution failed for execution node internal reasons")
		return nil, rpc.ConvertError(errToReturn, "failed to execute script on execution nodes", codes.Internal)
	}
}

// shouldLogScript checks if the script hash is unique in the time window
func (b *backendScripts) shouldLogScript(execTime time.Time, scriptHash [md5.Size]byte) bool {
	timestamp, seen := b.loggedScripts.Get(scriptHash)
	if seen {
		// safe cast
		return execTime.Sub(timestamp) >= uniqueScriptLoggingTimeWindow
	}
	return true

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
		return nil, status.Errorf(status.Code(err), "failed to execute the script on the execution node %s: %v", executorAddress, err)
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
		return nil, status.Errorf(status.Code(err), "failed to execute the script on archive node %s: %v",
			executorAddress, err)
	}
	return resp.GetValue(), nil
}
