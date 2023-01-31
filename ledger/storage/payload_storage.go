package storage

import (
	"fmt"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
)

var _ ledger.PayloadStorage = (*PayloadStorage)(nil)

type PayloadStorage struct {
	storage ledger.Storage
}

func NewPayloadStorage(store ledger.Storage) *PayloadStorage {
	return &PayloadStorage{store}
}

func (s *PayloadStorage) Get(hash hash.Hash) (ledger.Path, *ledger.Payload, error) {
	value, err := s.storage.Get(hash)
	if err != nil {
		return ledger.DummyPath, nil, fmt.Errorf("could not get by hash: %w", err)
	}

	path, payload, err := DecodePayload(value)
	if err != nil {
		return ledger.DummyPath, nil, fmt.Errorf("could not decode payload from storage: %w", err)
	}

	return path, payload, nil
}

func (s *PayloadStorage) Add(updates []ledger.LeafNode) error {
	keys := make([]hash.Hash, 0, len(updates))
	values := make([][]byte, 0, len(updates))
	scratch := make([]byte, 1024*4)

	for _, update := range updates {
		key := update.Hash
		value, err := EncodePayload(update.Path, &update.Payload, scratch)
		if err != nil {
			return fmt.Errorf("could not encode payload: %w", err)
		}

		keys = append(keys, key)
		values = append(values, value)
	}

	err := s.storage.SetMul(keys, values)
	if err != nil {
		return fmt.Errorf("could not store %v key-value pairs: %w", len(keys), err)
	}

	return nil
}
