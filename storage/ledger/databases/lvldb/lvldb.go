package lvldb

import (
	"bytes"

	"github.com/dapperlabs/flow-go/storage/ledger/databases"
	"github.com/syndtr/goleveldb/leveldb"
)

// KVDB_PATH is the path to the KV Database
const KVDB_PATH = "db/valuedb"

// TDB_PATH is the path to the Trie Database
const TDB_PATH = "db/triedb"

var _ databases.DAL = &LvlDb{}

// LevelDB_DAL is a levelDB implementation of the DAL interface
type LvlDb struct {
	KVDB    *leveldb.DB    // The Key-Value Database
	TDB     *leveldb.DB    // The Trie Database
	batcher *leveldb.Batch // A batcher for Atomtically Updating
}

// GetKVDB calls a get on the Key-Value database
func (s *LvlDb) GetKVDB(key []byte) ([]byte, error) {
	return s.KVDB.Get(key, nil)
}

// GetTrieDB calls Get on the TrieDB in the interface
func (s *LvlDb) GetTrieDB(key []byte) ([]byte, error) {
	return s.TDB.Get(key, nil)
}

// NewLDB_DAL creates a new LevelDB database interface
func NewLvlDb() (*LvlDb, error) {
	dal := new(LvlDb)
	// Opening the KV database
	kvdb, err := leveldb.OpenFile(KVDB_PATH, nil)
	if err != nil {
		return nil, err
	}

	dal.KVDB = kvdb

	// Opening the Trie database
	tdb, err := leveldb.OpenFile(TDB_PATH, nil)
	if err != nil {
		return nil, err
	}

	dal.TDB = tdb
	dal.batcher = new(leveldb.Batch)

	return dal, nil
}

// NewBackupLvlDb creates a new LevelDB database interface at the specified path db/stateRootIndex
func NewBackupLvlDb(stateRootIndex string) (*LvlDb, error) {
	dal := new(LvlDb)

	kvdbpath := "db/" + stateRootIndex + "/valuedb"
	tdbpath := "db/" + stateRootIndex + "/triedb"

	// Opening the KV database
	kvdb, err := leveldb.OpenFile(kvdbpath, nil)
	if err != nil {
		return nil, err
	}

	dal.KVDB = kvdb

	// Opening the Trie database
	tdb, err := leveldb.OpenFile(tdbpath, nil)
	if err != nil {
		return nil, err
	}

	dal.TDB = tdb
	dal.batcher = new(leveldb.Batch)

	return dal, nil
}

// newBatch creates a new batch, effectively clearing the old batch
func (s *LvlDb) NewBatch() {
	s.batcher = new(leveldb.Batch)
}

// PutIntoBatcher inserts or Deletes a KV pair into the batcher
func (s *LvlDb) PutIntoBatcher(key []byte, value []byte) {
	if value == nil {
		s.batcher.Delete(key)
	}
	s.batcher.Put(key, value)
}

// safeClose is a helper function that closes databases safely
func (s *LvlDb) SafeClose() (error, error) {

	return s.KVDB.Close(), s.TDB.Close()
}

// UpdateKVDB updates the Key: Value pair in the key-value database
func (s *LvlDb) UpdateKVDB(keys [][]byte, values [][]byte) error {

	kvBatcher := new(leveldb.Batch)
	for i := 0; i < len(keys); i++ {
		kvBatcher.Put(keys[i], values[i])
	}

	err := s.KVDB.Write(kvBatcher, nil)
	if err != nil {
		return err
	}

	return nil
}

// UpdateTrieDB updates the trie DB with the new paths in the update
func (s *LvlDb) UpdateTrieDB() error {

	err := s.TDB.Write(s.batcher, nil)
	if err != nil {
		return err
	}

	return nil
}

func (s *LvlDb) CopyDB(stateRootIndex string) (*LvlDb, error) {
	backup, err := NewBackupLvlDb(stateRootIndex)

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

// PruneDb takes the current LvlDB instance and removed any key value pairs that also appear in the LvlDB instance next
func (s *LvlDb) PruneDB(next *LvlDb) error {
	iterKVDB := s.KVDB.NewIterator(nil, nil)
	defer iterKVDB.Release()

	for iterKVDB.Next() {
		key := iterKVDB.Key()

		// here we ignore the error because we don't care if the value is actually in the other db
		// we only care if about the value retrieved from the next db and if it is equal to the
		// value retrieved from our current db instance
		val, _ := next.KVDB.Get(key, nil)
		oval, err2 := s.KVDB.Get(key, nil)

		if err2 != nil {
			return err2
		}

		if bytes.Equal(val, oval) {
			err := s.KVDB.Delete(key, nil)

			if err != nil {
				return err
			}
		}
	}

	err := iterKVDB.Error()

	if err != nil {
		return err
	}
	iterTDB := s.TDB.NewIterator(nil, nil)
	defer iterTDB.Release()
	for iterTDB.Next() {

		key := iterTDB.Key()

		// see above rationale for ignoring error
		val, _ := next.TDB.Get(key, nil)
		oval, err2 := s.TDB.Get(key, nil)

		if err2 != nil {
			return err2
		}

		if bytes.Equal(val, oval) {
			err = s.TDB.Delete(key, nil)

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
func (s *LvlDb) copyKVDB(copy *LvlDb) error {
	iter := s.KVDB.NewIterator(nil, nil)
	defer iter.Release()

	for iter.Next() {
		key := iter.Key()
		value := iter.Value()

		err := copy.KVDB.Put(key, value, nil)

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

func (s *LvlDb) copyTDB(copy *LvlDb) error {
	iter := s.TDB.NewIterator(nil, nil)
	defer iter.Release()

	for iter.Next() {
		key := iter.Key()
		value := iter.Value()

		err := copy.TDB.Put(key, value, nil)

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
