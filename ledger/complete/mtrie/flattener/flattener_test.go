package flattener_test

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/ledger"
	"github.com/dapperlabs/flow-go/ledger/common"
	"github.com/dapperlabs/flow-go/ledger/complete/mtrie"
	"github.com/dapperlabs/flow-go/ledger/complete/mtrie/flattener"
	"github.com/dapperlabs/flow-go/module/metrics"
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

	p1 := common.OneBytePath(1)
	v1 := common.LightPayload8('A', 'a')
	p2 := common.OneBytePath(2)
	v2 := common.LightPayload8('B', 'b')
	p3 := common.OneBytePath(130)
	v3 := common.LightPayload8('C', 'c')
	p4 := common.OneBytePath(131)
	v4 := common.LightPayload8('D', 'd')
	p5 := common.OneBytePath(132)
	v5 := common.LightPayload8('E', 'e')

	paths := []ledger.Path{p1, p2, p3, p4, p5}
	payloads := []ledger.Payload{*v1, *v2, *v3, *v4, *v5}
	newTrie, err := mForest.Update(rootHash, paths, payloads)
	require.NoError(t, err)
	rootHash = newTrie.RootHash()

	p6 := common.OneBytePath(133)
	v6 := common.LightPayload8('F', 'f')

	newTrie, err = mForest.Update(rootHash, []ledger.Path{p6}, []ledger.Payload{*v6})
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

	retPayloads, err := mForest.Read(rootHash, paths)
	require.NoError(t, err)
	newRetPayloads, err := newMForest.Read(rootHash, paths)
	require.NoError(t, err)
	for i := range paths {
		require.True(t, bytes.Equal(retPayloads[i].Encode(), newRetPayloads[i].Encode()))
	}
}
