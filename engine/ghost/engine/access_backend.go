package engine

import (
	"context"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
)

var _ accessproto.AccessAPIServer = AccessAPIHandler{}

type AccessAPIHandler struct {
	
}

func (a AccessAPIHandler) Ping(ctx context.Context, request *accessproto.PingRequest) (*accessproto.PingResponse, error) {
	panic("implement me")
}

func (a AccessAPIHandler) GetLatestBlockHeader(ctx context.Context, request *accessproto.GetLatestBlockHeaderRequest) (*accessproto.BlockHeaderResponse, error) {
	panic("implement me")
}

func (a AccessAPIHandler) GetBlockHeaderByID(ctx context.Context, request *accessproto.GetBlockHeaderByIDRequest) (*accessproto.BlockHeaderResponse, error) {
	panic("implement me")
}

func (a AccessAPIHandler) GetBlockHeaderByHeight(ctx context.Context, request *accessproto.GetBlockHeaderByHeightRequest) (*accessproto.BlockHeaderResponse, error) {
	panic("implement me")
}

func (a AccessAPIHandler) GetLatestBlock(ctx context.Context, request *accessproto.GetLatestBlockRequest) (*accessproto.BlockResponse, error) {
	panic("implement me")
}

func (a AccessAPIHandler) GetBlockByID(ctx context.Context, request *accessproto.GetBlockByIDRequest) (*accessproto.BlockResponse, error) {
	panic("implement me")
}

func (a AccessAPIHandler) GetBlockByHeight(ctx context.Context, request *accessproto.GetBlockByHeightRequest) (*accessproto.BlockResponse, error) {
	panic("implement me")
}

func (a AccessAPIHandler) GetCollectionByID(ctx context.Context, request *accessproto.GetCollectionByIDRequest) (*accessproto.CollectionResponse, error) {
	panic("implement me")
}

func (a AccessAPIHandler) SendTransaction(ctx context.Context, request *accessproto.SendTransactionRequest) (*accessproto.SendTransactionResponse, error) {
	panic("implement me")
}

func (a AccessAPIHandler) GetTransaction(ctx context.Context, request *accessproto.GetTransactionRequest) (*accessproto.TransactionResponse, error) {
	panic("implement me")
}

func (a AccessAPIHandler) GetTransactionResult(ctx context.Context, request *accessproto.GetTransactionRequest) (*accessproto.TransactionResultResponse, error) {
	panic("implement me")
}

func (a AccessAPIHandler) GetAccount(ctx context.Context, request *accessproto.GetAccountRequest) (*accessproto.GetAccountResponse, error) {
	panic("implement me")
}

func (a AccessAPIHandler) GetAccountAtLatestBlock(ctx context.Context, request *accessproto.GetAccountAtLatestBlockRequest) (*accessproto.AccountResponse, error) {
	panic("implement me")
}

func (a AccessAPIHandler) GetAccountAtBlockHeight(ctx context.Context, request *accessproto.GetAccountAtBlockHeightRequest) (*accessproto.AccountResponse, error) {
	panic("implement me")
}

func (a AccessAPIHandler) ExecuteScriptAtLatestBlock(ctx context.Context, request *accessproto.ExecuteScriptAtLatestBlockRequest) (*accessproto.ExecuteScriptResponse, error) {
	panic("implement me")
}

func (a AccessAPIHandler) ExecuteScriptAtBlockID(ctx context.Context, request *accessproto.ExecuteScriptAtBlockIDRequest) (*accessproto.ExecuteScriptResponse, error) {
	panic("implement me")
}

func (a AccessAPIHandler) ExecuteScriptAtBlockHeight(ctx context.Context, request *accessproto.ExecuteScriptAtBlockHeightRequest) (*accessproto.ExecuteScriptResponse, error) {
	panic("implement me")
}

func (a AccessAPIHandler) GetEventsForHeightRange(ctx context.Context, request *accessproto.GetEventsForHeightRangeRequest) (*accessproto.EventsResponse, error) {
	panic("implement me")
}

func (a AccessAPIHandler) GetEventsForBlockIDs(ctx context.Context, request *accessproto.GetEventsForBlockIDsRequest) (*accessproto.EventsResponse, error) {
	panic("implement me")
}

func (a AccessAPIHandler) GetNetworkParameters(ctx context.Context, request *accessproto.GetNetworkParametersRequest) (*accessproto.GetNetworkParametersResponse, error) {
	panic("implement me")
}

