package unittest

import (
	"sync"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/storage"
)

func CreateMockPayloadStore() *storage.PayloadStorage {
	store := CreateMockStore()
	return storage.NewPayloadStorage(store)
}

func CreateMockStore() *PayloadStore {
	return &PayloadStore{
		stored: make(map[hash.Hash][]byte),
	}
}

// a mock key-value storage
type PayloadStore struct {
	sync.RWMutex
	stored map[hash.Hash][]byte
}

func (s *PayloadStore) Get(key hash.Hash) ([]byte, error) {
	s.RLock()
	defer s.RUnlock()
	node, found := s.stored[key]
	if !found {
		return nil, ledger.ErrStorageMissingKeys{
			Keys: []hash.Hash{key},
		}
	}

	// return the copied data
	buf := make([]byte, len(node))
	copy(buf[:], node)
	return buf, nil
}

func (s *PayloadStore) GetMul(hashs []hash.Hash) ([][]byte, error) {
	s.RLock()
	defer s.RUnlock()

	missingHashs := make([]hash.Hash, 0, len(hashs))
	values := make([][]byte, len(hashs))
	for i, hash := range hashs {
		node, found := s.stored[hash]
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

func (s *PayloadStore) SetMul(pairs map[hash.Hash][]byte) error {
	s.Lock()
	defer s.Unlock()

	for key, value := range pairs {
		s.stored[key] = value
	}

	return nil
}

func (s *PayloadStore) Count() int {
	s.RLock()
	defer s.RUnlock()
	return len(s.stored)
}
