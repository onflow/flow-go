package access

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UnimplementedAccessAPIServer struct{}

func (u *UnimplementedAccessAPIServer) Ping(context.Context, *access.PingRequest) (*access.PingResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetNodeVersionInfo(context.Context, *access.GetNodeVersionInfoRequest) (*access.GetNodeVersionInfoResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetLatestBlockHeader(context.Context, *access.GetLatestBlockHeaderRequest) (*access.BlockHeaderResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetBlockHeaderByID(context.Context, *access.GetBlockHeaderByIDRequest) (*access.BlockHeaderResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetBlockHeaderByHeight(context.Context, *access.GetBlockHeaderByHeightRequest) (*access.BlockHeaderResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetLatestBlock(context.Context, *access.GetLatestBlockRequest) (*access.BlockResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetBlockByID(context.Context, *access.GetBlockByIDRequest) (*access.BlockResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetBlockByHeight(context.Context, *access.GetBlockByHeightRequest) (*access.BlockResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetCollectionByID(context.Context, *access.GetCollectionByIDRequest) (*access.CollectionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetFullCollectionByID(context.Context, *access.GetFullCollectionByIDRequest) (*access.FullCollectionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) SendTransaction(context.Context, *access.SendTransactionRequest) (*access.SendTransactionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetTransaction(context.Context, *access.GetTransactionRequest) (*access.TransactionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetTransactionResult(context.Context, *access.GetTransactionRequest) (*access.TransactionResultResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetTransactionResultByIndex(context.Context, *access.GetTransactionByIndexRequest) (*access.TransactionResultResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetTransactionResultsByBlockID(context.Context, *access.GetTransactionsByBlockIDRequest) (*access.TransactionResultsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetTransactionsByBlockID(context.Context, *access.GetTransactionsByBlockIDRequest) (*access.TransactionsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetSystemTransaction(context.Context, *access.GetSystemTransactionRequest) (*access.TransactionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetSystemTransactionResult(context.Context, *access.GetSystemTransactionResultRequest) (*access.TransactionResultResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetAccount(context.Context, *access.GetAccountRequest) (*access.GetAccountResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetAccountAtLatestBlock(context.Context, *access.GetAccountAtLatestBlockRequest) (*access.AccountResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetAccountAtBlockHeight(context.Context, *access.GetAccountAtBlockHeightRequest) (*access.AccountResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetAccountBalanceAtLatestBlock(context.Context, *access.GetAccountBalanceAtLatestBlockRequest) (*access.AccountBalanceResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetAccountBalanceAtBlockHeight(context.Context, *access.GetAccountBalanceAtBlockHeightRequest) (*access.AccountBalanceResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetAccountKeysAtLatestBlock(context.Context, *access.GetAccountKeysAtLatestBlockRequest) (*access.AccountKeysResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetAccountKeysAtBlockHeight(context.Context, *access.GetAccountKeysAtBlockHeightRequest) (*access.AccountKeysResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetAccountKeyAtLatestBlock(context.Context, *access.GetAccountKeyAtLatestBlockRequest) (*access.AccountKeyResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetAccountKeyAtBlockHeight(context.Context, *access.GetAccountKeyAtBlockHeightRequest) (*access.AccountKeyResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) ExecuteScriptAtLatestBlock(context.Context, *access.ExecuteScriptAtLatestBlockRequest) (*access.ExecuteScriptResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) ExecuteScriptAtBlockID(context.Context, *access.ExecuteScriptAtBlockIDRequest) (*access.ExecuteScriptResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) ExecuteScriptAtBlockHeight(context.Context, *access.ExecuteScriptAtBlockHeightRequest) (*access.ExecuteScriptResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetEventsForHeightRange(context.Context, *access.GetEventsForHeightRangeRequest) (*access.EventsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetEventsForBlockIDs(context.Context, *access.GetEventsForBlockIDsRequest) (*access.EventsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetNetworkParameters(context.Context, *access.GetNetworkParametersRequest) (*access.GetNetworkParametersResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetLatestProtocolStateSnapshot(context.Context, *access.GetLatestProtocolStateSnapshotRequest) (*access.ProtocolStateSnapshotResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetProtocolStateSnapshotByBlockID(context.Context, *access.GetProtocolStateSnapshotByBlockIDRequest) (*access.ProtocolStateSnapshotResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetProtocolStateSnapshotByHeight(context.Context, *access.GetProtocolStateSnapshotByHeightRequest) (*access.ProtocolStateSnapshotResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetExecutionResultForBlockID(context.Context, *access.GetExecutionResultForBlockIDRequest) (*access.ExecutionResultForBlockIDResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) GetExecutionResultByID(context.Context, *access.GetExecutionResultByIDRequest) (*access.ExecutionResultByIDResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) SubscribeBlocksFromStartBlockID(*access.SubscribeBlocksFromStartBlockIDRequest, access.AccessAPI_SubscribeBlocksFromStartBlockIDServer) error {
	return status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) SubscribeBlocksFromStartHeight(*access.SubscribeBlocksFromStartHeightRequest, access.AccessAPI_SubscribeBlocksFromStartHeightServer) error {
	return status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) SubscribeBlocksFromLatest(*access.SubscribeBlocksFromLatestRequest, access.AccessAPI_SubscribeBlocksFromLatestServer) error {
	return status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) SubscribeBlockHeadersFromStartBlockID(*access.SubscribeBlockHeadersFromStartBlockIDRequest, access.AccessAPI_SubscribeBlockHeadersFromStartBlockIDServer) error {
	return status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) SubscribeBlockHeadersFromStartHeight(*access.SubscribeBlockHeadersFromStartHeightRequest, access.AccessAPI_SubscribeBlockHeadersFromStartHeightServer) error {
	return status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) SubscribeBlockHeadersFromLatest(*access.SubscribeBlockHeadersFromLatestRequest, access.AccessAPI_SubscribeBlockHeadersFromLatestServer) error {
	return status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) SubscribeBlockDigestsFromStartBlockID(*access.SubscribeBlockDigestsFromStartBlockIDRequest, access.AccessAPI_SubscribeBlockDigestsFromStartBlockIDServer) error {
	return status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) SubscribeBlockDigestsFromStartHeight(*access.SubscribeBlockDigestsFromStartHeightRequest, access.AccessAPI_SubscribeBlockDigestsFromStartHeightServer) error {
	return status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) SubscribeBlockDigestsFromLatest(*access.SubscribeBlockDigestsFromLatestRequest, access.AccessAPI_SubscribeBlockDigestsFromLatestServer) error {
	return status.Errorf(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedAccessAPIServer) SendAndSubscribeTransactionStatuses(*access.SendAndSubscribeTransactionStatusesRequest, access.AccessAPI_SendAndSubscribeTransactionStatusesServer) error {
	return status.Errorf(codes.Unimplemented, "not implemented")
}
