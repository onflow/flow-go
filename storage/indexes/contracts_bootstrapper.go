package indexes

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"
	"go.uber.org/atomic"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// ContractDeploymentsBootstrapper wraps a [ContractDeploymentsIndex] and performs
// just-in-time initialization of the index when the initial block is provided.
//
// Contract deployments may not be available for the root block during bootstrapping.
// This struct acts as a proxy for the underlying [ContractDeploymentsIndex] and
// encapsulates the complexity of initializing the index when the initial block is eventually provided.
type ContractDeploymentsBootstrapper struct {
	db                 storage.DB
	initialStartHeight uint64

	store *atomic.Pointer[ContractDeploymentsIndex]
}

var _ storage.ContractDeploymentsIndexBootstrapper = (*ContractDeploymentsBootstrapper)(nil)

// NewContractDeploymentsBootstrapper creates a new contract deployments bootstrapper.
//
// No error returns are expected during normal operation.
func NewContractDeploymentsBootstrapper(db storage.DB, initialStartHeight uint64) (*ContractDeploymentsBootstrapper, error) {
	store, err := NewContractDeploymentsIndex(db)
	if err != nil {
		if !errors.Is(err, storage.ErrNotBootstrapped) {
			return nil, fmt.Errorf("could not create contract deployments index: %w", err)
		}
		// not yet bootstrapped — start with a nil store
		store = nil
	}

	return &ContractDeploymentsBootstrapper{
		db:                 db,
		initialStartHeight: initialStartHeight,
		store:              atomic.NewPointer(store),
	}, nil
}

// FirstIndexedHeight returns the first (oldest) block height that has been indexed.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotBootstrapped] if the index has not been initialized
func (b *ContractDeploymentsBootstrapper) FirstIndexedHeight() (uint64, error) {
	store := b.store.Load()
	if store == nil {
		return 0, storage.ErrNotBootstrapped
	}
	return store.FirstIndexedHeight(), nil
}

// LatestIndexedHeight returns the latest block height that has been indexed.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotBootstrapped] if the index has not been initialized
func (b *ContractDeploymentsBootstrapper) LatestIndexedHeight() (uint64, error) {
	store := b.store.Load()
	if store == nil {
		return 0, storage.ErrNotBootstrapped
	}
	return store.LatestIndexedHeight(), nil
}

// UninitializedFirstHeight returns the height the index will accept as the first height, and a
// boolean indicating if the index is initialized.
// If the index is not initialized, the first call to Store must include data for this height.
func (b *ContractDeploymentsBootstrapper) UninitializedFirstHeight() (uint64, bool) {
	store := b.store.Load()
	if store == nil {
		return b.initialStartHeight, false
	}
	return store.FirstIndexedHeight(), true
}

// ByContractID returns the most recent deployment for the given contract identifier.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotBootstrapped] if the index has not been initialized
//   - [storage.ErrNotFound] if no deployment for the given contract ID exists
func (b *ContractDeploymentsBootstrapper) ByContractID(id string) (accessmodel.ContractDeployment, error) {
	store := b.store.Load()
	if store == nil {
		return accessmodel.ContractDeployment{}, storage.ErrNotBootstrapped
	}
	return store.ByContractID(id)
}

// DeploymentsByContractID returns an iterator over all recorded deployments for the given contract,
// ordered from most recent to oldest. See [ContractDeploymentsIndex.DeploymentsByContractID].
//
// Expected error returns during normal operations:
//   - [storage.ErrNotBootstrapped] if the index has not been initialized
func (b *ContractDeploymentsBootstrapper) DeploymentsByContractID(
	id string,
	cursor *accessmodel.ContractDeploymentCursor,
) (storage.ContractDeploymentIterator, error) {
	store := b.store.Load()
	if store == nil {
		return nil, storage.ErrNotBootstrapped
	}
	return store.DeploymentsByContractID(id, cursor)
}

// ByAddress returns an iterator over the latest deployment for each contract deployed by the
// given address. See [ContractDeploymentsIndex.ByAddress].
//
// Expected error returns during normal operations:
//   - [storage.ErrNotBootstrapped] if the index has not been initialized
func (b *ContractDeploymentsBootstrapper) ByAddress(
	account flow.Address,
	cursor *accessmodel.ContractDeploymentCursor,
) (storage.ContractDeploymentIterator, error) {
	store := b.store.Load()
	if store == nil {
		return nil, storage.ErrNotBootstrapped
	}
	return store.ByAddress(account, cursor)
}

// All returns an iterator over the latest deployment for each indexed contract.
// See [ContractDeploymentsIndex.All].
//
// Expected error returns during normal operations:
//   - [storage.ErrNotBootstrapped] if the index has not been initialized
func (b *ContractDeploymentsBootstrapper) All(
	cursor *accessmodel.ContractDeploymentCursor,
) (storage.ContractDeploymentIterator, error) {
	store := b.store.Load()
	if store == nil {
		return nil, storage.ErrNotBootstrapped
	}
	return store.All(cursor)
}

// Store indexes all contract deployments from the given block and advances the latest indexed
// height to blockHeight. Must be called with consecutive heights.
// The caller must hold the [storage.LockIndexContractDeployments] lock until the batch is committed.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotBootstrapped] if the index has not been initialized and the provided block
//     height is not the initial start height
//   - [storage.ErrAlreadyExists] if the block height is already indexed
func (b *ContractDeploymentsBootstrapper) Store(
	lctx lockctx.Proof,
	rw storage.ReaderBatchWriter,
	blockHeight uint64,
	deployments []accessmodel.ContractDeployment,
) error {
	// if the index is already initialized, store the data directly
	if store := b.store.Load(); store != nil {
		return store.Store(lctx, rw, blockHeight, deployments)
	}

	// otherwise bootstrap the index
	if blockHeight != b.initialStartHeight {
		return fmt.Errorf("expected first indexed height %d, got %d: %w", b.initialStartHeight, blockHeight, storage.ErrNotBootstrapped)
	}

	store, err := BootstrapContractDeployments(lctx, rw, b.db, b.initialStartHeight, deployments)
	if err != nil {
		return fmt.Errorf("could not initialize contract deployments storage: %w", err)
	}

	if !b.store.CompareAndSwap(nil, store) {
		// this should never happen. if it does, there is a bug. this indicates another goroutine
		// successfully initialized `store` since we checked the value above. since the bootstrap
		// operation is protected by the lock and it performs sanity checks to ensure the table
		// is actually empty, the bootstrap operation should fail if there was concurrent access.
		return fmt.Errorf("contract deployments index initialized during bootstrap")
	}

	return nil
}
