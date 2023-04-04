package storage

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/engine/execution/state/cargo"
	"github.com/onflow/flow-go/model/flow"
)

type InMemStorage struct {
	historyCap int
	data       map[flow.RegisterID]entries
	headerMeta *headerMeta
	lock       sync.RWMutex
}

var _ cargo.Storage = &InMemStorage{}

func NewInMemStorage(
	historyCap int,
	genesis *flow.Header,
) *InMemStorage {
	return &InMemStorage{
		historyCap: historyCap,
		data:       make(map[flow.RegisterID]entries, 0),
		headerMeta: &headerMeta{genesis.Height, genesis.ID()},
	}
}

func (s *InMemStorage) Commit(header *flow.Header, update map[flow.RegisterID]flow.RegisterValue) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	h := header.Height
	// check commit sequence
	if s.headerMeta.height != h+1 {
		return fmt.Errorf("commit height mismatch [%d] != [%d]", s.headerMeta.height, h+1)
	}
	if s.headerMeta.id != header.ParentID {
		return fmt.Errorf("commit parent id mismatch [%x] != [%x]", s.headerMeta.id, header.ParentID)
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
	s.headerMeta = &headerMeta{h, header.ID()}
	return nil
}

func (s *InMemStorage) RegisterAt(height uint64, id flow.RegisterID) (value flow.RegisterValue, err error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	var minHeightAllowed uint64
	if s.headerMeta.height >= uint64(s.historyCap) {
		minHeightAllowed = s.headerMeta.height - uint64(s.historyCap) + 1
	}
	if height > minHeightAllowed {
		return nil, fmt.Errorf("height out side of historic range allowed %d > %d", height, minHeightAllowed)
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

func (s *InMemStorage) LastCommittedBlockHeight() (uint64, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.headerMeta.height, nil
}

func (s *InMemStorage) LastCommittedBlockID() (flow.Identifier, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.headerMeta.id, nil
}

type headerMeta struct {
	height uint64
	id     flow.Identifier
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
