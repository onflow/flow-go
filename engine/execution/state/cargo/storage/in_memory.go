package storage

import (
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
	historyCapacity    int
	registers          map[flow.RegisterID][]valueAtHeight
	headers            map[flow.Identifier]*flow.Header
	lastCommittedBlock *flow.Header
	minHeightAvailable uint64
	lock               sync.RWMutex
}

var _ Storage = &InMemoryStorage{}

// NewInMemoryStorage constructs a new InMemoryStorage
func NewInMemoryStorage(
	historyCapacity int,
	genesis *flow.Header,
	data map[flow.RegisterID]flow.RegisterValue,
) *InMemoryStorage {
	height := genesis.Height
	registers := make(map[flow.RegisterID][]valueAtHeight, 0)
	for id, val := range data {
		registers[id] = []valueAtHeight{{
			height: height,
			value:  val,
		}}
	}

	headers := make(map[flow.Identifier]*flow.Header)
	headers[genesis.ID()] = genesis

	return &InMemoryStorage{
		historyCapacity:    historyCapacity,
		registers:          registers,
		headers:            headers,
		lastCommittedBlock: genesis,
		minHeightAvailable: height,
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
		// prune if at capacity (prune first prevents potentially extra allocation)
		if len(vv) == s.historyCapacity {
			vv = vv[1:]
		}
		s.registers[key] = append(vv, valueAtHeight{
			height: header.Height,
			value:  value,
		})
	}
	s.lastCommittedBlock = header
	s.headers[header.ID()] = header

	s.minHeightAvailable = s.lastCommittedBlock.Height - uint64(s.historyCapacity) + 1
	if s.lastCommittedBlock.Height < 0 {
		s.lastCommittedBlock.Height = 0
	}
	return nil
}

// LastCommittedBlock returns the last commited block
func (s *InMemoryStorage) LastCommittedBlock() (*flow.Header, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.lastCommittedBlock, nil
}

// RegisterValueAt returns the value for a register at the given height
func (s *InMemoryStorage) RegisterValueAt(height uint64, blockID flow.Identifier, id flow.RegisterID) (value flow.RegisterValue, err error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if height < s.minHeightAvailable ||
		height > s.lastCommittedBlock.Height {
		return nil, &HeightNotAvailableError{height, s.minHeightAvailable, s.lastCommittedBlock.Height}
	}

	if _, found := s.headers[blockID]; !found {
		return nil, &InvalidBlockIDError{blockID}
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
