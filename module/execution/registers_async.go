package execution

import (
	"fmt"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	"github.com/onflow/flow-go/storage"
)

// RegistersAsyncStore wraps an underlying register store so it can be used before the index is
// initialized.
type RegistersAsyncStore struct {
	registerIndex *atomic.Pointer[storage.RegisterIndex]
}

func NewRegistersAsyncStore() *RegistersAsyncStore {
	return &RegistersAsyncStore{
		registerIndex: atomic.NewPointer[storage.RegisterIndex](nil),
	}
}

// Initialize initializes the underlying storage.RegisterIndex
// This method can be called at any time after the RegisterStore object is created. and before RegisterValues is called
// since we can't disambiguate between the underlying store before bootstrapping or just simply being behind sync
func (r *RegistersAsyncStore) Initialize(registers storage.RegisterIndex) error {
	if r.registerIndex.CompareAndSwap(nil, &registers) {
		return nil
	}
	return fmt.Errorf("registers already initialized")
}

// RegisterValues gets the register values from the underlying storage.RegisterIndex
// Expected errors:
//   - indexer.ErrIndexNotInitialized if the store is still bootstrapping
//   - storage.ErrHeightNotIndexed if the values at the height is not indexed yet
//   - storage.ErrNotFound if the register does not exist at the height
func (r *RegistersAsyncStore) RegisterValues(ids flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error) {
	registerStore, err := r.getRegisterStore()
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

func (r *RegistersAsyncStore) getRegisterStore() (storage.RegisterIndex, error) {
	registerStore := r.registerIndex.Load()
	if registerStore == nil {
		return nil, indexer.ErrIndexNotInitialized
	}

	return *registerStore, nil
}
