package storehouse

import (
	"sync"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
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

func (s *BlockEndStateSnapshot) Get(id flow.RegisterID) (flow.RegisterValue, error) {
	value, ok := s.getFromCache(id)
	if ok {
		return value, nil
	}

	value, err := s.getFromStorage(id)
	if err != nil {
		return nil, err
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.readCache[id] = value
	return value, nil
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
