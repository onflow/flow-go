package lvldb

import (
	"bytes"

	"github.com/dapperlabs/flow-go/storage/ledger/databases"
	"github.com/syndtr/goleveldb/leveldb"
)

var _ databases.DAL = &LvlDB{}

// LvlDB is a levelDB implementation of the DAL interface
type LvlDB struct {
	kvdb    *leveldb.DB    // The key-value database
	tdb     *leveldb.DB    // The trie database
	batcher *leveldb.Batch // A batcher used for atomic updates
}

// GetKVDB calls a get on the key-value database
func (s *LvlDB) GetKVDB(key []byte) ([]byte, error) {
	return s.kvdb.Get(key, nil)
}

// GetTrieDB calls Get on the TrieDB in the interface
func (s *LvlDB) GetTrieDB(key []byte) ([]byte, error) {
	return s.tdb.Get(key, nil)
}

// NewLvlDB creates a new LevelDB database.
func NewLvlDB(kvdbPath, tdbPath string) (*LvlDB, error) {
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

	return &LvlDB{
		kvdb:    kvdb,
		tdb:     tdb,
		batcher: new(leveldb.Batch),
	}, nil
}

// NewBackupLvlDB creates a new LevelDB database interface at the specified path db/stateRootIndex
func NewBackupLvlDB(stateRootIndex string) (*LvlDB, error) {
	dal := new(LvlDB)

	kvdbpath := "db/" + stateRootIndex + "/valuedb"
	tdbpath := "db/" + stateRootIndex + "/triedb"

	// Opening the KV database
	kvdb, err := leveldb.OpenFile(kvdbpath, nil)
	if err != nil {
		return nil, err
	}

	dal.kvdb = kvdb

	// Opening the Trie database
	tdb, err := leveldb.OpenFile(tdbpath, nil)
	if err != nil {
		return nil, err
	}

	dal.tdb = tdb
	dal.batcher = new(leveldb.Batch)

	return dal, nil
}

// newBatch creates a new batch, effectively clearing the old batch
func (s *LvlDB) NewBatch() {
	s.batcher = new(leveldb.Batch)
}

// PutIntoBatcher inserts or Deletes a KV pair into the batcher.
func (s *LvlDB) PutIntoBatcher(key []byte, value []byte) {
	if value == nil {
		s.batcher.Delete(key)
	}
	s.batcher.Put(key, value)
}

// SafeClose is a helper function that closes databases safely.
func (s *LvlDB) SafeClose() (error, error) {
	return s.kvdb.Close(), s.tdb.Close()
}

// UpdateKVDB updates the provided key-value pairs in the key-value database.
func (s *LvlDB) UpdateKVDB(keys [][]byte, values [][]byte) error {

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
func (s *LvlDB) UpdateTrieDB() error {

	err := s.tdb.Write(s.batcher, nil)
	if err != nil {
		return err
	}

	return nil
}

func (s *LvlDB) CopyDB(stateRootIndex string) (*LvlDB, error) {
	backup, err := NewBackupLvlDB(stateRootIndex)

	if err != nil {
		return nil, err
	}

	err1 := s.copyKVDB(backup)

	if err1 != nil {
		return nil, err1
	}

	err2 := s.copyTDB(backup)

	if err2 != nil {
		return nil, err2
	}

	return backup, nil
}

// PruneDB takes the current LvlDB instance and removed any key value pairs that also appear in the LvlDB instance next
func (s *LvlDB) PruneDB(next *LvlDB) error {
	iterKVDB := s.kvdb.NewIterator(nil, nil)
	defer iterKVDB.Release()

	for iterKVDB.Next() {
		key := iterKVDB.Key()

		// here we ignore the error because we don't care if the value is actually in the other db
		// we only care if about the value retrieved from the next db and if it is equal to the
		// value retrieved from our current db instance
		val, _ := next.kvdb.Get(key, nil)
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
		val, _ := next.tdb.Get(key, nil)
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

// TODO Replace copyKVDB and copyTDB with a single function
func (s *LvlDB) copyKVDB(copy *LvlDB) error {
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

func (s *LvlDB) copyTDB(copy *LvlDB) error {
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
