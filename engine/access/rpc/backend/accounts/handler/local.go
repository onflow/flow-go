package handler

import (
	"context"
	"errors"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/rpc/backend"
	"github.com/onflow/flow-go/engine/common/rpc"
	fvmerrors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"

	"github.com/onflow/flow-go/model/flow"
)

type Local struct {
	log            zerolog.Logger
	state          protocol.State
	scriptExecutor execution.ScriptExecutor
}

var _ Handler = (*Local)(nil)

func NewLocalHandler(
	log zerolog.Logger,
	state protocol.State,
	scriptExecutor execution.ScriptExecutor,
) *Local {
	return &Local{
		log:            zerolog.New(log).With().Str("handler", "local").Logger(),
		state:          state,
		scriptExecutor: scriptExecutor,
	}
}

func (l *Local) GetAccount(ctx context.Context, address flow.Address) (*flow.Account, error) {
	return l.GetAccountAtLatestBlock(ctx, address)
}

func (l *Local) GetAccountAtLatestBlock(ctx context.Context, address flow.Address) (*flow.Account, error) {
	sealed, err := l.state.Sealed().Head()
	if err != nil {
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, err
	}

	sealedBlockID := sealed.ID()
	account, err := l.GetAccountAtBlockHeight(ctx, address, sealedBlockID, sealed.Height)
	if err != nil {
		l.log.Debug().Err(err).Msgf("failed to get account at blockID: %v", sealedBlockID)
		return nil, err
	}

	return account, nil
}

func (l *Local) GetAccountAtBlockHeight(
	ctx context.Context,
	address flow.Address,
	_ flow.Identifier,
	height uint64,
) (*flow.Account, error) {
	account, err := l.scriptExecutor.GetAccountAtBlockHeight(ctx, address, height)
	if err != nil {
		return nil, convertAccountError(backend.ResolveHeightError(l.state.Params(), height, err), address, height)
	}
	return account, nil
}

func (l *Local) GetAccountBalanceAtLatestBlock(ctx context.Context, address flow.Address) (uint64, error) {
	sealed, err := l.state.Sealed().Head()
	if err != nil {
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return 0, err
	}

	sealedBlockID := sealed.ID()
	balance, err := l.GetAccountBalanceAtBlockHeight(ctx, address, sealedBlockID, sealed.Height)
	if err != nil {
		l.log.Debug().Err(err).Msgf("failed to get account at blockID: %v", sealedBlockID)
		return 0, err
	}

	return balance, nil
}

func (l *Local) GetAccountBalanceAtBlockHeight(
	ctx context.Context,
	address flow.Address,
	blockID flow.Identifier,
	height uint64,
) (uint64, error) {
	accountBalance, err := l.scriptExecutor.GetAccountBalance(ctx, address, height)
	if err != nil {
		l.log.Debug().Err(err).Msgf("failed to get account balance at blockID: %v", blockID)
		return 0, err
	}

	return accountBalance, nil
}

func (l *Local) GetAccountKeyAtLatestBlock(
	ctx context.Context,
	address flow.Address,
	keyIndex uint32,
) (*flow.AccountPublicKey, error) {
	sealed, err := l.state.Sealed().Head()
	if err != nil {
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, err
	}

	sealedBlockID := sealed.ID()
	key, err := l.GetAccountKeyAtBlockHeight(ctx, address, keyIndex, sealedBlockID, sealed.Height)
	if err != nil {
		l.log.Debug().Err(err).Msgf("failed to get account key at blockID: %v", sealedBlockID)
		return nil, err
	}

	return key, nil
}

func (l *Local) GetAccountKeyAtBlockHeight(
	ctx context.Context,
	address flow.Address,
	keyIndex uint32,
	_ flow.Identifier,
	height uint64,
) (*flow.AccountPublicKey, error) {
	accountKey, err := l.scriptExecutor.GetAccountKey(ctx, address, keyIndex, height)
	if err != nil {
		l.log.Debug().Err(err).Msgf("failed to get account key at height: %d", height)
		return nil, err
	}

	return accountKey, nil
}

func (l *Local) GetAccountKeysAtLatestBlock(
	ctx context.Context,
	address flow.Address,
) ([]flow.AccountPublicKey, error) {
	sealed, err := l.state.Sealed().Head()
	if err != nil {
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, err
	}

	sealedBlockID := sealed.ID()
	keys, err := l.GetAccountKeysAtBlockHeight(ctx, address, sealedBlockID, sealed.Height)
	if err != nil {
		l.log.Debug().Err(err).Msgf("failed to get account keys at blockID: %v", sealedBlockID)
		return nil, err
	}

	return keys, nil
}

func (l *Local) GetAccountKeysAtBlockHeight(
	ctx context.Context,
	address flow.Address,
	_ flow.Identifier,
	height uint64,
) ([]flow.AccountPublicKey, error) {
	accountKeys, err := l.scriptExecutor.GetAccountKeys(ctx, address, height)
	if err != nil {
		l.log.Debug().Err(err).Msgf("failed to get account keys at height: %d", height)
		return nil, err
	}

	return accountKeys, nil
}

// convertAccountError converts the script execution error to a gRPC error
func convertAccountError(err error, address flow.Address, height uint64) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, storage.ErrNotFound) {
		return status.Errorf(codes.NotFound, "account with address %s not found: %v", address, err)
	}

	if fvmerrors.IsAccountNotFoundError(err) {
		return status.Errorf(codes.NotFound, "account not found")
	}

	return rpc.ConvertIndexError(err, height, "failed to get account")
}
