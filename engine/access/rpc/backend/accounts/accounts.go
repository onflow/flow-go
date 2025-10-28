package accounts

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rpc/backend/accounts/provider"
	"github.com/onflow/flow-go/engine/access/rpc/backend/common"
	"github.com/onflow/flow-go/engine/access/rpc/backend/node_communicator"
	"github.com/onflow/flow-go/engine/access/rpc/backend/query_mode"
	"github.com/onflow/flow-go/engine/access/rpc/connection"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type Accounts struct {
	log      zerolog.Logger
	state    protocol.State
	headers  storage.Headers
	provider provider.AccountProvider
}

var _ access.AccountsAPI = (*Accounts)(nil)

func NewAccountsBackend(
	log zerolog.Logger,
	state protocol.State,
	headers storage.Headers,
	connFactory connection.ConnectionFactory,
	nodeCommunicator node_communicator.Communicator,
	scriptExecMode query_mode.IndexQueryMode,
	scriptExecutor execution.ScriptExecutor,
	execNodeIdentitiesProvider *commonrpc.ExecutionNodeIdentitiesProvider,
	registersAsyncStore *execution.RegistersAsyncStore,
) (*Accounts, error) {
	var accountProvider provider.AccountProvider

	switch scriptExecMode {
	case query_mode.IndexQueryModeLocalOnly:
		accountProvider = provider.NewLocalAccountProvider(log, state, scriptExecutor, registersAsyncStore)

	case query_mode.IndexQueryModeExecutionNodesOnly:
		accountProvider = provider.NewENAccountProvider(log, state, connFactory, nodeCommunicator, execNodeIdentitiesProvider)

	case query_mode.IndexQueryModeFailover:
		local := provider.NewLocalAccountProvider(log, state, scriptExecutor, registersAsyncStore)
		execNode := provider.NewENAccountProvider(log, state, connFactory, nodeCommunicator, execNodeIdentitiesProvider)
		accountProvider = provider.NewFailoverAccountProvider(log, state, local, execNode)

	default:
		return nil, fmt.Errorf("unknown execution mode: %v", scriptExecMode)
	}

	return &Accounts{
		log:      log,
		state:    state,
		headers:  headers,
		provider: accountProvider,
	}, nil
}

// GetAccount returns the account details at the latest sealed block.
// Alias for GetAccountAtLatestBlock
func (a *Accounts) GetAccount(ctx context.Context, address flow.Address) (*flow.Account, error) {
	return a.GetAccountAtLatestBlock(ctx, address)
}

// GetAccountAtLatestBlock returns the account details at the latest sealed block.
func (a *Accounts) GetAccountAtLatestBlock(ctx context.Context, address flow.Address) (*flow.Account, error) {
	sealed, err := a.state.Sealed().Head()
	if err != nil {
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, err
	}

	sealedBlockID := sealed.ID()
	account, err := a.provider.GetAccountAtBlock(ctx, address, sealedBlockID, sealed.Height)
	if err != nil {
		a.log.Debug().Err(err).Msgf("failed to get account at blockID: %v", sealedBlockID)
		return nil, err
	}

	return account, nil
}

// GetAccountAtBlockHeight returns the account details at the given block height.
func (a *Accounts) GetAccountAtBlockHeight(
	ctx context.Context,
	address flow.Address,
	height uint64,
) (*flow.Account, error) {
	blockID, err := a.headers.BlockIDByHeight(height)
	if err != nil {
		return nil, commonrpc.ConvertStorageError(common.ResolveHeightError(a.state.Params(), height, err))
	}

	account, err := a.provider.GetAccountAtBlock(ctx, address, blockID, height)
	if err != nil {
		a.log.Debug().Err(err).Msgf("failed to get account at height: %d", height)
		return nil, err
	}

	return account, nil
}

// GetAccountBalanceAtLatestBlock returns the account balance at the latest sealed block.
func (a *Accounts) GetAccountBalanceAtLatestBlock(ctx context.Context, address flow.Address) (uint64, error) {
	sealed, err := a.state.Sealed().Head()
	if err != nil {
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return 0, err
	}

	sealedBlockID := sealed.ID()
	balance, err := a.provider.GetAccountBalanceAtBlock(ctx, address, sealedBlockID, sealed.Height)
	if err != nil {
		a.log.Debug().Err(err).Msgf("failed to get account balance at blockID: %v", sealedBlockID)
		return 0, err
	}

	return balance, nil
}

// GetAccountBalanceAtBlockHeight returns the account balance at the given block height.
func (a *Accounts) GetAccountBalanceAtBlockHeight(
	ctx context.Context,
	address flow.Address,
	height uint64,
) (uint64, error) {
	blockID, err := a.headers.BlockIDByHeight(height)
	if err != nil {
		return 0, commonrpc.ConvertStorageError(common.ResolveHeightError(a.state.Params(), height, err))
	}

	balance, err := a.provider.GetAccountBalanceAtBlock(ctx, address, blockID, height)
	if err != nil {
		a.log.Debug().Err(err).Msgf("failed to get account balance at height: %v", height)
		return 0, err
	}

	return balance, nil
}

// GetAccountKeyAtLatestBlock returns the account public key at the latest sealed block.
func (a *Accounts) GetAccountKeyAtLatestBlock(
	ctx context.Context,
	address flow.Address,
	keyIndex uint32,
) (*flow.AccountPublicKey, error) {
	sealed, err := a.state.Sealed().Head()
	if err != nil {
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, err
	}

	sealedBlockID := sealed.ID()
	key, err := a.provider.GetAccountKeyAtBlock(ctx, address, keyIndex, sealedBlockID, sealed.Height)
	if err != nil {
		a.log.Debug().Err(err).Msgf("failed to get account key at blockID: %v", sealedBlockID)
		return nil, err
	}

	return key, nil
}

// GetAccountKeyAtBlockHeight returns the account public key by key index at the given block height.
func (a *Accounts) GetAccountKeyAtBlockHeight(
	ctx context.Context,
	address flow.Address,
	keyIndex uint32,
	height uint64,
) (*flow.AccountPublicKey, error) {
	blockID, err := a.headers.BlockIDByHeight(height)
	if err != nil {
		return nil, commonrpc.ConvertStorageError(common.ResolveHeightError(a.state.Params(), height, err))
	}

	key, err := a.provider.GetAccountKeyAtBlock(ctx, address, keyIndex, blockID, height)
	if err != nil {
		a.log.Debug().Err(err).Msgf("failed to get account key at height: %v", height)
		return nil, err
	}

	return key, nil
}

// GetAccountKeysAtLatestBlock returns the account public keys at the latest sealed block.
func (a *Accounts) GetAccountKeysAtLatestBlock(
	ctx context.Context,
	address flow.Address,
) ([]flow.AccountPublicKey, error) {
	sealed, err := a.state.Sealed().Head()
	if err != nil {
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, err
	}

	sealedBlockID := sealed.ID()
	keys, err := a.provider.GetAccountKeysAtBlock(ctx, address, sealedBlockID, sealed.Height)
	if err != nil {
		a.log.Debug().Err(err).Msgf("failed to get account keys at blockID: %v", sealedBlockID)
		return nil, err
	}

	return keys, nil
}

// GetAccountKeysAtBlockHeight returns the account public keys at the given block height.
func (a *Accounts) GetAccountKeysAtBlockHeight(
	ctx context.Context,
	address flow.Address,
	height uint64,
) ([]flow.AccountPublicKey, error) {
	blockID, err := a.headers.BlockIDByHeight(height)
	if err != nil {
		return nil, commonrpc.ConvertStorageError(common.ResolveHeightError(a.state.Params(), height, err))
	}

	keys, err := a.provider.GetAccountKeysAtBlock(ctx, address, blockID, height)
	if err != nil {
		a.log.Debug().Err(err).Msgf("failed to get account keys at height: %v", height)
		return nil, err
	}

	return keys, nil
}
