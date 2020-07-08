package flattener_test

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/ledger/outright/mtrie"

	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/ledger/outright/mtrie/flattener"
)

func TestForestStoreAndLoad(t *testing.T) {
	pathByteSize := 1
	dir, err := ioutil.TempDir("", "test-mtrie-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	metricsCollector := &metrics.NoopCollector{}
	mForest, err := mtrie.NewMForest(pathByteSize, dir, 5, metricsCollector, nil)
	require.NoError(t, err)
	rootHash := mForest.GetEmptyRootHash()

	p1 := []byte([]uint8{uint8(1)})
	k1 := []byte{'A'}
	v1 := []byte{'a'}
	p2 := []byte([]uint8{uint8(2)})
	k2 := []byte{'B'}
	v2 := []byte{'b'}
	p3 := []byte([]uint8{uint8(130)})
	k3 := []byte{'C'}
	v3 := []byte{'c'}
	p4 := []byte([]uint8{uint8(131)})
	k4 := []byte{'D'}
	v4 := []byte{'d'}
	p5 := []byte([]uint8{uint8(132)})
	k5 := []byte{'E'}
	v5 := []byte{'e'}

	paths := [][]byte{p1, p2, p3, p4, p5}
	keys := [][]byte{k1, k2, k3, k4, k5}
	values := [][]byte{v1, v2, v3, v4, v5}
	newTrie, err := mForest.Update(rootHash, paths, keys, values)
	require.NoError(t, err)
	rootHash = newTrie.RootHash()

	p6 := []byte([]uint8{uint8(133)})
	k6 := []byte{'F'}
	v6 := []byte{'f'}

	newTrie, err = mForest.Update(rootHash, [][]byte{p6}, [][]byte{k6}, [][]byte{v6})
	require.NoError(t, err)
	rootHash = newTrie.RootHash()

	file, err := ioutil.TempFile("", "flow-mtrie-load")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(file.Name())

	forestSequencing, err := flattener.FlattenForest(mForest)
	require.NoError(t, err)

	newMForest, err := mtrie.NewMForest(pathByteSize, dir, 5, metricsCollector, nil)
	require.NoError(t, err)

	//forests are different
	assert.NotEqual(t, mForest, newMForest)

	rebuiltTries, err := flattener.RebuildTries(forestSequencing)
	require.NoError(t, err)
	err = newMForest.AddTries(rebuiltTries)
	require.NoError(t, err)

	//forests are the same now
	assert.Equal(t, *mForest, *newMForest)

	retValues, err := mForest.Read(rootHash, paths)
	require.NoError(t, err)
	newRetValues, err := newMForest.Read(rootHash, paths)
	require.NoError(t, err)
	for i := range paths {
		require.True(t, bytes.Equal(values[i], retValues[i]))
		require.True(t, bytes.Equal(values[i], newRetValues[i]))
	}
}
