package leveldb

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/syndtr/goleveldb/leveldb"

	"github.com/dapperlabs/flow-go/storage/ledger/databases"
	"github.com/dapperlabs/flow-go/utils/io"
)

var _ databases.DAL = &LevelDB{}

const (
	kvdbSuffix = "kvdb"
	trieSuffix = "trie"
)

// LevelDB is a LevelDB implementation of the DAL interface.
type LevelDB struct {
	path string
	kvdb *leveldb.DB // The key-value database
	tdb  *leveldb.DB // The trie database
}

// NewLevelDB creates a new LevelDB database.
func NewLevelDB(path string) (*LevelDB, error) {
	kvdbPath := filepath.Join(path, kvdbSuffix)
	tdbPath := filepath.Join(path, trieSuffix)

	// open the key-value database
	kvdb, err := leveldb.OpenFile(kvdbPath, nil)
	if err != nil {
		return nil, err
	}

	// open the trie database
	tdb, err := leveldb.OpenFile(tdbPath, nil)
	if err != nil {
		return nil, err
	}

	return &LevelDB{
		path: path,
		kvdb: kvdb,
		tdb:  tdb,
	}, nil
}

// GetKVDB calls a get on the key-value database.
func (s *LevelDB) GetKVDB(key []byte) ([]byte, error) {
	value, err := s.kvdb.Get(key, nil)
	if errors.Is(err, leveldb.ErrNotFound) {
		return value, databases.ErrNotFound
	}

	return value, err
}

// GetTrieDB calls a get on the trie database.
func (s *LevelDB) GetTrieDB(key []byte) ([]byte, error) {
	return s.tdb.Get(key, nil)
}

// NewBatcher returns a new batcher.
func (s *LevelDB) NewBatcher() databases.Batcher {
	return new(leveldb.Batch)
}

// SafeClose is a helper function that closes databases safely.
func (s *LevelDB) SafeClose() (error, error) {
	return s.kvdb.Close(), s.tdb.Close()
}

// UpdateKVDB updates the provided key-value pairs in the key-value database.
func (s *LevelDB) UpdateKVDB(keys [][]byte, values [][]byte) error {

	kvBatcher := new(leveldb.Batch)
	for i := 0; i < len(keys); i++ {
		kvBatcher.Put(keys[i], values[i])
	}

	err := s.kvdb.Write(kvBatcher, nil)
	if err != nil {
		return err
	}

	return nil
}

// UpdateTrieDB updates the trie database with the new paths in the update.
func (s *LevelDB) UpdateTrieDB(batcher databases.Batcher) error {
	levelDBBatcher, ok := batcher.(*leveldb.Batch)
	if !ok {
		return fmt.Errorf("supplied batcher is not of type leveldb.Batch - make sure you supply batcher produced by NewBatcher")
	}

	err := s.tdb.Write(levelDBBatcher, nil)
	if err != nil {
		return err
	}

	return nil
}

func (s *LevelDB) CopyTo(dest databases.DAL) error {
	destDB, ok := dest.(*LevelDB)
	if !ok {
		return fmt.Errorf("copy supported between the same databases only - make sure you supply instance of LevelDB here")
	}

	err := copyDB(s.kvdb, destDB.kvdb)
	if err != nil {
		return fmt.Errorf("failed to copy key-value database: %w", err)
	}

	err = copyDB(s.tdb, destDB.tdb)
	if err != nil {
		return fmt.Errorf("failed to copy trie database: %w", err)
	}

	return nil
}

// PruneDB takes the current LevelDB instance and removes any key-value pairs that also
// appear in the next LevelDB instance.
func (s *LevelDB) PruneDB(next databases.DAL) error {
	nextLevelDB, ok := next.(*LevelDB)
	if !ok {
		return fmt.Errorf("pruning supported between the same databases only - make sure you supply instance of LevelDB here")
	}

	iterKVDB := s.kvdb.NewIterator(nil, nil)
	defer iterKVDB.Release()

	for iterKVDB.Next() {
		key := iterKVDB.Key()

		// here we ignore the error because we don't care if the value is actually in the other db
		// we only care if about the value retrieved from the next db and if it is equal to the
		// value retrieved from our current db instance
		val, _ := nextLevelDB.kvdb.Get(key, nil)
		oval, err2 := s.kvdb.Get(key, nil)

		if err2 != nil {
			return err2
		}

		if bytes.Equal(val, oval) {
			err := s.kvdb.Delete(key, nil)

			if err != nil {
				return err
			}
		}
	}

	err := iterKVDB.Error()

	if err != nil {
		return err
	}
	iterTDB := s.tdb.NewIterator(nil, nil)
	defer iterTDB.Release()
	for iterTDB.Next() {

		key := iterTDB.Key()

		// see above rationale for ignoring error
		val, _ := nextLevelDB.tdb.Get(key, nil)
		oval, err2 := s.tdb.Get(key, nil)

		if err2 != nil {
			return err2
		}

		if bytes.Equal(val, oval) {
			err = s.tdb.Delete(key, nil)

			if err != nil {
				return err
			}
		}
	}

	err = iterTDB.Error()

	if err != nil {
		return err
	}

	return nil
}

// Size returns the size of the current LevelDB on disk in bytes.
func (s *LevelDB) Size() (int64, error) {
	return io.DirSize(s.path)
}

// copyDB copies the contents of one LevelDB instance to another.
func copyDB(src, dest *leveldb.DB) error {
	iter := src.NewIterator(nil, nil)
	defer iter.Release()

	for iter.Next() {
		key := iter.Key()
		value := iter.Value()

		err := dest.Put(key, value, nil)
		if err != nil {
			return err
		}
	}

	err := iter.Error()
	if err != nil {
		return err
	}

	return nil
}
