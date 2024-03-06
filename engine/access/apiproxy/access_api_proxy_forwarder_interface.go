package apiproxy

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/access"
)

type FlowAccessAPIForwarderInterface interface {
	Ping(context context.Context, req *access.PingRequest) (*access.PingResponse, error)
	GetNodeVersionInfo(context context.Context, req *access.GetNodeVersionInfoRequest) (*access.GetNodeVersionInfoResponse, error)
	GetLatestBlockHeader(context context.Context, req *access.GetLatestBlockHeaderRequest) (*access.BlockHeaderResponse, error)
	GetBlockHeaderByID(context context.Context, req *access.GetBlockHeaderByIDRequest) (*access.BlockHeaderResponse, error)
	GetBlockHeaderByHeight(context context.Context, req *access.GetBlockHeaderByHeightRequest) (*access.BlockHeaderResponse, error)
	GetLatestBlock(context context.Context, req *access.GetLatestBlockRequest) (*access.BlockResponse, error)
	GetBlockByID(context context.Context, req *access.GetBlockByIDRequest) (*access.BlockResponse, error)
	GetBlockByHeight(context context.Context, req *access.GetBlockByHeightRequest) (*access.BlockResponse, error)
	GetCollectionByID(context context.Context, req *access.GetCollectionByIDRequest) (*access.CollectionResponse, error)
	SendTransaction(context context.Context, req *access.SendTransactionRequest) (*access.SendTransactionResponse, error)
	GetTransaction(context context.Context, req *access.GetTransactionRequest) (*access.TransactionResponse, error)
	GetTransactionResult(context context.Context, req *access.GetTransactionRequest) (*access.TransactionResultResponse, error)
	GetSystemTransaction(context context.Context, req *access.GetSystemTransactionRequest) (*access.TransactionResponse, error)
	GetSystemTransactionResult(context context.Context, req *access.GetSystemTransactionResultRequest) (*access.TransactionResultResponse, error)
	GetTransactionResultByIndex(context context.Context, req *access.GetTransactionByIndexRequest) (*access.TransactionResultResponse, error)
	GetTransactionResultsByBlockID(context context.Context, req *access.GetTransactionsByBlockIDRequest) (*access.TransactionResultsResponse, error)
	GetTransactionsByBlockID(context context.Context, req *access.GetTransactionsByBlockIDRequest) (*access.TransactionsResponse, error)
	GetAccount(context context.Context, req *access.GetAccountRequest) (*access.GetAccountResponse, error)
	GetAccountAtLatestBlock(context context.Context, req *access.GetAccountAtLatestBlockRequest) (*access.AccountResponse, error)
	GetAccountAtBlockHeight(context context.Context, req *access.GetAccountAtBlockHeightRequest) (*access.AccountResponse, error)
	ExecuteScriptAtLatestBlock(context context.Context, req *access.ExecuteScriptAtLatestBlockRequest) (*access.ExecuteScriptResponse, error)
	ExecuteScriptAtBlockID(context context.Context, req *access.ExecuteScriptAtBlockIDRequest) (*access.ExecuteScriptResponse, error)
	ExecuteScriptAtBlockHeight(context context.Context, req *access.ExecuteScriptAtBlockHeightRequest) (*access.ExecuteScriptResponse, error)
	GetEventsForHeightRange(context context.Context, req *access.GetEventsForHeightRangeRequest) (*access.EventsResponse, error)
	GetEventsForBlockIDs(context context.Context, req *access.GetEventsForBlockIDsRequest) (*access.EventsResponse, error)
	GetNetworkParameters(context context.Context, req *access.GetNetworkParametersRequest) (*access.GetNetworkParametersResponse, error)
	GetLatestProtocolStateSnapshot(context context.Context, req *access.GetLatestProtocolStateSnapshotRequest) (*access.ProtocolStateSnapshotResponse, error)
	GetExecutionResultForBlockID(context context.Context, req *access.GetExecutionResultForBlockIDRequest) (*access.ExecutionResultForBlockIDResponse, error)
	GetExecutionResultByID(context context.Context, req *access.GetExecutionResultByIDRequest) (*access.ExecutionResultByIDResponse, error)
}
