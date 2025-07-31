package provider

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
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
) (*flow.Account, error) {
	localAccount, localErr := f.localRequester.GetAccountAtBlock(ctx, address, blockID, height)
	if localErr == nil {
		return localAccount, nil
	}

	execNodeAccount, execNodeErr := f.execNodeRequester.GetAccountAtBlock(ctx, address, blockID, height)
	return execNodeAccount, execNodeErr
}

func (f *FailoverAccountProvider) GetAccountBalanceAtBlock(
	ctx context.Context,
	address flow.Address,
	blockID flow.Identifier,
	height uint64,
) (uint64, error) {
	localBalance, localErr := f.localRequester.GetAccountBalanceAtBlock(ctx, address, blockID, height)
	if localErr == nil {
		return localBalance, nil
	}

	execNodeBalance, execNodeErr := f.execNodeRequester.GetAccountBalanceAtBlock(ctx, address, blockID, height)
	if execNodeErr != nil {
		return 0, execNodeErr
	}

	return execNodeBalance, nil
}

func (f *FailoverAccountProvider) GetAccountKeyAtBlock(
	ctx context.Context,
	address flow.Address,
	keyIndex uint32,
	blockID flow.Identifier,
	height uint64,
) (*flow.AccountPublicKey, error) {
	localKey, localErr := f.localRequester.GetAccountKeyAtBlock(ctx, address, keyIndex, blockID, height)
	if localErr == nil {
		return localKey, nil
	}

	execNodeKey, execNodeErr := f.execNodeRequester.GetAccountKeyAtBlock(ctx, address, keyIndex, blockID, height)
	if execNodeErr != nil {
		return nil, execNodeErr
	}

	return execNodeKey, nil
}

func (f *FailoverAccountProvider) GetAccountKeysAtBlock(
	ctx context.Context,
	address flow.Address,
	blockID flow.Identifier,
	height uint64,
) ([]flow.AccountPublicKey, error) {
	localKeys, localErr := f.localRequester.GetAccountKeysAtBlock(ctx, address, blockID, height)
	if localErr == nil {
		return localKeys, nil
	}

	execNodeKeys, execNodeErr := f.execNodeRequester.GetAccountKeysAtBlock(ctx, address, blockID, height)
	if execNodeErr != nil {
		return nil, execNodeErr
	}

	return execNodeKeys, nil
}
