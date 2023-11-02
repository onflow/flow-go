package database

import (
	stdErrors "errors"
	"sync"

	gethCommon "github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	gethDB "github.com/ethereum/go-ethereum/ethdb"

	"github.com/onflow/atree"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

const (
	FlowEVMRootSlabKey = "RootSlabKey"
	FlowEVMRootHashKey = "RootHash"
	StorageIDSize      = 16
)

// Database is where EVM data is stored.
//
// under the hood, databases uses an Atree map
// stored under account `flowEVMRootAddress`
// each key value pairs inserted into this map is
// of type of ByteStringValue; we use this type instead
// of atree array, given the EVM environment is not smart enough
// to interact with a portion of the value and would load everything under a key
// before opearting on it. This means it could lead to having large slabs for a single value.
type Database struct {
	flowEVMRootAddress flow.Address
	led                atree.Ledger
	storage            *atree.PersistentSlabStorage
	atreemap           *atree.OrderedMap
	// Ramtin: other database implementations for EVM uses a lock
	// to protect the storage against concurrent operations
	// though one might do more research to see if we need
	// these type of locking if the underlying structure (atree)
	// has such protections or if EVM really needs it
	lock sync.RWMutex
}

var _ types.Database = &Database{}

// NewDatabase returns a wrapped map that implements all the required database interface methods.
func NewDatabase(led atree.Ledger, flowEVMRootAddress flow.Address) (*Database, error) {
	baseStorage := atree.NewLedgerBaseStorage(led)

	storage, err := NewPersistentSlabStorage(baseStorage)
	if err != nil {
		return nil, handleError(err)
	}

	db := &Database{
		led:                led,
		flowEVMRootAddress: flowEVMRootAddress,
		storage:            storage,
	}

	err = db.retrieveOrCreateMapRoot()
	if err != nil {
		return nil, err
	}
	return db, nil
}

func (db *Database) retrieveOrCreateMapRoot() error {
	rootIDBytes, err := db.led.GetValue(db.flowEVMRootAddress.Bytes(), []byte(FlowEVMRootSlabKey))
	if err != nil {
		return handleError(err)
	}

	var m *atree.OrderedMap
	if len(rootIDBytes) == 0 {
		m, err = atree.NewMap(db.storage, atree.Address(db.flowEVMRootAddress), atree.NewDefaultDigesterBuilder(), emptyTypeInfo{})
		if err != nil {
			return handleError(err)
		}
		rootIDBytes := make([]byte, StorageIDSize)
		_, err := m.StorageID().ToRawBytes(rootIDBytes)
		if err != nil {
			return handleError(err)
		}
		err = db.led.SetValue(db.flowEVMRootAddress.Bytes(), []byte(FlowEVMRootSlabKey), rootIDBytes[:])
		if err != nil {
			return handleError(err)
		}
	} else {
		storageID, err := atree.NewStorageIDFromRawBytes(rootIDBytes)
		if err != nil {
			return handleError(err)
		}
		m, err = atree.NewMapWithRootID(db.storage, storageID, atree.NewDefaultDigesterBuilder())
		if err != nil {
			return handleError(err)
		}
	}
	db.atreemap = m
	return nil
}

// Get retrieves the given key if it's present in the key-value store.
func (db *Database) Get(key []byte) ([]byte, error) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	return db.get(key)
}

func (db *Database) get(key []byte) ([]byte, error) {
	data, err := db.atreemap.Get(compare, hashInputProvider, NewByteStringValue(key))
	if err != nil {
		return nil, handleError(err)
	}

	v, err := data.StoredValue(db.atreemap.Storage)
	if err != nil {
		return nil, handleError(err)
	}

	return v.(ByteStringValue).Bytes(), nil
}

// Put inserts the given value into the key-value store.
func (db *Database) Put(key []byte, value []byte) error {
	db.lock.Lock()
	defer db.lock.Unlock()
	return db.put(key, value)
}

func (db *Database) put(key []byte, value []byte) error {
	existingValueStorable, err := db.atreemap.Set(compare, hashInputProvider, NewByteStringValue(key), NewByteStringValue(value))
	if err != nil {
		return handleError(err)
	}

	if id, ok := existingValueStorable.(atree.StorageIDStorable); ok {
		// NOTE: deep remove isn't necessary because value is ByteStringValue (not container)
		err := db.storage.Remove(atree.StorageID(id))
		if err != nil {
			return handleError(err)
		}
	}

	return nil
}

// Has checks if a key is present in the key-value store.
func (db *Database) Has(key []byte) (bool, error) {
	db.lock.RLock()
	defer db.lock.RUnlock()
	return db.has(key)
}

func (db *Database) has(key []byte) (bool, error) {
	has, err := db.atreemap.Has(compare, hashInputProvider, NewByteStringValue(key))
	if err != nil {
		return false, handleError(err)
	}
	return has, nil
}

// Delete removes the key from the key-value store.
func (db *Database) Delete(key []byte) error {
	db.lock.Lock()
	defer db.lock.Unlock()
	return db.delete(key)
}

func (db *Database) delete(key []byte) error {
	removedMapKeyStorable, removedMapValueStorable, err := db.atreemap.Remove(compare, hashInputProvider, NewByteStringValue(key))
	if err != nil {
		return handleError(err)
	}

	if id, ok := removedMapKeyStorable.(atree.StorageIDStorable); ok {
		// NOTE: deep remove isn't necessary because key is ByteStringValue (not container)
		err := db.storage.Remove(atree.StorageID(id))
		if err != nil {
			return handleError(err)
		}
	}

	if id, ok := removedMapValueStorable.(atree.StorageIDStorable); ok {
		// NOTE: deep remove isn't necessary because value is ByteStringValue (not container)
		err := db.storage.Remove(atree.StorageID(id))
		if err != nil {
			return handleError(err)
		}
	}
	return nil
}

// ApplyBatch applys changes from a batch into the database
func (db *Database) ApplyBatch(b *batch) error {
	db.lock.Lock()
	defer db.lock.Unlock()
	return db.applyBatch(b)
}

func (db *Database) applyBatch(b *batch) error {
	var err error
	for _, keyvalue := range b.writes {
		if err != nil {
			return err
		}
		if keyvalue.delete {
			err = db.delete(keyvalue.key)
			continue
		}
		err = db.put(keyvalue.key, keyvalue.value)
	}
	return err
}

// SetRootHash sets the active root hash to a new one
func (db *Database) SetRootHash(root gethCommon.Hash) error {
	db.lock.Lock()
	defer db.lock.Unlock()

	err := db.led.SetValue(db.flowEVMRootAddress[:], []byte(FlowEVMRootHashKey), root[:])
	if err != nil {
		return handleError(err)
	}
	return nil
}

// GetRootHash returns the active root hash
func (db *Database) GetRootHash() (gethCommon.Hash, error) {
	db.lock.Lock()
	defer db.lock.Unlock()

	data, err := db.led.GetValue(db.flowEVMRootAddress[:], []byte(FlowEVMRootHashKey))
	if err != nil {
		return gethCommon.Hash{}, handleError(err)
	}
	if len(data) == 0 {
		return gethTypes.EmptyRootHash, nil
	}
	return gethCommon.Hash(data), nil
}

// Commits the changes from atree into the underlying storage
//
// this method can be merged as part of batcher
func (db *Database) Commit() error {
	err := db.storage.Commit()
	if err != nil {
		return types.NewFatalError(err)
	}
	return nil
}

// Close commits db changes
func (db *Database) Close() error {
	return db.Commit()
}

// NewBatch creates a write-only key-value store that buffers changes to its host
// database until a final write is called.
func (db *Database) NewBatch() gethDB.Batch {
	return &batch{
		db: db,
	}
}

// NewBatchWithSize creates a write-only database batch with pre-allocated buffer.
func (db *Database) NewBatchWithSize(size int) gethDB.Batch {
	return &batch{
		db:     db,
		writes: make([]keyvalue, 0, size),
	}
}

// NewIterator is not supported in this database
// if needed in the future we could implement it using atree iterators
func (db *Database) NewIterator(prefix []byte, start []byte) gethDB.Iterator {
	panic(types.ErrNotImplemented)
}

// NewSnapshot is not supported
func (db *Database) NewSnapshot() (gethDB.Snapshot, error) {
	return nil, types.ErrNotImplemented
}

// Stat method is not supported
func (db *Database) Stat(property string) (string, error) {
	return "", types.ErrNotImplemented
}

// Compact is a no op
func (db *Database) Compact(start []byte, limit []byte) error {
	return nil
}

// Len returns the number of entries currently present in the memory database.
//
// Note, this method is only used for testing (i.e. not public in general) and
// does not have explicit checks for closed-ness to allow simpler testing code.
func (db *Database) Len() int {
	db.lock.RLock()
	defer db.lock.RUnlock()

	return db.storage.Count()
}

// keyvalue is a key-value tuple tagged with a deletion field to allow creating
// memory-database write batches.
type keyvalue struct {
	key    []byte
	value  []byte
	delete bool
}

// batch is a write-only memory batch that commits changes to its host
// database when Write is called. A batch cannot be used concurrently.
type batch struct {
	db     *Database
	writes []keyvalue
	size   int
}

// Put inserts the given value into the batch for later committing.
func (b *batch) Put(key, value []byte) error {
	return b.set(key, value, false)
}

// Delete inserts the a key removal into the batch for later committing.
func (b *batch) Delete(key []byte) error {
	return b.set(key, nil, true)
}

func (b *batch) set(key []byte, value []byte, delete bool) error {
	b.writes = append(b.writes, keyvalue{gethCommon.CopyBytes(key), gethCommon.CopyBytes(value), delete})
	b.size += len(key) + len(value)
	return nil
}

// ValueSize retrieves the amount of data queued up for writing.
func (b *batch) ValueSize() int {
	return b.size
}

// Write flushes any accumulated data to the memory database.
func (b *batch) Write() error {
	return b.db.ApplyBatch(b)
}

// Reset resets the batch for reuse.
func (b *batch) Reset() {
	b.writes = b.writes[:0]
	b.size = 0
}

// Replay replays the batch contents.
func (b *batch) Replay(w gethDB.KeyValueWriter) error {
	for _, keyvalue := range b.writes {
		if keyvalue.delete {
			if err := w.Delete(keyvalue.key); err != nil {
				return err
			}
			continue
		}
		if err := w.Put(keyvalue.key, keyvalue.value); err != nil {
			return err
		}
	}
	return nil
}

func handleError(err error) error {
	if err == nil {
		return nil
	}
	var atreeFatalError *atree.FatalError
	if stdErrors.As(err, &atreeFatalError) || errors.IsFailure(err) {
		return types.NewFatalError(err)
	}
	// the rest is user error
	return err
}
