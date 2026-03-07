package debug

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"

	rpcConvert "github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
)

func GetAccessAPIBlockHeader(
	ctx context.Context,
	client access.AccessAPIClient,
	blockID flow.Identifier,
) (*flow.Header, error) {
	req := &access.GetBlockHeaderByIDRequest{
		Id: blockID[:],
	}

	resp, err := client.GetBlockHeaderByID(ctx, req)
	if err != nil {
		return nil, err
	}

	return rpcConvert.MessageToBlockHeader(resp.Block)
}

func convertBlockStatus(status flow.BlockStatus) entities.BlockStatus {
	switch status {
	case flow.BlockStatusUnknown:
		return entities.BlockStatus_BLOCK_UNKNOWN
	case flow.BlockStatusFinalized:
		return entities.BlockStatus_BLOCK_FINALIZED
	case flow.BlockStatusSealed:
		return entities.BlockStatus_BLOCK_SEALED
	}
	return entities.BlockStatus_BLOCK_UNKNOWN
}

func SubscribeAccessAPIBlockHeadersFromStartBlockID(
	ctx context.Context,
	client access.AccessAPIClient,
	startBlockID flow.Identifier,
	blockStatus flow.BlockStatus,
) (
	func() (*flow.Header, error),
	error,
) {
	req := &access.SubscribeBlockHeadersFromStartBlockIDRequest{
		StartBlockId: startBlockID[:],
		BlockStatus:  convertBlockStatus(blockStatus),
	}

	cl, err := client.SubscribeBlockHeadersFromStartBlockID(ctx, req)
	if err != nil {
		return nil, err
	}

	return func() (*flow.Header, error) {
		resp, err := cl.Recv()
		if err != nil {
			return nil, err
		}

		return rpcConvert.MessageToBlockHeader(resp.Header)
	}, nil
}

func SubscribeAccessAPIBlockHeadersFromLatest(
	ctx context.Context,
	client access.AccessAPIClient,
	blockStatus flow.BlockStatus,
) (
	func() (*flow.Header, error),
	error,
) {
	req := &access.SubscribeBlockHeadersFromLatestRequest{
		BlockStatus: convertBlockStatus(blockStatus),
	}

	cl, err := client.SubscribeBlockHeadersFromLatest(ctx, req)
	if err != nil {
		return nil, err
	}

	return func() (*flow.Header, error) {
		resp, err := cl.Recv()
		if err != nil {
			return nil, err
		}

		return rpcConvert.MessageToBlockHeader(resp.Header)
	}, nil
}
