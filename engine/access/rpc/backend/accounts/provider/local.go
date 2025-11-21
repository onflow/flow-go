package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/rpc/backend/common"
	"github.com/onflow/flow-go/engine/common/rpc"
	fvmerrors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type LocalAccountProvider struct {
	log            zerolog.Logger
	state          protocol.State
	scriptExecutor execution.ScriptExecutor

	// TODO(Data availability #7650): remove registersAsyncStore and current usages when fork-aware account endpoints will be implemented
	registersAsyncStore *execution.RegistersAsyncStore
}

var _ AccountProvider = (*LocalAccountProvider)(nil)

func NewLocalAccountProvider(
	log zerolog.Logger,
	state protocol.State,
	scriptExecutor execution.ScriptExecutor,
	registersAsyncStore *execution.RegistersAsyncStore,
) *LocalAccountProvider {
	return &LocalAccountProvider{
		log:                 log.With().Str("account_provider", "local").Logger(),
		state:               state,
		scriptExecutor:      scriptExecutor,
		registersAsyncStore: registersAsyncStore,
	}
}

func (l *LocalAccountProvider) GetAccountAtBlock(
	ctx context.Context,
	address flow.Address,
	_ flow.Identifier,
	height uint64,
) (*flow.Account, error) {
	registerSnapshotReader, err := l.registersAsyncStore.RegisterSnapshotReader()
	if err != nil {
		return nil, fmt.Errorf("failed to get register snapshot reader: %w", err)
	}

	account, err := l.scriptExecutor.GetAccountAtBlockHeight(ctx, address, height, registerSnapshotReader)
	if err != nil {
		return nil, convertAccountError(common.ResolveHeightError(l.state.Params(), height, err), address, height)
	}

	return account, nil
}

func (l *LocalAccountProvider) GetAccountBalanceAtBlock(
	ctx context.Context,
	address flow.Address,
	blockID flow.Identifier,
	height uint64,
) (uint64, error) {
	registerSnapshotReader, err := l.registersAsyncStore.RegisterSnapshotReader()
	if err != nil {
		return 0, fmt.Errorf("failed to get register snapshot reader: %w", err)
	}

	accountBalance, err := l.scriptExecutor.GetAccountBalance(ctx, address, height, registerSnapshotReader)
	if err != nil {
		l.log.Debug().Err(err).Msgf("failed to get account balance at blockID: %v", blockID)
		return 0, err
	}

	return accountBalance, nil
}

func (l *LocalAccountProvider) GetAccountKeyAtBlock(
	ctx context.Context,
	address flow.Address,
	keyIndex uint32,
	_ flow.Identifier,
	height uint64,
) (*flow.AccountPublicKey, error) {
	registerSnapshotReader, err := l.registersAsyncStore.RegisterSnapshotReader()
	if err != nil {
		return nil, fmt.Errorf("failed to get register snapshot reader: %w", err)
	}

	accountKey, err := l.scriptExecutor.GetAccountKey(ctx, address, keyIndex, height, registerSnapshotReader)
	if err != nil {
		l.log.Debug().Err(err).Msgf("failed to get account key at height: %d", height)
		return nil, err
	}

	return accountKey, nil
}

func (l *LocalAccountProvider) GetAccountKeysAtBlock(
	ctx context.Context,
	address flow.Address,
	_ flow.Identifier,
	height uint64,
) ([]flow.AccountPublicKey, error) {
	registerSnapshotReader, err := l.registersAsyncStore.RegisterSnapshotReader()
	if err != nil {
		return nil, fmt.Errorf("failed to get register snapshot reader: %w", err)
	}

	accountKeys, err := l.scriptExecutor.GetAccountKeys(ctx, address, height, registerSnapshotReader)
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
