package api

import (
	"context"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/model/flow"
)

// RestBackendApi is the backend API for REST service.
type RestBackendApi interface {
	GetNetworkParameters(ctx context.Context) access.NetworkParameters
	GetNodeVersionInfo(ctx context.Context) (*access.NodeVersionInfo, error)

	GetLatestBlockHeader(ctx context.Context, isSealed bool) (*flow.Header, flow.BlockStatus, error)

	GetLatestBlock(ctx context.Context, isSealed bool) (*flow.Block, flow.BlockStatus, error)
	GetBlockByHeight(ctx context.Context, height uint64) (*flow.Block, flow.BlockStatus, error)
	GetBlockByID(ctx context.Context, id flow.Identifier) (*flow.Block, flow.BlockStatus, error)

	GetCollectionByID(ctx context.Context, id flow.Identifier) (*flow.LightCollection, error)

	SendTransaction(ctx context.Context, tx *flow.TransactionBody) error
	GetTransaction(ctx context.Context, id flow.Identifier) (*flow.TransactionBody, error)
	GetTransactionResult(ctx context.Context, id flow.Identifier, blockID flow.Identifier, collectionID flow.Identifier) (*access.TransactionResult, error)

	GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64) (*flow.Account, error)

	ExecuteScriptAtLatestBlock(ctx context.Context, script []byte, arguments [][]byte) ([]byte, error)
	ExecuteScriptAtBlockHeight(ctx context.Context, blockHeight uint64, script []byte, arguments [][]byte) ([]byte, error)
	ExecuteScriptAtBlockID(ctx context.Context, blockID flow.Identifier, script []byte, arguments [][]byte) ([]byte, error)

	GetEventsForHeightRange(ctx context.Context, eventType string, startHeight, endHeight uint64) ([]flow.BlockEvents, error)
	GetEventsForBlockIDs(ctx context.Context, eventType string, blockIDs []flow.Identifier) ([]flow.BlockEvents, error)

	GetExecutionResultForBlockID(ctx context.Context, blockID flow.Identifier) (*flow.ExecutionResult, error)
	GetExecutionResultByID(ctx context.Context, id flow.Identifier) (*flow.ExecutionResult, error)
}
