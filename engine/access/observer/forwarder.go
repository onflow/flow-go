package observer

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/access"
	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
)

type Forwarder struct {
	UpstreamHandler accessproto.AccessAPIClient
}

func (f *Forwarder) Ping(context context.Context, req *access.PingRequest) (*access.PingResponse, error) {
	return f.UpstreamHandler.Ping(context, req)
}

func (f *Forwarder) GetLatestBlockHeader(context context.Context, req *access.GetLatestBlockHeaderRequest) (*access.BlockHeaderResponse, error) {
	return f.UpstreamHandler.GetLatestBlockHeader(context, req)
}

func (f *Forwarder) GetBlockHeaderByID(context context.Context, req *access.GetBlockHeaderByIDRequest) (*access.BlockHeaderResponse, error) {
	return f.UpstreamHandler.GetBlockHeaderByID(context, req)
}

func (f *Forwarder) GetBlockHeaderByHeight(context context.Context, req *access.GetBlockHeaderByHeightRequest) (*access.BlockHeaderResponse, error) {
	return f.UpstreamHandler.GetBlockHeaderByHeight(context, req)
}

func (f *Forwarder) GetLatestBlock(context context.Context, req *access.GetLatestBlockRequest) (*access.BlockResponse, error) {
	return f.UpstreamHandler.GetLatestBlock(context, req)
}

func (f *Forwarder) GetBlockByID(context context.Context, req *access.GetBlockByIDRequest) (*access.BlockResponse, error) {
	return f.UpstreamHandler.GetBlockByID(context, req)
}

func (f *Forwarder) GetBlockByHeight(context context.Context, req *access.GetBlockByHeightRequest) (*access.BlockResponse, error) {
	return f.UpstreamHandler.GetBlockByHeight(context, req)
}

func (f *Forwarder) GetCollectionByID(context context.Context, req *access.GetCollectionByIDRequest) (*access.CollectionResponse, error) {
	return f.UpstreamHandler.GetCollectionByID(context, req)
}

func (f *Forwarder) SendTransaction(context context.Context, req *access.SendTransactionRequest) (*access.SendTransactionResponse, error) {
	return f.UpstreamHandler.SendTransaction(context, req)
}

func (f *Forwarder) GetTransaction(context context.Context, req *access.GetTransactionRequest) (*access.TransactionResponse, error) {
	return f.UpstreamHandler.GetTransaction(context, req)
}

func (f *Forwarder) GetTransactionResult(context context.Context, req *access.GetTransactionRequest) (*access.TransactionResultResponse, error) {
	return f.UpstreamHandler.GetTransactionResult(context, req)
}

func (f *Forwarder) GetTransactionResultByIndex(context context.Context, req *access.GetTransactionByIndexRequest) (*access.TransactionResultResponse, error) {
	return f.UpstreamHandler.GetTransactionResultByIndex(context, req)
}

func (f *Forwarder) GetAccount(context context.Context, req *access.GetAccountRequest) (*access.GetAccountResponse, error) {
	return f.UpstreamHandler.GetAccount(context, req)
}

func (f *Forwarder) GetAccountAtLatestBlock(context context.Context, req *access.GetAccountAtLatestBlockRequest) (*access.AccountResponse, error) {
	return f.UpstreamHandler.GetAccountAtLatestBlock(context, req)
}

func (f *Forwarder) GetAccountAtBlockHeight(context context.Context, req *access.GetAccountAtBlockHeightRequest) (*access.AccountResponse, error) {
	return f.UpstreamHandler.GetAccountAtBlockHeight(context, req)
}

func (f *Forwarder) ExecuteScriptAtLatestBlock(context context.Context, req *access.ExecuteScriptAtLatestBlockRequest) (*access.ExecuteScriptResponse, error) {
	return f.UpstreamHandler.ExecuteScriptAtLatestBlock(context, req)
}

func (f *Forwarder) ExecuteScriptAtBlockID(context context.Context, req *access.ExecuteScriptAtBlockIDRequest) (*access.ExecuteScriptResponse, error) {
	return f.UpstreamHandler.ExecuteScriptAtBlockID(context, req)
}

func (f *Forwarder) ExecuteScriptAtBlockHeight(context context.Context, req *access.ExecuteScriptAtBlockHeightRequest) (*access.ExecuteScriptResponse, error) {
	return f.UpstreamHandler.ExecuteScriptAtBlockHeight(context, req)
}

func (f *Forwarder) GetEventsForHeightRange(context context.Context, req *access.GetEventsForHeightRangeRequest) (*access.EventsResponse, error) {
	return f.UpstreamHandler.GetEventsForHeightRange(context, req)
}

func (f *Forwarder) GetEventsForBlockIDs(context context.Context, req *access.GetEventsForBlockIDsRequest) (*access.EventsResponse, error) {
	return f.UpstreamHandler.GetEventsForBlockIDs(context, req)
}

func (f *Forwarder) GetNetworkParameters(context context.Context, req *access.GetNetworkParametersRequest) (*access.GetNetworkParametersResponse, error) {
	return f.UpstreamHandler.GetNetworkParameters(context, req)
}

func (f *Forwarder) GetLatestProtocolStateSnapshot(context context.Context, req *access.GetLatestProtocolStateSnapshotRequest) (*access.ProtocolStateSnapshotResponse, error) {
	return f.UpstreamHandler.GetLatestProtocolStateSnapshot(context, req)
}

func (f *Forwarder) GetExecutionResultForBlockID(context context.Context, req *access.GetExecutionResultForBlockIDRequest) (*access.ExecutionResultForBlockIDResponse, error) {
	return f.UpstreamHandler.GetExecutionResultForBlockID(context, req)
}

func (f *Forwarder) GetTransactionResultsByBlockID(context context.Context, req *access.GetTransactionsByBlockIDRequest) (*access.TransactionResultsResponse, error) {
	return f.UpstreamHandler.GetTransactionResultsByBlockID(context, req)
}

func (f *Forwarder) GetTransactionsByBlockID(context context.Context, req *access.GetTransactionsByBlockIDRequest) (*access.TransactionsResponse, error) {
	return f.UpstreamHandler.GetTransactionsByBlockID(context, req)
}
