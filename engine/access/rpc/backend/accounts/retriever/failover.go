package retriever

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

type FailoverAccountRetriever struct {
	log               zerolog.Logger
	state             protocol.State
	localRequester    AccountRetriever
	execNodeRequester AccountRetriever
}

var _ AccountRetriever = (*FailoverAccountRetriever)(nil)

func NewFailoverAccountRetriever(
	log zerolog.Logger,
	state protocol.State,
	localRequester AccountRetriever,
	execNodeRequester AccountRetriever,
) *FailoverAccountRetriever {
	return &FailoverAccountRetriever{
		log:               log.With().Str("account_retriever", "failover").Logger(),
		state:             state,
		localRequester:    localRequester,
		execNodeRequester: execNodeRequester,
	}
}

func (f *FailoverAccountRetriever) GetAccountAtBlock(
	ctx context.Context,
	address flow.Address,
	blockID flow.Identifier,
	height uint64, //TODO: fix ALL places with unused arguments
) (*flow.Account, error) {
	localAccount, localErr := f.localRequester.GetAccountAtBlock(ctx, address, blockID, height)
	if localErr == nil {
		return localAccount, nil
	}

	ENAccount, ENErr := f.execNodeRequester.GetAccountAtBlock(ctx, address, blockID, height)
	return ENAccount, ENErr
}

func (f *FailoverAccountRetriever) GetAccountBalanceAtBlock(
	ctx context.Context,
	address flow.Address,
	blockID flow.Identifier,
	height uint64,
) (uint64, error) {
	localBalance, localErr := f.localRequester.GetAccountBalanceAtBlock(ctx, address, blockID, height)
	if localErr == nil {
		return localBalance, nil
	}

	ENBalance, ENErr := f.execNodeRequester.GetAccountBalanceAtBlock(ctx, address, blockID, height)
	if ENErr != nil {
		return 0, ENErr
	}

	return ENBalance, nil
}

func (f *FailoverAccountRetriever) GetAccountKeyAtBlock(
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

	ENKey, ENErr := f.execNodeRequester.GetAccountKeyAtBlock(ctx, address, keyIndex, blockID, height)
	if ENErr != nil {
		return nil, ENErr
	}

	return ENKey, nil
}

func (f *FailoverAccountRetriever) GetAccountKeysAtBlock(
	ctx context.Context,
	address flow.Address,
	blockID flow.Identifier,
	height uint64,
) ([]flow.AccountPublicKey, error) {
	localKeys, localErr := f.localRequester.GetAccountKeysAtBlock(ctx, address, blockID, height)
	if localErr == nil {
		return localKeys, nil
	}

	ENKeys, ENErr := f.execNodeRequester.GetAccountKeysAtBlock(ctx, address, blockID, height)
	if ENErr != nil {
		return nil, ENErr
	}

	return ENKeys, nil
}
