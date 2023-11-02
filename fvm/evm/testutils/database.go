package testutils

import (
	gethCommon "github.com/ethereum/go-ethereum/common"
	gethDB "github.com/ethereum/go-ethereum/ethdb"

	"github.com/onflow/flow-go/fvm/evm/types"
)

type TestDatabase struct {
	GetFunc              func(key []byte) ([]byte, error)
	HasFunc              func(key []byte) (bool, error)
	PutFunc              func(key []byte, value []byte) error
	DeleteFunc           func(key []byte) error
	StatFunc             func(property string) (string, error)
	NewBatchFunc         func() gethDB.Batch
	NewBatchWithSizeFunc func(size int) gethDB.Batch
	NewIteratorFunc      func(prefix []byte, start []byte) gethDB.Iterator
	CompactFunc          func(start []byte, limit []byte) error
	NewSnapshotFunc      func() (gethDB.Snapshot, error)
	CloseFunc            func() error
	GetRootHashFunc      func() (gethCommon.Hash, error)
	CommitFunc           func(roothash gethCommon.Hash) error
}

var _ types.Database = &TestDatabase{}

func (db *TestDatabase) Get(key []byte) ([]byte, error) {
	if db.GetFunc == nil {
		panic("method not set")
	}
	return db.GetFunc(key)
}

func (db *TestDatabase) Has(key []byte) (bool, error) {
	if db.HasFunc == nil {
		panic("method not set")
	}
	return db.HasFunc(key)
}

func (db *TestDatabase) Put(key []byte, value []byte) error {
	if db.PutFunc == nil {
		panic("method not set")
	}
	return db.PutFunc(key, value)
}

func (db *TestDatabase) Delete(key []byte) error {
	if db.DeleteFunc == nil {
		panic("method not set")
	}
	return db.DeleteFunc(key)
}

func (db *TestDatabase) Commit(root gethCommon.Hash) error {
	if db.CommitFunc == nil {
		panic("method not set")
	}
	return db.CommitFunc(root)
}

func (db *TestDatabase) GetRootHash() (gethCommon.Hash, error) {
	if db.GetRootHashFunc == nil {
		panic("method not set")
	}
	return db.GetRootHashFunc()
}

func (db *TestDatabase) Stat(property string) (string, error) {
	if db.StatFunc == nil {
		panic("method not set")
	}
	return db.StatFunc(property)
}

func (db *TestDatabase) NewBatch() gethDB.Batch {
	if db.NewBatchFunc == nil {
		panic("method not set")
	}
	return db.NewBatchFunc()
}

func (db *TestDatabase) NewBatchWithSize(size int) gethDB.Batch {
	if db.NewBatchWithSizeFunc == nil {
		panic("method not set")
	}
	return db.NewBatchWithSizeFunc(size)
}

func (db *TestDatabase) NewIterator(prefix []byte, start []byte) gethDB.Iterator {
	if db.NewIteratorFunc == nil {
		panic("method not set")
	}
	return db.NewIteratorFunc(prefix, start)
}

func (db *TestDatabase) Compact(start []byte, limit []byte) error {
	if db.CompactFunc == nil {
		panic("method not set")
	}
	return db.CompactFunc(start, limit)
}

func (db *TestDatabase) NewSnapshot() (gethDB.Snapshot, error) {
	if db.NewSnapshotFunc == nil {
		panic("method not set")
	}
	return db.NewSnapshotFunc()
}

func (db *TestDatabase) Close() error {
	if db.CloseFunc == nil {
		panic("method not set")
	}
	return db.CloseFunc()
}
