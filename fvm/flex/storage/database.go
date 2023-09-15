package storage

import (
	"errors"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"

	"github.com/onflow/atree"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/model/flow"
)

var (
	// errMemorydbNotFound is returned if a key is requested that is not found in
	// the provided memory database.
	errMemorydbNotFound = errors.New("not found")

	// errSnapshotReleased is returned if callers want to retrieve data from a
	// released snapshot.
	errSnapshotReleased = errors.New("snapshot released")

	// err not implemented
	errNotImplemented = errors.New("not implemented yet")
)

var FlexAddress = flow.BytesToAddress([]byte("Flex"))
var RootHashKey = "RootHash"

// Database is an ephemeral key-value store. Apart from basic data storage
// functionality it also supports batch writes and iterating over the keyspace in
// binary-alphabetical order.
type Database struct {
	led *ledger

	storage *atree.OrderedMap

	lock sync.RWMutex // Ramtin: do we need this?
}

// New returns a wrapped map with all the required database interface methods
// implemented.
func NewDatabase(accounts environment.Accounts) *Database {
	// TODO figure out these details
	// var typeInfo
	led := &ledger{accounts}
	led.Setup()
	baseStorage := atree.NewLedgerBaseStorage(led)

	storage := NewPersistentSlabStorage(baseStorage)
	m, err := atree.NewMap(storage, atree.Address(FlexAddress), atree.NewDefaultDigesterBuilder(), nil)
	// TODO do not panic
	if err != nil {
		panic(err)
	}

	return &Database{
		led:     led,
		storage: m,
	}
}

// Get retrieves the given key if it's present in the key-value store.
func (db *Database) Get(key []byte) ([]byte, error) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	data, err := db.storage.Get(compare, hashInputProvider, NewStringValue(string(key)))

	if err != nil {
		return nil, err
	}

	return []byte(data.(StringValue).String()), nil
}

// Put inserts the given value into the key-value store.
func (db *Database) Put(key []byte, value []byte) error {
	db.lock.Lock()
	defer db.lock.Unlock()

	_, err := db.storage.Set(compare, hashInputProvider, NewStringValue(string(key)), NewStringValue(string(value)))
	return err
}

// Delete removes the key from the key-value store.
func (db *Database) Delete(key []byte) error {
	db.lock.Lock()
	defer db.lock.Unlock()

	_, _, err := db.storage.Remove(compare, hashInputProvider, NewStringValue(string(key)))
	return err
}

// SetRootHash sets the root hash
// we have this functionality given we only allow on state to exist
func (db *Database) SetRootHash(root common.Hash) error {
	return db.led.SetValue(FlexAddress[:], []byte(RootHashKey), root[:])
}

// GetRootHash returns the latest root hash
// we have this functionality given we only allow on state to exist
func (db *Database) GetRootHash() (common.Hash, error) {
	data, err := db.led.GetValue(FlexAddress[:], []byte(RootHashKey))
	if len(data) == 0 {
		return types.EmptyRootHash, err
	}
	return common.Hash(data), err
}

// Close deallocates the internal map and ensures any consecutive data access op
// fails with an error.
func (db *Database) Close() error {
	return nil
}

// Has retrieves if a key is present in the key-value store.
func (db *Database) Has(key []byte) (bool, error) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	data, err := db.Get(key)
	return len(data) > 0, err
}

// NewBatch creates a write-only key-value store that buffers changes to its host
// database until a final write is called.
func (db *Database) NewBatch() ethdb.Batch {
	return &batch{
		db: db,
	}
}

// NewBatchWithSize creates a write-only database batch with pre-allocated buffer.
func (db *Database) NewBatchWithSize(size int) ethdb.Batch {
	return &batch{
		db: db,
	}
}

// NewIterator creates a binary-alphabetical iterator over a subset
// of database content with a particular key prefix, starting at a particular
// initial key (or after, if it does not exist).
func (db *Database) NewIterator(prefix []byte, start []byte) ethdb.Iterator {
	// TODO(ramtin): implement this if needed with iterator over atree
	panic(errNotImplemented)
}

// NewSnapshot creates a database snapshot based on the current state.
// The created snapshot will not be affected by all following mutations
// happened on the database.
func (db *Database) NewSnapshot() (ethdb.Snapshot, error) {
	panic(errNotImplemented)
}

// Stat returns a particular internal stat of the database.
func (db *Database) Stat(property string) (string, error) {
	return "", errors.New("unknown property")
}

// Compact is not supported on a memory database, but there's no need either as
// a memory database doesn't waste space anyway.
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

	return int(db.storage.Count())
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
	b.writes = append(b.writes, keyvalue{common.CopyBytes(key), common.CopyBytes(value), false})
	b.size += len(key) + len(value)
	return nil
}

// Delete inserts the a key removal into the batch for later committing.
func (b *batch) Delete(key []byte) error {
	b.writes = append(b.writes, keyvalue{common.CopyBytes(key), nil, true})
	b.size += len(key)
	return nil
}

// ValueSize retrieves the amount of data queued up for writing.
func (b *batch) ValueSize() int {
	return b.size
}

// Write flushes any accumulated data to the memory database.
func (b *batch) Write() error {
	// TODO we could optimize this by locking once and do the update using underlying put and delete method
	for _, keyvalue := range b.writes {
		if keyvalue.delete {
			b.db.Delete(keyvalue.key)
			continue
		}
		b.db.Put(keyvalue.key, keyvalue.value)
	}
	return nil
}

// Reset resets the batch for reuse.
func (b *batch) Reset() {
	b.writes = b.writes[:0]
	b.size = 0
}

// Replay replays the batch contents.
func (b *batch) Replay(w ethdb.KeyValueWriter) error {
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
