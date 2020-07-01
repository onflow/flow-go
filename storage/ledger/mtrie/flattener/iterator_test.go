package flattener_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/flattener"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/trie"
)

func TestEmptyTrie(t *testing.T) {
	emptyTrie, err := trie.NewEmptyMTrie(1)
	require.NoError(t, err)

	itr := flattener.NewNodeIterator(emptyTrie)
	require.True(t, nil == itr.Value()) // initial iterator should return nil

	require.True(t, itr.Next())
	require.Equal(t, emptyTrie.RootNode(), itr.Value())
	require.Equal(t, emptyTrie.RootNode(), itr.Value()) // test that recalling twice has no problem
	require.False(t, itr.Next())
	require.True(t, nil == itr.Value())
}

func TestPopulatedTrie(t *testing.T) {
	emptyTrie, err := trie.NewEmptyMTrie(1)
	require.NoError(t, err)

	k1 := []byte([]uint8{uint8(1)}) // key: 0000...
	v1 := []byte{'A'}
	k2 := []byte([]uint8{uint8(64)}) // key: 0100....
	v2 := []byte{'B'}
	keys := [][]byte{k1, k2}
	values := [][]byte{v1, v2}

	testTrie, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, keys, values)
	require.NoError(t, err)
	fmt.Println("BASE TRIE:")
	fmt.Println(testTrie.String())
	fmt.Println("================================")

	for itr := flattener.NewNodeIterator(testTrie); itr.Next(); {
		fmt.Println(itr.Value().FmtStr("", ""))
		fmt.Println()
	}

	itr := flattener.NewNodeIterator(testTrie)

	require.True(t, itr.Next())
	k1_leaf := itr.Value()
	require.Equal(t, k1, k1_leaf.Key())
	require.Equal(t, v1, k1_leaf.Value())

	require.True(t, itr.Next())
	k2_leaf := itr.Value()
	require.Equal(t, k2, k2_leaf.Key())
	require.Equal(t, v2, k2_leaf.Value())

	require.True(t, itr.Next())
	k_parent := itr.Value()
	require.Equal(t, k1_leaf, k_parent.LeftChild())
	require.Equal(t, k2_leaf, k_parent.RigthChild())

	require.True(t, itr.Next())
	root := itr.Value()
	require.Equal(t, testTrie.RootNode(), root)
	require.Equal(t, k_parent, root.LeftChild())
	require.True(t, nil == root.RigthChild())

	require.False(t, itr.Next())
	require.True(t, nil == itr.Value())
}
