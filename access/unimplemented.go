package access

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/engine/access/subscription"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
)

// UnimplementedAPI provides an implementation of the access.API interface where all methods return
// unimplemented errors.
type UnimplementedAPI struct{}

var _ API = (*UnimplementedAPI)(nil)

// NewUnimplementedAPI creates a new UnimplementedAPI instance.
func NewUnimplementedAPI() *UnimplementedAPI {
	return &UnimplementedAPI{}
}

// GetAccount returns an unimplemented error.
func (u *UnimplementedAPI) GetAccount(ctx context.Context, address flow.Address) (*flow.Account, error) {
	return nil, status.Error(codes.Unimplemented, "method GetAccount not implemented")
}

// GetAccountAtLatestBlock returns an unimplemented error.
func (u *UnimplementedAPI) GetAccountAtLatestBlock(ctx context.Context, address flow.Address) (*flow.Account, error) {
	return nil, status.Error(codes.Unimplemented, "method GetAccountAtLatestBlock not implemented")
}

// GetAccountAtBlockHeight returns an unimplemented error.
func (u *UnimplementedAPI) GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64) (*flow.Account, error) {
	return nil, status.Error(codes.Unimplemented, "method GetAccountAtBlockHeight not implemented")
}

// GetAccountBalanceAtLatestBlock returns an unimplemented error.
func (u *UnimplementedAPI) GetAccountBalanceAtLatestBlock(ctx context.Context, address flow.Address) (uint64, error) {
	return 0, status.Error(codes.Unimplemented, "method GetAccountBalanceAtLatestBlock not implemented")
}

// GetAccountBalanceAtBlockHeight returns an unimplemented error.
func (u *UnimplementedAPI) GetAccountBalanceAtBlockHeight(ctx context.Context, address flow.Address, height uint64) (uint64, error) {
	return 0, status.Error(codes.Unimplemented, "method GetAccountBalanceAtBlockHeight not implemented")
}

// GetAccountKeyAtLatestBlock returns an unimplemented error.
func (u *UnimplementedAPI) GetAccountKeyAtLatestBlock(ctx context.Context, address flow.Address, keyIndex uint32) (*flow.AccountPublicKey, error) {
	return nil, status.Error(codes.Unimplemented, "method GetAccountKeyAtLatestBlock not implemented")
}

// GetAccountKeyAtBlockHeight returns an unimplemented error.
func (u *UnimplementedAPI) GetAccountKeyAtBlockHeight(ctx context.Context, address flow.Address, keyIndex uint32, height uint64) (*flow.AccountPublicKey, error) {
	return nil, status.Error(codes.Unimplemented, "method GetAccountKeyAtBlockHeight not implemented")
}

// GetAccountKeysAtLatestBlock returns an unimplemented error.
func (u *UnimplementedAPI) GetAccountKeysAtLatestBlock(ctx context.Context, address flow.Address) ([]flow.AccountPublicKey, error) {
	return nil, status.Error(codes.Unimplemented, "method GetAccountKeysAtLatestBlock not implemented")
}

// GetAccountKeysAtBlockHeight returns an unimplemented error.
func (u *UnimplementedAPI) GetAccountKeysAtBlockHeight(ctx context.Context, address flow.Address, height uint64) ([]flow.AccountPublicKey, error) {
	return nil, status.Error(codes.Unimplemented, "method GetAccountKeysAtBlockHeight not implemented")
}

// GetEventsForHeightRange returns an unimplemented error.
func (u *UnimplementedAPI) GetEventsForHeightRange(
	ctx context.Context,
	eventType string,
	startHeight,
	endHeight uint64,
	requiredEventEncodingVersion entities.EventEncodingVersion,
	criteria optimistic_sync.Criteria,
) ([]flow.BlockEvents, *accessmodel.ExecutorMetadata, error) {
	return nil, nil, status.Error(codes.Unimplemented, "method GetEventsForHeightRange not implemented")
}

// GetEventsForBlockIDs returns an unimplemented error.
func (u *UnimplementedAPI) GetEventsForBlockIDs(
	ctx context.Context,
	eventType string,
	blockIDs []flow.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
	criteria optimistic_sync.Criteria,
) ([]flow.BlockEvents, *accessmodel.ExecutorMetadata, error) {
	return nil, nil, status.Error(codes.Unimplemented, "method GetEventsForBlockIDs not implemented")
}

// ExecuteScriptAtLatestBlock returns an unimplemented error.
func (u *UnimplementedAPI) ExecuteScriptAtLatestBlock(ctx context.Context, script []byte, arguments [][]byte) ([]byte, error) {
	return nil, status.Error(codes.Unimplemented, "method ExecuteScriptAtLatestBlock not implemented")
}

// ExecuteScriptAtBlockHeight returns an unimplemented error.
func (u *UnimplementedAPI) ExecuteScriptAtBlockHeight(ctx context.Context, blockHeight uint64, script []byte, arguments [][]byte) ([]byte, error) {
	return nil, status.Error(codes.Unimplemented, "method ExecuteScriptAtBlockHeight not implemented")
}

// ExecuteScriptAtBlockID returns an unimplemented error.
func (u *UnimplementedAPI) ExecuteScriptAtBlockID(ctx context.Context, blockID flow.Identifier, script []byte, arguments [][]byte) ([]byte, error) {
	return nil, status.Error(codes.Unimplemented, "method ExecuteScriptAtBlockID not implemented")
}

// SendTransaction returns an unimplemented error.
func (u *UnimplementedAPI) SendTransaction(ctx context.Context, tx *flow.TransactionBody) error {
	return status.Error(codes.Unimplemented, "method SendTransaction not implemented")
}

// GetTransaction returns an unimplemented error.
func (u *UnimplementedAPI) GetTransaction(ctx context.Context, id flow.Identifier) (*flow.TransactionBody, error) {
	return nil, status.Error(codes.Unimplemented, "method GetTransaction not implemented")
}

// GetTransactionsByBlockID returns an unimplemented error.
func (u *UnimplementedAPI) GetTransactionsByBlockID(ctx context.Context, blockID flow.Identifier) ([]*flow.TransactionBody, error) {
	return nil, status.Error(codes.Unimplemented, "method GetTransactionsByBlockID not implemented")
}

// GetTransactionResult returns an unimplemented error.
func (u *UnimplementedAPI) GetTransactionResult(ctx context.Context, txID flow.Identifier, blockID flow.Identifier, collectionID flow.Identifier, encodingVersion entities.EventEncodingVersion) (*accessmodel.TransactionResult, error) {
	return nil, status.Error(codes.Unimplemented, "method GetTransactionResult not implemented")
}

// GetTransactionResultByIndex returns an unimplemented error.
func (u *UnimplementedAPI) GetTransactionResultByIndex(ctx context.Context, blockID flow.Identifier, index uint32, encodingVersion entities.EventEncodingVersion) (*accessmodel.TransactionResult, error) {
	return nil, status.Error(codes.Unimplemented, "method GetTransactionResultByIndex not implemented")
}

// GetTransactionResultsByBlockID returns an unimplemented error.
func (u *UnimplementedAPI) GetTransactionResultsByBlockID(ctx context.Context, blockID flow.Identifier, encodingVersion entities.EventEncodingVersion) ([]*accessmodel.TransactionResult, error) {
	return nil, status.Error(codes.Unimplemented, "method GetTransactionResultsByBlockID not implemented")
}

// GetSystemTransaction returns an unimplemented error.
func (u *UnimplementedAPI) GetSystemTransaction(ctx context.Context, txID flow.Identifier, blockID flow.Identifier) (*flow.TransactionBody, error) {
	return nil, status.Error(codes.Unimplemented, "method GetSystemTransaction not implemented")
}

// GetSystemTransactionResult returns an unimplemented error.
func (u *UnimplementedAPI) GetSystemTransactionResult(ctx context.Context, txID flow.Identifier, blockID flow.Identifier, encodingVersion entities.EventEncodingVersion) (*accessmodel.TransactionResult, error) {
	return nil, status.Error(codes.Unimplemented, "method GetSystemTransactionResult not implemented")
}

// GetScheduledTransaction returns an unimplemented error.
func (u *UnimplementedAPI) GetScheduledTransaction(ctx context.Context, scheduledTxID uint64) (*flow.TransactionBody, error) {
	return nil, status.Error(codes.Unimplemented, "method GetScheduledTransaction not implemented")
}

// GetScheduledTransactionResult returns an unimplemented error.
func (u *UnimplementedAPI) GetScheduledTransactionResult(ctx context.Context, scheduledTxID uint64, encodingVersion entities.EventEncodingVersion) (*accessmodel.TransactionResult, error) {
	return nil, status.Error(codes.Unimplemented, "method GetScheduledTransactionResult not implemented")
}

// SubscribeTransactionStatuses returns a failed subscription.
func (u *UnimplementedAPI) SubscribeTransactionStatuses(
	ctx context.Context,
	txID flow.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) subscription.Subscription {
	msg := "method SubscribeTransactionStatuses not implemented"
	return subscription.NewFailedSubscription(status.Error(codes.Unimplemented, msg), msg)
}

// SendAndSubscribeTransactionStatuses returns a failed subscription.
func (u *UnimplementedAPI) SendAndSubscribeTransactionStatuses(
	ctx context.Context,
	tx *flow.TransactionBody,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) subscription.Subscription {
	msg := "method SendAndSubscribeTransactionStatuses not implemented"
	return subscription.NewFailedSubscription(status.Error(codes.Unimplemented, msg), msg)
}

// Ping returns an unimplemented error.
func (u *UnimplementedAPI) Ping(ctx context.Context) error {
	return status.Error(codes.Unimplemented, "method Ping not implemented")
}

// GetNetworkParameters returns an unimplemented error.
func (u *UnimplementedAPI) GetNetworkParameters(ctx context.Context) accessmodel.NetworkParameters {
	return accessmodel.NetworkParameters{}
}

// GetNodeVersionInfo returns an unimplemented error.
func (u *UnimplementedAPI) GetNodeVersionInfo(ctx context.Context) (*accessmodel.NodeVersionInfo, error) {
	return nil, status.Error(codes.Unimplemented, "method GetNodeVersionInfo not implemented")
}

// GetLatestBlockHeader returns an unimplemented error.
func (u *UnimplementedAPI) GetLatestBlockHeader(ctx context.Context, isSealed bool) (*flow.Header, flow.BlockStatus, error) {
	return nil, flow.BlockStatusUnknown, status.Error(codes.Unimplemented, "method GetLatestBlockHeader not implemented")
}

// GetBlockHeaderByHeight returns an unimplemented error.
func (u *UnimplementedAPI) GetBlockHeaderByHeight(ctx context.Context, height uint64) (*flow.Header, flow.BlockStatus, error) {
	return nil, flow.BlockStatusUnknown, status.Error(codes.Unimplemented, "method GetBlockHeaderByHeight not implemented")
}

// GetBlockHeaderByID returns an unimplemented error.
func (u *UnimplementedAPI) GetBlockHeaderByID(ctx context.Context, id flow.Identifier) (*flow.Header, flow.BlockStatus, error) {
	return nil, flow.BlockStatusUnknown, status.Error(codes.Unimplemented, "method GetBlockHeaderByID not implemented")
}

// GetLatestBlock returns an unimplemented error.
func (u *UnimplementedAPI) GetLatestBlock(ctx context.Context, isSealed bool) (*flow.Block, flow.BlockStatus, error) {
	return nil, flow.BlockStatusUnknown, status.Error(codes.Unimplemented, "method GetLatestBlock not implemented")
}

// GetBlockByHeight returns an unimplemented error.
func (u *UnimplementedAPI) GetBlockByHeight(ctx context.Context, height uint64) (*flow.Block, flow.BlockStatus, error) {
	return nil, flow.BlockStatusUnknown, status.Error(codes.Unimplemented, "method GetBlockByHeight not implemented")
}

// GetBlockByID returns an unimplemented error.
func (u *UnimplementedAPI) GetBlockByID(ctx context.Context, id flow.Identifier) (*flow.Block, flow.BlockStatus, error) {
	return nil, flow.BlockStatusUnknown, status.Error(codes.Unimplemented, "method GetBlockByID not implemented")
}

// GetCollectionByID returns an unimplemented error.
func (u *UnimplementedAPI) GetCollectionByID(ctx context.Context, id flow.Identifier) (*flow.LightCollection, error) {
	return nil, status.Error(codes.Unimplemented, "method GetCollectionByID not implemented")
}

// GetFullCollectionByID returns an unimplemented error.
func (u *UnimplementedAPI) GetFullCollectionByID(ctx context.Context, id flow.Identifier) (*flow.Collection, error) {
	return nil, status.Error(codes.Unimplemented, "method GetFullCollectionByID not implemented")
}

// GetLatestProtocolStateSnapshot returns an unimplemented error.
func (u *UnimplementedAPI) GetLatestProtocolStateSnapshot(ctx context.Context) ([]byte, error) {
	return nil, status.Error(codes.Unimplemented, "method GetLatestProtocolStateSnapshot not implemented")
}

// GetProtocolStateSnapshotByBlockID returns an unimplemented error.
func (u *UnimplementedAPI) GetProtocolStateSnapshotByBlockID(ctx context.Context, blockID flow.Identifier) ([]byte, error) {
	return nil, status.Error(codes.Unimplemented, "method GetProtocolStateSnapshotByBlockID not implemented")
}

// GetProtocolStateSnapshotByHeight returns an unimplemented error.
func (u *UnimplementedAPI) GetProtocolStateSnapshotByHeight(ctx context.Context, blockHeight uint64) ([]byte, error) {
	return nil, status.Error(codes.Unimplemented, "method GetProtocolStateSnapshotByHeight not implemented")
}

// GetExecutionResultForBlockID returns an unimplemented error.
func (u *UnimplementedAPI) GetExecutionResultForBlockID(ctx context.Context, blockID flow.Identifier) (*flow.ExecutionResult, error) {
	return nil, status.Error(codes.Unimplemented, "method GetExecutionResultForBlockID not implemented")
}

// GetExecutionResultByID returns an unimplemented error.
func (u *UnimplementedAPI) GetExecutionResultByID(ctx context.Context, id flow.Identifier) (*flow.ExecutionResult, error) {
	return nil, status.Error(codes.Unimplemented, "method GetExecutionResultByID not implemented")
}

// SubscribeBlocksFromStartBlockID returns a failed subscription.
func (u *UnimplementedAPI) SubscribeBlocksFromStartBlockID(ctx context.Context, startBlockID flow.Identifier, blockStatus flow.BlockStatus) subscription.Subscription {
	msg := "method SubscribeBlocksFromStartBlockID not implemented"
	return subscription.NewFailedSubscription(status.Error(codes.Unimplemented, msg), msg)
}

// SubscribeBlocksFromStartHeight returns a failed subscription.
func (u *UnimplementedAPI) SubscribeBlocksFromStartHeight(ctx context.Context, startHeight uint64, blockStatus flow.BlockStatus) subscription.Subscription {
	msg := "method SubscribeBlocksFromStartHeight not implemented"
	return subscription.NewFailedSubscription(status.Error(codes.Unimplemented, msg), msg)
}

// SubscribeBlocksFromLatest returns a failed subscription.
func (u *UnimplementedAPI) SubscribeBlocksFromLatest(ctx context.Context, blockStatus flow.BlockStatus) subscription.Subscription {
	msg := "method SubscribeBlocksFromLatest not implemented"
	return subscription.NewFailedSubscription(status.Error(codes.Unimplemented, msg), msg)
}

// SubscribeBlockHeadersFromStartBlockID returns a failed subscription.
func (u *UnimplementedAPI) SubscribeBlockHeadersFromStartBlockID(ctx context.Context, startBlockID flow.Identifier, blockStatus flow.BlockStatus) subscription.Subscription {
	msg := "method SubscribeBlockHeadersFromStartBlockID not implemented"
	return subscription.NewFailedSubscription(status.Error(codes.Unimplemented, msg), msg)
}

// SubscribeBlockHeadersFromStartHeight returns a failed subscription.
func (u *UnimplementedAPI) SubscribeBlockHeadersFromStartHeight(ctx context.Context, startHeight uint64, blockStatus flow.BlockStatus) subscription.Subscription {
	msg := "method SubscribeBlockHeadersFromStartHeight not implemented"
	return subscription.NewFailedSubscription(status.Error(codes.Unimplemented, msg), msg)
}

// SubscribeBlockHeadersFromLatest returns a failed subscription.
func (u *UnimplementedAPI) SubscribeBlockHeadersFromLatest(ctx context.Context, blockStatus flow.BlockStatus) subscription.Subscription {
	msg := "method SubscribeBlockHeadersFromLatest not implemented"
	return subscription.NewFailedSubscription(status.Error(codes.Unimplemented, msg), msg)
}

// SubscribeBlockDigestsFromStartBlockID returns a failed subscription.
func (u *UnimplementedAPI) SubscribeBlockDigestsFromStartBlockID(ctx context.Context, startBlockID flow.Identifier, blockStatus flow.BlockStatus) subscription.Subscription {
	msg := "method SubscribeBlockDigestsFromStartBlockID not implemented"
	return subscription.NewFailedSubscription(status.Error(codes.Unimplemented, msg), msg)
}

// SubscribeBlockDigestsFromStartHeight returns a failed subscription.
func (u *UnimplementedAPI) SubscribeBlockDigestsFromStartHeight(ctx context.Context, startHeight uint64, blockStatus flow.BlockStatus) subscription.Subscription {
	msg := "method SubscribeBlockDigestsFromStartHeight not implemented"
	return subscription.NewFailedSubscription(status.Error(codes.Unimplemented, msg), msg)
}

// SubscribeBlockDigestsFromLatest returns a failed subscription.
func (u *UnimplementedAPI) SubscribeBlockDigestsFromLatest(ctx context.Context, blockStatus flow.BlockStatus) subscription.Subscription {
	msg := "method SubscribeBlockDigestsFromLatest not implemented"
	return subscription.NewFailedSubscription(status.Error(codes.Unimplemented, msg), msg)
}
