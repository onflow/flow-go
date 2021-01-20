package backend

import (
	"context"

	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

type backendScripts struct {
	logger       zerolog.Logger
	headers      storage.Headers
	state        protocol.State
	executionRPC execproto.ExecutionAPIClient
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

	b.logger.Debug().
		Hex("block_id", logging.ID(blockID)).
		Str("script", string(script)).
		Msg("an script is sent for execution")

	execReq := execproto.ExecuteScriptAtBlockIDRequest{
		BlockId:   blockID[:],
		Script:    script,
		Arguments: arguments,
	}

	execResp, err := b.executionRPC.ExecuteScriptAtBlockID(ctx, &execReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to execute the script on the execution node: %v", err)
	}

	return execResp.GetValue(), nil
}
