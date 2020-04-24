package trie

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/storage/ledger/utils"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestRestart(t *testing.T) {

	dir := unittest.TempDBDir(t)

	trie, err := NewSMT(dir, 9, 10, 100, 5)
	if err != nil {
		t.Fatalf("failed to initialize SMT instance: %s", err)
	}
	defer func() {
		trie.SafeClose()
		os.RemoveAll(dir)
	}()

	key1 := make([]byte, 1)
	value1 := []byte{'a'}

	key2 := make([]byte, 1)
	value2 := []byte{'b'}
	utils.SetBit(key2, 5)

	keysA := [][]byte{key1}
	//keysB := [][]byte{key1, key2}

	valuesA := [][]byte{value1}
	valuesB := [][]byte{value2}

	emptyCom := newCommitment([]byte{}, GetDefaultHashForHeight(9-1))

	rootA, err := trie.Update(keysA, valuesA, emptyCom)
	require.NoError(t, err)

	rootB, err := trie.Update(keysA, valuesB, rootA)
	require.NoError(t, err)

	// check values
	readEmpty, _, err := trie.Read(keysA, true, emptyCom)
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
	trie.SafeClose()

	trie, err = NewSMT(dir, 9, 10, 100, 5)
	if err != nil {
		t.Fatalf("failed to initialize SMT instance for second time: %s", err)
	}

	readEmpty, _, err = trie.Read(keysA, true, emptyCom)
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

}

func TestRestartMultipleKeys(t *testing.T) {

	dir := unittest.TempDBDir(t)

	trie, err := NewSMT(dir, 9, 10, 100, 5)
	if err != nil {
		t.Fatalf("failed to initialize SMT instance: %s", err)
	}
	defer func() {
		trie.SafeClose()
		os.RemoveAll(dir)
	}()

	emptyCom := newCommitment([]byte{}, GetDefaultHashForHeight(9-1))

	numKeys := 10
	keys := make([][]byte, numKeys)
	values := make([][]byte, numKeys)
	valuesStart := byte('a')

	for i := 0; i < numKeys; i++ {
		keys[i] = make([]byte, 1)
		keys[i][0] = byte(i)

		values[i] = []byte{valuesStart}
		valuesStart++
	}

	newCom, err := trie.Update(keys, values, emptyCom)
	require.NoError(t, err)

	// close and start SMT with the same DB again
	trie.SafeClose()

	trie, err = NewSMT(dir, 9, 10, 100, 5)
	if err != nil {
		t.Fatalf("failed to initialize SMT instance for second time: %s", err)
	}

	readValues, _, err := trie.Read(keys, false, newCom)
	require.NoError(t, err)
	assert.Equal(t, values, readValues)
}

//func closeAllDBs(t *testing.T, smt *SMT) {
//	err1, err2 := smt.database.SafeClose()
//	require.NoError(t, err1)
//	require.NoError(t, err2)
//	for _, v := range smt.historicalStates {
//		err1, err2 = v.SafeClose()
//		require.NoError(t, err1)
//		require.NoError(t, err2)
//	}
//}
