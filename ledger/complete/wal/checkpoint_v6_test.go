package wal

import (
	"testing"

	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

func createTries() ([]*trie.MTrie, error) {
	tries := make([]*trie.MTrie, 0)
	for i := 0; i < 10; i++ {
		trie, err := randomMTrie()
		if err != nil {
			return nil, err
		}
		tries = append(tries, trie)
	}
	return tries, nil
}

func requireTriesEqual(t *testing.T, tries1, tries2 []*trie.MTrie) {
	require.Equal(t, len(tries1), len(tries2), "tries have different length")
	for i, expect := range tries1 {
		actual := tries2[i]
		require.True(t, expect.Equals(actual), "%v-th trie is different", i)
	}
}

func TestWriteAndReadCheckpoint(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries, err := createTries()
		// require.NoError(t, err)
		fileName := "checkpoint"
		logger := unittest.Logger()
		require.NoErrorf(t, StoreCheckpointV6(tries, dir, fileName, &logger), "fail to store checkpoint")
		decoded, err := ReadCheckpointV6(dir, fileName)
		require.NoErrorf(t, err, "fail to read checkpoint %v/%v", dir, fileName)
		requireTriesEqual(t, tries, decoded)
	})
}
