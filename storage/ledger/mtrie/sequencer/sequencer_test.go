package sequencer_test

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/storage/ledger/mtrie"

	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/sequencer"
)

func TestForestStoreAndLoad(t *testing.T) {
	trieHeight := 9
	dir, err := ioutil.TempDir("", "test-mtrie-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	metricsCollector := &metrics.NoopCollector{}
	mForest, err := mtrie.NewMForest(trieHeight, dir, 5, metricsCollector, nil)
	require.NoError(t, err)
	rootHash := mForest.GetEmptyRootHash()

	k1 := []byte([]uint8{uint8(1)})
	v1 := []byte{'A'}
	k2 := []byte([]uint8{uint8(2)})
	v2 := []byte{'B'}
	k3 := []byte([]uint8{uint8(130)})
	v3 := []byte{'C'}
	k4 := []byte([]uint8{uint8(131)})
	v4 := []byte{'D'}
	k5 := []byte([]uint8{uint8(132)})
	v5 := []byte{'E'}

	keys := [][]byte{k1, k2, k3, k4, k5}
	values := [][]byte{v1, v2, v3, v4, v5}
	newTrie, err := mForest.Update(rootHash, keys, values)
	require.NoError(t, err)
	rootHash = newTrie.RootHash()

	k6 := []byte([]uint8{uint8(133)})
	v6 := []byte{'F'}

	keys6 := [][]byte{k6}
	values6 := [][]byte{v6}
	newTrie, err = mForest.Update(rootHash, keys6, values6)
	require.NoError(t, err)
	rootHash = newTrie.RootHash()

	file, err := ioutil.TempFile("", "flow-mtrie-load")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(file.Name())

	forestSequencing, err := sequencer.SequenceForest(mForest)
	require.NoError(t, err)

	newMForest, err := mtrie.NewMForest(trieHeight, dir, 5, metricsCollector, nil)
	require.NoError(t, err)

	//forests are different
	assert.NotEqual(t, mForest, newMForest)

	rebuiltTries, err := sequencer.RebuildTries(forestSequencing)
	require.NoError(t, err)
	err = newMForest.AddTries(rebuiltTries)
	require.NoError(t, err)

	//forests are the same now
	assert.Equal(t, *mForest, *newMForest)

	retValues, err := mForest.Read(rootHash, keys)
	require.NoError(t, err)
	newRetValues, err := newMForest.Read(rootHash, keys)
	require.NoError(t, err)
	for i := range keys {
		require.True(t, bytes.Equal(values[i], retValues[i]))
		require.True(t, bytes.Equal(values[i], newRetValues[i]))
	}
}
