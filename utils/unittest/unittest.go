package unittest

import (
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
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

// AssertClosesBefore asserts that the given channel closes before the
// duration expires.
func AssertClosesBefore(t *testing.T, done <-chan struct{}, duration time.Duration) {
	select {
	case <-time.After(duration):
		t.Log("channel did not return in time")
		t.Fail()
	case <-done:
		return
	}
}

// RequireReturnBefore requires that the given function returns before the
// duration expires.
func RequireReturnsBefore(t testing.TB, f func(), duration time.Duration, message string) {
	done := make(chan struct{})

	go func() {
		f()
		close(done)
	}()

	select {
	case <-time.After(duration):
		require.Fail(t, "function did not return on time: "+message)
	case <-done:
		return
	}
}

// RequireConcurrentCallsReturnBefore is a test helper that runs function `f` count-many times concurrently,
// and requires all invocations to return within duration.
func RequireConcurrentCallsReturnBefore(t *testing.T, f func(), count int, duration time.Duration, message string) {
	wg := &sync.WaitGroup{}
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			f()
			wg.Done()
		}()
	}

	RequireReturnsBefore(t, wg.Wait, duration, message)
}

// AssertErrSubstringMatch asserts that two errors match with substring
// checking on the Error method (`expected` must be a substring of `actual`, to
// account for the actual error being wrapped). Fails the test if either error
// is nil.
//
// NOTE: This should only be used in cases where `errors.Is` cannot be, like
// when errors are transmitted over the network without type information.
func AssertErrSubstringMatch(t testing.TB, expected, actual error) {
	require.NotNil(t, expected)
	require.NotNil(t, actual)
	assert.True(
		t,
		strings.Contains(actual.Error(), expected.Error()) || strings.Contains(expected.Error(), actual.Error()),
		"expected error: '%s', got: '%s'", expected.Error(), actual.Error(),
	)
}

func TempDir(t testing.TB) string {
	dir, err := ioutil.TempDir("", "flow-testing-temp-")
	require.NoError(t, err)
	return dir
}

func RunWithTempDir(t testing.TB, f func(string)) {
	dbDir := TempDir(t)
	defer os.RemoveAll(dbDir)
	f(dbDir)
}

func BadgerDB(t testing.TB, dir string) *badger.DB {
	opts := badger.
		DefaultOptions(dir).
		WithKeepL0InMemory(true).
		WithLogger(nil)
	db, err := badger.Open(opts)
	require.NoError(t, err)
	return db
}

func RunWithBadgerDB(t testing.TB, f func(*badger.DB)) {
	RunWithTempDir(t, func(dir string) {
		db := BadgerDB(t, dir)
		defer db.Close()
		f(db)
	})
}

func TempBadgerDB(t testing.TB) (*badger.DB, string) {
	dir := TempDir(t)
	db := BadgerDB(t, dir)
	return db, dir
}
