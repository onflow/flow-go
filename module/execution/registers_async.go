package execution

import (
	"fmt"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	"github.com/onflow/flow-go/storage"
)

// RegistersAsyncStore wraps an underlying RegisterSnapshotReader so it can be used before the storage is
// initialized.
type RegistersAsyncStore struct {
	registerSnapshotReader *atomic.Pointer[storage.RegisterSnapshotReader]
}

// NewRegistersAsyncStore creates a new RegistersAsyncStore instance with no
// underlying RegisterSnapshotReader initialized yet.
//
// The storage must be initialized later by calling [RegistersAsyncStore.Initialize].
func NewRegistersAsyncStore() *RegistersAsyncStore {
	return &RegistersAsyncStore{
		registerSnapshotReader: atomic.NewPointer[storage.RegisterSnapshotReader](nil),
	}
}

// Initialize initializes the underlying storage.RegisterSnapshotReader.
// This method can be called at any time after the RegisterSnapshotReader object is created and before RegisterValues is called
// since we can't disambiguate between the underlying storage before bootstrapping or just simply being behind sync.
//
// No error returns are expected during normal operations.
func (r *RegistersAsyncStore) Initialize(registers storage.RegisterSnapshotReader) error {
	if r.registerSnapshotReader.CompareAndSwap(nil, &registers) {
		return nil
	}
	return fmt.Errorf("registers storage already initialized")
}

// RegisterValues gets the register values from the underlying storage.RegisterSnapshotReader.
//
// Expected error returns during normal operation:
//   - [indexer.ErrIndexNotInitialized]: If the storage is still bootstrapping.
//   - [storage.ErrHeightNotIndexed]: If the requested height is below the first indexed height or above the latest indexed height.
//   - [storage.ErrNotFound]: If the register does not exist at the height.
//
// TODO: Refactor state stream backend to use snapshot.StorageSnapshot directly and remove this method.
func (r *RegistersAsyncStore) RegisterValues(ids flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error) {
	registerStore, err := r.RegisterSnapshotReader()
	if err != nil {
		return nil, err
	}

	if height > registerStore.LatestHeight() || height < registerStore.FirstHeight() {
		return nil, storage.ErrHeightNotIndexed
	}

	result := make([]flow.RegisterValue, len(ids))
	for i, regID := range ids {
		val, err := registerStore.Get(regID, height)
		if err != nil {
			return nil, fmt.Errorf("failed to get register value for id %d: %w", i, err)
		}
		result[i] = val
	}
	return result, nil
}

// RegisterSnapshotReader returns the underlying [storage.RegisterSnapshotReader] if it has been initialized.
//
// Expected error returns during normal operation:
//   - [indexer.ErrIndexNotInitialized]: If the storage is still bootstrapping.
func (r *RegistersAsyncStore) RegisterSnapshotReader() (storage.RegisterSnapshotReader, error) {
	registerSnapshotReader := r.registerSnapshotReader.Load()
	if registerSnapshotReader == nil {
		return nil, indexer.ErrIndexNotInitialized
	}

	return *registerSnapshotReader, nil
}
