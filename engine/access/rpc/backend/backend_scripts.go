package backend

import (
	"context"
	"crypto/md5" //nolint:gosec
	"time"

	lru "github.com/hashicorp/golang-lru"

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
	headers            storage.Headers
	executionReceipts  storage.ExecutionReceipts
	state              protocol.State
	connFactory        ConnectionFactory
	log                zerolog.Logger
	metrics            module.BackendScriptsMetrics
	loggedScripts      *lru.Cache
	archiveAddressList []string
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
	return b.executeScriptOnExecutionNode(ctx, latestBlockID, script, arguments)
}

func (b *backendScripts) ExecuteScriptAtBlockID(
	ctx context.Context,
	blockID flow.Identifier,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {
	// execute script on the execution node at that block id
	return b.executeScriptOnExecutionNode(ctx, blockID, script, arguments)
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
	return b.executeScriptOnExecutionNode(ctx, blockID, script, arguments)
}

func (b *backendScripts) findScriptExecutors(
	ctx context.Context,
	blockID flow.Identifier,
) ([]string, error) {
	// send script queries to archive nodes if archive addres is configured
	if len(b.archiveAddressList) > 0 {
		return b.archiveAddressList, nil
	}

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
func (b *backendScripts) executeScriptOnExecutionNode(
	ctx context.Context,
	blockID flow.Identifier,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {

	execReq := &execproto.ExecuteScriptAtBlockIDRequest{
		BlockId:   blockID[:],
		Script:    script,
		Arguments: arguments,
	}

	// find few execution nodes which have executed the block earlier and provided an execution receipt for it
	scriptExecutors, err := b.findScriptExecutors(ctx, blockID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to find script executors at blockId %v: %v", blockID.String(), err)
	}
	// encode to MD5 as low compute/memory lookup key
	// CAUTION: cryptographically insecure md5 is used here, but only to de-duplicate logs.
	// *DO NOT* use this hash for any protocol-related or cryptographic functions.
	insecureScriptHash := md5.Sum(script) //nolint:gosec

	// try each of the execution nodes found
	var errors *multierror.Error
	// try to execute the script on one of the execution nodes
	for _, executorAddress := range scriptExecutors {
		execStartTime := time.Now() // record start time
		result, err := b.tryExecuteScript(ctx, executorAddress, execReq)
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
		// return if it's just a script failure as opposed to an EN failure and skip trying other ENs
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
		b.log.Error().Err(errToReturn).Msg("script execution failed for execution node internal reasons")
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

func (b *backendScripts) tryExecuteScript(ctx context.Context, executorAddress string, req *execproto.ExecuteScriptAtBlockIDRequest) ([]byte, error) {
	execRPCClient, closer, err := b.connFactory.GetExecutionAPIClient(executorAddress)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create client for execution node %s: %v", executorAddress, err)
	}
	defer closer.Close()

	execResp, err := execRPCClient.ExecuteScriptAtBlockID(ctx, req)
	if err != nil {
		if status.Code(err) == codes.Unavailable {
			b.connFactory.InvalidateExecutionAPIClient(executorAddress)
		}
		return nil, status.Errorf(status.Code(err), "failed to execute the script on the execution node %s: %v", executorAddress, err)
	}
	return execResp.GetValue(), nil
}
