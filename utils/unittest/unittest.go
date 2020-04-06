package unittest

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/dgraph-io/badger/v2/options"
	"github.com/stretchr/testify/require"
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

func TempDBDir(t *testing.T) string {
	dir, err := ioutil.TempDir("", "flow-test-db")
	require.NoError(t, err)
	return dir
}

func RunWithTempDBDir(t *testing.T, f func(string)) {
	dbDir := TempDBDir(t)
	defer os.RemoveAll(dbDir)

	f(dbDir)
}

func TempBadgerDB(t *testing.T) (*badger.DB, string) {

	dir := TempDBDir(t)

	db, err := badger.Open(badger.DefaultOptions(dir).WithLogger(nil).WithValueLogLoadingMode(options.FileIO))
	require.Nil(t, err)

	return db, dir
}

func RunWithBadgerDB(t *testing.T, f func(*badger.DB)) {
	RunWithTempDBDir(t, func(dir string) {
		// Ref: https://github.com/dgraph-io/badger#memory-usage
		db, err := badger.Open(badger.DefaultOptions(dir).WithLogger(nil).WithValueLogLoadingMode(options.FileIO))
		require.NoError(t, err)

		f(db)

		db.Close()
	})
}
