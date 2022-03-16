package backend

import (
	"context"
	"crypto/md5" //nolint:gosec
	"time"

	"github.com/hashicorp/go-multierror"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// uniqueScriptLoggingTimeWindow is the duration for checking the uniqueness of scripts sent for execution
const uniqueScriptLoggingTimeWindow = 10 * time.Minute

type backendScripts struct {
	headers           storage.Headers
	executionReceipts storage.ExecutionReceipts
	state             protocol.State
	connFactory       ConnectionFactory
	log               zerolog.Logger
	seenScripts       map[[md5.Size]byte]time.Time // to keep track of unique scripts sent by clients. bounded to 1MB (2^16*2*8) due to fixed key size
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
		err = convertStorageError(err)
		return nil, err
	}

	blockID := header.ID()

	// execute script on the execution node at that block id
	return b.executeScriptOnExecutionNode(ctx, blockID, script, arguments)
}

// executeScriptOnExecutionNode forwards the request to the execution node using the execution node
// grpc client and converts the response back to the access node api response format
func (b *backendScripts) executeScriptOnExecutionNode(
	ctx context.Context,
	blockID flow.Identifier,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {

	execReq := execproto.ExecuteScriptAtBlockIDRequest{
		BlockId:   blockID[:],
		Script:    script,
		Arguments: arguments,
	}

	// find few execution nodes which have executed the block earlier and provided an execution receipt for it
	execNodes, err := executionNodesForBlockID(ctx, blockID, b.executionReceipts, b.state, b.log)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to find execution nodes at blockId %v: %v", blockID.String(), err)
	}
	// encode to MD5 as low compute/memory lookup key
	encodedScript := md5.Sum(script) //nolint:gosec

	// try each of the execution nodes found
	var errors *multierror.Error
	// try to execute the script on one of the execution nodes
	for _, execNode := range execNodes {
		result, err := b.tryExecuteScript(ctx, execNode, execReq)
		if err == nil {
			if b.log.GetLevel() == zerolog.DebugLevel {
				executionTime := time.Now()
				timestamp, seen := b.seenScripts[encodedScript]
				// log if the script is unique in the time window
				if !seen || executionTime.Sub(timestamp) >= uniqueScriptLoggingTimeWindow {
					b.log.Debug().
						Str("execution_node", execNode.String()).
						Hex("block_id", blockID[:]).
						Hex("script_hash", encodedScript[:]).
						Str("script", string(script)).
						Msg("Successfully executed script")
					b.seenScripts[encodedScript] = executionTime
				}
			}
			return result, nil
		}
		// return if it's just a script failure as opposed to an EN failure and skip trying other ENs
		if status.Code(err) == codes.InvalidArgument {
			b.log.Debug().Err(err).
				Str("execution_node", execNode.String()).
				Hex("block_id", blockID[:]).
				Hex("script_hash", encodedScript[:]).
				Str("script", string(script)).
				Msg("script failed to execute on the execution node")
			return nil, err
		}
		errors = multierror.Append(errors, err)
	}
	errToReturn := errors.ErrorOrNil()
	if errToReturn != nil {
		b.log.Error().Err(err).Msg("script execution failed for execution node internal reasons")
	}
	return nil, errToReturn
}

func (b *backendScripts) tryExecuteScript(ctx context.Context, execNode *flow.Identity, req execproto.ExecuteScriptAtBlockIDRequest) ([]byte, error) {
	execRPCClient, closer, err := b.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create client for execution node %s: %v", execNode.String(), err)
	}
	defer closer.Close()
	execResp, err := execRPCClient.ExecuteScriptAtBlockID(ctx, &req)
	if err != nil {
		return nil, status.Errorf(status.Code(err), "failed to execute the script on the execution node %s: %v", execNode.String(), err)
	}
	return execResp.GetValue(), nil
}
