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
	GetTransactionsByBlockID(ctx context.Context, blockID flow.Identifier) ([]*flow.TransactionBody, error)
	GetTransactionResult(ctx context.Context, id flow.Identifier) (*TransactionResult, error)
	GetTransactionResultByIndex(ctx context.Context, blockID flow.Identifier, index uint32) (*TransactionResult, error)
	GetTransactionResultsByBlockID(ctx context.Context, blockID flow.Identifier) ([]*TransactionResult, error)

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
	GetExecutionResultByID(ctx context.Context, id flow.Identifier) (*flow.ExecutionResult, error)
}

// TODO: Combine this with flow.TransactionResult?
type TransactionResult struct {
	Status        flow.TransactionStatus
	StatusCode    uint
	Events        []flow.Event
	ErrorMessage  string
	BlockID       flow.Identifier
	TransactionID flow.Identifier
	CollectionID  flow.Identifier
}

func TransactionResultToMessage(result *TransactionResult) *access.TransactionResultResponse {
	return &access.TransactionResultResponse{
		Status:        entities.TransactionStatus(result.Status),
		StatusCode:    uint32(result.StatusCode),
		ErrorMessage:  result.ErrorMessage,
		Events:        convert.EventsToMessages(result.Events),
		BlockId:       result.BlockID[:],
		TransactionId: result.TransactionID[:],
		CollectionId:  result.CollectionID[:],
	}
}

func TransactionResultsToMessage(results []*TransactionResult) *access.TransactionResultsResponse {
	messages := make([]*access.TransactionResultResponse, len(results))
	for i, result := range results {
		messages[i] = TransactionResultToMessage(result)
	}

	return &access.TransactionResultsResponse{
		TransactionResults: messages,
	}
}

func MessageToTransactionResult(message *access.TransactionResultResponse) *TransactionResult {

	return &TransactionResult{
		Status:        flow.TransactionStatus(message.Status),
		StatusCode:    uint(message.StatusCode),
		ErrorMessage:  message.ErrorMessage,
		Events:        convert.MessagesToEvents(message.Events),
		BlockID:       flow.HashToID(message.BlockId),
		TransactionID: flow.HashToID(message.TransactionId),
		CollectionID:  flow.HashToID(message.CollectionId),
	}
}

// NetworkParameters contains the network-wide parameters for the Flow blockchain.
type NetworkParameters struct {
	ChainID flow.ChainID
}
