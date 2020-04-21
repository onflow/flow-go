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

var (
	kvdbPrefix = "kvdb"
	triePrefix = "trie"
)

// LevelDB is a levelDB implementation of the DAL interface
type LevelDB struct {
	//kvdbPath string
	//tdbPath  string
	path    string
	kvdb    *leveldb.DB    // The key-value database
	tdb     *leveldb.DB    // The trie database
	batcher *leveldb.Batch // A batcher used for atomic updates of trie DB
}

// GetKVDB calls a get on the key-value database
func (s *LevelDB) GetKVDB(key []byte) ([]byte, error) {
	value, err := s.kvdb.Get(key, nil)
	if errors.Is(err, leveldb.ErrNotFound) {
		return value, databases.ErrNotFound
	}
	return value, err
}

// GetTrieDB calls Get on the TrieDB in the interface
func (s *LevelDB) GetTrieDB(key []byte) ([]byte, error) {
	return s.tdb.Get(key, nil)
}

// NewLevelDB creates a new LevelDB database.
func NewLevelDB(path string) (*LevelDB, error) {

	kvdbPath := filepath.Join(path, kvdbPrefix)
	tdbPath := filepath.Join(path, triePrefix)

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
		path:    path,
		kvdb:    kvdb,
		tdb:     tdb,
		batcher: new(leveldb.Batch),
	}, nil
}

// NewBackupLevelDB creates a new LevelDB database interface at the specified path db/stateRootIndex
//func NewBackupLevelDB(kvdbPath, tdbPath, stateRootIndex string) (*LevelDB, error) {
//	kvdbPath = filepath.Join(kvdbPath, stateRootIndex)
//	tdbPath = filepath.Join(tdbPath, stateRootIndex)
//
//	return NewLevelDB(kvdbPath, tdbPath)
//}

// newBatch creates a new batch, effectively clearing the old batch
func (s *LevelDB) NewBatcher() databases.Batcher {
	return new(leveldb.Batch)
}

// PutIntoBatcher inserts or Deletes a KV pair into the batcher.
func (s *LevelDB) PutIntoBatcher(key []byte, value []byte) {
	if value == nil {
		s.batcher.Delete(key)
	}
	s.batcher.Put(key, value)
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

// UpdateTrieDB updates the trie DB with the new paths in the update
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

func (s *LevelDB) CopyTo(to databases.DAL) error {
	levelDB, ok := to.(*LevelDB)
	if !ok {
		return fmt.Errorf("copy supported between the same databases only - make sure you supply instance of LevelDB here")
	}

	err := s.copyKVDB(levelDB)

	if err != nil {
		return fmt.Errorf("error while copying KVDB: %w", err)
	}

	err = s.copyTDB(levelDB)

	if err != nil {
		return fmt.Errorf("error while copying TrieDB: %w", err)
	}

	return nil
}

// PruneDB takes the current LevelDB instance and removes any key-value pairs that also appear in the next LevelDB instance.
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

// Size returns the size of the current LevelDB on disk in bytes
func (s *LevelDB) Size() (int64, error) {
	return io.DirSize(s.path)
}

// TODO Replace copyKVDB and copyTDB with a single function
func (s *LevelDB) copyKVDB(copy *LevelDB) error {
	iter := s.kvdb.NewIterator(nil, nil)
	defer iter.Release()

	for iter.Next() {
		key := iter.Key()
		value := iter.Value()

		err := copy.kvdb.Put(key, value, nil)

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

func (s *LevelDB) copyTDB(copy *LevelDB) error {
	iter := s.tdb.NewIterator(nil, nil)
	defer iter.Release()

	for iter.Next() {
		key := iter.Key()
		value := iter.Value()

		err := copy.tdb.Put(key, value, nil)

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
