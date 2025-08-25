package debug

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/execution"

	rpcConvert "github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
)

func GetExecutionAPIBlockHeader(
	client execution.ExecutionAPIClient,
	ctx context.Context,
	blockID flow.Identifier,
) (
	*flow.Header,
	error,
) {
	req := &execution.GetBlockHeaderByIDRequest{
		Id: blockID[:],
	}

	resp, err := client.GetBlockHeaderByID(ctx, req)
	if err != nil {
		return nil, err
	}

	return rpcConvert.MessageToBlockHeader(resp.Block)
}

func GetAccessAPIBlockHeader(
	client access.AccessAPIClient,
	ctx context.Context,
	blockID flow.Identifier,
) (
	*flow.Header,
	error,
) {
	req := &access.GetBlockHeaderByIDRequest{
		Id: blockID[:],
	}

	resp, err := client.GetBlockHeaderByID(ctx, req)
	if err != nil {
		return nil, err
	}

	return rpcConvert.MessageToBlockHeader(resp.Block)
}
