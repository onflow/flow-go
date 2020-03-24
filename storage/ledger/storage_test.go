package ledger

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/ledger/databases/leveldb"
	"github.com/dapperlabs/flow-go/storage/ledger/utils"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestNewTrieStorage(t *testing.T) {
	unittest.RunWithLevelDB(t, func(db *leveldb.LevelDB) {
		_, err := NewTrieStorage(db)
		assert.NoError(t, err)
	})
}

func TestTrieStorage_UpdateRegisters(t *testing.T) {
	t.Run("mismatched IDs and values", func(t *testing.T) {
		unittest.RunWithLevelDB(t, func(db *leveldb.LevelDB) {
			f, err := NewTrieStorage(db)
			require.NoError(t, err)

			ids, values := makeTestValues()

			// add extra id but not value
			ids = append(ids, flow.RegisterID{42})

			currentRoot := f.tree.GetRoot().GetValue()

			_, err = f.UpdateRegisters(ids, values)
			assert.Error(t, err)

			newRoot := f.tree.GetRoot().GetValue()

			// root should not change
			assert.Equal(t, currentRoot, newRoot)
		})
	})

	t.Run("empty update", func(t *testing.T) {
		unittest.RunWithLevelDB(t, func(db *leveldb.LevelDB) {
			f, err := NewTrieStorage(db)
			require.NoError(t, err)

			// create empty values
			ids := []flow.RegisterID{}
			values := []flow.RegisterValue{}

			currentRoot := f.tree.GetRoot().GetValue()

			newRoot, err := f.UpdateRegisters(ids, values)
			require.NoError(t, err)

			// root should not change
			assert.Equal(t, currentRoot, newRoot)
		})
	})

	t.Run("non-empty update", func(t *testing.T) {
		unittest.RunWithLevelDB(t, func(db *leveldb.LevelDB) {
			f, err := NewTrieStorage(db)
			require.NoError(t, err)

			ids, values := makeTestValues()

			currentRoot := f.tree.GetRoot().GetValue()

			newRoot, err := f.UpdateRegisters(ids, values)
			require.NoError(t, err)
			assert.Equal(t, f.tree.GetRoot().GetValue(), newRoot)

			newValues, err := f.GetRegisters(ids, newRoot)
			require.NoError(t, err)

			assert.Equal(t, values, newValues)
			assert.NotEqual(t, currentRoot, newRoot)
		})
	})
}

func TestTrieStorage_UpdateRegistersWithProof(t *testing.T) {
	t.Run("mismatched IDs and values", func(t *testing.T) {
		unittest.RunWithLevelDB(t, func(db *leveldb.LevelDB) {
			f, err := NewTrieStorage(db)
			require.NoError(t, err)

			ids, values := makeTestValues()

			// add extra id but not value
			ids = append(ids, flow.RegisterID{42})

			currentRoot := f.tree.GetRoot().GetValue()

			_, _, err = f.UpdateRegistersWithProof(ids, values)
			assert.Error(t, err)

			newRoot := f.tree.GetRoot().GetValue()

			// root should not change
			assert.Equal(t, currentRoot, newRoot)
		})
	})

	t.Run("empty update", func(t *testing.T) {
		unittest.RunWithLevelDB(t, func(db *leveldb.LevelDB) {
			f, err := NewTrieStorage(db)
			require.NoError(t, err)

			currentRoot := f.tree.GetRoot().GetValue()

			// create empty values
			ids := []flow.RegisterID{}
			values := []flow.RegisterValue{}

			newRoot, _, err := f.UpdateRegistersWithProof(ids, values)
			require.NoError(t, err)

			// root should not change
			assert.Equal(t, currentRoot, newRoot)
		})
	})

	t.Run("non-empty update", func(t *testing.T) {
		unittest.RunWithLevelDB(t, func(db *leveldb.LevelDB) {
			f, err := NewTrieStorage(db)
			require.NoError(t, err)

			ids, values := makeTestValues()

			currentRoot := f.tree.GetRoot().GetValue()

			newRoot, _, err := f.UpdateRegistersWithProof(ids, values)
			require.NoError(t, err)
			assert.Equal(t, f.tree.GetRoot().GetValue(), newRoot)

			newValues, _, err := f.GetRegistersWithProof(ids, newRoot)
			require.NoError(t, err)

			assert.Equal(t, values, newValues)
			assert.NotEqual(t, currentRoot, newRoot)
		})
	})
}

func makeTestValues() ([][]byte, [][]byte) {
	id1 := make([]byte, 32)
	value1 := []byte{'a'}

	id2 := make([]byte, 32)
	value2 := []byte{'b'}
	utils.SetBit(id2, 5)

	id3 := make([]byte, 32)
	value3 := []byte{'c'}
	utils.SetBit(id3, 0)

	id4 := make([]byte, 32)
	value4 := []byte{'d'}
	utils.SetBit(id4, 0)
	utils.SetBit(id4, 5)

	ids := make([][]byte, 0)
	values := make([][]byte, 0)

	ids = append(ids, id1, id2, id3, id4)
	values = append(values, value1, value2, value3, value4)

	return ids, values
}
