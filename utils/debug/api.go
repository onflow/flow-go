package debug

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/execution"

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

	return &flow.Header{
		ChainID:   flow.ChainID(resp.Block.ChainId),
		ParentID:  flow.Identifier(resp.Block.ParentId),
		Height:    resp.Block.Height,
		Timestamp: resp.Block.Timestamp.AsTime(),
	}, nil
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

	return &flow.Header{
		ChainID:   flow.ChainID(resp.Block.ChainId),
		ParentID:  flow.Identifier(resp.Block.ParentId),
		Height:    resp.Block.Height,
		Timestamp: resp.Block.Timestamp.AsTime(),
	}, nil
}
