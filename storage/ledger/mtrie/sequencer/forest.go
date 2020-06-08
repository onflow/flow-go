package sequencer

import (
	"encoding/hex"
	"fmt"

	"github.com/dapperlabs/flow-go/storage/ledger/mtrie"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/node"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/trie"
)

// MForestSerialization is an sequence of serialized forest nodes.
// Consists of:
// a list of storable nodes, where references to nodes are replaced by index in the slice
// and a list of storable tries, referencing root node by index.
// 0 is a special index, meaning nil, but is included in this list for ease of use
// and removing would make it necessary to constantly add/subtract indexes
type MForestSequencing struct {
	Nodes []*StorableNode
	Tries []*StorableTrie
}

// node2indexMap maps a node pointer to the node index in the serialization
type node2indexMap map[*node.Node]uint64

// SequenceForest returns forest MForestSequencing, which contains all nodes and tries of the MForest.
func SequenceForest(f *mtrie.MForest) (*MForestSequencing, error) {
	tries, err := f.GetTries()
	if err != nil {
		return nil, fmt.Errorf("cannot get cached tries root hashes: %w", err)
	}

	storableTries := make([]*StorableTrie, 0, len(tries))
	storableNodes := []*StorableNode{nil} // 0th element is nil

	// assign unique value to every node
	allNodes := make(node2indexMap)
	allNodes[nil] = 0 // 0th element is nil

	counter := uint64(1) // start from 1, as 0 marks nil
	for _, t := range tries {
		itr := t.NewNodeIterator()
		for n, hasNext := itr.Next(); hasNext; n, hasNext = itr.Next() {
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

		//fix root nodes indices
		// since we indexed all nodes, root must be present
		storableTrie, err := toStorableTrie(t, allNodes)
		if err != nil {
			return nil, fmt.Errorf("failed to construct storable trie: %w", err)
		}
		storableTries = append(storableTries, storableTrie)
	}

	return &MForestSequencing{
		Nodes: storableNodes,
		Tries: storableTries,
	}, nil
}

func toStorableNode(node *node.Node, indexForNode node2indexMap) (*StorableNode, error) {
	leftIndex, found := indexForNode[node.LeftChild()]
	if !found {
		return nil, fmt.Errorf("internal error: missing node with hash %s", hex.EncodeToString(node.LeftChild().Hash()))
	}
	rightIndex, found := indexForNode[node.RigthChild()]
	if !found {
		return nil, fmt.Errorf("internal error: missing node with hash %s", hex.EncodeToString(node.RigthChild().Hash()))
	}

	storableNode := &StorableNode{
		LIndex:    leftIndex,
		RIndex:    rightIndex,
		Height:    uint16(node.Height()),
		Key:       node.Key(),
		Value:     node.Value(),
		HashValue: node.Hash(),
	}
	return storableNode, nil
}

func toStorableTrie(mtrie *trie.MTrie, indexForNode node2indexMap) (*StorableTrie, error) {
	rootIndex, found := indexForNode[mtrie.RootNode()]
	if !found {
		return nil, fmt.Errorf("internal error: missing node with hash %s", hex.EncodeToString(mtrie.RootNode().Hash()))
	}
	storableTrie := &StorableTrie{
		RootIndex:      rootIndex,
		Number:         mtrie.Number(),
		RootHash:       mtrie.RootHash(),
		ParentRootHash: mtrie.ParentRootHash(),
	}

	return storableTrie, nil
}

func RebuildTries(forestSequencing *MForestSequencing) ([]*trie.MTrie, error) {
	tries := make([]*trie.MTrie, 0, len(forestSequencing.Tries))
	nodes, err := RebuildNodes(forestSequencing.Nodes)
	if err != nil {
		return nil, fmt.Errorf("reconstructing nodes from storables failed: %w", err)
	}

	//restore tries
	for _, storableTrie := range forestSequencing.Tries {
		mtrie, err := trie.NewMTrie(
			nodes[storableTrie.RootIndex],
			storableTrie.Number,
			storableTrie.ParentRootHash,
		)
		if err != nil {
			return nil, fmt.Errorf("restoring trie failed: %w", err)
		}
		tries = append(tries, mtrie)
	}
	return tries, nil
}

// RebuildFromStorables generates a list of nodes from a sequence of StorableNodes.
// The sequence must obey the DESCENDANTS-FIRST-RELATIONSHIP
func RebuildNodes(storableNodes []*StorableNode) ([]*node.Node, error) {
	nodes := make([]*node.Node, 0, len(storableNodes))
	for i, snode := range storableNodes {
		if snode == nil {
			nodes = append(nodes, nil)
			continue
		}
		if (snode.LIndex >= uint64(i)) || (snode.RIndex >= uint64(i)) {
			return nil, fmt.Errorf("sequence of StorableNodes does not satisfy Descendents-First-Relationship")
		}
		nodes = append(nodes, node.NewNode(int(snode.Height), nodes[snode.LIndex], nodes[snode.RIndex], snode.Key, snode.Value, snode.HashValue))
	}
	return nodes, nil
}
