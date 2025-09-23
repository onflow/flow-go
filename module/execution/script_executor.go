package execution

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// TODO(Uliana): add godoc
type ScriptExecutor interface {
	// ExecuteAtBlockHeight executes provided script against the block height.
	// A result value is returned encoded as byte array. An error will be returned if script
	// doesn't successfully execute.
	// Expected errors:
	// - storage.ErrNotFound if block or registerSnapshot value at height was not found.
	// - storage.ErrHeightNotIndexed if the data for the block height is not available
	ExecuteAtBlockHeight(
		ctx context.Context,
		script []byte,
		arguments [][]byte,
		height uint64,
		registerSnapshot storage.RegisterSnapshotReader,
	) ([]byte, error)

	// GetAccountAtBlockHeight returns a Flow account by the provided address and block height.
	// Expected errors:
	// - storage.ErrHeightNotIndexed if the data for the block height is not available
	GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64, registerSnapshot storage.RegisterSnapshotReader) (*flow.Account, error)

	// GetAccountBalance returns a Flow account balance by the provided address and block height.
	// Expected errors:
	// - storage.ErrHeightNotIndexed if the data for the block height is not available
	GetAccountBalance(ctx context.Context, address flow.Address, height uint64, registerSnapshot storage.RegisterSnapshotReader) (uint64, error)

	// GetAccountAvailableBalance returns a Flow account available balance by the provided address and block height.
	// Expected errors:
	// - storage.ErrHeightNotIndexed if the data for the block height is not available
	GetAccountAvailableBalance(ctx context.Context, address flow.Address, height uint64, registerSnapshot storage.RegisterSnapshotReader) (uint64, error)

	// GetAccountKeys returns a Flow account public keys by the provided address and block height.
	// Expected errors:
	// - storage.ErrHeightNotIndexed if the data for the block height is not available
	GetAccountKeys(ctx context.Context, address flow.Address, height uint64, registerSnapshot storage.RegisterSnapshotReader) ([]flow.AccountPublicKey, error)

	// GetAccountKey returns a Flow account public key by the provided address, block height and index.
	// Expected errors:
	// - storage.ErrHeightNotIndexed if the data for the block height is not available
	GetAccountKey(ctx context.Context, address flow.Address, keyIndex uint32, height uint64, registerSnapshot storage.RegisterSnapshotReader) (*flow.AccountPublicKey, error)
}
