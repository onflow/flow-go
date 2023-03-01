package storage

import (
	"sync"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
)

type InMemStorage struct {
	store map[hash.Hash][]byte
	rwMU  sync.RWMutex
}

func NewInMemStorage() *InMemStorage {
	return &InMemStorage{
		store: make(map[hash.Hash][]byte),
	}
}

func (s *InMemStorage) Get(key hash.Hash) ([]byte, error) {
	s.rwMU.RLock()
	defer s.rwMU.RUnlock()

	value, ok := s.store[key]
	if !ok {
		return nil, ledger.ErrStorageMissingKeys{
			Keys: []hash.Hash{key},
		}
	}

	return value, nil
}

func (s *InMemStorage) GetMul(hashs []hash.Hash) ([][]byte, error) {
	s.rwMU.RLock()
	defer s.rwMU.RUnlock()

	missingHashs := make([]hash.Hash, 0, len(hashs))
	values := make([][]byte, len(hashs))
	for i, hash := range hashs {
		node, found := s.store[hash]
		if !found {
			missingHashs = append(missingHashs, hash)
			continue
		}

		values[i] = node
	}

	if len(missingHashs) > 0 {
		return nil, ledger.ErrStorageMissingKeys{
			Keys: missingHashs,
		}
	}
	return values, nil
}

func (s *InMemStorage) SetMul(pairs map[hash.Hash][]byte) error {
	s.rwMU.Lock()
	defer s.rwMU.Unlock()

	for key, value := range pairs {
		s.store[key] = value
	}

	return nil
}
