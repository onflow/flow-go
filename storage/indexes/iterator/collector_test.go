package iterator_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/indexes/iterator"
)

// testEntry is a simple IteratorEntry implementation for testing.
type testEntry struct {
	value  string
	cursor int
}

func (e testEntry) Cursor() (int, error) { return e.cursor, nil }
func (e testEntry) Value() (string, error) { return e.value, nil }

// errEntry is an IteratorEntry that always returns an error.
type errEntry struct{ err error }

func (e errEntry) Cursor() (int, error)  { return 0, e.err }
func (e errEntry) Value() (string, error) { return "", e.err }

func newIter(entries ...storage.IteratorEntry[string, int]) storage.IndexIterator[string, int] {
	return func(yield func(storage.IteratorEntry[string, int]) bool) {
		for _, e := range entries {
			if !yield(e) {
				return
			}
		}
	}
}

func TestCollectResults(t *testing.T) {
	t.Parallel()

	t.Run("limit zero returns immediately with no results", func(t *testing.T) {
		iter := newIter(testEntry{value: "a", cursor: 1})

		results, cursor, err := iterator.CollectResults(iter, 0, nil)
		require.NoError(t, err)
		assert.Nil(t, results)
		assert.Nil(t, cursor)
	})

	t.Run("fewer results than limit returns all with nil cursor", func(t *testing.T) {
		iter := newIter(
			testEntry{value: "a", cursor: 1},
			testEntry{value: "b", cursor: 2},
		)

		results, cursor, err := iterator.CollectResults(iter, 10, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b"}, results)
		assert.Nil(t, cursor)
	})

	t.Run("exactly limit results returns all with nil cursor", func(t *testing.T) {
		iter := newIter(
			testEntry{value: "a", cursor: 1},
			testEntry{value: "b", cursor: 2},
		)

		results, cursor, err := iterator.CollectResults(iter, 2, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b"}, results)
		assert.Nil(t, cursor)
	})

	t.Run("more results than limit returns limit results and next cursor", func(t *testing.T) {
		iter := newIter(
			testEntry{value: "a", cursor: 1},
			testEntry{value: "b", cursor: 2},
			testEntry{value: "c", cursor: 3},
		)

		results, nextCursor, err := iterator.CollectResults(iter, 2, nil)
		require.NoError(t, err)
		require.Equal(t, []string{"a", "b"}, results)
		require.NotNil(t, nextCursor)
		assert.Equal(t, 3, *nextCursor)
	})

	t.Run("nil filter passes all items", func(t *testing.T) {
		iter := newIter(
			testEntry{value: "a", cursor: 1},
			testEntry{value: "b", cursor: 2},
		)

		results, _, err := iterator.CollectResults(iter, 10, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b"}, results)
	})

	t.Run("filter rejects non-matching items", func(t *testing.T) {
		iter := newIter(
			testEntry{value: "keep", cursor: 1},
			testEntry{value: "skip", cursor: 2},
			testEntry{value: "keep", cursor: 3},
		)
		filter := func(v *string) bool { return *v == "keep" }

		results, cursor, err := iterator.CollectResults(iter, 10, filter)
		require.NoError(t, err)
		assert.Equal(t, []string{"keep", "keep"}, results)
		assert.Nil(t, cursor)
	})

	t.Run("error from Value propagates", func(t *testing.T) {
		valErr := errors.New("value error")
		iter := newIter(errEntry{err: valErr})

		_, _, err := iterator.CollectResults(iter, 10, nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, valErr)
	})

	t.Run("error from Cursor propagates when building next cursor", func(t *testing.T) {
		cursorErr := errors.New("cursor error")
		iter := newIter(
			testEntry{value: "a", cursor: 1},
			errEntry{err: cursorErr},
		)

		_, _, err := iterator.CollectResults(iter, 1, nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, cursorErr)
	})

	t.Run("empty iterator returns nil results and nil cursor", func(t *testing.T) {
		iter := newIter()

		results, cursor, err := iterator.CollectResults(iter, 10, nil)
		require.NoError(t, err)
		assert.Nil(t, results)
		assert.Nil(t, cursor)
	})
}
