package storage

import (
	"errors"
	"io"
	"sync"

	"github.com/hashicorp/go-multierror"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"

	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/vfs"
)

type PebleStorageOptions struct {
	*pebble.Options
	dirname string
}

func TestPebleStorageOptions() PebleStorageOptions {
	return PebleStorageOptions{
		Options: &pebble.Options{
			FS: vfs.NewMem(),
		},
		dirname: "",
	}
}

var _ ledger.Storage = (*pebleStorage)(nil)
var _ io.Closer = (*pebleStorage)(nil)

type pebleStorage struct {
	db *pebble.DB

	// rw mutex is needed because pebble doesnt ahve transactions
	// as we are saving new values we shouldn't read any values
	// TODO: check if we can do something with batches.
	rw sync.RWMutex
}

// NewPebbleStorage creates a new pebble storage with `pebble.Options`
func NewPebbleStorage(
	storageOptions PebleStorageOptions,
) (ledger.Storage, error) {
	db, err := pebble.Open(storageOptions.dirname, storageOptions.Options)

	if err != nil {
		return nil, err
	}

	return &pebleStorage{
		db: db,
	}, nil
}

// Get implements ledger.Storage
func (p *pebleStorage) Get(h hash.Hash) (value []byte, err error) {
	p.rw.RLock()
	defer p.rw.RUnlock()

	return p.getUnsafe(h)
}

func (p *pebleStorage) getUnsafe(h hash.Hash) (value []byte, err error) {
	var v []byte
	var c io.Closer
	v, c, err = p.db.Get(h[:])

	if errors.Is(err, pebble.ErrNotFound) {
		return nil, ledger.ErrStorageMissingKeys{
			Keys: []hash.Hash{h},
		}
	}
	if err != nil {
		return nil, err
	}

	defer func() {
		cerr := c.Close()
		if cerr == nil {
			return
		}
		if err == nil {
			err = cerr
			return
		}
		err = multierror.Append(err, cerr).ErrorOrNil()
	}()

	// `v` should not be used after `c.Close()` so we need to copy it
	// TODO: make sure we are not unnecesarily copying it again up the stack
	value = make([]byte, len(v))
	copy(value, v)
	return value, nil
}

// GetMul implements ledger.Storage
func (p *pebleStorage) GetMul(hs []hash.Hash) ([][]byte, error) {
	p.rw.RLock()
	defer p.rw.RUnlock()

	// TODO: optimise
	// maybe its worth doing it in parralel
	// pebble doesnt have a batch get
	values := make([][]byte, 0, len(hs))
	err := ledger.ErrStorageMissingKeys{
		Keys: make([]hash.Hash, 0, len(hs)),
	}

	for _, h := range hs {
		v, gerr := p.getUnsafe(h)
		if gerr != nil {
			if errors.Is(gerr, ledger.ErrStorageMissingKeys{}) {
				err.Keys = append(err.Keys, h)
				continue
			}

			return nil, gerr
		}
		if len(err.Keys) > 0 {
			// since we are going to only return errors anyway
			continue
		}
		values = append(values, v)
	}

	if len(err.Keys) > 0 {
		return nil, err
	}

	return values, nil
}

// SetMul implements ledger.Storage
func (p *pebleStorage) SetMul(pairs map[hash.Hash][]byte) error {
	b := p.db.NewBatch()
	for key, value := range pairs {
		err := b.Set(key[:], value, nil)
		if err != nil {
			return err
		}
	}

	p.rw.Lock()
	defer p.rw.Unlock()

	err := b.Commit(pebble.Sync)

	return err
}

// Close implements io.Closer
func (p *pebleStorage) Close() error {
	return p.db.Close()
}
