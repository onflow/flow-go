package storehouse

import (
	"errors"
	"sync"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

var _ snapshot.StorageSnapshot = (*BlockEndStateSnapshot)(nil)

// BlockEndStateSnapshot represents the storage at the end of a block.
type BlockEndStateSnapshot struct {
	storage execution.RegisterStore

	blockID flow.Identifier
	height  uint64

	mutex     sync.RWMutex
	readCache map[flow.RegisterID]flow.RegisterValue // cache the reads from storage at baseBlock
}

// the caller must ensure the block height is for the given block
func NewBlockEndStateSnapshot(
	storage execution.RegisterStore,
	blockID flow.Identifier,
	height uint64,
) *BlockEndStateSnapshot {
	return &BlockEndStateSnapshot{
		storage:   storage,
		blockID:   blockID,
		height:    height,
		readCache: make(map[flow.RegisterID]flow.RegisterValue),
	}
}

// Get returns the value of the register with the given register ID.
// It returns:
// - (value, nil) if the register exists
// - (nil, nil) if the register does not exist
// - (nil, storage.ErrHeightNotIndexed) if the height is below the first height that is indexed.
// - (nil, storehouse.ErrNotExecuted) if the block is not executed yet
// - (nil, storehouse.ErrNotExecuted) if the block is conflicting with finalized block
// - (nil, err) for any other exceptions
func (s *BlockEndStateSnapshot) Get(id flow.RegisterID) (flow.RegisterValue, error) {
	value, ok := s.getFromCache(id)
	if ok {
		if value == nil {
			return nil, storage.ErrNotFound
		}
		return value, nil
	}

	value, err := s.getFromStorage(id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			// if the error is not found, we return a nil RegisterValue,
			// in this case, the nil value can be cached, because the storage will not change it
			value = nil
		} else {
			// if the error is not ErrNotFound, such as storage.ErrHeightNotIndexed, storehouse.ErrNotExecuted
			// we return the error without caching
			return nil, err
		}
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// TODO: consider adding a limit/eviction policy for the cache
	s.readCache[id] = value
	return value, err
}

func (s *BlockEndStateSnapshot) getFromCache(id flow.RegisterID) (flow.RegisterValue, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	value, ok := s.readCache[id]
	return value, ok
}

func (s *BlockEndStateSnapshot) getFromStorage(id flow.RegisterID) (flow.RegisterValue, error) {
	return s.storage.GetRegister(s.height, s.blockID, id)
}
