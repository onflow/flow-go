package storage

import (
	"github.com/jordanschalm/lockctx"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

// ContractDeploymentIterator iterates over contract deployments with a [accessmodel.ContractDeploymentsCursor].
type ContractDeploymentIterator = IndexIterator[accessmodel.ContractDeployment, accessmodel.ContractDeploymentsCursor]

// ContractDeploymentsIndexReader provides read access to the contract deployments index.
//
// All methods are safe for concurrent access.
type ContractDeploymentsIndexReader interface {
	// ByContract returns the most recent deployment for the given contract.
	//
	// Expected error returns during normal operation:
	//   - [ErrNotFound]: if no deployment for the given contract exists
	//   - [ErrNotBootstrapped]: if the index has not been initialized
	ByContract(account flow.Address, name string) (accessmodel.ContractDeployment, error)

	// DeploymentsByContract returns an iterator over all recorded deployments for the given
	// contract, ordered from most recent to oldest (descending block height).
	//
	// cursor is a pointer to an [accessmodel.ContractDeploymentsCursor]:
	//   - nil means start from the most recent deployment
	//   - non-nil means start at the cursor position (inclusive)
	//
	// Expected error returns during normal operation:
	//   - [ErrNotBootstrapped]: if the index has not been initialized
	DeploymentsByContract(
		account flow.Address,
		name string,
		cursor *accessmodel.ContractDeploymentsCursor,
	) (ContractDeploymentIterator, error)

	// ByAddress returns an iterator over the latest deployment for each contract deployed
	// by the given address, ordered by contract identifier (ascending).
	//
	// cursor is a pointer to an [accessmodel.ContractDeploymentsCursor]:
	//   - nil means start from the first contract (by identifier)
	//   - non-nil resumes from cursor.Address and cursor.ContractName (inclusive); other cursor fields are ignored
	//
	// Expected error returns during normal operation:
	//   - [ErrNotBootstrapped]: if the index has not been initialized
	ByAddress(
		account flow.Address,
		cursor *accessmodel.ContractDeploymentsCursor,
	) (ContractDeploymentIterator, error)

	// All returns an iterator over the latest deployment for each indexed contract,
	// ordered by contract identifier (ascending).
	//
	// cursor is a pointer to an [accessmodel.ContractDeploymentsCursor]:
	//   - nil means start from the first contract (by identifier)
	//   - non-nil resumes from cursor.Address and cursor.ContractName (inclusive); other cursor fields are ignored
	//
	// Expected error returns during normal operation:
	//   - [ErrNotBootstrapped]: if the index has not been initialized
	All(cursor *accessmodel.ContractDeploymentsCursor) (ContractDeploymentIterator, error)
}

// ContractDeploymentsIndexRangeReader provides access to the range of indexed heights.
//
// All methods are safe for concurrent access.
type ContractDeploymentsIndexRangeReader interface {
	// FirstIndexedHeight returns the first (oldest) block height that has been indexed.
	FirstIndexedHeight() uint64

	// LatestIndexedHeight returns the latest block height that has been indexed.
	LatestIndexedHeight() uint64
}

// ContractDeploymentsIndexWriter provides write access to the contract deployments index.
//
// NOT CONCURRENTLY SAFE.
type ContractDeploymentsIndexWriter interface {
	// Store indexes all contract deployments from the given block and advances the latest indexed
	// height to blockHeight. Must be called with consecutive heights.
	// The caller must hold the [LockIndexContractDeployments] lock until the batch is committed.
	//
	// Expected error returns during normal operation:
	//   - [ErrAlreadyExists]: if blockHeight has already been indexed
	Store(
		lctx lockctx.Proof,
		rw ReaderBatchWriter,
		blockHeight uint64,
		deployments []accessmodel.ContractDeployment,
	) error
}

// ContractDeploymentsIndex provides full read and write access to the contract deployments index.
type ContractDeploymentsIndex interface {
	ContractDeploymentsIndexReader
	ContractDeploymentsIndexRangeReader
	ContractDeploymentsIndexWriter
}

// ContractDeploymentsIndexBootstrapper wraps [ContractDeploymentsIndex] and performs
// just-in-time initialization of the index when the initial block is provided.
//
// All read and write methods proxy to the underlying index once initialized.
type ContractDeploymentsIndexBootstrapper interface {
	ContractDeploymentsIndexReader
	ContractDeploymentsIndexWriter

	// FirstIndexedHeight returns the first (oldest) block height that has been indexed.
	//
	// Expected error returns during normal operation:
	//   - [ErrNotBootstrapped]: if the index has not been initialized
	FirstIndexedHeight() (uint64, error)

	// LatestIndexedHeight returns the latest block height that has been indexed.
	//
	// Expected error returns during normal operation:
	//   - [ErrNotBootstrapped]: if the index has not been initialized
	LatestIndexedHeight() (uint64, error)

	// UninitializedFirstHeight returns the height the index will accept as the first height,
	// and a boolean indicating whether the index is already initialized.
	UninitializedFirstHeight() (uint64, bool)
}
