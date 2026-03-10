package iterator_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/indexes/iterator"
)

// memItem implements [storage.IterItem] for testing.
type memItem struct {
	key []byte
	val []byte
}

func (m *memItem) Key() []byte                        { return m.key }
func (m *memItem) KeyCopy(dst []byte) []byte          { return append(dst[:0], m.key...) }
func (m *memItem) Value(fn func([]byte) error) error  { return fn(m.val) }

// memIterator implements [storage.Iterator] backed by an in-memory slice.
type memIterator struct {
	items []*memItem
	idx   int
}

func newMemIterator(items ...*memItem) *memIterator {
	return &memIterator{items: items, idx: -1}
}

func (m *memIterator) First() bool {
	m.idx = 0
	return m.Valid()
}

func (m *memIterator) Valid() bool { return m.idx >= 0 && m.idx < len(m.items) }

func (m *memIterator) Next() { m.idx++ }

func (m *memIterator) IterItem() storage.IterItem {
	if !m.Valid() {
		return nil
	}
	return m.items[m.idx]
}

func (m *memIterator) Close() error { return nil }

// decodeStringKey interprets the key as a string cursor.
func decodeStringKey(key []byte) (string, error) {
	return string(key), nil
}

// reconstructString builds a string value from cursor and raw bytes.
func reconstructString(_ string, val []byte) (*string, error) {
	s := string(val)
	return &s, nil
}

func TestBuild(t *testing.T) {
	t.Parallel()

	t.Run("iterates all entries", func(t *testing.T) {
		iter := newMemIterator(
			&memItem{key: []byte("k1"), val: []byte("v1")},
			&memItem{key: []byte("k2"), val: []byte("v2")},
			&memItem{key: []byte("k3"), val: []byte("v3")},
		)

		var results []string
		for entry, err := range iterator.Build(iter, decodeStringKey, reconstructString) {
			require.NoError(t, err)
			val, err := entry.Value()
			require.NoError(t, err)
			results = append(results, val)
		}
		assert.Equal(t, []string{"v1", "v2", "v3"}, results)
	})

	t.Run("empty iterator yields nothing", func(t *testing.T) {
		iter := newMemIterator()

		var count int
		for _, err := range iterator.Build(iter, decodeStringKey, reconstructString) {
			require.NoError(t, err)
			count++
		}
		assert.Zero(t, count)
	})

	t.Run("decodeKey error is yielded", func(t *testing.T) {
		decodeErr := errors.New("decode error")
		failDecode := func([]byte) (string, error) { return "", decodeErr }

		iter := newMemIterator(
			&memItem{key: []byte("k1"), val: []byte("v1")},
		)

		for _, err := range iterator.Build(iter, failDecode, reconstructString) {
			require.ErrorIs(t, err, decodeErr)
			break
		}
	})

	t.Run("reconstruct error is surfaced via Value", func(t *testing.T) {
		reconstructErr := errors.New("reconstruct error")
		failReconstruct := func(string, []byte) (*string, error) { return nil, reconstructErr }

		iter := newMemIterator(
			&memItem{key: []byte("k1"), val: []byte("v1")},
		)

		for entry, err := range iterator.Build(iter, decodeStringKey, failReconstruct) {
			require.NoError(t, err)
			_, err = entry.Value()
			require.ErrorIs(t, err, reconstructErr)
			break
		}
	})
}

func TestBuildPrefixIterator(t *testing.T) {
	t.Parallel()

	// keyPrefix returns the first 2 bytes as the group prefix.
	keyPrefix := func(key []byte) ([]byte, error) {
		if len(key) < 2 {
			return nil, fmt.Errorf("key too short: %d", len(key))
		}
		return key[:2], nil
	}

	t.Run("deduplicates consecutive entries with same prefix", func(t *testing.T) {
		// Keys share prefix "aa" — only first should be yielded
		iter := newMemIterator(
			&memItem{key: []byte("aa1"), val: []byte("first")},
			&memItem{key: []byte("aa2"), val: []byte("second")},
			&memItem{key: []byte("aa3"), val: []byte("third")},
		)

		var results []string
		for entry, err := range iterator.BuildPrefixIterator(iter, decodeStringKey, reconstructString, keyPrefix) {
			require.NoError(t, err)
			val, err := entry.Value()
			require.NoError(t, err)
			results = append(results, val)
		}
		assert.Equal(t, []string{"first"}, results)
	})

	t.Run("yields one entry per distinct prefix group", func(t *testing.T) {
		iter := newMemIterator(
			&memItem{key: []byte("aa1"), val: []byte("a-first")},
			&memItem{key: []byte("aa2"), val: []byte("a-second")},
			&memItem{key: []byte("bb1"), val: []byte("b-first")},
			&memItem{key: []byte("bb2"), val: []byte("b-second")},
			&memItem{key: []byte("cc1"), val: []byte("c-first")},
		)

		var results []string
		for entry, err := range iterator.BuildPrefixIterator(iter, decodeStringKey, reconstructString, keyPrefix) {
			require.NoError(t, err)
			val, err := entry.Value()
			require.NoError(t, err)
			results = append(results, val)
		}
		assert.Equal(t, []string{"a-first", "b-first", "c-first"}, results)
	})

	t.Run("empty iterator yields nothing", func(t *testing.T) {
		iter := newMemIterator()

		var count int
		for _, err := range iterator.BuildPrefixIterator(iter, decodeStringKey, reconstructString, keyPrefix) {
			require.NoError(t, err)
			count++
		}
		assert.Zero(t, count)
	})

	t.Run("single entry is yielded", func(t *testing.T) {
		iter := newMemIterator(
			&memItem{key: []byte("aa1"), val: []byte("only")},
		)

		var results []string
		for entry, err := range iterator.BuildPrefixIterator(iter, decodeStringKey, reconstructString, keyPrefix) {
			require.NoError(t, err)
			val, err := entry.Value()
			require.NoError(t, err)
			results = append(results, val)
		}
		assert.Equal(t, []string{"only"}, results)
	})

	t.Run("keyPrefix error is yielded", func(t *testing.T) {
		// key with only 1 byte triggers the "key too short" error
		iter := newMemIterator(
			&memItem{key: []byte("x"), val: []byte("val")},
		)

		for _, err := range iterator.BuildPrefixIterator(iter, decodeStringKey, reconstructString, keyPrefix) {
			require.Error(t, err)
			assert.Contains(t, err.Error(), "key too short")
			break
		}
	})

	t.Run("keyPrefix error on second entry still yields first", func(t *testing.T) {
		iter := newMemIterator(
			&memItem{key: []byte("aa1"), val: []byte("good")},
			&memItem{key: []byte("x"), val: []byte("bad")}, // triggers keyPrefix error
		)

		var results []string
		var gotErr error
		for entry, err := range iterator.BuildPrefixIterator(iter, decodeStringKey, reconstructString, keyPrefix) {
			if err != nil {
				gotErr = err
				break
			}
			val, err := entry.Value()
			require.NoError(t, err)
			results = append(results, val)
		}
		assert.Equal(t, []string{"good"}, results)
		require.Error(t, gotErr)
	})

	t.Run("decodeKey error is yielded", func(t *testing.T) {
		decodeErr := errors.New("decode error")
		failDecode := func([]byte) (string, error) { return "", decodeErr }

		iter := newMemIterator(
			&memItem{key: []byte("aa1"), val: []byte("v1")},
		)

		for _, err := range iterator.BuildPrefixIterator(iter, failDecode, reconstructString, keyPrefix) {
			require.ErrorIs(t, err, decodeErr)
			break
		}
	})

	t.Run("reconstruct error is surfaced via Value", func(t *testing.T) {
		reconstructErr := errors.New("reconstruct error")
		failReconstruct := func(string, []byte) (*string, error) { return nil, reconstructErr }

		iter := newMemIterator(
			&memItem{key: []byte("aa1"), val: []byte("v1")},
		)

		for entry, err := range iterator.BuildPrefixIterator(iter, decodeStringKey, failReconstruct, keyPrefix) {
			require.NoError(t, err)
			_, err = entry.Value()
			require.ErrorIs(t, err, reconstructErr)
			break
		}
	})

	t.Run("early break stops iteration", func(t *testing.T) {
		iter := newMemIterator(
			&memItem{key: []byte("aa1"), val: []byte("first")},
			&memItem{key: []byte("bb1"), val: []byte("second")},
			&memItem{key: []byte("cc1"), val: []byte("third")},
		)

		var results []string
		for entry, err := range iterator.BuildPrefixIterator(iter, decodeStringKey, reconstructString, keyPrefix) {
			require.NoError(t, err)
			val, err := entry.Value()
			require.NoError(t, err)
			results = append(results, val)
			break // stop after first
		}
		assert.Equal(t, []string{"first"}, results)
	})
}
