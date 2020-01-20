package unittest

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/go-test/deep"
	"github.com/stretchr/testify/assert"
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

// ReturnsWithin returns true if the given function returns within the given
// duration, or false otherwise.
func ReturnsWithin(f func(), duration time.Duration) bool {
	done := make(chan struct{})

	go func() {
		f()
		close(done)
	}()

	select {
	case <-time.After(duration):
		return false
	case <-done:
		return true
	}
}

// AssertEqualWithDiff asserts that two objects are equal.
//
// If the objects are not equal, this function prints a human-readable diff.
func AssertEqualWithDiff(t *testing.T, expected, actual interface{}) {
	// the maximum levels of a struct to recurse into
	// this prevents infinite recursion from circular references
	deep.MaxDepth = 100

	diff := deep.Equal(expected, actual)

	if len(diff) != 0 {
		s := strings.Builder{}

		for i, d := range diff {
			if i == 0 {
				s.WriteString("diff    : ")
			} else {
				s.WriteString("          ")
			}

			s.WriteString(d)
			s.WriteString("\n")
		}

		assert.Fail(t, fmt.Sprintf("Not equal: \n"+
			"expected: %s\n"+
			"actual  : %s\n\n"+
			"%s", expected, actual, s.String()))
	}
}

func RunWithBadgerDB(t *testing.T, f func(*badger.DB)) {
	dir := filepath.Join(os.TempDir(), fmt.Sprintf("flow-test-db-%d", rand.Uint64()))

	db, err := badger.Open(badger.DefaultOptions(dir).WithLogger(nil))
	require.Nil(t, err)

	defer func() {
		db.Close()
		os.RemoveAll(dir)
	}()

	f(db)
}

func RunWithLevelDB(t *testing.T, f func(db *leveldb.LevelDB)) {
	dir := filepath.Join(os.TempDir(), fmt.Sprintf("flow-test-db-%d", rand.Uint64()))

	kvdbPath := filepath.Join(dir, "kvdb")
	tdbPath := filepath.Join(dir, "tdb")

	db, err := leveldb.NewLevelDB(kvdbPath, tdbPath)
	require.Nil(t, err)

	defer func() {
		db.SafeClose()
		os.RemoveAll(dir)
	}()

	f(db)
}
