package backend

import (
	"context"

	"github.com/hashicorp/go-multierror"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type backendScripts struct {
	headers           storage.Headers
	executionReceipts storage.ExecutionReceipts
	state             protocol.State
	connFactory       ConnectionFactory
	log               zerolog.Logger
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
		return nil, status.Errorf(codes.Internal, "failed to execute the script on the execution node: %v", err)
	}

	// try each of the execution nodes found
	var errors *multierror.Error
	// try to execute the script on one of the execution nodes
	for _, execNode := range execNodes {
		result, err := b.tryExecuteScript(ctx, execNode, execReq)
		if err == nil {
			b.log.Debug().
				Str("execution_node", execNode.String()).
				Hex("block_id", blockID[:]).
				Str("script", string(script)).
				Msg("Successfully executed script")
			return result, nil
		}
		errors = multierror.Append(errors, err)
	}
	return nil, errors.ErrorOrNil()
}

func (b *backendScripts) tryExecuteScript(ctx context.Context, execNode *flow.Identity, req execproto.ExecuteScriptAtBlockIDRequest) ([]byte, error) {
	execRPCClient, closer, err := b.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to execute the script on the execution node %s: %v", execNode.String(), err)
	}
	defer closer.Close()
	execResp, err := execRPCClient.ExecuteScriptAtBlockID(ctx, &req)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to execute the script on the execution node %s: %v", execNode.String(), err)
	}
	return execResp.GetValue(), nil
}
