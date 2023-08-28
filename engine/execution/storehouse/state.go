package storehouse

import (
	"sync"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/model/flow"
)

type BlockStorageSnapshot struct {
	storage         execution.RegisterStore
	commitment      flow.StateCommitment
	blockID         flow.Identifier
	height          uint64
	registerUpdates map[flow.RegisterID]flow.RegisterValue

	mutex     sync.RWMutex
	readCache map[flow.RegisterID]flow.RegisterValue // cache the reads from storage
}

func NewBlockStorageSnapshot(
	commitment flow.StateCommitment,
	blockID flow.Identifier,
	height uint64,
	registerUpdates map[flow.RegisterID]flow.RegisterValue,
) *BlockStorageSnapshot {
	if registerUpdates == nil {
		registerUpdates = make(map[flow.RegisterID]flow.RegisterValue)
	}
	return &BlockStorageSnapshot{
		commitment:      commitment,
		blockID:         blockID,
		height:          height,
		registerUpdates: registerUpdates,
		readCache:       make(map[flow.RegisterID]flow.RegisterValue),
	}
}

func (s *BlockStorageSnapshot) Commitment() flow.StateCommitment {
	return s.commitment
}

func (s *BlockStorageSnapshot) BlockID() flow.Identifier {
	return s.blockID
}

func (s *BlockStorageSnapshot) Height() uint64 {
	return s.height
}

func (s *BlockStorageSnapshot) Get(id flow.RegisterID) (flow.RegisterValue, error) {
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

func (s *BlockStorageSnapshot) RegisterUpdates() map[flow.RegisterID]flow.RegisterValue {
	return s.registerUpdates
}

func (s *BlockStorageSnapshot) getFromCache(id flow.RegisterID) (flow.RegisterValue, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	value, ok := s.readCache[id]
	return value, ok
}

func (s *BlockStorageSnapshot) getFromStorage(id flow.RegisterID) (flow.RegisterValue, error) {
	return s.storage.GetRegister(s.height, s.blockID, id)
}
