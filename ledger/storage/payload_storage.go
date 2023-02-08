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
	return &PayloadStorage{
		storage: store,
	}
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
	keys := make([]hash.Hash, len(updates))
	values := make([][]byte, len(updates))
	scratch := make([]byte, 1024*4)

	for i, update := range updates {
		key := update.Hash
		buf, err := EncodePayload(update.Path, &update.Payload, scratch)
		if err != nil {
			return fmt.Errorf("could not encode payload: %w", err)
		}

		// scratch is being reused to hold encoded data
		// we need to copy it to a new slice in order to prevent being overwritten by
		// next encoding operation
		value := make([]byte, len(buf))
		copy(value[:], buf)

		keys[i] = key
		values[i] = value
	}

	err := s.storage.SetMul(keys, values)
	if err != nil {
		return fmt.Errorf("could not store %v key-value pairs: %w", len(keys), err)
	}

	return nil
}

// TODO: replace the storage with other key-value store
func CreatePayloadStorage() *PayloadStorage {
	return NewPayloadStorage(NewInMemStorage())
}
