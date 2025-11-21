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
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type Accounts struct {
	log                     zerolog.Logger
	state                   protocol.State
	headers                 storage.Headers
	provider                provider.AccountProvider
	executionResultProvider optimistic_sync.ExecutionResultInfoProvider
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
	executionResultProvider optimistic_sync.ExecutionResultInfoProvider,
	executionStateCache optimistic_sync.ExecutionStateCache,
) (*Accounts, error) {
	var accountProvider provider.AccountProvider

	switch scriptExecMode {
	case query_mode.IndexQueryModeLocalOnly:
		accountProvider = provider.NewLocalAccountProvider(log, state, scriptExecutor, executionStateCache)

	case query_mode.IndexQueryModeExecutionNodesOnly:
		accountProvider = provider.NewENAccountProvider(log, state, connFactory, nodeCommunicator, execNodeIdentitiesProvider)

	case query_mode.IndexQueryModeFailover:
		local := provider.NewLocalAccountProvider(log, state, scriptExecutor, executionStateCache)
		execNode := provider.NewENAccountProvider(log, state, connFactory, nodeCommunicator, execNodeIdentitiesProvider)
		accountProvider = provider.NewFailoverAccountProvider(log, state, local, execNode)

	default:
		return nil, fmt.Errorf("unknown execution mode: %v", scriptExecMode)
	}

	return &Accounts{
		log:                     log,
		state:                   state,
		headers:                 headers,
		provider:                accountProvider,
		executionResultProvider: executionResultProvider,
	}, nil
}

// GetAccount returns the account details at the latest sealed block.
// Alias for GetAccountAtLatestBlock
func (a *Accounts) GetAccount(ctx context.Context, address flow.Address, criteria optimistic_sync.Criteria) (*flow.Account, *accessmodel.ExecutorMetadata, error) {
	return a.GetAccountAtLatestBlock(ctx, address, criteria)
}

// GetAccountAtLatestBlock returns the account details at the latest sealed block.
func (a *Accounts) GetAccountAtLatestBlock(ctx context.Context, address flow.Address, criteria optimistic_sync.Criteria) (*flow.Account, *accessmodel.ExecutorMetadata, error) {
	sealed, err := a.state.Sealed().Head()
	if err != nil {
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, nil, err
	}

	sealedBlockID := sealed.ID()
	executionResultInfo, err := a.executionResultProvider.ExecutionResultInfo(
		sealedBlockID,
		criteria,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get execution result info for block: %w", err)
	}

	account, metadata, err := a.provider.GetAccountAtBlock(ctx, address, sealedBlockID, sealed.Height, executionResultInfo)
	if err != nil {
		a.log.Debug().Err(err).Msgf("failed to get account at blockID: %v", sealedBlockID)
		return nil, nil, err
	}

	return account, metadata, nil
}

// GetAccountAtBlockHeight returns the account details at the given block height.
func (a *Accounts) GetAccountAtBlockHeight(
	ctx context.Context,
	address flow.Address,
	height uint64,
	criteria optimistic_sync.Criteria,
) (*flow.Account, *accessmodel.ExecutorMetadata, error) {
	blockID, err := a.headers.BlockIDByHeight(height)
	if err != nil {
		return nil, nil, commonrpc.ConvertStorageError(common.ResolveHeightError(a.state.Params(), height, err))
	}

	executionResultInfo, err := a.executionResultProvider.ExecutionResultInfo(
		blockID,
		criteria,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get execution result info for block: %w", err)
	}

	account, metadata, err := a.provider.GetAccountAtBlock(ctx, address, blockID, height, executionResultInfo)
	if err != nil {
		a.log.Debug().Err(err).Msgf("failed to get account at height: %d", height)
		return nil, nil, err
	}

	return account, metadata, nil
}

// GetAccountBalanceAtLatestBlock returns the account balance at the latest sealed block.
func (a *Accounts) GetAccountBalanceAtLatestBlock(ctx context.Context, address flow.Address, criteria optimistic_sync.Criteria,
) (uint64, *accessmodel.ExecutorMetadata, error) {
	sealed, err := a.state.Sealed().Head()
	if err != nil {
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return 0, nil, err
	}

	sealedBlockID := sealed.ID()
	executionResultInfo, err := a.executionResultProvider.ExecutionResultInfo(
		sealedBlockID,
		criteria,
	)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to get execution result info for block: %w", err)
	}

	balance, metadata, err := a.provider.GetAccountBalanceAtBlock(ctx, address, sealedBlockID, sealed.Height, executionResultInfo)
	if err != nil {
		a.log.Debug().Err(err).Msgf("failed to get account balance at blockID: %v", sealedBlockID)
		return 0, nil, err
	}

	return balance, metadata, nil
}

// GetAccountBalanceAtBlockHeight returns the account balance at the given block height.
func (a *Accounts) GetAccountBalanceAtBlockHeight(
	ctx context.Context,
	address flow.Address,
	height uint64,
	criteria optimistic_sync.Criteria,
) (uint64, *accessmodel.ExecutorMetadata, error) {
	blockID, err := a.headers.BlockIDByHeight(height)
	if err != nil {
		return 0, nil, commonrpc.ConvertStorageError(common.ResolveHeightError(a.state.Params(), height, err))
	}

	executionResultInfo, err := a.executionResultProvider.ExecutionResultInfo(
		blockID,
		criteria,
	)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to get execution result info for block: %w", err)
	}

	balance, metadata, err := a.provider.GetAccountBalanceAtBlock(ctx, address, blockID, height, executionResultInfo)
	if err != nil {
		a.log.Debug().Err(err).Msgf("failed to get account balance at height: %v", height)
		return 0, nil, err
	}

	return balance, metadata, nil
}

// GetAccountKeyAtLatestBlock returns the account public key at the latest sealed block.
func (a *Accounts) GetAccountKeyAtLatestBlock(
	ctx context.Context,
	address flow.Address,
	keyIndex uint32,
	criteria optimistic_sync.Criteria,
) (*flow.AccountPublicKey, *accessmodel.ExecutorMetadata, error) {
	sealed, err := a.state.Sealed().Head()
	if err != nil {
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, nil, err
	}

	sealedBlockID := sealed.ID()
	executionResultInfo, err := a.executionResultProvider.ExecutionResultInfo(
		sealedBlockID,
		criteria,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get execution result info for block: %w", err)
	}

	key, metadata, err := a.provider.GetAccountKeyAtBlock(ctx, address, keyIndex, sealedBlockID, sealed.Height, executionResultInfo)
	if err != nil {
		a.log.Debug().Err(err).Msgf("failed to get account key at blockID: %v", sealedBlockID)
		return nil, nil, err
	}

	return key, metadata, nil
}

// GetAccountKeyAtBlockHeight returns the account public key by key index at the given block height.
func (a *Accounts) GetAccountKeyAtBlockHeight(
	ctx context.Context,
	address flow.Address,
	keyIndex uint32,
	height uint64,
	criteria optimistic_sync.Criteria,
) (*flow.AccountPublicKey, *accessmodel.ExecutorMetadata, error) {
	blockID, err := a.headers.BlockIDByHeight(height)
	if err != nil {
		return nil, nil, commonrpc.ConvertStorageError(common.ResolveHeightError(a.state.Params(), height, err))
	}

	executionResultInfo, err := a.executionResultProvider.ExecutionResultInfo(
		blockID,
		criteria,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get execution result info for block: %w", err)
	}

	key, metadata, err := a.provider.GetAccountKeyAtBlock(ctx, address, keyIndex, blockID, height, executionResultInfo)
	if err != nil {
		a.log.Debug().Err(err).Msgf("failed to get account key at height: %v", height)
		return nil, nil, err
	}

	return key, metadata, nil
}

// GetAccountKeysAtLatestBlock returns the account public keys at the latest sealed block.
func (a *Accounts) GetAccountKeysAtLatestBlock(
	ctx context.Context,
	address flow.Address,
	criteria optimistic_sync.Criteria,
) ([]flow.AccountPublicKey, *accessmodel.ExecutorMetadata, error) {
	sealed, err := a.state.Sealed().Head()
	if err != nil {
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, nil, err
	}

	sealedBlockID := sealed.ID()
	executionResultInfo, err := a.executionResultProvider.ExecutionResultInfo(
		sealedBlockID,
		criteria,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get execution result info for block: %w", err)
	}

	keys, metadata, err := a.provider.GetAccountKeysAtBlock(ctx, address, sealedBlockID, sealed.Height, executionResultInfo)
	if err != nil {
		a.log.Debug().Err(err).Msgf("failed to get account keys at blockID: %v", sealedBlockID)
		return nil, nil, err
	}

	return keys, metadata, nil
}

// GetAccountKeysAtBlockHeight returns the account public keys at the given block height.
func (a *Accounts) GetAccountKeysAtBlockHeight(
	ctx context.Context,
	address flow.Address,
	height uint64,
	criteria optimistic_sync.Criteria,
) ([]flow.AccountPublicKey, *accessmodel.ExecutorMetadata, error) {
	blockID, err := a.headers.BlockIDByHeight(height)
	if err != nil {
		return nil, nil, commonrpc.ConvertStorageError(common.ResolveHeightError(a.state.Params(), height, err))
	}

	executionResultInfo, err := a.executionResultProvider.ExecutionResultInfo(
		blockID,
		criteria,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get execution result info for block: %w", err)
	}

	keys, metadata, err := a.provider.GetAccountKeysAtBlock(ctx, address, blockID, height, executionResultInfo)
	if err != nil {
		a.log.Debug().Err(err).Msgf("failed to get account keys at height: %v", height)
		return nil, nil, err
	}

	return keys, metadata, nil
}
