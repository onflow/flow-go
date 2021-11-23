package unittest

import (
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/onflow/flow-go/model/flow"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/util"
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
func AssertReturnsBefore(t *testing.T, f func(), duration time.Duration, msgAndArgs ...interface{}) {
	done := make(chan struct{})

	go func() {
		f()
		close(done)
	}()

	select {
	case <-time.After(duration):
		t.Log("function did not return in time")
		assert.Fail(t, "function did not close in time", msgAndArgs...)
	case <-done:
		return
	}
}

// AssertClosesBefore asserts that the given channel closes before the
// duration expires.
func AssertClosesBefore(t assert.TestingT, done <-chan struct{}, duration time.Duration, msgAndArgs ...interface{}) {
	select {
	case <-time.After(duration):
		assert.Fail(t, "channel did not return in time", msgAndArgs...)
	case <-done:
		return
	}
}

// AssertNotClosesBefore asserts that the given channel does not close before the duration expires.
func AssertNotClosesBefore(t assert.TestingT, done <-chan struct{}, duration time.Duration, msgAndArgs ...interface{}) {
	select {
	case <-time.After(duration):
		return
	case <-done:
		assert.Fail(t, "channel closed before timeout", msgAndArgs...)
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

	RequireCloseBefore(t, done, duration, message+": function did not return on time")
}

// RequireComponentsDoneBefore invokes the done method of each of the input components concurrently, and
// fails the test if any components shutdown takes longer than the specified duration.
func RequireComponentsDoneBefore(t testing.TB, duration time.Duration, components ...module.ReadyDoneAware) {
	done := util.AllDone(components...)
	RequireCloseBefore(t, done, duration, "failed to shutdown all components on time")
}

// RequireComponentsReadyBefore invokes the ready method of each of the input components concurrently, and
// fails the test if any components startup takes longer than the specified duration.
func RequireComponentsReadyBefore(t testing.TB, duration time.Duration, components ...module.ReadyDoneAware) {
	ready := util.AllReady(components...)
	RequireCloseBefore(t, ready, duration, "failed to start all components on time")
}

// RequireCloseBefore requires that the given channel returns before the
// duration expires.
func RequireCloseBefore(t testing.TB, c <-chan struct{}, duration time.Duration, message string) {
	select {
	case <-time.After(duration):
		require.Fail(t, "could not close done channel on time: "+message)
	case <-c:
		return
	}
}

// RequireClosed is a test helper function that fails the test if channel `ch` is not closed.
func RequireClosed(t *testing.T, ch <-chan struct{}, message string) {
	select {
	case <-ch:
	default:
		require.Fail(t, "channel is not closed: "+message)
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

// RequireNeverReturnBefore is a test helper that tries invoking function `f` and fails the test if either:
// - function `f` is not invoked within 1 second.
// - function `f` returns before specified `duration`.
//
// It also returns a channel that is closed once the function `f` returns and hence its openness can evaluate
// return status of function `f` for intervals longer than duration.
func RequireNeverReturnBefore(t *testing.T, f func(), duration time.Duration, message string) <-chan struct{} {
	ch := make(chan struct{})
	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		wg.Done()
		f()
		close(ch)
	}()

	// requires function invoked within next 1 second
	RequireReturnsBefore(t, wg.Wait, 1*time.Second, "could not invoke the function: "+message)

	// requires function never returns within duration
	RequireNeverClosedWithin(t, ch, duration, "unexpected return: "+message)

	return ch
}

// RequireNeverClosedWithin is a test helper function that fails the test if channel `ch` is closed before the
// determined duration.
func RequireNeverClosedWithin(t *testing.T, ch <-chan struct{}, duration time.Duration, message string) {
	select {
	case <-time.After(duration):
	case <-ch:
		require.Fail(t, "channel closed before timeout: "+message)
	}
}

// RequireNotClosed is a test helper function that fails the test if channel `ch` is closed.
func RequireNotClosed(t *testing.T, ch <-chan struct{}, message string) {
	select {
	case <-ch:
		require.Fail(t, "channel is closed: "+message)
	default:
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
	defer func() {
		require.NoError(t, os.RemoveAll(dbDir))
	}()
	f(dbDir)
}

func badgerDB(t testing.TB, dir string, create func(badger.Options) (*badger.DB, error)) *badger.DB {
	opts := badger.
		DefaultOptions(dir).
		WithKeepL0InMemory(true).
		WithLogger(nil)
	db, err := create(opts)
	require.NoError(t, err)
	return db
}

func BadgerDB(t testing.TB, dir string) *badger.DB {
	return badgerDB(t, dir, badger.Open)
}

func TypedBadgerDB(t testing.TB, dir string, create func(badger.Options) (*badger.DB, error)) *badger.DB {
	return badgerDB(t, dir, create)
}

func RunWithBadgerDB(t testing.TB, f func(*badger.DB)) {
	RunWithTempDir(t, func(dir string) {
		db := BadgerDB(t, dir)
		defer func() {
			assert.NoError(t, db.Close())
		}()
		f(db)
	})
}

// RunWithTypedBadgerDB creates a Badger DB that is passed to f and closed
// after f returns. The extra create parameter allows passing in a database
// constructor function which instantiates a database with a particular type
// marker, for testing storage modules which require a backed with a particular
// type.
func RunWithTypedBadgerDB(t testing.TB, create func(badger.Options) (*badger.DB, error), f func(*badger.DB)) {
	RunWithTempDir(t, func(dir string) {
		db := badgerDB(t, dir, create)
		defer func() {
			assert.NoError(t, db.Close())
		}()
		f(db)
	})
}

func TempBadgerDB(t testing.TB) (*badger.DB, string) {
	dir := TempDir(t)
	db := BadgerDB(t, dir)
	return db, dir
}

func Concurrently(n int, f func(int)) {
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			f(i)
			wg.Done()
		}(i)
	}
	wg.Wait()
}

// AssertEqualBlocksLenAndOrder asserts that both a segment of blocks have the same len and blocks are in the same order
func AssertEqualBlocksLenAndOrder(t *testing.T, expectedBlocks, actualSegmentBlocks []*flow.Block) {
	assert.Equal(t, flow.GetIDs(expectedBlocks), flow.GetIDs(actualSegmentBlocks))
}
