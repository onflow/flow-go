package flattener_test

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
)

func TestTrieStoreAndLoad(t *testing.T) {
	dir, err := ioutil.TempDir("", "test-mtrie-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	emptyTrie := trie.NewEmptyMTrie()
	require.NoError(t, err)

	p1 := utils.PathByUint8(1)
	v1 := utils.LightPayload8('A', 'a')
	p2 := utils.PathByUint8(2)
	v2 := utils.LightPayload8('B', 'b')
	p3 := utils.PathByUint8(130)
	v3 := utils.LightPayload8('C', 'c')
	p4 := utils.PathByUint8(131)
	v4 := utils.LightPayload8('D', 'd')
	p5 := utils.PathByUint8(132)
	v5 := utils.LightPayload8('E', 'e')

	paths := []ledger.Path{p1, p2, p3, p4, p5}
	payloads := []ledger.Payload{*v1, *v2, *v3, *v4, *v5}

	newTrie, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads, true)
	require.NoError(t, err)

	flattedTrie, err := flattener.FlattenTrie(newTrie)
	require.NoError(t, err)

	rebuiltTrie, err := flattener.RebuildTrie(flattedTrie)
	require.NoError(t, err)

	//tries are the same now
	assert.Equal(t, newTrie, rebuiltTrie)

	retPayloads := newTrie.UnsafeRead(paths)
	newRetPayloads := rebuiltTrie.UnsafeRead(paths)
	for i := range paths {
		require.True(t, retPayloads[i].Equals(newRetPayloads[i]))
	}
}
