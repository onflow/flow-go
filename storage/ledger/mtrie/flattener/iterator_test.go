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

	p1 := []byte([]uint8{uint8(1)}) // key: 0000...
	k1 := []byte{'A'}
	v1 := []byte{'a'}
	p2 := []byte([]uint8{uint8(64)}) // key: 0100....
	k2 := []byte{'B'}
	v2 := []byte{'b'}
	paths := [][]byte{p1, p2}
	keys := [][]byte{k1, k2}
	values := [][]byte{v1, v2}

	testTrie, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, keys, values)
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
	p1_leaf := itr.Value()
	require.Equal(t, p1, p1_leaf.Path())
	require.Equal(t, k1, p1_leaf.Key())
	require.Equal(t, v1, p1_leaf.Value())

	require.True(t, itr.Next())
	p2_leaf := itr.Value()
	require.Equal(t, p2, p2_leaf.Path())
	require.Equal(t, k2, p2_leaf.Key())
	require.Equal(t, v2, p2_leaf.Value())

	require.True(t, itr.Next())
	p_parent := itr.Value()
	require.Equal(t, p1_leaf, p_parent.LeftChild())
	require.Equal(t, p2_leaf, p_parent.RigthChild())

	require.True(t, itr.Next())
	root := itr.Value()
	require.Equal(t, testTrie.RootNode(), root)
	require.Equal(t, p_parent, root.LeftChild())
	require.True(t, nil == root.RigthChild())

	require.False(t, itr.Next())
	require.True(t, nil == itr.Value())
}
