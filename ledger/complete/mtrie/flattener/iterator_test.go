package flattener_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/ledger"
	"github.com/dapperlabs/flow-go/ledger/common"
	"github.com/dapperlabs/flow-go/ledger/complete/mtrie/flattener"
	"github.com/dapperlabs/flow-go/ledger/complete/mtrie/trie"
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

	// key: 0000...
	p1 := common.OneBytePath(1)
	v1 := common.LightPayload8('A', 'a')

	// key: 0100....
	p2 := common.OneBytePath(64)
	v2 := common.LightPayload8('B', 'b')

	paths := []ledger.Path{p1, p2}
	payloads := []ledger.Payload{*v1, *v2}

	testTrie, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads)
	require.NoError(t, err)

	for itr := flattener.NewNodeIterator(testTrie); itr.Next(); {
		fmt.Println(itr.Value().FmtStr("", ""))
		fmt.Println()
	}

	itr := flattener.NewNodeIterator(testTrie)

	require.True(t, itr.Next())
	p1_leaf := itr.Value()
	require.Equal(t, p1, p1_leaf.Path())
	require.Equal(t, v1, p1_leaf.Payload())

	require.True(t, itr.Next())
	p2_leaf := itr.Value()
	require.Equal(t, p2, p2_leaf.Path())
	require.Equal(t, v2, p2_leaf.Payload())

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
