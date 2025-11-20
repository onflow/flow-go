package execution

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// ScriptExecutor is used to execute Cadence scripts and query account data.
type ScriptExecutor interface {
	// ExecuteAtBlockHeight executes the provided script against the given block height.
	// A result value is returned encoded as a byte array.
	//
	// Expected error returns during normal operation:
	//   - [version.ErrOutOfRange]: If the incoming block height is higher than the last handled block height.
	//   - [execution.ErrIncompatibleNodeVersion]: If the block height is not compatible with the node version.
	//   - [storage.ErrNotFound]: If no block is finalized at the provided height.
	//   - [storage.ErrHeightNotIndexed]: If the requested height is outside the range of indexed blocks.
	//   - [fvmerrors.ErrCodeScriptExecutionCancelledError]: If script execution is cancelled.
	//   - [fvmerrors.ErrCodeScriptExecutionTimedOutError]: If script execution timed out.
	//   - [fvmerrors.ErrCodeComputationLimitExceededError]: If script execution exceeded the computation limit.
	//   - [fvmerrors.ErrCodeMemoryLimitExceededError]: If script execution exceeded the memory limit.
	//   - [fvmerrors.FailureCodeLedgerFailure] - if the script execution fails due to ledger errors.
	ExecuteAtBlockHeight(
		ctx context.Context,
		script []byte,
		arguments [][]byte,
		height uint64,
		registerSnapshot storage.RegisterSnapshotReader,
	) ([]byte, error)

	// GetAccountAtBlockHeight returns a Flow account for the provided address and block height.
	//
	// Expected error returns during normal operation:
	//   - [version.ErrOutOfRange]: If the incoming block height is higher than the last handled block height.
	//   - [execution.ErrIncompatibleNodeVersion]: If the block height is not compatible with the node version.
	//   - [storage.ErrNotFound]: If no block is finalized at the provided height.
	//   - [storage.ErrHeightNotIndexed]: If the requested height is outside the range of indexed blocks.
	//   - [fvmerrors.ErrCodeAccountNotFoundError]: If the account is not found by address.
	GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64, registerSnapshot storage.RegisterSnapshotReader) (*flow.Account, error)

	// GetAccountBalance returns the balance of a Flow account by the provided address and block height.
	//
	// Expected error returns during normal operation:
	//   - [version.ErrOutOfRange]: If the incoming block height is higher than the last handled block height.
	//   - [execution.ErrIncompatibleNodeVersion]: If the block height is not compatible with the node version.
	//   - [storage.ErrNotFound]: If no block is finalized at the provided height.
	//   - [storage.ErrHeightNotIndexed]: If the requested height is outside the range of indexed blocks.
	GetAccountBalance(ctx context.Context, address flow.Address, height uint64, registerSnapshot storage.RegisterSnapshotReader) (uint64, error)

	// GetAccountAvailableBalance returns the available balance of a Flow account by the provided address and block height.
	//
	// Expected error returns during normal operation:
	//   - [version.ErrOutOfRange]: If the incoming block height is higher than the last handled block height.
	//   - [execution.ErrIncompatibleNodeVersion]: If the block height is not compatible with the node version.
	//   - [storage.ErrNotFound]: If no block is finalized at the provided height.
	//   - [storage.ErrHeightNotIndexed]: If the requested height is outside the range of indexed blocks.
	GetAccountAvailableBalance(ctx context.Context, address flow.Address, height uint64, registerSnapshot storage.RegisterSnapshotReader) (uint64, error)

	// GetAccountKeys returns the public keys of a Flow account by the provided address and block height.
	//
	// Expected error returns during normal operation:
	//   - [version.ErrOutOfRange]: If the incoming block height is higher than the last handled block height.
	//   - [execution.ErrIncompatibleNodeVersion]: If the block height is not compatible with the node version.
	//   - [storage.ErrNotFound]: If no block is finalized at the provided height.
	//   - [storage.ErrHeightNotIndexed]: If the requested height is outside the range of indexed blocks.
	//   - [fvmerrors.ErrCodeAccountPublicKeyNotFoundError]: If public keys are not found for the given address.
	GetAccountKeys(ctx context.Context, address flow.Address, height uint64, registerSnapshot storage.RegisterSnapshotReader) ([]flow.AccountPublicKey, error)

	// GetAccountKey returns a public key of a Flow account by the provided address, block height, and key index.
	//
	// Expected error returns during normal operation:
	//   - [version.ErrOutOfRange]: If the incoming block height is higher than the last handled block height.
	//   - [execution.ErrIncompatibleNodeVersion]: If the block height is not compatible with the node version.
	//   - [storage.ErrNotFound]: If no block is finalized at the provided height.
	//   - [storage.ErrHeightNotIndexed]: If the requested height is outside the range of indexed blocks.
	//   - [fvmerrors.ErrCodeAccountPublicKeyNotFoundError]: If a public key is not found for the given address and key index.
	GetAccountKey(ctx context.Context, address flow.Address, keyIndex uint32, height uint64, registerSnapshot storage.RegisterSnapshotReader) (*flow.AccountPublicKey, error)
}
