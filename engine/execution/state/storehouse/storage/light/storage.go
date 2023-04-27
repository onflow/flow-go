package light

import (
	"errors"
	"sync"

	"github.com/cockroachdb/pebble"
	"github.com/onflow/flow-go/engine/execution/state/storehouse/storage"
	"github.com/onflow/flow-go/model/flow"
)

var headerKey = []byte("header")
var registerPrefix = []byte("r")

// LightStorage implements a light weight persistent storage (no historic lookup, no support for forks),
type LightStorage struct {
	db                 *pebble.DB
	lastCommittedBlock *flow.Header // cached value
	lock               sync.RWMutex
}

var _ storage.Storage = &LightStorage{}

// NewStorage constructs a new LightStorage
// genesis block is going to be populated if the last commited block is empty
func NewStorage(
	db *pebble.DB,
	genesis *flow.Header,
) (*LightStorage, error) {
	s := &LightStorage{
		db: db,
	}
	h, err := s.getLastCommittedBlock()
	if err != nil {
		return nil, err
	}
	if h == nil {
		h = genesis
	}
	s.lastCommittedBlock = h
	return s, nil
}

func (s *LightStorage) getLastCommittedBlock() (*flow.Header, error) {
	data, closer, err := s.db.Get(headerKey)
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}

	var header *flow.Header
	if err := header.UnmarshalCBOR(data); err != nil {
		return nil, err
	}

	if err := closer.Close(); err != nil {
		return nil, err
	}
	return header, nil
}

// CommitBlock commits block updates
func (s *LightStorage) CommitBlock(header *flow.Header, update map[flow.RegisterID]flow.RegisterValue) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	h, err := header.MarshalCBOR()
	if err != nil {
		return err
	}

	batch := s.db.NewBatch()
	for k, v := range update {
		err = batch.Set(append(registerPrefix, k.Bytes()...), v, pebble.Sync)
		if err != nil {
			return err
		}
	}
	err = batch.Set(headerKey, h, pebble.Sync)
	if err != nil {
		return err
	}

	err = batch.Commit(pebble.Sync)
	if err != nil {
		return err
	}

	s.lastCommittedBlock = header
	return nil
}

// LastCommittedBlock returns the last commited block
func (s *LightStorage) LastCommittedBlock() (*flow.Header, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.lastCommittedBlock, nil
}

// BlockView construct a reader object at specific block
func (s *LightStorage) BlockView(height uint64, blockID flow.Identifier) (storage.BlockView, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if height != s.lastCommittedBlock.Height {
		return nil, &storage.HeightNotAvailableError{
			RequestedHeight:    height,
			MinHeightAvailable: s.lastCommittedBlock.Height,
			MaxHeightAvailable: s.lastCommittedBlock.Height,
		}
	}

	if blockID != s.lastCommittedBlock.ID() {
		return nil, &storage.InvalidBlockIDError{BlockID: blockID}
	}

	return &view{
		height:  height,
		getFunc: s.valueAt,
	}, nil
}

// valueAt returns the value for a register at the given height
func (s *LightStorage) valueAt(height uint64, id flow.RegisterID) (value flow.RegisterValue, err error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	// min height might have changed since
	if height != s.lastCommittedBlock.Height {
		return nil, &storage.HeightNotAvailableError{
			RequestedHeight:    height,
			MinHeightAvailable: s.lastCommittedBlock.Height,
			MaxHeightAvailable: s.lastCommittedBlock.Height,
		}
	}

	value, closer, err := s.db.Get(append(registerPrefix, id.Bytes()...))
	if err != nil {
		// if not found closer.Close is not needed
		if errors.Is(err, pebble.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}

	if err := closer.Close(); err != nil {
		return nil, err
	}

	return value, nil
}

type view struct {
	height  uint64
	getFunc func(height uint64, key flow.RegisterID) (flow.RegisterValue, error)
}

func (v *view) Get(id flow.RegisterID) (flow.RegisterValue, error) {
	return v.getFunc(v.height, id)
}
