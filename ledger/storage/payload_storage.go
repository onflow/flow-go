package storage

import (
	"fmt"

	"github.com/cockroachdb/pebble"

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
		return ledger.DummyPath, nil, fmt.Errorf("could not get payload by hash: %w", err)
	}

	path, payload, err := DecodePayload(value)
	if err != nil {
		return ledger.DummyPath, nil, fmt.Errorf("could not decode payload from storage: %w", err)
	}

	return path, payload, nil
}

func (s *PayloadStorage) Add(updates []ledger.LeafNode) error {
	pairs := make(map[hash.Hash][]byte)
	scratch := make([]byte, 1024*4)

	for _, update := range updates {
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
		pairs[key] = value
	}

	err := s.storage.SetMul(pairs)
	if err != nil {
		return fmt.Errorf("could not store %v key-value pairs: %w", len(pairs), err)
	}

	return nil
}

// TODO: replace with flags to specify the location
func CreatePayloadStorage() *PayloadStorage {
	dirname := "/data/payloads"
	return CreatePayloadStorageWithDir(dirname)
}

func CreatePayloadStorageWithDir(dir string) *PayloadStorage {
	pebbleOptions := PebleStorageOptions{
		Options: &pebble.Options{},
		Dirname: dir,
	}

	store, err := NewPebbleStorage(pebbleOptions)
	if err != nil {
		panic(fmt.Sprintf("fail to initialize pebble store: %v", err))
	}
	return NewPayloadStorage(store)
}
