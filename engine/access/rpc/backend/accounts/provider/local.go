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
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type LocalAccountProvider struct {
	log                 zerolog.Logger
	state               protocol.State
	scriptExecutor      execution.ScriptExecutor
	executionStateCache optimistic_sync.ExecutionStateCache
}

var _ AccountProvider = (*LocalAccountProvider)(nil)

func NewLocalAccountProvider(
	log zerolog.Logger,
	state protocol.State,
	scriptExecutor execution.ScriptExecutor,
	executionStateCache optimistic_sync.ExecutionStateCache,
) *LocalAccountProvider {
	return &LocalAccountProvider{
		log:                 log.With().Str("account_provider", "local").Logger(),
		state:               state,
		scriptExecutor:      scriptExecutor,
		executionStateCache: executionStateCache,
	}
}

func (l *LocalAccountProvider) GetAccountAtBlock(
	ctx context.Context,
	address flow.Address,
	_ flow.Identifier,
	height uint64,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (*flow.Account, *accessmodel.ExecutorMetadata, error) {
	executionResultID := executionResultInfo.ExecutionResultID
	snapshot, err := l.executionStateCache.Snapshot(executionResultID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get snapshot for execution result %s: %w", executionResultID, err)
	}

	registers, err := snapshot.Registers()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get register snapshot reader: %w", err)
	}

	account, err := l.scriptExecutor.GetAccountAtBlockHeight(ctx, address, height, registers)
	if err != nil {
		return nil, nil, convertAccountError(common.ResolveHeightError(l.state.Params(), height, err), address, height)
	}

	metadata := &accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultID,
		ExecutorIDs:       executionResultInfo.ExecutionNodes.NodeIDs(),
	}

	return account, metadata, nil
}

func (l *LocalAccountProvider) GetAccountBalanceAtBlock(
	ctx context.Context,
	address flow.Address,
	blockID flow.Identifier,
	height uint64,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (uint64, *accessmodel.ExecutorMetadata, error) {
	executionResultID := executionResultInfo.ExecutionResultID
	snapshot, err := l.executionStateCache.Snapshot(executionResultID)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to get snapshot for execution result %s: %w", executionResultID, err)
	}

	registers, err := snapshot.Registers()
	if err != nil {
		return 0, nil, fmt.Errorf("failed to get register snapshot reader: %w", err)
	}

	accountBalance, err := l.scriptExecutor.GetAccountBalance(ctx, address, height, registers)
	if err != nil {
		l.log.Debug().Err(err).Msgf("failed to get account balance at blockID: %v", blockID)
		return 0, nil, err
	}

	metadata := &accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultID,
		ExecutorIDs:       executionResultInfo.ExecutionNodes.NodeIDs(),
	}

	return accountBalance, metadata, nil
}

func (l *LocalAccountProvider) GetAccountKeyAtBlock(
	ctx context.Context,
	address flow.Address,
	keyIndex uint32,
	_ flow.Identifier,
	height uint64,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (*flow.AccountPublicKey, *accessmodel.ExecutorMetadata, error) {
	executionResultID := executionResultInfo.ExecutionResultID
	snapshot, err := l.executionStateCache.Snapshot(executionResultID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get snapshot for execution result %s: %w", executionResultID, err)
	}

	registers, err := snapshot.Registers()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get register snapshot reader: %w", err)
	}

	accountKey, err := l.scriptExecutor.GetAccountKey(ctx, address, keyIndex, height, registers)
	if err != nil {
		l.log.Debug().Err(err).Msgf("failed to get account key at height: %d", height)
		return nil, nil, err
	}

	metadata := &accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultID,
		ExecutorIDs:       executionResultInfo.ExecutionNodes.NodeIDs(),
	}

	return accountKey, metadata, nil
}

func (l *LocalAccountProvider) GetAccountKeysAtBlock(
	ctx context.Context,
	address flow.Address,
	_ flow.Identifier,
	height uint64,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) ([]flow.AccountPublicKey, *accessmodel.ExecutorMetadata, error) {
	executionResultID := executionResultInfo.ExecutionResultID
	snapshot, err := l.executionStateCache.Snapshot(executionResultID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get snapshot for execution result %s: %w", executionResultID, err)
	}

	registers, err := snapshot.Registers()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get register snapshot reader: %w", err)
	}

	accountKeys, err := l.scriptExecutor.GetAccountKeys(ctx, address, height, registers)
	if err != nil {
		l.log.Debug().Err(err).Msgf("failed to get account keys at height: %d", height)
		return nil, nil, err
	}

	metadata := &accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultID,
		ExecutorIDs:       executionResultInfo.ExecutionNodes.NodeIDs(),
	}

	return accountKeys, metadata, nil
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
