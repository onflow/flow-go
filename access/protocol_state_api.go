package access

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
)

type PROTOCOL_STATE_API interface {
	GetLatestBlockHeader(ctx context.Context, isSealed bool) (*flow.Header, error)
	GetBlockHeaderByHeight(ctx context.Context, height uint64) (*flow.Header, error)
	GetBlockHeaderByID(ctx context.Context, id flow.Identifier) (*flow.Header, error)
}
