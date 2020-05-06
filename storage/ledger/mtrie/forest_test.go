package mtrie_test

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/storage/ledger/mtrie"
)

func TestForest(t *testing.T) {
	trieHeight := 17 // should be key size (in bits) + 1
	experimentRep := 20
	fStore := mtrie.NewMForest(trieHeight)
	rootHash := fStore.GetEmptyRootHash()
	for e := 0; e < experimentRep; e++ {

		// insert some values to an empty trie
		keys := make([][]byte, 0)
		values := make([][]byte, 0)
		rand.Seed(time.Now().UnixNano())

		// rejection sampling
		numberOfKeys := rand.Intn(3) + 1
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
		fmt.Println(newRootHash)
		require.NoError(t, err, "error commiting changes")
		rootHash = newRootHash

		retKeys, retValues, err := fStore.Read(keys, rootHash)
		for i, k := range retKeys {
			require.True(t, bytes.Equal(keyValueMap[string(k)], retValues[i]))
		}
	}
}
