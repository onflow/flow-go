package trie

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/storage/ledger/utils"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestRestart(t *testing.T) {

	dir, err := unittest.TempDBDir()
	require.Nil(t, err)

	db := unittest.LevelDBInDir(t, dir)

	trie, err := NewSMT(db, 8, cacheSize, 10, 100, 5)
	if err != nil {
		t.Fatalf("failed to initialize SMT instance: %s", err)
	}

	trie.database.NewBatch()

	key1 := make([]byte, 1)
	value1 := []byte{'a'}

	key2 := make([]byte, 1)
	value2 := []byte{'b'}
	utils.SetBit(key2, 5)

	keysA := [][]byte{key1}
	//keysB := [][]byte{key1, key2}

	valuesA := [][]byte{value1}
	valuesB := [][]byte{value2}

	rootEmpty := trie.GetRoot().value

	err = trie.Update(keysA, valuesA)
	require.NoError(t, err)

	rootA := trie.GetRoot().value

	err = trie.Update(keysA, valuesB)

	rootB := trie.GetRoot().value

	// check values
	readEmpty, _, err := trie.Read(keysA, true, rootEmpty)
	require.NoError(t, err)

	assert.Len(t, readEmpty, 1)
	assert.Nil(t, readEmpty[0])

	readA, _, err := trie.Read(keysA, true, rootA)
	require.NoError(t, err)

	assert.Len(t, readA, 1)
	assert.Equal(t, value1, readA[0])

	readB, _, err := trie.Read(keysA, true, rootB)
	require.NoError(t, err)

	assert.Len(t, readB, 1)
	assert.Equal(t, value2, readB[0])

	// close and start SMT with the same DB again
	closeAllDBs(t, trie)

	db = unittest.LevelDBInDir(t, dir)

	trie, err = NewSMT(db, 8, cacheSize, 10, 100, 5)
	if err != nil {
		t.Fatalf("failed to initialize SMT instance for seocnd time: %s", err)
	}

	newRoot := trie.root.value
	assert.Equal(t, rootB, newRoot)

	readEmpty, _, err = trie.Read(keysA, true, rootEmpty)
	require.NoError(t, err)

	assert.Len(t, readEmpty, 1)
	assert.Nil(t, readEmpty[0])

	readA, _, err = trie.Read(keysA, true, rootA)
	require.NoError(t, err)

	assert.Len(t, readA, 1)
	assert.Equal(t, value1, readA[0])

	readB, _, err = trie.Read(keysA, true, rootB)
	require.NoError(t, err)

	assert.Len(t, readB, 1)
	assert.Equal(t, value2, readB[0])

	closeAllDBs(t, trie)
}

func closeAllDBs(t *testing.T, smt *SMT) {
	err1, err2 := smt.database.SafeClose()
	require.NoError(t, err1)
	require.NoError(t, err2)
	for _, v := range smt.historicalStates {
		err1, err2 = v.SafeClose()
		require.NoError(t, err1)
		require.NoError(t, err2)
	}
}
