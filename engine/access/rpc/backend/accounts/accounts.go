package accounts

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/rpc/backend"
	"github.com/onflow/flow-go/engine/access/rpc/backend/accounts/handler"
	"github.com/onflow/flow-go/engine/access/rpc/connection"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type Accounts struct {
	log                        zerolog.Logger
	state                      protocol.State
	headers                    storage.Headers
	connFactory                connection.ConnectionFactory
	nodeCommunicator           backend.Communicator
	endpointHandler            handler.Handler
	execNodeIdentitiesProvider *commonrpc.ExecutionNodeIdentitiesProvider
}

func NewAccounts(
	log zerolog.Logger,
	state protocol.State,
	headers storage.Headers,
	connFactory connection.ConnectionFactory,
	nodeCommunicator backend.Communicator,
	scriptExecMode backend.IndexQueryMode,
	scriptExecutor execution.ScriptExecutor,
	execNodeIdentitiesProvider *commonrpc.ExecutionNodeIdentitiesProvider,
) *Accounts {
	var h handler.Handler

	// TODO: we can instantiate these strategies outside of backend (in access_node_builder e.g.)
	switch scriptExecMode {
	case backend.IndexQueryModeLocalOnly:
		h = handler.NewLocalHandler(log, state, scriptExecutor)

	case backend.IndexQueryModeExecutionNodesOnly:
		h = handler.NewExecutionNodeHandler(log, state, connFactory, nodeCommunicator, execNodeIdentitiesProvider)

	case backend.IndexQueryModeFailover:
		local := handler.NewLocalHandler(log, state, scriptExecutor)
		execNode := handler.NewExecutionNodeHandler(log, state, connFactory, nodeCommunicator, execNodeIdentitiesProvider)
		h = handler.NewFailoverHandler(log, state, local, execNode)

	case backend.IndexQueryModeCompare:
		local := handler.NewLocalHandler(log, state, scriptExecutor)
		execNode := handler.NewExecutionNodeHandler(log, state, connFactory, nodeCommunicator, execNodeIdentitiesProvider)
		h = handler.NewCompareHandler(log, state, local, execNode)
	}

	return &Accounts{
		log:                        log,
		state:                      state,
		headers:                    headers,
		connFactory:                connFactory,
		nodeCommunicator:           nodeCommunicator,
		endpointHandler:            h,
		execNodeIdentitiesProvider: execNodeIdentitiesProvider,
	}
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
	account, err := a.endpointHandler.GetAccountAtBlockHeight(ctx, address, sealedBlockID, sealed.Height)
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
		return nil, commonrpc.ConvertStorageError(backend.ResolveHeightError(a.state.Params(), height, err))
	}

	account, err := a.endpointHandler.GetAccountAtBlockHeight(ctx, address, blockID, height)
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
	balance, err := a.endpointHandler.GetAccountBalanceAtBlockHeight(ctx, address, sealedBlockID, sealed.Height)
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
		return 0, commonrpc.ConvertStorageError(backend.ResolveHeightError(a.state.Params(), height, err))
	}

	balance, err := a.endpointHandler.GetAccountBalanceAtBlockHeight(ctx, address, blockID, height)
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
	accountKey, err := a.endpointHandler.GetAccountKeyAtBlockHeight(ctx, address, keyIndex, sealedBlockID, sealed.Height)
	if err != nil {
		a.log.Debug().Err(err).Msgf("failed to get account key at blockID: %v", sealedBlockID)
		return nil, err
	}

	return accountKey, nil
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
		return nil, commonrpc.ConvertStorageError(backend.ResolveHeightError(a.state.Params(), height, err))
	}

	accountKey, err := a.endpointHandler.GetAccountKeyAtBlockHeight(ctx, address, keyIndex, blockID, height)
	if err != nil {
		a.log.Debug().Err(err).Msgf("failed to get account key at height: %v", height)
		return nil, err
	}

	return accountKey, nil
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
	accountKeys, err := a.endpointHandler.GetAccountKeysAtBlockHeight(ctx, address, sealedBlockID, sealed.Height)
	if err != nil {
		a.log.Debug().Err(err).Msgf("failed to get account keys at blockID: %v", sealedBlockID)
		return nil, err
	}

	return accountKeys, nil
}

// GetAccountKeysAtBlockHeight returns the account public keys at the given block height.
func (a *Accounts) GetAccountKeysAtBlockHeight(
	ctx context.Context,
	address flow.Address,
	height uint64,
) ([]flow.AccountPublicKey, error) {
	blockID, err := a.headers.BlockIDByHeight(height)
	if err != nil {
		return nil, commonrpc.ConvertStorageError(backend.ResolveHeightError(a.state.Params(), height, err))
	}

	accountKeys, err := a.endpointHandler.GetAccountKeysAtBlockHeight(ctx, address, blockID, height)
	if err != nil {
		a.log.Debug().Err(err).Msgf("failed to get account keys at height: %v", height)
		return nil, err
	}

	return accountKeys, nil
}
