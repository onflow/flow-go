package handler

import (
	"bytes"
	"context"
	"errors"

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
		log:               log,
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
	blockID flow.Identifier,
	_ uint64, //TODO: fix ALL places with unused arguments
) (*flow.Account, error) {
	localAccount, localErr := f.localRequester.GetAccount(ctx, address)
	if localErr == nil {
		return localAccount, nil
	}

	ENAccount, ENErr := f.execNodeRequester.GetAccount(ctx, address)

	//TODO: should we call this in this handler?
	// It is supposed to be called only in handler.Compare handler, isn't it?
	f.compareAccountResults(ENAccount, ENErr, localAccount, localErr, blockID, address)

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

// compareAccountResults compares the result and error returned from local and remote getAccount calls
// and logs the results if they are different
func (f *Failover) compareAccountResults(
	execNodeResult *flow.Account,
	execErr error,
	localResult *flow.Account,
	localErr error,
	blockID flow.Identifier,
	address flow.Address,
) {
	if f.log.GetLevel() > zerolog.DebugLevel {
		return
	}

	lgCtx := f.log.With().
		Hex("block_id", blockID[:]).
		Str("address", address.String())

	// errors are different
	if !errors.Is(execErr, localErr) {
		lgCtx = lgCtx.
			AnErr("execution_node_error", execErr).
			AnErr("local_error", localErr)

		lg := lgCtx.Logger()
		lg.Debug().Msg("errors from getting account on local and EN do not match")
		return
	}

	// both errors are nil, compare the accounts
	if execErr == nil {
		lgCtx, ok := compareAccountsLogger(execNodeResult, localResult, lgCtx)
		if !ok {
			lg := lgCtx.Logger()
			lg.Debug().Msg("accounts from local and EN do not match")
		}
	}
}

// compareAccountsLogger compares accounts produced by the execution node and local storage and
// return a logger configured to log the differences
func compareAccountsLogger(exec, local *flow.Account, lgCtx zerolog.Context) (zerolog.Context, bool) {
	different := false

	if exec.Address != local.Address {
		lgCtx = lgCtx.
			Str("exec_node_address", exec.Address.String()).
			Str("local_address", local.Address.String())
		different = true
	}

	if exec.Balance != local.Balance {
		lgCtx = lgCtx.
			Uint64("exec_node_balance", exec.Balance).
			Uint64("local_balance", local.Balance)
		different = true
	}

	contractListMatches := true
	if len(exec.Contracts) != len(local.Contracts) {
		lgCtx = lgCtx.
			Int("exec_node_contract_count", len(exec.Contracts)).
			Int("local_contract_count", len(local.Contracts))
		contractListMatches = false
		different = true
	}

	missingContracts := zerolog.Arr()
	mismatchContracts := zerolog.Arr()

	for name, execContract := range exec.Contracts {
		localContract, ok := local.Contracts[name]

		if !ok {
			missingContracts.Str(name)
			contractListMatches = false
			different = true
		}

		if !bytes.Equal(execContract, localContract) {
			mismatchContracts.Str(name)
			different = true
		}
	}

	lgCtx = lgCtx.
		Array("missing_contracts", missingContracts).
		Array("mismatch_contracts", mismatchContracts)

	// only check if there were any missing
	if !contractListMatches {
		extraContracts := zerolog.Arr()
		for name := range local.Contracts {
			if _, ok := exec.Contracts[name]; !ok {
				extraContracts.Str(name)
				different = true
			}
		}
		lgCtx = lgCtx.Array("extra_contracts", extraContracts)
	}

	if len(exec.Keys) != len(local.Keys) {
		lgCtx = lgCtx.
			Int("exec_node_key_count", len(exec.Keys)).
			Int("local_key_count", len(local.Keys))
		different = true
	}

	mismatchKeys := zerolog.Arr()

	for i, execKey := range exec.Keys {
		localKey := local.Keys[i]

		if !execKey.PublicKey.Equals(localKey.PublicKey) {
			mismatchKeys.Uint32(execKey.Index)
			different = true
		}
	}

	lgCtx = lgCtx.Array("mismatch_keys", mismatchKeys)

	return lgCtx, !different
}
