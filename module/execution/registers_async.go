package execution

import (
	"fmt"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// RegistersAsyncStore has the same basic structure as access/backend.ScriptExecutor
// TODO: use this implementation in the `scripts.ScriptExecutor` passed into the AccessAPI
type RegistersAsyncStore struct {
	registerIndex atomic.Pointer[storage.RegisterIndex]
}

func NewRegistersAsyncStore() *RegistersAsyncStore {
	return &RegistersAsyncStore{atomic.Pointer[storage.RegisterIndex]{}}
}

// InitDataAvailable initializes the underlying storage.RegisterIndex
// This method can be called at any time after the RegistersAsyncStore object is created and before RegisterValues is called
// since we can't disambiguate between the underlying store before bootstrapping or just simply being behind sync
func (r *RegistersAsyncStore) InitDataAvailable(registers storage.RegisterIndex) error {
	if r.registerIndex.CompareAndSwap(nil, r.registerIndex.Load()) {
		r.registerIndex.Store(&registers)
		return nil
	}
	return fmt.Errorf("registers already initialized")
}

// RegisterValues gets the register values from the underlying storage.RegisterIndex
// Expected errors:
//   - storage.ErrHeightNotIndexed if the store is still bootstrapping or if the values at the height is not indexed yet
//   - storage.ErrNotFound if the register does not exist at the height
func (r *RegistersAsyncStore) RegisterValues(ids flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error) {
	if !r.isDataAvailable(height) {
		return nil, storage.ErrHeightNotIndexed
	}
	result := make([]flow.RegisterValue, len(ids))
	registerStore := *r.registerIndex.Load()
	for i, regId := range ids {
		val, err := registerStore.Get(regId, height)
		if err != nil {
			return nil, fmt.Errorf("failed to get register value for id %d: %w", i, err)
		}
		result[i] = val
	}
	return result, nil
}

func (r *RegistersAsyncStore) isDataAvailable(height uint64) bool {
	if r.registerIndex.Load() != nil {
		registerStore := *r.registerIndex.Load()
		return height <= registerStore.LatestHeight() && height >= registerStore.FirstHeight()
	}
	return false
}
