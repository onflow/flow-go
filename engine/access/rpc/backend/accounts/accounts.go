package accounts

import (
	"context"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/rpc/backend/accounts/retriever"
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

type API interface {
	GetAccount(ctx context.Context, address flow.Address) (*flow.Account, error)
	GetAccountAtLatestBlock(ctx context.Context, address flow.Address) (*flow.Account, error)
	GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64) (*flow.Account, error)

	GetAccountBalanceAtLatestBlock(ctx context.Context, address flow.Address) (uint64, error)
	GetAccountBalanceAtBlockHeight(ctx context.Context, address flow.Address, height uint64) (uint64, error)

	GetAccountKeyAtLatestBlock(ctx context.Context, address flow.Address, keyIndex uint32) (*flow.AccountPublicKey, error)
	GetAccountKeyAtBlockHeight(ctx context.Context, address flow.Address, keyIndex uint32, height uint64) (*flow.AccountPublicKey, error)
	GetAccountKeysAtLatestBlock(ctx context.Context, address flow.Address) ([]flow.AccountPublicKey, error)
	GetAccountKeysAtBlockHeight(ctx context.Context, address flow.Address, height uint64) ([]flow.AccountPublicKey, error)
}

type Accounts struct {
	log       zerolog.Logger
	state     protocol.State
	headers   storage.Headers
	retriever retriever.AccountRetriever
}

var _ API = (*Accounts)(nil)

func NewAccountsBackend(
	log zerolog.Logger,
	state protocol.State,
	headers storage.Headers,
	connFactory connection.ConnectionFactory,
	nodeCommunicator node_communicator.Communicator,
	scriptExecMode query_mode.IndexQueryMode,
	scriptExecutor execution.ScriptExecutor,
	execNodeIdentitiesProvider *commonrpc.ExecutionNodeIdentitiesProvider,
) (*Accounts, error) {
	var accountsRetriever retriever.AccountRetriever

	switch scriptExecMode {
	case query_mode.IndexQueryModeLocalOnly:
		accountsRetriever = retriever.NewLocalAccountRetriever(log, state, scriptExecutor)

	case query_mode.IndexQueryModeExecutionNodesOnly:
		accountsRetriever = retriever.NewENAccountRetriever(log, state, connFactory, nodeCommunicator, execNodeIdentitiesProvider)

	case query_mode.IndexQueryModeFailover:
		local := retriever.NewLocalAccountRetriever(log, state, scriptExecutor)
		execNode := retriever.NewENAccountRetriever(log, state, connFactory, nodeCommunicator, execNodeIdentitiesProvider)
		accountsRetriever = retriever.NewFailoverAccountRetriever(log, state, local, execNode)

	case query_mode.IndexQueryModeCompare:
		local := retriever.NewLocalAccountRetriever(log, state, scriptExecutor)
		execNode := retriever.NewENAccountRetriever(log, state, connFactory, nodeCommunicator, execNodeIdentitiesProvider)
		accountsRetriever = retriever.NewComparingAccountRetriever(log, state, local, execNode)

	default:
		return nil, status.Errorf(codes.Internal, "unknown execution mode: %v", scriptExecMode)
	}

	return &Accounts{
		log:       log,
		state:     state,
		headers:   headers,
		retriever: accountsRetriever,
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
	account, err := a.retriever.GetAccountAtBlock(ctx, address, sealedBlockID, sealed.Height)
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

	account, err := a.retriever.GetAccountAtBlock(ctx, address, blockID, height)
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
	balance, err := a.retriever.GetAccountBalanceAtBlock(ctx, address, sealedBlockID, sealed.Height)
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

	balance, err := a.retriever.GetAccountBalanceAtBlock(ctx, address, blockID, height)
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
	key, err := a.retriever.GetAccountKeyAtBlock(ctx, address, keyIndex, sealedBlockID, sealed.Height)
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

	key, err := a.retriever.GetAccountKeyAtBlock(ctx, address, keyIndex, blockID, height)
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
	keys, err := a.retriever.GetAccountKeysAtBlock(ctx, address, sealedBlockID, sealed.Height)
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

	keys, err := a.retriever.GetAccountKeysAtBlock(ctx, address, blockID, height)
	if err != nil {
		a.log.Debug().Err(err).Msgf("failed to get account keys at height: %v", height)
		return nil, err
	}

	return keys, nil
}
