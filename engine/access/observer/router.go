package observer

import (
	"context"

	"github.com/grpc-ecosystem/go-grpc-middleware/providers/zerolog/v2"
	"github.com/onflow/flow/protobuf/go/flow/access"
	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
)

type Router struct {
	PingHandler                           *Handler[*access.PingRequest, *access.PingResponse]
	GetLatestBlockHeaderHandler           *Handler[*access.GetLatestBlockHeaderRequest, *access.BlockHeaderResponse]
	GetBlockHeaderByIDHandler             *Handler[*access.GetBlockHeaderByIDRequest, *access.BlockHeaderResponse]
	GetBlockHeaderByHeightHandler         *Handler[*access.GetBlockHeaderByHeightRequest, *access.BlockHeaderResponse]
	GetLatestBlockHandler                 *Handler[*access.GetLatestBlockRequest, *access.BlockResponse]
	GetBlockByIDHandler                   *Handler[*access.GetBlockByIDRequest, *access.BlockResponse]
	GetBlockByHeightHandler               *Handler[*access.GetBlockByHeightRequest, *access.BlockResponse]
	GetCollectionByIDHandler              *Handler[*access.GetCollectionByIDRequest, *access.CollectionResponse]
	SendTransactionHandler                *Handler[*access.SendTransactionRequest, *access.SendTransactionResponse]
	GetTransactionHandler                 *Handler[*access.GetTransactionRequest, *access.TransactionResponse]
	GetTransactionResultHandler           *Handler[*access.GetTransactionRequest, *access.TransactionResultResponse]
	GetTransactionResultByIndexHandler    *Handler[*access.GetTransactionByIndexRequest, *access.TransactionResultResponse]
	GetTransactionResultsByBlockIDHandler *Handler[*access.GetTransactionsByBlockIDRequest, *access.TransactionResultsResponse]
	GetTransactionsByBlockIDHandler       *Handler[*access.GetTransactionsByBlockIDRequest, *access.TransactionsResponse]
	GetAccountHandler                     *Handler[*access.GetAccountRequest, *access.GetAccountResponse]
	GetAccountAtLatestBlockHandler        *Handler[*access.GetAccountAtLatestBlockRequest, *access.AccountResponse]
	GetAccountAtBlockHeightHandler        *Handler[*access.GetAccountAtBlockHeightRequest, *access.AccountResponse]
	ExecuteScriptAtLatestBlockHandler     *Handler[*access.ExecuteScriptAtLatestBlockRequest, *access.ExecuteScriptResponse]
	ExecuteScriptAtBlockIDHandler         *Handler[*access.ExecuteScriptAtBlockIDRequest, *access.ExecuteScriptResponse]
	ExecuteScriptAtBlockHeightHandler     *Handler[*access.ExecuteScriptAtBlockHeightRequest, *access.ExecuteScriptResponse]
	GetEventsForHeightRangeHandler        *Handler[*access.GetEventsForHeightRangeRequest, *access.EventsResponse]
	GetEventsForBlockIDsHandler           *Handler[*access.GetEventsForBlockIDsRequest, *access.EventsResponse]
	GetNetworkParametersHandler           *Handler[*access.GetNetworkParametersRequest, *access.GetNetworkParametersResponse]
	GetLatestProtocolStateSnapshotHandler *Handler[*access.GetLatestProtocolStateSnapshotRequest, *access.ProtocolStateSnapshotResponse]
	GetExecutionResultForBlockIDHandler   *Handler[*access.GetExecutionResultForBlockIDRequest, *access.ExecutionResultForBlockIDResponse]
}

func NewRouter(observer, upstream access.AccessAPIServer, logger zerolog.Logger, metrics RPCHandlerMetrics) *Router {
	return &Router{
		PingHandler: &Handler[*accessproto.PingRequest, *accessproto.PingResponse]{
			Name:        "Ping",
			ForwardOnly: true,
			Upstream:    upstream.Ping,
			Observer:    observer.Ping,
			Metrics:     metrics,
			Logger:      logger,
		},
		GetLatestBlockHeaderHandler: &Handler[*accessproto.GetLatestBlockHeaderRequest, *accessproto.BlockHeaderResponse]{
			Name:        "GetLatestBlockHeader",
			ForwardOnly: true,
			Upstream:    upstream.GetLatestBlockHeader,
			Observer:    upstream.GetLatestBlockHeader,
			Metrics:     metrics,
			Logger:      logger,
		},
		GetBlockHeaderByIDHandler: &Handler[*accessproto.GetBlockHeaderByIDRequest, *accessproto.BlockHeaderResponse]{
			Name:        "GetBlockHeaderByID",
			ForwardOnly: true,
			Upstream:    upstream.GetBlockHeaderByID,
			Observer:    observer.GetBlockHeaderByID,
			Metrics:     metrics,
			Logger:      logger,
		},
		GetBlockHeaderByHeightHandler: &Handler[*accessproto.GetBlockHeaderByHeightRequest, *accessproto.BlockHeaderResponse]{
			Name:        "",
			ForwardOnly: true,
			Upstream:    upstream.GetBlockHeaderByHeight,
			Observer:    observer.GetBlockHeaderByHeight,
			Metrics:     metrics,
			Logger:      logger,
		},
		GetLatestBlockHandler: &Handler[*accessproto.GetLatestBlockRequest, *accessproto.BlockResponse]{
			Name:        "GetLatestBlock",
			ForwardOnly: true,
			Upstream:    upstream.GetLatestBlock,
			Observer:    observer.GetLatestBlock,
			Metrics:     metrics,
			Logger:      logger,
		},
		GetBlockByIDHandler: &Handler[*accessproto.GetBlockByIDRequest, *accessproto.BlockResponse]{
			Name:        "GetBlockByID",
			ForwardOnly: true,
			Upstream:    upstream.GetBlockByID,
			Observer:    observer.GetBlockByID,
			Metrics:     metrics,
			Logger:      logger,
		},
		GetBlockByHeightHandler: &Handler[*accessproto.GetBlockByHeightRequest, *accessproto.BlockResponse]{
			Name:        "GetBlockByHeight",
			ForwardOnly: true,
			Upstream:    upstream.GetBlockByHeight,
			Observer:    observer.GetBlockByHeight,
			Metrics:     metrics,
			Logger:      logger,
		},
		GetCollectionByIDHandler: &Handler[*accessproto.GetCollectionByIDRequest, *accessproto.CollectionResponse]{
			Name:        "GetCollectionByID",
			ForwardOnly: true,
			Upstream:    upstream.GetCollectionByID,
			Observer:    observer.GetCollectionByID,
			Metrics:     metrics,
			Logger:      logger,
		},
		SendTransactionHandler: &Handler[*accessproto.SendTransactionRequest, *accessproto.SendTransactionResponse]{
			Name:        "SendTransaction",
			ForwardOnly: true,
			Upstream:    upstream.SendTransaction,
			Observer:    observer.SendTransaction,
			Metrics:     metrics,
			Logger:      logger,
		},
		GetTransactionHandler: &Handler[*accessproto.GetTransactionRequest, *accessproto.TransactionResponse]{
			Name:        "GetTransaction",
			ForwardOnly: true,
			Upstream:    upstream.GetTransaction,
			Observer:    observer.GetTransaction,
			Metrics:     metrics,
			Logger:      logger,
		},
		GetTransactionResultHandler: &Handler[*accessproto.GetTransactionRequest, *accessproto.TransactionResultResponse]{
			Name:        "GetTransactionResult",
			ForwardOnly: true,
			Upstream:    upstream.GetTransactionResult,
			Observer:    observer.GetTransactionResult,
			Metrics:     metrics,
			Logger:      logger,
		},
		GetTransactionResultByIndexHandler: &Handler[*accessproto.GetTransactionByIndexRequest, *accessproto.TransactionResultResponse]{
			Name:        "GetTransactionResultByIndex",
			ForwardOnly: true,
			Upstream:    upstream.GetTransactionResultByIndex,
			Observer:    observer.GetTransactionResultByIndex,
			Metrics:     metrics,
			Logger:      logger,
		},
		GetTransactionResultsByBlockIDHandler: &Handler[*accessproto.GetTransactionsByBlockIDRequest, *accessproto.TransactionResultsResponse]{
			Name:        "GetTransactionResultsByBlockID",
			ForwardOnly: true,
			Upstream:    upstream.GetTransactionResultsByBlockID,
			Observer:    observer.GetTransactionResultsByBlockID,
			Metrics:     metrics,
			Logger:      logger,
		},
		GetTransactionsByBlockIDHandler: &Handler[*accessproto.GetTransactionsByBlockIDRequest, *accessproto.TransactionsResponse]{
			Name:        "GetTransactionsByBlockID",
			ForwardOnly: true,
			Upstream:    upstream.GetTransactionsByBlockID,
			Observer:    observer.GetTransactionsByBlockID,
			Metrics:     metrics,
			Logger:      logger,
		},
		GetAccountHandler: &Handler[*accessproto.GetAccountRequest, *accessproto.GetAccountResponse]{
			Name:        "GetAccount",
			ForwardOnly: true,
			Upstream:    upstream.GetAccount,
			Observer:    observer.GetAccount,
			Metrics:     metrics,
			Logger:      logger,
		},
		GetAccountAtLatestBlockHandler: &Handler[*accessproto.GetAccountAtLatestBlockRequest, *accessproto.AccountResponse]{
			Name:        "GetAccountAtLatestBlock",
			ForwardOnly: true,
			Upstream:    upstream.GetAccountAtLatestBlock,
			Observer:    observer.GetAccountAtLatestBlock,
			Metrics:     metrics,
			Logger:      logger,
		},
		GetAccountAtBlockHeightHandler: &Handler[*accessproto.GetAccountAtBlockHeightRequest, *accessproto.AccountResponse]{
			Name:        "GetAccountAtBlockHeight",
			ForwardOnly: true,
			Upstream:    upstream.GetAccountAtBlockHeight,
			Observer:    observer.GetAccountAtBlockHeight,
			Metrics:     metrics,
			Logger:      logger,
		},
		ExecuteScriptAtLatestBlockHandler: &Handler[*accessproto.ExecuteScriptAtLatestBlockRequest, *accessproto.ExecuteScriptResponse]{
			Name:        "ExecuteScriptAtLatestBlock",
			ForwardOnly: true,
			Upstream:    upstream.ExecuteScriptAtLatestBlock,
			Observer:    observer.ExecuteScriptAtLatestBlock,
			Metrics:     metrics,
			Logger:      logger,
		},
		ExecuteScriptAtBlockIDHandler: &Handler[*accessproto.ExecuteScriptAtBlockIDRequest, *accessproto.ExecuteScriptResponse]{
			Name:        "ExecuteScriptAtBlockID",
			ForwardOnly: true,
			Upstream:    upstream.ExecuteScriptAtBlockID,
			Observer:    observer.ExecuteScriptAtBlockID,
			Metrics:     metrics,
			Logger:      logger,
		},
		ExecuteScriptAtBlockHeightHandler: &Handler[*accessproto.ExecuteScriptAtBlockHeightRequest, *accessproto.ExecuteScriptResponse]{
			Name:        "ExecuteScriptAtBlockHeight",
			ForwardOnly: true,
			Upstream:    upstream.ExecuteScriptAtBlockHeight,
			Observer:    observer.ExecuteScriptAtBlockHeight,
			Metrics:     metrics,
			Logger:      logger,
		},
		GetEventsForHeightRangeHandler: &Handler[*accessproto.GetEventsForHeightRangeRequest, *accessproto.EventsResponse]{
			Name:        "GetEventsForHeightRange",
			ForwardOnly: true,
			Upstream:    upstream.GetEventsForHeightRange,
			Observer:    observer.GetEventsForHeightRange,
			Metrics:     metrics,
			Logger:      logger,
		},
		GetEventsForBlockIDsHandler: &Handler[*accessproto.GetEventsForBlockIDsRequest, *accessproto.EventsResponse]{
			Name:        "GetEventsForBlockIDs",
			ForwardOnly: true,
			Upstream:    upstream.GetEventsForBlockIDs,
			Observer:    observer.GetEventsForBlockIDs,
			Metrics:     metrics,
			Logger:      logger,
		},
		GetNetworkParametersHandler: &Handler[*accessproto.GetNetworkParametersRequest, *accessproto.GetNetworkParametersResponse]{
			Name:        "GetNetworkParameters",
			ForwardOnly: true,
			Upstream:    upstream.GetNetworkParameters,
			Observer:    observer.GetNetworkParameters,
			Metrics:     metrics,
			Logger:      logger,
		},
		GetLatestProtocolStateSnapshotHandler: &Handler[*accessproto.GetLatestProtocolStateSnapshotRequest, *accessproto.ProtocolStateSnapshotResponse]{
			Name:        "GetLatestProtocolStateSnapshot",
			ForwardOnly: true,
			Upstream:    upstream.GetLatestProtocolStateSnapshot,
			Observer:    observer.GetLatestProtocolStateSnapshot,
			Metrics:     metrics,
			Logger:      logger,
		},
		GetExecutionResultForBlockIDHandler: &Handler[*accessproto.GetExecutionResultForBlockIDRequest, *accessproto.ExecutionResultForBlockIDResponse]{
			Name:        "GetExecutionResultForBlockID",
			ForwardOnly: true,
			Upstream:    upstream.GetExecutionResultForBlockID,
			Observer:    observer.GetExecutionResultForBlockID,
			Metrics:     metrics,
			Logger:      logger,
		},
	}
}

func (r *Router) Ping(context context.Context, req *access.PingRequest) (*access.PingResponse, error) {
	return r.PingHandler.Call(context, req)
}

func (r *Router) GetLatestBlockHeader(context context.Context, req *access.GetLatestBlockHeaderRequest) (*access.BlockHeaderResponse, error) {
	return r.GetLatestBlockHeaderHandler.Call(context, req)
}

func (r *Router) GetBlockHeaderByID(context context.Context, req *access.GetBlockHeaderByIDRequest) (*access.BlockHeaderResponse, error) {
	return r.GetBlockHeaderByIDHandler.Call(context, req)
}

func (r *Router) GetBlockHeaderByHeight(context context.Context, req *access.GetBlockHeaderByHeightRequest) (*access.BlockHeaderResponse, error) {
	return r.GetBlockHeaderByHeightHandler.Call(context, req)
}

func (r *Router) GetLatestBlock(context context.Context, req *access.GetLatestBlockRequest) (*access.BlockResponse, error) {
	return r.GetLatestBlockHandler.Call(context, req)
}

func (r *Router) GetBlockByID(context context.Context, req *access.GetBlockByIDRequest) (*access.BlockResponse, error) {
	return r.GetBlockByIDHandler.Call(context, req)
}

func (r *Router) GetBlockByHeight(context context.Context, req *access.GetBlockByHeightRequest) (*access.BlockResponse, error) {
	return r.GetBlockByHeightHandler.Call(context, req)
}

func (r *Router) GetCollectionByID(context context.Context, req *access.GetCollectionByIDRequest) (*access.CollectionResponse, error) {
	return r.GetCollectionByIDHandler.Call(context, req)
}

func (r *Router) SendTransaction(context context.Context, req *access.SendTransactionRequest) (*access.SendTransactionResponse, error) {
	return r.SendTransactionHandler.Call(context, req)
}

func (r *Router) GetTransaction(context context.Context, req *access.GetTransactionRequest) (*access.TransactionResponse, error) {
	return r.GetTransactionHandler.Call(context, req)
}

func (r *Router) GetTransactionResult(context context.Context, req *access.GetTransactionRequest) (*access.TransactionResultResponse, error) {
	return r.GetTransactionResultHandler.Call(context, req)
}

func (r *Router) GetTransactionResultByIndex(context context.Context, req *access.GetTransactionByIndexRequest) (*access.TransactionResultResponse, error) {
	return r.GetTransactionResultByIndexHandler.Call(context, req)
}

func (r *Router) GetAccount(context context.Context, req *access.GetAccountRequest) (*access.GetAccountResponse, error) {
	return r.GetAccount(context, req)
}

func (r *Router) GetAccountAtLatestBlock(context context.Context, req *access.GetAccountAtLatestBlockRequest) (*access.AccountResponse, error) {
	return r.GetAccountAtLatestBlockHandler.Call(context, req)
}

func (r *Router) GetAccountAtBlockHeight(context context.Context, req *access.GetAccountAtBlockHeightRequest) (*access.AccountResponse, error) {
	return r.GetAccountAtBlockHeightHandler.Call(context, req)
}

func (r *Router) ExecuteScriptAtLatestBlock(context context.Context, req *access.ExecuteScriptAtLatestBlockRequest) (*access.ExecuteScriptResponse, error) {
	return r.ExecuteScriptAtLatestBlockHandler.Call(context, req)
}

func (r *Router) ExecuteScriptAtBlockID(context context.Context, req *access.ExecuteScriptAtBlockIDRequest) (*access.ExecuteScriptResponse, error) {
	return r.ExecuteScriptAtBlockIDHandler.Call(context, req)
}

func (r *Router) ExecuteScriptAtBlockHeight(context context.Context, req *access.ExecuteScriptAtBlockHeightRequest) (*access.ExecuteScriptResponse, error) {
	return r.ExecuteScriptAtBlockHeightHandler.Call(context, req)
}

func (r *Router) GetEventsForHeightRange(context context.Context, req *access.GetEventsForHeightRangeRequest) (*access.EventsResponse, error) {
	return r.GetEventsForHeightRangeHandler.Call(context, req)
}

func (r *Router) GetEventsForBlockIDs(context context.Context, req *access.GetEventsForBlockIDsRequest) (*access.EventsResponse, error) {
	return r.GetEventsForBlockIDsHandler.Call(context, req)
}

func (r *Router) GetNetworkParameters(context context.Context, req *access.GetNetworkParametersRequest) (*access.GetNetworkParametersResponse, error) {
	return r.GetNetworkParametersHandler.Call(context, req)
}

func (r *Router) GetLatestProtocolStateSnapshot(context context.Context, req *access.GetLatestProtocolStateSnapshotRequest) (*access.ProtocolStateSnapshotResponse, error) {
	return r.GetLatestProtocolStateSnapshotHandler.Call(context, req)
}

func (r *Router) GetExecutionResultForBlockID(context context.Context, req *access.GetExecutionResultForBlockIDRequest) (*access.ExecutionResultForBlockIDResponse, error) {
	return r.GetExecutionResultForBlockIDHandler.Call(context, req)
}

func (r *Router) GetTransactionResultsByBlockID(context context.Context, req *access.GetTransactionsByBlockIDRequest) (*access.TransactionResultsResponse, error) {
	return r.GetTransactionResultsByBlockIDHandler.Call(context, req)
}

func (r *Router) GetTransactionsByBlockID(context context.Context, req *access.GetTransactionsByBlockIDRequest) (*access.TransactionsResponse, error) {
	return r.GetTransactionsByBlockIDHandler.Call(context, req)
}
