package ephemeral

import (
	"bytes"
	"sync"

	"github.com/onflow/flow-go/engine/execution/state/storehouse/storage"
	"github.com/onflow/flow-go/model/flow"
)

type valueAtHeight struct {
	height uint64
	value  flow.RegisterValue
}

// EphemeralStorage implements an non-fork-aware in-memory storage,
// keeping block register updates in memory and limiting historic look up of register to the last X commits
type EphemeralStorage struct {
	historyCapacity                 int
	registers                       map[flow.RegisterID][]valueAtHeight
	lastCommittedBlock              *flow.Header
	recentlyCommittedBlocksByHeight map[uint64]flow.Identifier
	minHeightAvailable              uint64
	lock                            sync.RWMutex
}

var _ storage.Storage = &EphemeralStorage{}

// NewStorage constructs a new EphemeralStorage
func NewStorage(
	historyCapacity int,
	genesis *flow.Header,
	data map[flow.RegisterID]flow.RegisterValue,
) *EphemeralStorage {
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
	return &EphemeralStorage{
		historyCapacity:                 historyCapacity,
		registers:                       registers,
		recentlyCommittedBlocksByHeight: recentlyCommittedBlocksByHeight,
		lastCommittedBlock:              genesis,
		minHeightAvailable:              genesis.Height,
	}
}

// CommitBlock commits block updates
func (s *EphemeralStorage) CommitBlock(header *flow.Header, update map[flow.RegisterID]flow.RegisterValue) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	// check commit sequence
	if s.lastCommittedBlock.Height+1 != header.Height || s.lastCommittedBlock.ID() != header.ParentID {
		return &storage.NonCompliantHeaderError{
			ExpectedBlockHeight:   s.lastCommittedBlock.Height + 1,
			ReceivedBlockHeight:   header.Height,
			ExpectedParentBlockID: s.lastCommittedBlock.ID(),
			ReceivedParentBlockID: header.ParentID,
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
func (s *EphemeralStorage) LastCommittedBlock() (*flow.Header, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.lastCommittedBlock, nil
}

// BlockView construct a reader object at specific block
func (s *EphemeralStorage) BlockView(height uint64, blockID flow.Identifier) (storage.BlockView, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if height < s.minHeightAvailable ||
		height > s.lastCommittedBlock.Height {
		return nil, &storage.HeightNotAvailableError{
			RequestedHeight:    height,
			MinHeightAvailable: s.minHeightAvailable,
			MaxHeightAvailable: s.lastCommittedBlock.Height,
		}
	}

	expectedID, found := s.recentlyCommittedBlocksByHeight[height]
	if !found || blockID != expectedID {
		return nil, &storage.InvalidBlockIDError{BlockID: blockID}
	}

	return &reader{
		height:  height,
		blockID: blockID,
		getFunc: s.valueAt,
	}, nil
}

// valueAt returns the value for a register at the given height
func (s *EphemeralStorage) valueAt(height uint64, blockID flow.Identifier, id flow.RegisterID) (value flow.RegisterValue, err error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	// min height might have changed since
	if height < s.minHeightAvailable {
		return nil, &storage.HeightNotAvailableError{
			RequestedHeight:    height,
			MinHeightAvailable: s.minHeightAvailable,
			MaxHeightAvailable: s.lastCommittedBlock.Height,
		}
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
