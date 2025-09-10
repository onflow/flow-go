package debug

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/access"

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
