package proxies

import (
	"fmt"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// RegistersStore has the same basic structure as access/backend.ScriptExecutor
// TODO: use this implementation in the `scripts.ScriptExecutor` passed into the AccessAPI
type RegistersStore struct {
	registerIndex atomic.Pointer[storage.RegisterIndex]
}

func NewRegistersStore() *RegistersStore {
	return &RegistersStore{atomic.Pointer[storage.RegisterIndex]{}}
}

// Initialize initializes the underlying storage.RegisterIndex
// This method can be called at any time after the RegistersStore object is created and before RegisterValues is called
// since we can't disambiguate between the underlying store before bootstrapping or just simply being behind sync
func (r *RegistersStore) Initialize(registers storage.RegisterIndex) error {
	if r.registerIndex.CompareAndSwap(nil, &registers) {
		return nil
	}
	return fmt.Errorf("registers already initialized")
}

// RegisterValues gets the register values from the underlying storage.RegisterIndex
// Expected errors:
//   - storage.ErrHeightNotIndexed if the store is still bootstrapping or if the values at the height is not indexed yet
//   - storage.ErrNotFound if the register does not exist at the height
func (r *RegistersStore) RegisterValues(ids flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error) {
	registerStore, isAvailable := r.isDataAvailable(height)
	if !isAvailable {
		return nil, storage.ErrHeightNotIndexed
	}
	result := make([]flow.RegisterValue, len(ids))
	for i, regId := range ids {
		val, err := registerStore.Get(regId, height)
		if err != nil {
			return nil, fmt.Errorf("failed to get register value for id %d: %w", i, err)
		}
		result[i] = val
	}
	return result, nil
}

func (r *RegistersStore) isDataAvailable(height uint64) (storage.RegisterIndex, bool) {
	str := r.registerIndex.Load()
	if str != nil {
		registerStore := *str
		return registerStore, height <= registerStore.LatestHeight() && height >= registerStore.FirstHeight()
	}
	return nil, false
}
