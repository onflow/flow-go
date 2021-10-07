package access

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
)

// API provides all public-facing functionality of the Flow Access API.
type API interface {
	Ping(ctx context.Context) error
	GetNetworkParameters(ctx context.Context) NetworkParameters

	GetLatestBlockHeader(ctx context.Context, isSealed bool) (*flow.Header, error)
	GetBlockHeaderByHeight(ctx context.Context, height uint64) (*flow.Header, error)
	GetBlockHeaderByID(ctx context.Context, id flow.Identifier) (*flow.Header, error)

	GetLatestBlock(ctx context.Context, isSealed bool) (*flow.Block, error)
	GetBlockByHeight(ctx context.Context, height uint64) (*flow.Block, error)
	GetBlockByID(ctx context.Context, id flow.Identifier) (*flow.Block, error)

	GetCollectionByID(ctx context.Context, id flow.Identifier) (*flow.LightCollection, error)

	SendTransaction(ctx context.Context, tx *flow.TransactionBody) error
	GetTransaction(ctx context.Context, id flow.Identifier) (*flow.TransactionBody, error)
	GetTransactionResult(ctx context.Context, id flow.Identifier) (*TransactionResult, error)

	GetAccount(ctx context.Context, address flow.Address) (*flow.Account, error)
	GetAccountAtLatestBlock(ctx context.Context, address flow.Address) (*flow.Account, error)
	GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64) (*flow.Account, error)

	ExecuteScriptAtLatestBlock(ctx context.Context, script []byte, arguments [][]byte) ([]byte, error)
	ExecuteScriptAtBlockHeight(ctx context.Context, blockHeight uint64, script []byte, arguments [][]byte) ([]byte, error)
	ExecuteScriptAtBlockID(ctx context.Context, blockID flow.Identifier, script []byte, arguments [][]byte) ([]byte, error)

	GetEventsForHeightRange(ctx context.Context, eventType string, startHeight, endHeight uint64) ([]flow.BlockEvents, error)
	GetEventsForBlockIDs(ctx context.Context, eventType string, blockIDs []flow.Identifier) ([]flow.BlockEvents, error)

	GetLatestProtocolStateSnapshot(ctx context.Context) ([]byte, error)

	GetExecutionResultForBlockID(ctx context.Context, blockID flow.Identifier) (*flow.ExecutionResult, error)

	GetRegisterAtBlockID(ctx context.Context, registerOwner, registerController, registerKey []byte, blockID flow.Identifier) ([]byte, error)
}

// TODO: Combine this with flow.TransactionResult?
type TransactionResult struct {
	Status       flow.TransactionStatus
	StatusCode   uint
	Events       []flow.Event
	ErrorMessage string
	BlockID      flow.Identifier
}

func TransactionResultToMessage(result *TransactionResult) *access.TransactionResultResponse {
	return &access.TransactionResultResponse{
		Status:       entities.TransactionStatus(result.Status),
		StatusCode:   uint32(result.StatusCode),
		ErrorMessage: result.ErrorMessage,
		Events:       convert.EventsToMessages(result.Events),
		BlockId:      result.BlockID[:],
	}
}

func MessageToTransactionResult(message *access.TransactionResultResponse) *TransactionResult {

	return &TransactionResult{
		Status:       flow.TransactionStatus(message.Status),
		StatusCode:   uint(message.StatusCode),
		ErrorMessage: message.ErrorMessage,
		Events:       convert.MessagesToEvents(message.Events),
		BlockID:      flow.HashToID(message.BlockId),
	}
}

// NetworkParameters contains the network-wide parameters for the Flow blockchain.
type NetworkParameters struct {
	ChainID flow.ChainID
}
