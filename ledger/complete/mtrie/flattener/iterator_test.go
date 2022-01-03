package flattener_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
)

func TestEmptyTrie(t *testing.T) {
	emptyTrie := trie.NewEmptyMTrie()

	itr := flattener.NewNodeIterator(emptyTrie)
	require.True(t, nil == itr.Value()) // initial iterator should return nil

	require.False(t, itr.Next())
	require.Equal(t, emptyTrie.RootNode(), itr.Value())
	require.Equal(t, emptyTrie.RootNode(), itr.Value()) // test that recalling twice has no problem
	require.False(t, itr.Next())
	require.True(t, nil == itr.Value())
}

func TestPopulatedTrie(t *testing.T) {
	emptyTrie := trie.NewEmptyMTrie()

	// key: 0000...
	p1 := utils.PathByUint8(1)
	v1 := utils.LightPayload8('A', 'a')

	// key: 0100....
	p2 := utils.PathByUint8(64)
	v2 := utils.LightPayload8('B', 'b')

	paths := []ledger.Path{p1, p2}
	payloads := []ledger.Payload{*v1, *v2}

	testTrie, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads, true)
	require.NoError(t, err)

	for itr := flattener.NewNodeIterator(testTrie); itr.Next(); {
		fmt.Println(itr.Value().FmtStr("", ""))
		fmt.Println()
	}

	itr := flattener.NewNodeIterator(testTrie)

	require.True(t, itr.Next())
	p1_leaf := itr.Value()
	require.Equal(t, p1, *p1_leaf.Path())
	require.Equal(t, v1, p1_leaf.Payload())

	require.True(t, itr.Next())
	p2_leaf := itr.Value()
	require.Equal(t, p2, *p2_leaf.Path())
	require.Equal(t, v2, p2_leaf.Payload())

	require.True(t, itr.Next())
	p_parent := itr.Value()
	require.Equal(t, p1_leaf, p_parent.LeftChild())
	require.Equal(t, p2_leaf, p_parent.RightChild())

	require.True(t, itr.Next())
	root := itr.Value()
	require.Equal(t, testTrie.RootNode(), root)
	require.Equal(t, p_parent, root.LeftChild())
	require.True(t, nil == root.RightChild())

	require.False(t, itr.Next())
	require.True(t, nil == itr.Value())
}
