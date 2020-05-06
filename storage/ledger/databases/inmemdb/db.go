package inmemdb

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/dapperlabs/flow-go/storage/ledger/databases"
)

var (
	kvdbPrefix = "kvdb"
	triePrefix = "trie"
)

// InMemDB is a fully in memory implementation of the DAL interface
type InMemDB struct {
	Path    string            `json:"path"`
	Kvdb    map[string][]byte `json:"kvdb"`
	Tdb     map[string][]byte `json:"rdb"`
	lock    sync.Mutex
	batcher *InMemBatcher
}

// GetKVDB calls a get on the key-value database
func (m *InMemDB) GetKVDB(key []byte) ([]byte, error) {
	value, ok := m.Kvdb[hex.EncodeToString(key)]
	if !ok {
		return value, databases.ErrNotFound
	}
	return value, nil
}

// GetTrieDB calls Get on the TrieDB in the interface
func (m *InMemDB) GetTrieDB(key []byte) ([]byte, error) {
	value, ok := m.Tdb[hex.EncodeToString(key)]
	if !ok {
		return value, databases.ErrNotFound
	}
	return value, nil
}

// NewInMemDB creates a new InMemDB database.
func NewInMemDB(path string) (*InMemDB, error) {
	m := &InMemDB{
		Path:    path,
		Kvdb:    make(map[string][]byte),
		Tdb:     make(map[string][]byte),
		batcher: NewInMemBatcher(),
	}
	m.Load()
	return m, nil
}

// NewBatcher creates a new batch
func (m *InMemDB) NewBatcher() databases.Batcher {
	return NewInMemBatcher()
}

// PutIntoBatcher inserts or Deletes a KV pair into the batcher.
func (m *InMemDB) PutIntoBatcher(key []byte, value []byte) {
	if value == nil {
		m.batcher.Delete(key)
	}
	m.batcher.Put(key, value)
}

func (m *InMemDB) Load() error {
	m.lock.Lock()
	defer m.lock.Unlock()

	fpath := filepath.Join(m.Path, "data.json")
	// check path exist
	if _, err := os.Stat(fpath); os.IsNotExist(err) {
		return err
	}

	f, err := ioutil.ReadFile(fpath)
	if err != nil {
		return err
	}
	err = json.Unmarshal([]byte(f), m)
	if err != nil {
		return err
	}
	return nil
}

func (m InMemDB) Persist() error {
	m.lock.Lock()
	defer m.lock.Unlock()

	file, err := json.MarshalIndent(m, "", " ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(m.Path, 0755); err != nil {
		return err
	}
	err = ioutil.WriteFile(filepath.Join(m.Path, "data.json"), file, 0644)
	if err != nil {
		return err
	}
	return nil
}

// SafeClose is a helper function that closes databases safely.
func (m *InMemDB) SafeClose() (error, error) {
	m.Persist()
	return nil, nil
}

// UpdateKVDB updates the provided key-value pairs in the key-value database.
func (m *InMemDB) UpdateKVDB(keys [][]byte, values [][]byte) error {
	for i := 0; i < len(keys); i++ {
		m.Kvdb[hex.EncodeToString(keys[i])] = values[i]
	}
	return nil
}

// UpdateTrieDB updates the trie DB with the new paths in the update
func (m *InMemDB) UpdateTrieDB(batcher databases.Batcher) error {
	b, ok := batcher.(*InMemBatcher)
	if !ok {
		return fmt.Errorf("supplied batcher is not of type InMemBatcher")
	}
	for key, value := range b.Batch {
		m.Tdb[key] = value
	}
	return nil
}

func (m *InMemDB) CopyTo(to databases.DAL) error {
	db, ok := to.(*InMemDB)
	if !ok {
		return fmt.Errorf("copy supported between the same database types only")
	}
	for key, value := range m.Kvdb {
		db.Kvdb[key] = value
	}
	for key, value := range m.Tdb {
		db.Tdb[key] = value
	}
	return nil
}

// PruneDB takes the current LevelDB instance and removes any key-value pairs that also appear in the next LevelDB instance.
func (m *InMemDB) PruneDB(next databases.DAL) error {
	nextDB, ok := next.(*InMemDB)
	if !ok {
		return fmt.Errorf("pruning supported between the same databases only")
	}

	for key, oval := range m.Kvdb {
		val, ok := nextDB.Kvdb[key]
		if !ok {
			// if doesn't exist in nextDB keep it
			continue
		}
		// if key values are the same, delete it from old db
		if bytes.Equal(val, oval) {
			delete(m.Kvdb, key)
		}
	}

	for key, oval := range m.Tdb {
		val, ok := nextDB.Tdb[key]
		if !ok {
			// if doesn't exist in nextDB keep it
			continue
		}
		// if key values are the same, delete it from old db
		if bytes.Equal(val, oval) {
			delete(m.Tdb, key)
		}
	}

	return nil
}

// Size returns the size of the current db
func (m *InMemDB) Size() (int64, error) {
	return int64(len(m.Kvdb)) + int64(len(m.Tdb)), nil
}
