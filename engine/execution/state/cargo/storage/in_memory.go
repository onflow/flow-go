package storage

import (
	"bytes"
	"sync"

	"github.com/onflow/flow-go/model/flow"
)

type valueAtHeight struct {
	height uint64
	value  flow.RegisterValue
}

// InMemoryStorage implements storage interface, keeping register updates in memory,
// it only limits historic look up of register to the last X commits
type InMemoryStorage struct {
	historyCapacity                 int
	registers                       map[flow.RegisterID][]valueAtHeight
	lastCommittedBlock              *flow.Header
	recentlyCommittedBlocksByHeight map[uint64]flow.Identifier
	minHeightAvailable              uint64
	lock                            sync.RWMutex
}

var _ Storage = &InMemoryStorage{}

// NewInMemoryStorage constructs a new InMemoryStorage
func NewInMemoryStorage(
	historyCapacity int,
	genesis *flow.Header,
	data map[flow.RegisterID]flow.RegisterValue,
) *InMemoryStorage {
	registers := make(map[flow.RegisterID][]valueAtHeight, 0)
	for id, val := range data {
		registers[id] = []valueAtHeight{{
			height: genesis.Height,
			value:  val,
		}}
	}
	recentlyCommittedBlocksByHeight := map[uint64]flow.Identifier{
		genesis.Height: genesis.ID(),
	}
	return &InMemoryStorage{
		historyCapacity:                 historyCapacity,
		registers:                       registers,
		recentlyCommittedBlocksByHeight: recentlyCommittedBlocksByHeight,
		lastCommittedBlock:              genesis,
		minHeightAvailable:              genesis.Height,
	}
}

// CommitBlock commits block updates
func (s *InMemoryStorage) CommitBlock(header *flow.Header, update map[flow.RegisterID]flow.RegisterValue) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	// check commit sequence
	if s.lastCommittedBlock.Height+1 != header.Height || s.lastCommittedBlock.ID() != header.ParentID {
		return &NonCompliantHeaderError{
			s.lastCommittedBlock.Height + 1,
			header.Height,
			s.lastCommittedBlock.ID(),
			header.ParentID,
		}
	}

	for key, value := range update {
		vv, found := s.registers[key]
		if !found {
			// Note: this pre-allocation of memory might not be a good idea for registers that
			// don't get updated that often, we might revisit in the future
			vv = make([]valueAtHeight, 0, s.historyCapacity)
		}

		// optimization - if the value hasn't change, don't append and skip
		if len(vv) > 0 && bytes.Equal(vv[len(vv)-1].value, value) {
			continue
		}

		// prune if at capacity (prune first prevents potentially extra allocation)
		if len(vv) == s.historyCapacity {
			vv = vv[1:]
		}

		// TODO - future optimization might consider pruning historic values that are not going to be accssible anymore

		// append value to the end of the list
		s.registers[key] = append(vv, valueAtHeight{
			height: header.Height,
			value:  value,
		})
	}
	s.lastCommittedBlock = header
	newHeight := header.Height
	s.recentlyCommittedBlocksByHeight[newHeight] = header.ID()

	if newHeight-s.minHeightAvailable == uint64(s.historyCapacity) {
		delete(s.recentlyCommittedBlocksByHeight, s.minHeightAvailable)
		s.minHeightAvailable += 1
	}

	return nil
}

// LastCommittedBlock returns the last commited block
func (s *InMemoryStorage) LastCommittedBlock() (*flow.Header, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.lastCommittedBlock, nil
}

// BlockView construct a reader object at specific block
func (s *InMemoryStorage) BlockView(height uint64, blockID flow.Identifier) (BlockView, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if height < s.minHeightAvailable ||
		height > s.lastCommittedBlock.Height {
		return nil, &HeightNotAvailableError{height, s.minHeightAvailable, s.lastCommittedBlock.Height}
	}

	expectedID, found := s.recentlyCommittedBlocksByHeight[height]
	if !found || blockID != expectedID {
		return nil, &InvalidBlockIDError{blockID}
	}

	return &reader{
		height:  height,
		blockID: blockID,
		getFunc: s.valueAt,
	}, nil
}

// valueAt returns the value for a register at the given height
func (s *InMemoryStorage) valueAt(height uint64, blockID flow.Identifier, id flow.RegisterID) (value flow.RegisterValue, err error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	// min height might have changed since
	if height < s.minHeightAvailable {
		return nil, &HeightNotAvailableError{height, s.minHeightAvailable, s.lastCommittedBlock.Height}
	}

	// TODO(ramtin): future improvement could use a binary search when history size is large
	entries := s.registers[id]
	for _, ent := range entries {
		if ent.height > height {
			break
		}
		value = ent.value
	}
	return value, nil
}

type blockAwareGetFunc func(
	height uint64,
	blockID flow.Identifier,
	key flow.RegisterID,
) (
	flow.RegisterValue,
	error,
)

type reader struct {
	height  uint64
	blockID flow.Identifier
	getFunc blockAwareGetFunc
}

func (r *reader) Get(id flow.RegisterID) (flow.RegisterValue, error) {
	return r.getFunc(r.height, r.blockID, id)
}
