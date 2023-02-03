package storage

import (
	"fmt"

	"github.com/onflow/flow-go/ledger/common/hash"
)

type InMemStorage struct {
	store map[hash.Hash][]byte
}

func NewInMemStorage() *InMemStorage {
	return &InMemStorage{
		store: make(map[hash.Hash][]byte),
	}
}

func (s *InMemStorage) Get(key hash.Hash) ([]byte, error) {
	value, ok := s.store[key]
	if !ok {
		return nil, fmt.Errorf("path not found")
	}

	return value, nil
}

func (s *InMemStorage) SetMul(keys []hash.Hash, values [][]byte) error {
	for i, key := range keys {
		value := values[i]
		s.store[key] = value
	}

	return nil
}
