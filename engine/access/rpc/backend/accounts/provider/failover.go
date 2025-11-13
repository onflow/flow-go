package provider

import (
	"context"

	"github.com/rs/zerolog"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/state/protocol"
)

type FailoverAccountProvider struct {
	log               zerolog.Logger
	state             protocol.State
	localRequester    AccountProvider
	execNodeRequester AccountProvider
}

var _ AccountProvider = (*FailoverAccountProvider)(nil)

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
