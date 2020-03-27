package unittest

import (
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/dgraph-io/badger/v2/options"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/storage/ledger/databases/leveldb"
)

func ExpectPanic(expectedMsg string, t *testing.T) {
	if r := recover(); r != nil {
		err := r.(error)
		if err.Error() != expectedMsg {
			t.Errorf("expected %v to be %v", err, expectedMsg)
		}
		return
	}
	t.Errorf("Expected to panic with `%s`, but did not panic", expectedMsg)
}

// AssertReturnsBefore asserts that the given function returns before the
// duration expires.
func AssertReturnsBefore(t *testing.T, f func(), duration time.Duration) {
	done := make(chan struct{})

	go func() {
		f()
		close(done)
	}()

	select {
	case <-time.After(duration):
		t.Log("function did not return in time")
		t.Fail()
	case <-done:
		return
	}
}

func TempDBDir() (string, error) {
	return ioutil.TempDir("", "flow-test-db")
}

func TempBadgerDB(t *testing.T) *badger.DB {
	dir, err := TempDBDir()
	require.Nil(t, err)

	// Ref: https://github.com/dgraph-io/badger#memory-usage
	db, err := badger.Open(badger.DefaultOptions(dir).WithLogger(nil).WithValueLogLoadingMode(options.FileIO))
	require.Nil(t, err)

	return db
}

func LevelDBInDir(t *testing.T, dir string) *leveldb.LevelDB {
	kvdbPath := filepath.Join(dir, "kvdb")
	tdbPath := filepath.Join(dir, "tdb")

	db, err := leveldb.NewLevelDB(kvdbPath, tdbPath)
	require.NoError(t, err)

	return db
}

func TempLevelDB(t *testing.T) *leveldb.LevelDB {
	dir, err := TempDBDir()
	require.Nil(t, err)

	return LevelDBInDir(t, dir)
}

func RunWithBadgerDB(t *testing.T, f func(*badger.DB)) {
	db := TempBadgerDB(t)

	defer func() {
		err := db.Close()
		require.Nil(t, err)
	}()

	f(db)
}

func RunWithLevelDB(t *testing.T, f func(db *leveldb.LevelDB)) {
	db := TempLevelDB(t)

	defer func() {
		err1, err2 := db.SafeClose()
		require.NoError(t, err1)
		require.NoError(t, err2)
	}()

	f(db)
}
