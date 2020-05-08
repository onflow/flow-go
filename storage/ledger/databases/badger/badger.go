package badger

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/storage/ledger/databases"
	"github.com/dapperlabs/flow-go/utils/io"
)

var _ databases.DAL = &BadgerDB{}

const (
	kvdbPrefix = "kvdb"
	triePrefix = "trie"
)

// BadgerDB is a Badger implementation of the DAL interface.
type BadgerDB struct {
	path string
	kvdb *badger.DB
	tdb  *badger.DB
}

type Batcher struct {
	keys   [][]byte
	values [][]byte
}

// NewBadgerDB creates a new Badger database.
func NewBadgerDB(path string) (*BadgerDB, error) {
	fmt.Println("OPENING DB:", path)

	kvdbPath := filepath.Join(path, kvdbPrefix)
	tdbPath := filepath.Join(path, triePrefix)

	// open the key-value database
	kvdb, err := open(kvdbPath)
	if err != nil {
		return nil, err
	}

	// open the trie database
	tdb, err := open(tdbPath)
	if err != nil {
		return nil, err
	}

	return &BadgerDB{
		path: path,
		kvdb: kvdb,
		tdb:  tdb,
	}, nil
}

// GetKVDB gets a value from the key-value database.
func (s *BadgerDB) GetKVDB(key []byte) ([]byte, error) {
	return get(s.kvdb, key)
}

// GetTrieDB gets a value from the trie database.
func (s *BadgerDB) GetTrieDB(key []byte) ([]byte, error) {
	return get(s.tdb, key)
}
func (b *Batcher) Put(key, value []byte) {
	b.keys = append(b.keys, key)
	b.values = append(b.values, value)
}

// NewBatcher returns a new batcher.
func (s *BadgerDB) NewBatcher() databases.Batcher {
	return &Batcher{
		keys:   make([][]byte, 0),
		values: make([][]byte, 0),
	}
}

// SafeClose is a helper function that closes databases safely.
func (s *BadgerDB) SafeClose() (error, error) {
	fmt.Println("SAFE CLOSING DB:", s.path)
	return s.kvdb.Close(), s.tdb.Close()
}

// UpdateKVDB updates the provided key-value pairs in the key-value database.
func (s *BadgerDB) UpdateKVDB(keys [][]byte, values [][]byte) error {
	return writeBatch(s.kvdb, keys, values)
}

// UpdateTrieDB updates the trie DB with the new paths in the update
func (s *BadgerDB) UpdateTrieDB(batcher databases.Batcher) error {
	b := batcher.(*Batcher)

	return writeBatch(s.kvdb, b.keys, b.values)
}

func (s *BadgerDB) CopyTo(to databases.DAL) error {
	other := to.(*BadgerDB)

	err := streamCopy(s.kvdb, other.kvdb)
	if err != nil {
		fmt.Errorf("error while copying KVDB: %w", err)
	}

	err = streamCopy(s.tdb, other.tdb)
	if err != nil {
		fmt.Errorf("error while copying KVDB: %w", err)
	}

	return nil
}

// PruneDB takes the current BadgerDB instance and removes any key-value pairs that also appear in the next BadgerDB instance.
func (s *BadgerDB) PruneDB(next databases.DAL) error {
	return nil
}

// Size returns the size of the current BadgerDB on disk in bytes
func (s *BadgerDB) Size() (int64, error) {
	return io.DirSize(s.path)
}

func open(path string) (*badger.DB, error) {
	err := os.MkdirAll(path, 0700)
	if err != nil {
		return nil, err
	}

	return badger.Open(badger.DefaultOptions(path))
}

func get(db *badger.DB, key []byte) ([]byte, error) {
	var value []byte

	err := db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}

		value, err = item.ValueCopy(nil)
		return err
	})

	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil, databases.ErrNotFound
		}

		return nil, err
	}

	return value, nil
}

func writeBatch(db *badger.DB, keys [][]byte, values [][]byte) error {
	wb := db.NewWriteBatch()
	defer wb.Cancel()

	for i := 0; i < len(keys); i++ {
		err := wb.Set(keys[i], values[i])
		if err != nil {
			return nil
		}
	}

	return wb.Flush()
}

func streamCopy(dest, orig *badger.DB) error {
	sw := dest.NewStreamWriter()
	if err := sw.Prepare(); err != nil {
		return fmt.Errorf("could not prepare stream writer: %w", err)
	}

	s := orig.NewStream()
	s.Send = sw.Write

	if err := s.Orchestrate(context.Background()); err != nil {
		return fmt.Errorf("could not orchestrate writes: %w", err)
	}

	if err := sw.Flush(); err != nil {
		return fmt.Errorf("could not flush stream writer: %w", err)
	}

	return nil
}
