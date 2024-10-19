package access

import (
	"context"

	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ API = (*UnimplementedHandler)(nil)

type UnimplementedHandler struct{}

func (u *UnimplementedHandler) Ping(ctx context.Context) error {
	return status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetNetworkParameters(ctx context.Context) NetworkParameters {
	return NetworkParameters{}
}

func (u *UnimplementedHandler) GetNodeVersionInfo(ctx context.Context) (*NodeVersionInfo, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetLatestBlockHeader(ctx context.Context, isSealed bool) (*flow.Header, flow.BlockStatus, error) {
	return nil, flow.BlockStatusUnknown, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetBlockHeaderByHeight(ctx context.Context, height uint64) (*flow.Header, flow.BlockStatus, error) {
	return nil, flow.BlockStatusUnknown, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetBlockHeaderByID(ctx context.Context, id flow.Identifier) (*flow.Header, flow.BlockStatus, error) {
	return nil, flow.BlockStatusUnknown, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetLatestBlock(ctx context.Context, isSealed bool) (*flow.Block, flow.BlockStatus, error) {
	return nil, flow.BlockStatusUnknown, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetBlockByHeight(ctx context.Context, height uint64) (*flow.Block, flow.BlockStatus, error) {
	return nil, flow.BlockStatusUnknown, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetBlockByID(ctx context.Context, id flow.Identifier) (*flow.Block, flow.BlockStatus, error) {
	return nil, flow.BlockStatusUnknown, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetCollectionByID(ctx context.Context, id flow.Identifier) (*flow.LightCollection, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetFullCollectionByID(ctx context.Context, id flow.Identifier) (*flow.Collection, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) SendTransaction(ctx context.Context, tx *flow.TransactionBody) error {
	return status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetTransaction(ctx context.Context, id flow.Identifier) (*flow.TransactionBody, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetTransactionsByBlockID(ctx context.Context, blockID flow.Identifier) ([]*flow.TransactionBody, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetTransactionResult(ctx context.Context, id flow.Identifier, blockID flow.Identifier, collectionID flow.Identifier, requiredEventEncodingVersion entities.EventEncodingVersion) (*TransactionResult, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetTransactionResultByIndex(ctx context.Context, blockID flow.Identifier, index uint32, requiredEventEncodingVersion entities.EventEncodingVersion) (*TransactionResult, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetTransactionResultsByBlockID(ctx context.Context, blockID flow.Identifier, requiredEventEncodingVersion entities.EventEncodingVersion) ([]*TransactionResult, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetSystemTransaction(ctx context.Context, blockID flow.Identifier) (*flow.TransactionBody, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetSystemTransactionResult(ctx context.Context, blockID flow.Identifier, requiredEventEncodingVersion entities.EventEncodingVersion) (*TransactionResult, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetAccount(ctx context.Context, address flow.Address) (*flow.Account, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetAccountAtLatestBlock(ctx context.Context, address flow.Address) (*flow.Account, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64) (*flow.Account, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetAccountBalanceAtLatestBlock(ctx context.Context, address flow.Address) (uint64, error) {
	return 0, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetAccountBalanceAtBlockHeight(ctx context.Context, address flow.Address, height uint64) (uint64, error) {
	return 0, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetAccountKeyAtLatestBlock(ctx context.Context, address flow.Address, keyIndex uint32) (*flow.AccountPublicKey, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetAccountKeyAtBlockHeight(ctx context.Context, address flow.Address, keyIndex uint32, height uint64) (*flow.AccountPublicKey, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetAccountKeysAtLatestBlock(ctx context.Context, address flow.Address) ([]flow.AccountPublicKey, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetAccountKeysAtBlockHeight(ctx context.Context, address flow.Address, height uint64) ([]flow.AccountPublicKey, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) ExecuteScriptAtLatestBlock(ctx context.Context, script []byte, arguments [][]byte) ([]byte, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) ExecuteScriptAtBlockHeight(ctx context.Context, blockHeight uint64, script []byte, arguments [][]byte) ([]byte, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) ExecuteScriptAtBlockID(ctx context.Context, blockID flow.Identifier, script []byte, arguments [][]byte) ([]byte, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetEventsForHeightRange(ctx context.Context, eventType string, startHeight, endHeight uint64, requiredEventEncodingVersion entities.EventEncodingVersion) ([]flow.BlockEvents, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetEventsForBlockIDs(ctx context.Context, eventType string, blockIDs []flow.Identifier, requiredEventEncodingVersion entities.EventEncodingVersion) ([]flow.BlockEvents, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetLatestProtocolStateSnapshot(ctx context.Context) ([]byte, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetProtocolStateSnapshotByBlockID(ctx context.Context, blockID flow.Identifier) ([]byte, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetProtocolStateSnapshotByHeight(ctx context.Context, blockHeight uint64) ([]byte, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetExecutionResultForBlockID(ctx context.Context, blockID flow.Identifier) (*flow.ExecutionResult, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) GetExecutionResultByID(ctx context.Context, id flow.Identifier) (*flow.ExecutionResult, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (u *UnimplementedHandler) SubscribeBlocksFromStartBlockID(ctx context.Context, startBlockID flow.Identifier, blockStatus flow.BlockStatus) subscription.Subscription {
	return subscription.NewFailedSubscription(status.Error(codes.Unimplemented, "not implemented"), "streaming not implemented")
}

func (u *UnimplementedHandler) SubscribeBlocksFromStartHeight(ctx context.Context, startHeight uint64, blockStatus flow.BlockStatus) subscription.Subscription {
	return subscription.NewFailedSubscription(status.Error(codes.Unimplemented, "not implemented"), "streaming not implemented")
}

func (u *UnimplementedHandler) SubscribeBlocksFromLatest(ctx context.Context, blockStatus flow.BlockStatus) subscription.Subscription {
	return subscription.NewFailedSubscription(status.Error(codes.Unimplemented, "not implemented"), "streaming not implemented")
}

func (u *UnimplementedHandler) SubscribeBlockHeadersFromStartBlockID(ctx context.Context, startBlockID flow.Identifier, blockStatus flow.BlockStatus) subscription.Subscription {
	return subscription.NewFailedSubscription(status.Error(codes.Unimplemented, "not implemented"), "streaming not implemented")
}

func (u *UnimplementedHandler) SubscribeBlockHeadersFromStartHeight(ctx context.Context, startHeight uint64, blockStatus flow.BlockStatus) subscription.Subscription {
	return subscription.NewFailedSubscription(status.Error(codes.Unimplemented, "not implemented"), "streaming not implemented")
}

func (u *UnimplementedHandler) SubscribeBlockHeadersFromLatest(ctx context.Context, blockStatus flow.BlockStatus) subscription.Subscription {
	return subscription.NewFailedSubscription(status.Error(codes.Unimplemented, "not implemented"), "streaming not implemented")
}

func (u *UnimplementedHandler) SubscribeBlockDigestsFromStartBlockID(ctx context.Context, startBlockID flow.Identifier, blockStatus flow.BlockStatus) subscription.Subscription {
	return subscription.NewFailedSubscription(status.Error(codes.Unimplemented, "not implemented"), "streaming not implemented")
}

func (u *UnimplementedHandler) SubscribeBlockDigestsFromStartHeight(ctx context.Context, startHeight uint64, blockStatus flow.BlockStatus) subscription.Subscription {
	return subscription.NewFailedSubscription(status.Error(codes.Unimplemented, "not implemented"), "streaming not implemented")
}

func (u *UnimplementedHandler) SubscribeBlockDigestsFromLatest(ctx context.Context, blockStatus flow.BlockStatus) subscription.Subscription {
	return subscription.NewFailedSubscription(status.Error(codes.Unimplemented, "not implemented"), "streaming not implemented")
}

func (u *UnimplementedHandler) SubscribeTransactionStatuses(ctx context.Context, tx *flow.TransactionBody, requiredEventEncodingVersion entities.EventEncodingVersion) subscription.Subscription {
	return subscription.NewFailedSubscription(status.Error(codes.Unimplemented, "not implemented"), "streaming not implemented")
}
