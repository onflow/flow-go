package handler

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
)

type Failover struct {
	log               zerolog.Logger
	state             protocol.State
	localRequester    Handler
	execNodeRequester Handler
}

var _ Handler = (*Failover)(nil)

func NewFailoverHandler(
	log zerolog.Logger,
	state protocol.State,
	localRequester Handler,
	execNodeRequester Handler,
) *Failover {
	return &Failover{
		log:               zerolog.New(log).With().Str("handler", "failover").Logger(),
		state:             state,
		localRequester:    localRequester,
		execNodeRequester: execNodeRequester,
	}
}

func (f *Failover) GetAccount(ctx context.Context, address flow.Address) (*flow.Account, error) {
	return f.GetAccountAtLatestBlock(ctx, address)
}

func (f *Failover) GetAccountAtLatestBlock(ctx context.Context, address flow.Address) (*flow.Account, error) {
	sealed, err := f.state.Sealed().Head()
	if err != nil {
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, err
	}

	sealedBlockID := sealed.ID()
	account, err := f.GetAccountAtBlockHeight(ctx, address, sealedBlockID, sealed.Height)
	if err != nil {
		f.log.Debug().Err(err).Msgf("failed to get account at latest block")
		return nil, err
	}

	return account, nil
}

func (f *Failover) GetAccountAtBlockHeight(
	ctx context.Context,
	address flow.Address,
	_ flow.Identifier,
	_ uint64, //TODO: fix ALL places with unused arguments
) (*flow.Account, error) {
	localAccount, localErr := f.localRequester.GetAccount(ctx, address)
	if localErr == nil {
		return localAccount, nil
	}

	ENAccount, ENErr := f.execNodeRequester.GetAccount(ctx, address)

	//TODO: I commented this function call. Ask Peter if this is OK
	// It is supposed to be called only in handler.Compare handler, isn't it?
	//f.compareAccountResults(ENAccount, ENErr, localAccount, localErr, blockID, address)

	return ENAccount, ENErr
}

func (f *Failover) GetAccountBalanceAtLatestBlock(ctx context.Context, address flow.Address) (uint64, error) {
	sealed, err := f.state.Sealed().Head()
	if err != nil {
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return 0, err
	}

	sealedBlockID := sealed.ID()
	balance, err := f.GetAccountBalanceAtBlockHeight(ctx, address, sealedBlockID, sealed.Height)
	if err != nil {
		f.log.Debug().Err(err).Msgf("failed to get account balance at latest block")
		return 0, err
	}

	return balance, nil
}

func (f *Failover) GetAccountBalanceAtBlockHeight(
	ctx context.Context,
	address flow.Address,
	blockID flow.Identifier,
	height uint64,
) (uint64, error) {
	localBalance, localErr := f.localRequester.GetAccountBalanceAtBlockHeight(ctx, address, blockID, height)
	if localErr == nil {
		return localBalance, nil
	}

	ENBalance, ENErr := f.execNodeRequester.GetAccountBalanceAtBlockHeight(ctx, address, blockID, height)
	if ENErr != nil {
		return 0, ENErr
	}

	return ENBalance, nil
}

func (f *Failover) GetAccountKeyAtLatestBlock(
	ctx context.Context,
	address flow.Address,
	keyIndex uint32,
) (*flow.AccountPublicKey, error) {
	sealed, err := f.state.Sealed().Head()
	if err != nil {
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, err
	}

	sealedBlockID := sealed.ID()
	key, err := f.GetAccountKeyAtBlockHeight(ctx, address, keyIndex, sealedBlockID, sealed.Height)
	if err != nil {
		f.log.Debug().Err(err).Msgf("failed to get account key at latest block")
		return nil, err
	}

	return key, nil
}

func (f *Failover) GetAccountKeyAtBlockHeight(
	ctx context.Context,
	address flow.Address,
	keyIndex uint32,
	blockID flow.Identifier,
	height uint64,
) (*flow.AccountPublicKey, error) {
	localKey, localErr := f.localRequester.GetAccountKeyAtBlockHeight(ctx, address, keyIndex, blockID, height)
	if localErr == nil {
		return localKey, nil
	}

	ENKey, ENErr := f.execNodeRequester.GetAccountKeyAtBlockHeight(ctx, address, keyIndex, blockID, height)
	if ENErr != nil {
		return nil, ENErr
	}

	return ENKey, nil
}

func (f *Failover) GetAccountKeysAtLatestBlock(
	ctx context.Context,
	address flow.Address,
) ([]flow.AccountPublicKey, error) {
	sealed, err := f.state.Sealed().Head()
	if err != nil {
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, err
	}

	sealedBlockID := sealed.ID()
	keys, err := f.GetAccountKeysAtBlockHeight(ctx, address, sealedBlockID, sealed.Height)
	if err != nil {
		f.log.Debug().Err(err).Msgf("failed to get account keys at latest block")
		return nil, err
	}

	return keys, nil
}

func (f *Failover) GetAccountKeysAtBlockHeight(
	ctx context.Context,
	address flow.Address,
	blockID flow.Identifier,
	height uint64,
) ([]flow.AccountPublicKey, error) {
	localKeys, localErr := f.localRequester.GetAccountKeysAtBlockHeight(ctx, address, blockID, height)
	if localErr == nil {
		return localKeys, nil
	}

	ENKeys, ENErr := f.execNodeRequester.GetAccountKeysAtBlockHeight(ctx, address, blockID, height)
	if ENErr != nil {
		return nil, ENErr
	}

	return ENKeys, nil
}
