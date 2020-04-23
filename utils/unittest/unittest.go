package unittest

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/dgraph-io/badger/v2/options"
	"github.com/stretchr/testify/assert"
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

// RequireReturnBefore requires that the given function returns before the
// duration expires.
func RequireReturnsBefore(t *testing.T, f func(), duration time.Duration) {
	done := make(chan struct{})

	go func() {
		f()
		close(done)
	}()

	select {
	case <-time.After(duration):
		require.Fail(t, "function did not return in time")
	case <-done:
		return
	}
}

// AssertErrSubstringMatch asserts that two errors match with substring
// checking on the Error method (`expected` must be a substring of `actual`, to
// account for the actual error being wrapped). Fails the test if either error
// is nil.
//
// NOTE: This should only be used in cases where `errors.Is` cannot be, like
// when errors are transmitted over the network without type information.
func AssertErrSubstringMatch(t testing.TB, expected, actual error) {
	assert.NotNil(t, expected)
	assert.NotNil(t, actual)
	assert.True(t, strings.Contains(actual.Error(), expected.Error()))
}

func TempDBDir(t testing.TB) string {
	dir, err := ioutil.TempDir("", "flow-test-db")
	require.NoError(t, err)
	return dir
}

func RunWithTempDBDir(t testing.TB, f func(string)) {
	dbDir := TempDBDir(t)
	defer os.RemoveAll(dbDir)

	f(dbDir)
}

func TempBadgerDB(t testing.TB) (*badger.DB, string) {

	dir := TempDBDir(t)

	db, err := badger.Open(badger.DefaultOptions(dir).WithLogger(nil).WithValueLogLoadingMode(options.FileIO))
	require.Nil(t, err)

	return db, dir
}

func RunWithBadgerDB(t testing.TB, f func(*badger.DB)) {
	RunWithTempDBDir(t, func(dir string) {
		// Ref: https://github.com/dgraph-io/badger#memory-usage
		db, err := badger.Open(badger.DefaultOptions(dir).WithLogger(nil).WithValueLogLoadingMode(options.FileIO))
		require.NoError(t, err)

		f(db)

		db.Close()
	})
}
