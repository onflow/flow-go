package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/common/version"
	fvmerrors "github.com/onflow/flow-go/fvm/errors"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// LocalAccountProvider is a provider that read account data using only the local node's storage.
type LocalAccountProvider struct {
	log                 zerolog.Logger
	state               protocol.State
	scriptExecutor      execution.ScriptExecutor
	executionStateCache optimistic_sync.ExecutionStateCache
}

var _ AccountProvider = (*LocalAccountProvider)(nil)

// NewLocalAccountProvider creates a new instance of LocalAccountProvider.
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

// GetAccountAtBlock returns a Flow account by the provided address and block height.
//
// Expected error returns during normal operation:
//   - [access.InvalidRequestError] - if the request failed due to invalid arguments or runtime errors.
//   - [access.ResourceExhausted] - if computation or memory limits were exceeded.
//   - [access.DataNotFoundError] - if data was not found.
//   - [access.OutOfRangeError] - if the data for the requested height is outside the node's available range.
//   - [access.PreconditionFailedError] - if the registers storage is still bootstrapping.
//   - [access.RequestCanceledError] - if the request was canceled.
//   - [access.RequestTimedOutError] - if the request timed out.
//   - [access.InternalError] - for internal failures or index conversion errors.
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
		err = access.RequireErrorIs(ctx, err, storage.ErrNotFound)
		err = fmt.Errorf("could not find snapshot for execution result %s: %w", executionResultInfo.ExecutionResultID, err)
		return nil, nil, access.NewDataNotFoundError("execution state snapshot", err)
	}

	registers, err := snapshot.Registers()
	if err != nil {
		err = access.RequireErrorIs(ctx, err, indexer.ErrIndexNotInitialized)
		err = fmt.Errorf("failed to get registers storage from snapshot: %w", err)
		return nil, nil, access.NewPreconditionFailedError(err)
	}

	account, err := l.scriptExecutor.GetAccountAtBlockHeight(ctx, address, height, registers)
	if err != nil {
		return nil, nil, convertAccountError(err)
	}

	metadata := &accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultID,
		ExecutorIDs:       executionResultInfo.ExecutionNodes.NodeIDs(),
	}

	return account, metadata, nil
}

// GetAccountBalanceAtBlock returns the balance of a Flow account at the given block.
//
// Expected error returns during normal operation:
//   - [access.InvalidRequestError] - if the request failed due to invalid arguments or runtime errors.
//   - [access.ResourceExhausted] - if computation or memory limits were exceeded.
//   - [access.DataNotFoundError] - if data was not found.
//   - [access.OutOfRangeError] - if the data for the requested height is outside the node's available range.
//   - [access.PreconditionFailedError] - if the registers storage is still bootstrapping.
//   - [access.RequestCanceledError] - if the request was canceled.
//   - [access.RequestTimedOutError] - if the request timed out.
//   - [access.InternalError] - for internal failures or index conversion errors.
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
		err = access.RequireErrorIs(ctx, err, storage.ErrNotFound)
		err = fmt.Errorf("could not find snapshot for execution result %s: %w", executionResultInfo.ExecutionResultID, err)
		return 0, nil, access.NewDataNotFoundError("execution state snapshot", err)
	}

	registers, err := snapshot.Registers()
	if err != nil {
		err = access.RequireErrorIs(ctx, err, indexer.ErrIndexNotInitialized)
		err = fmt.Errorf("failed to get registers storage from snapshot: %w", err)
		return 0, nil, access.NewPreconditionFailedError(err)
	}

	accountBalance, err := l.scriptExecutor.GetAccountBalance(ctx, address, height, registers)
	if err != nil {
		l.log.Debug().Err(err).Msgf("failed to get account balance at blockID: %v", blockID)
		return 0, nil, convertAccountError(err)
	}

	metadata := &accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultID,
		ExecutorIDs:       executionResultInfo.ExecutionNodes.NodeIDs(),
	}

	return accountBalance, metadata, nil
}

// GetAccountKeyAtBlock returns a specific public key of a Flow account
// by its key index and block height.
//
// Expected error returns during normal operation:
//   - [access.InvalidRequestError] - if the request failed due to invalid arguments or runtime errors.
//   - [access.ResourceExhausted] - if computation or memory limits were exceeded.
//   - [access.DataNotFoundError] - if data was not found.
//   - [access.OutOfRangeError] - if the data for the requested height is outside the node's available range.
//   - [access.PreconditionFailedError] - if the registers storage is still bootstrapping.
//   - [access.RequestCanceledError] - if the request was canceled.
//   - [access.RequestTimedOutError] - if the request timed out.
//   - [access.InternalError] - for internal failures or index conversion errors.
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
		err = access.RequireErrorIs(ctx, err, storage.ErrNotFound)
		err = fmt.Errorf("could not find snapshot for execution result %s: %w", executionResultInfo.ExecutionResultID, err)
		return nil, nil, access.NewDataNotFoundError("execution state snapshot", err)
	}

	registers, err := snapshot.Registers()
	if err != nil {
		err = access.RequireErrorIs(ctx, err, indexer.ErrIndexNotInitialized)
		err = fmt.Errorf("failed to get registers storage from snapshot: %w", err)
		return nil, nil, access.NewPreconditionFailedError(err)
	}

	accountKey, err := l.scriptExecutor.GetAccountKey(ctx, address, keyIndex, height, registers)
	if err != nil {
		l.log.Debug().Err(err).Msgf("failed to get account key at height: %d", height)
		return nil, nil, convertAccountError(err)
	}

	metadata := &accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultID,
		ExecutorIDs:       executionResultInfo.ExecutionNodes.NodeIDs(),
	}

	return accountKey, metadata, nil
}

// GetAccountKeysAtBlock returns all public keys associated with a Flow account
// at the given block height.
//
// Expected error returns during normal operation:
//   - [access.InvalidRequestError] - if the request failed due to invalid arguments or runtime errors.
//   - [access.ResourceExhausted] - if computation or memory limits were exceeded.
//   - [access.DataNotFoundError] - if data was not found.
//   - [access.OutOfRangeError] - if the data for the requested height is outside the node's available range.
//   - [access.PreconditionFailedError] - if the registers storage is still bootstrapping.
//   - [access.RequestCanceledError] - if the request was canceled.
//   - [access.RequestTimedOutError] - if the request timed out.
//   - [access.InternalError] - for internal failures or index conversion errors.
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
		err = access.RequireErrorIs(ctx, err, storage.ErrNotFound)
		err = fmt.Errorf("could not find snapshot for execution result %s: %w", executionResultInfo.ExecutionResultID, err)
		return nil, nil, access.NewDataNotFoundError("execution state snapshot", err)
	}

	registers, err := snapshot.Registers()
	if err != nil {
		err = access.RequireErrorIs(ctx, err, indexer.ErrIndexNotInitialized)
		err = fmt.Errorf("failed to get registers storage from snapshot: %w", err)
		return nil, nil, access.NewPreconditionFailedError(err)
	}

	accountKeys, err := l.scriptExecutor.GetAccountKeys(ctx, address, height, registers)
	if err != nil {
		l.log.Debug().Err(err).Msgf("failed to get account keys at height: %d", height)
		return nil, nil, convertAccountError(err)
	}

	metadata := &accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultID,
		ExecutorIDs:       executionResultInfo.ExecutionNodes.NodeIDs(),
	}

	return accountKeys, metadata, nil
}

// convertAccountError maps script executor and FVM errors to corresponding access-layer sentinel errors.
//
// Expected error returns during normal operation:
//   - [access.InvalidRequestError] - if the request failed due to invalid arguments or runtime errors.
//   - [access.ResourceExhausted] - if computation or memory limits were exceeded.
//   - [access.DataNotFoundError] - if data for the requested height was not found.
//   - [access.OutOfRangeError] - if the data for the requested height is outside the node's available range.
//   - [access.RequestCanceledError] - if the request was canceled.
//   - [access.RequestTimedOutError] - if the request timed out.
//   - [access.InternalError] - for internal failures or index conversion errors.
func convertAccountError(err error) error {
	switch {
	case errors.Is(err, version.ErrOutOfRange),
		errors.Is(err, storage.ErrHeightNotIndexed),
		errors.Is(err, execution.ErrIncompatibleNodeVersion):
		return access.NewOutOfRangeError(err)
	case errors.Is(err, storage.ErrNotFound):
		return access.NewDataNotFoundError("header", err)
	case errors.Is(err, context.Canceled):
		return access.NewRequestCanceledError(err)
	case errors.Is(err, context.DeadlineExceeded):
		return access.NewRequestTimedOutError(err)
	}

	var failure fvmerrors.CodedFailure
	if fvmerrors.As(err, &failure) {
		return access.NewInternalError(err)
	}

	// general FVM/ledger errors
	var coded fvmerrors.CodedError
	if fvmerrors.As(err, &coded) {
		switch coded.Code() {
		case fvmerrors.ErrCodeAccountNotFoundError:
			return access.NewDataNotFoundError("account", err)
		case fvmerrors.ErrCodeAccountPublicKeyNotFoundError:
			return access.NewDataNotFoundError("account public key", err)
		case fvmerrors.ErrCodeComputationLimitExceededError:
			return access.NewResourceExhausted(err)
		default:
			// runtime errors
			return access.NewInvalidRequestError(err)
		}
	}

	return err
}
