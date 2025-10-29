package execution

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// ScriptExecutor is used to execute Cadence scripts and querying account data.
type ScriptExecutor interface {
	// ExecuteAtBlockHeight executes provided script against the block height.
	// A result value is returned encoded as byte array.
	//
	// Expected error returns during normal operation:
	//   - [version.ErrOutOfRange] - if incoming block height is higher that last handled block height.
	//   - [execution.ErrIncompatibleNodeVersion] - if the block height is not compatible with the node version.
	//   - [storage.ErrNotFound] - if block or registerSnapshot value at height was not found.
	//   - [storage.ErrHeightNotIndexed] - if the requested height is outside the range of indexed blocks.
	//   - [fvmerrors.ErrCodeScriptExecutionCancelledError] - if script execution canceled.
	//   - [fvmerrors.ErrCodeScriptExecutionTimedOutError] - if script execution timed out.
	//   - [fvmerrors.ErrCodeComputationLimitExceededError] - if script execution computation limit exceeded.
	//   - [fvmerrors.ErrCodeMemoryLimitExceededError] - if script execution memory limit exceeded.
	//   - [indexer.ErrIndexNotInitialized] - if data for block is not available.
	ExecuteAtBlockHeight(
		ctx context.Context,
		script []byte,
		arguments [][]byte,
		height uint64,
		registerSnapshot storage.RegisterSnapshotReader,
	) ([]byte, error)

	// GetAccountAtBlockHeight returns a Flow account by the provided address and block height.
	//
	// Expected error returns during normal operation:
	//   - [version.ErrOutOfRange] - if incoming block height is higher that last handled block height.
	//   - [execution.ErrIncompatibleNodeVersion] - if the block height is not compatible with the node version.
	//   - [storage.ErrNotFound] - if block or registerSnapshot value at height was not found.
	//   - [storage.ErrHeightNotIndexed] - if the requested height is outside the range of indexed blocks.
	GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64, registerSnapshot storage.RegisterSnapshotReader) (*flow.Account, error)

	// GetAccountBalance returns a Flow account balance by the provided address and block height.
	//
	// Expected error returns during normal operation:
	//   - [version.ErrOutOfRange] - if incoming block height is higher that last handled block height.
	//   - [execution.ErrIncompatibleNodeVersion] - if the block height is not compatible with the node version.
	//   - [storage.ErrNotFound] - if block or registerSnapshot value at height was not found.
	//   - [storage.ErrHeightNotIndexed] - if the requested height is outside the range of indexed blocks.
	GetAccountBalance(ctx context.Context, address flow.Address, height uint64, registerSnapshot storage.RegisterSnapshotReader) (uint64, error)

	// GetAccountAvailableBalance returns a Flow account available balance by the provided address and block height.
	//
	// Expected error returns during normal operation:
	//   - [version.ErrOutOfRange] - if incoming block height is higher that last handled block height.
	//   - [execution.ErrIncompatibleNodeVersion] - if the block height is not compatible with the node version.
	//   - [storage.ErrNotFound] - if block or registerSnapshot value at height was not found.
	//   - [storage.ErrHeightNotIndexed] - if the requested height is outside the range of indexed blocks.
	GetAccountAvailableBalance(ctx context.Context, address flow.Address, height uint64, registerSnapshot storage.RegisterSnapshotReader) (uint64, error)

	// GetAccountKeys returns a Flow account public keys by the provided address and block height.
	//
	// Expected error returns during normal operation:
	//   - [version.ErrOutOfRange] - if incoming block height is higher that last handled block height.
	//   - [execution.ErrIncompatibleNodeVersion] - if the block height is not compatible with the node version.
	//   - [storage.ErrNotFound] - if block or registerSnapshot value at height was not found.
	//   - [storage.ErrHeightNotIndexed] - if the requested height is outside the range of indexed blocks.
	GetAccountKeys(ctx context.Context, address flow.Address, height uint64, registerSnapshot storage.RegisterSnapshotReader) ([]flow.AccountPublicKey, error)

	// GetAccountKey returns a Flow account public key by the provided address, block height and index.
	//
	// Expected error returns during normal operation:
	//   - [version.ErrOutOfRange] - if incoming block height is higher that last handled block height.
	//   - [execution.ErrIncompatibleNodeVersion] - if the block height is not compatible with the node version.
	//   - [storage.ErrNotFound] - if block or registerSnapshot value at height was not found.
	//   - [storage.ErrHeightNotIndexed] - if the requested height is outside the range of indexed blocks.
	GetAccountKey(ctx context.Context, address flow.Address, keyIndex uint32, height uint64, registerSnapshot storage.RegisterSnapshotReader) (*flow.AccountPublicKey, error)
}
