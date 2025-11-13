package provider

import (
	"context"

	"github.com/rs/zerolog"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/state/protocol"
)

// FailoverAccountProvider retrieves account data using local storage first,
// then falls back to execution nodes if data is unavailable or a non-user error occurs.
type FailoverAccountProvider struct {
	log               zerolog.Logger
	state             protocol.State
	localRequester    AccountProvider
	execNodeRequester AccountProvider
}

var _ AccountProvider = (*FailoverAccountProvider)(nil)

// NewFailoverAccountProvider creates a new instance of FailoverAccountProvider.
func NewFailoverAccountProvider(
	log zerolog.Logger,
	state protocol.State,
	localRequester AccountProvider,
	execNodeRequester AccountProvider,
) *FailoverAccountProvider {
	return &FailoverAccountProvider{
		log:               log.With().Str("account_provider", "failover").Logger(),
		state:             state,
		localRequester:    localRequester,
		execNodeRequester: execNodeRequester,
	}
}

// GetAccountAtBlock returns a Flow account for the given address and block height.
//
// Expected error returns during normal operation:
//   - [access.InvalidRequestError] - if the request failed due to invalid arguments or runtime errors.
//   - [access.ResourceExhausted] - if computation or memory limits were exceeded.
//   - [access.DataNotFoundError] - if data was not found.
//   - [access.OutOfRangeError] - if the data for the requested height is outside the node's available range.
//   - [access.PreconditionFailedError] - if the registers storage is still bootstrapping.
//   - [access.RequestCanceledError] - if the request was canceled.
//   - [access.RequestTimedOutError] - if the request timed out.
//   - [access.InternalError] - for internal failures or index conversion errors.
func (f *FailoverAccountProvider) GetAccountAtBlock(
	ctx context.Context,
	address flow.Address,
	blockID flow.Identifier,
	height uint64,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (*flow.Account, *accessmodel.ExecutorMetadata, error) {
	localAccount, localMetadata, localErr := f.localRequester.GetAccountAtBlock(ctx, address, blockID, height, executionResultInfo)
	if localErr == nil {
		return localAccount, localMetadata, nil
	}

	return f.execNodeRequester.GetAccountAtBlock(ctx, address, blockID, height, executionResultInfo)
}

// GetAccountBalanceAtBlock returns the balance of a Flow account
// at the given block height.
//
// Expected error returns during normal operation:
//   - [access.InvalidRequestError] - if the request failed due to invalid arguments or runtime errors.
//   - [access.ResourceExhausted] - if computation or memory limits were exceeded.
//   - [access.DataNotFoundError] - if data was not found.
//   - [access.OutOfRangeError] - if the data for the requested height is outside the node's available range.
//   - [access.PreconditionFailedError] - if the registers storage is still bootstrapping.
//   - [access.RequestCanceledError] - if the request was canceled.
//   - [access.RequestTimedOutError] - if the request timed out.
//   - [access.InternalError] - for internal failures or index conversion errors.
func (f *FailoverAccountProvider) GetAccountBalanceAtBlock(
	ctx context.Context,
	address flow.Address,
	blockID flow.Identifier,
	height uint64,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (uint64, *accessmodel.ExecutorMetadata, error) {
	localBalance, localMetadata, localErr := f.localRequester.GetAccountBalanceAtBlock(ctx, address, blockID, height, executionResultInfo)
	if localErr == nil {
		return localBalance, localMetadata, nil
	}

	return f.execNodeRequester.GetAccountBalanceAtBlock(ctx, address, blockID, height, executionResultInfo)
}

// GetAccountKeyAtBlock returns a specific public key of a Flow account
// by its key index and block height.
//
// Expected error returns during normal operation:
//   - [access.InvalidRequestError] - if the request failed due to invalid arguments or runtime errors.
//   - [access.ResourceExhausted] - if computation or memory limits were exceeded.
//   - [access.DataNotFoundError] - if data was not found.
//   - [access.OutOfRangeError] - if the data for the requested height is outside the node's available range.
//   - [access.PreconditionFailedError] - if the registers storage is still bootstrapping.
//   - [access.RequestCanceledError] - if the request was canceled.
//   - [access.RequestTimedOutError] - if the request timed out.
//   - [access.InternalError] - for internal failures or index conversion errors.
func (f *FailoverAccountProvider) GetAccountKeyAtBlock(
	ctx context.Context,
	address flow.Address,
	keyIndex uint32,
	blockID flow.Identifier,
	height uint64,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (*flow.AccountPublicKey, *accessmodel.ExecutorMetadata, error) {
	localKey, localMetadata, localErr := f.localRequester.GetAccountKeyAtBlock(ctx, address, keyIndex, blockID, height, executionResultInfo)
	if localErr == nil {
		return localKey, localMetadata, nil
	}

	return f.execNodeRequester.GetAccountKeyAtBlock(ctx, address, keyIndex, blockID, height, executionResultInfo)
}

// GetAccountKeysAtBlock returns all public keys associated with a Flow account
// at the given block height.
//
// Expected error returns during normal operation:
//   - [access.InvalidRequestError] - if the request failed due to invalid arguments or runtime errors.
//   - [access.ResourceExhausted] - if computation or memory limits were exceeded.
//   - [access.DataNotFoundError] - if data was not found.
//   - [access.OutOfRangeError] - if the data for the requested height is outside the node's available range.
//   - [access.PreconditionFailedError] - if the registers storage is still bootstrapping.
//   - [access.RequestCanceledError] - if the request was canceled.
//   - [access.RequestTimedOutError] - if the request timed out.
//   - [access.InternalError] - for internal failures or index conversion errors.
func (f *FailoverAccountProvider) GetAccountKeysAtBlock(
	ctx context.Context,
	address flow.Address,
	blockID flow.Identifier,
	height uint64,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) ([]flow.AccountPublicKey, *accessmodel.ExecutorMetadata, error) {
	localKeys, localMetadata, localErr := f.localRequester.GetAccountKeysAtBlock(ctx, address, blockID, height, executionResultInfo)
	if localErr == nil {
		return localKeys, localMetadata, nil
	}

	return f.execNodeRequester.GetAccountKeysAtBlock(ctx, address, blockID, height, executionResultInfo)
}
