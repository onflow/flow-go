package mtrie_test

import (
	"bytes"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/storage/ledger/mtrie"
	"github.com/dapperlabs/flow-go/storage/ledger/trie"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestSameInsert(t *testing.T) {
	trieHeight := 17 // should be key size (in bits) + 1
	fStore := mtrie.NewMForest(trieHeight)
	rootHash := fStore.GetEmptyRootHash()

	k1 := []byte([]uint8{uint8(53), uint8(74)})
	v1 := []byte{'A'}
	k2 := []byte([]uint8{uint8(116), uint8(129)})
	v2 := []byte{'B'}
	keys := [][]byte{k1, k2}
	values := [][]byte{v1, v2}
	rootHash, err := fStore.Update(keys, values, rootHash)
	require.NoError(t, err)
	k3 := []byte([]uint8{uint8(53), uint8(74)})
	v3 := []byte{'C'}
	keys = [][]byte{k3}
	values = [][]byte{v3}
	_, err = fStore.Update(keys, values, rootHash)
	require.NoError(t, err)
}

func TestForestAccuracy(t *testing.T) {
	trieHeight := 17 // should be key size (in bits) + 1
	experimentRep := 10
	fStore := mtrie.NewMForest(trieHeight)
	rootHash := fStore.GetEmptyRootHash()

	dbDir := unittest.TempDir(t)
	smt, err := trie.NewSMT(dbDir, trieHeight, 10, 100, experimentRep)
	require.NoError(t, err)
	defer func() {
		smt.SafeClose()
		os.RemoveAll(dbDir)
	}()

	emptyTree := trie.GetDefaultHashForHeight(trieHeight - 1)
	require.NoError(t, err)
	rootHashForSMT := emptyTree
	for e := 0; e < experimentRep; e++ {

		// insert some values to an empty trie
		keys := make([][]byte, 0)
		values := make([][]byte, 0)
		rand.Seed(time.Now().UnixNano())

		// rejection sampling
		numberOfKeys := rand.Intn(20) + 1
		keyValueMap := make(map[string][]byte)
		i := 0
		for i < numberOfKeys {
			key := make([]byte, 2)
			rand.Read(key)
			// deduplicate
			if _, found := keyValueMap[string(key)]; !found {
				keys = append(keys, key)
				value := make([]byte, 4)
				rand.Read(value)
				keyValueMap[string(key)] = value
				values = append(values, value)
				i++
			}
		}

		newRootHash, err := fStore.Update(keys, values, rootHash)
		require.NoError(t, err, "error commiting changes")
		rootHash = newRootHash

		// check values
		retKeys, retValues, err := fStore.Read(keys, rootHash)
		for i, k := range retKeys {
			require.True(t, bytes.Equal(keyValueMap[string(k)], retValues[i]))
		}

		// Test eqaulity to SMT
		newRootHashForSMT, err := smt.Update(keys, values, rootHashForSMT)
		rootHashForSMT = newRootHashForSMT
		require.True(t, bytes.Equal(newRootHashForSMT, newRootHash))

	}
}
