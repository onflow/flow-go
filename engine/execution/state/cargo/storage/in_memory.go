package storage

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/model/flow"
)

type InMemoryStorage struct {
	historyCap           int
	data                 map[flow.RegisterID]entries
	lastCommittedBlock   *flow.Header
	lastCommittedBlockID flow.Identifier
	minHeightAvailable   uint64
	lock                 sync.RWMutex
}

var _ Storage = &InMemoryStorage{}

func NewInMemoryStorage(
	historyCap int,
	genesis *flow.Header,
) *InMemoryStorage {
	return &InMemoryStorage{
		historyCap:           historyCap,
		data:                 make(map[flow.RegisterID]entries, 0),
		lastCommittedBlock:   genesis,
		lastCommittedBlockID: genesis.ID(),
	}
}

func (s *InMemoryStorage) CommitBlock(header *flow.Header, update map[flow.RegisterID]flow.RegisterValue) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	h := header.Height
	// check commit sequence
	if s.lastCommittedBlock.Height+1 != h {
		return fmt.Errorf("commit height mismatch [%d] != [%d]", s.lastCommittedBlock.Height+1, h)
	}
	if s.lastCommittedBlockID != header.ParentID {
		return fmt.Errorf("commit parent id mismatch [%x] != [%x]", s.lastCommittedBlockID, header.ParentID)
	}

	for key, value := range update {
		ent := entry{
			height: h,
			value:  value,
		}
		es, found := s.data[key]
		if !found {
			s.data[key] = newEntries(ent, s.historyCap)
			continue
		}
		es.insertAndPrune(ent, s.historyCap)
	}
	s.lastCommittedBlock = header
	s.lastCommittedBlockID = header.ID()

	if s.lastCommittedBlock.Height >= uint64(s.historyCap) {
		s.minHeightAvailable = s.lastCommittedBlock.Height - uint64(s.historyCap) + 1
	}
	return nil
}

func (s *InMemoryStorage) LastCommittedBlock() (*flow.Header, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.lastCommittedBlock, nil
}

func (s *InMemoryStorage) RegisterValueAt(height uint64, id flow.RegisterID) (value flow.RegisterValue, err error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if height > s.lastCommittedBlock.Height {
		return nil, fmt.Errorf("height out side of historic range allowed %d > %d", height, s.lastCommittedBlock.Height)
	}
	if height < s.minHeightAvailable {
		return nil, fmt.Errorf("height out side of historic range allowed %d < %d", height, s.minHeightAvailable)
	}

	entries := s.data[id]
	for _, ent := range entries {
		if ent.height > height {
			break
		}
		value = ent.value
	}
	return value, nil
}

type entry struct {
	height uint64
	value  flow.RegisterValue
}

type entries []entry

func newEntries(et entry, historyCap int) entries {
	es := make(entries, historyCap+1)
	es[0] = et
	return es
}

func (es entries) insertAndPrune(entry entry, historyCap int) {
	es = append(es, entry)
	if len(es) > historyCap {
		es = es[1:]
	}
}
