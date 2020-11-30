package flattener

import (
	"fmt"

	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
)

// FlattenedTrie is similar to FlattenedForest except only including a single trie
type FlattenedTrie struct {
	Nodes []*StorableNode
	Trie  *StorableTrie
}

// ToFlattenedForestWithASingleTrie converts the flattenedTrie into a FlattenedForest with only one trie included
func (ft *FlattenedTrie) ToFlattenedForestWithASingleTrie() *FlattenedForest {
	storableTries := make([]*StorableTrie, 1)
	storableTries[0] = ft.Trie
	return &FlattenedForest{
		Nodes: ft.Nodes,
		Tries: storableTries,
	}
}

// FlattenTrie returns the trie as a FlattenedTrie, which contains all nodes of that trie.
func FlattenTrie(trie *trie.MTrie) (*FlattenedTrie, error) {
	storableNodes := []*StorableNode{nil} // 0th element is nil

	// assign unique value to every node
	allNodes := make(map[*node.Node]uint64)
	allNodes[nil] = 0 // 0th element is nil

	counter := uint64(1) // start from 1, as 0 marks nil
	for itr := NewNodeIterator(trie); itr.Next(); {
		n := itr.Value()
		// if node not in map
		if _, has := allNodes[n]; !has {
			allNodes[n] = counter
			counter++
			storableNode, err := toStorableNode(n, allNodes)
			if err != nil {
				return nil, fmt.Errorf("failed to construct storable node: %w", err)
			}
			storableNodes = append(storableNodes, storableNode)
		}
	}
	// fix root nodes indices
	// since we indexed all nodes, root must be present
	storableTrie, err := toStorableTrie(trie, allNodes)
	if err != nil {
		return nil, fmt.Errorf("failed to construct storable trie: %w", err)
	}

	return &FlattenedTrie{
		Nodes: storableNodes,
		Trie:  storableTrie,
	}, nil
}

// RebuildTrie construct a trie from a storable FlattenedForest
func RebuildTrie(flatTrie *FlattenedTrie) (*trie.MTrie, error) {
	nodes, err := RebuildNodes(flatTrie.Nodes)
	if err != nil {
		return nil, fmt.Errorf("reconstructing nodes from storables failed: %w", err)
	}

	//restore tries
	// TODO - indices here
	mtrie, err := trie.NewMTrie(nodes[flatTrie.Trie.RootIndex], nil, nil)
	if err != nil {
		return nil, fmt.Errorf("restoring trie failed: %w", err)
	}
	return mtrie, nil
}
