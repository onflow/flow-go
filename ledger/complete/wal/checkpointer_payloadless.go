package wal

import (
	"encoding/hex"
	"fmt"
	"io"

	"github.com/onflow/flow-go/ledger/complete/payloadless"
)

// storeUniquePayloadlessNodes traverses the trie rooted at `root` in
// Descendents-First order, emits each previously-unvisited node to writer via
// [payloadless.EncodeNode], and assigns it a monotonically-increasing index in
// `visitedNodes`. nodeCounter is the next index to assign on entry; the updated
// counter is returned.
func storeUniquePayloadlessNodes(
	root *payloadless.Node,
	visitedNodes map[*payloadless.Node]uint64,
	nodeCounter uint64,
	scratch []byte,
	writer io.Writer,
	nodeCounterUpdated func(nodeCounter uint64),
) (uint64, error) {
	for itr := payloadless.NewUniqueNodeIterator(root, visitedNodes); itr.Next(); {
		n := itr.Value()
		visitedNodes[n] = nodeCounter
		nodeCounter++
		nodeCounterUpdated(nodeCounter)

		var lchildIndex, rchildIndex uint64
		if lchild := n.LeftChild(); lchild != nil {
			idx, found := visitedNodes[lchild]
			if !found {
				h := lchild.Hash()
				return 0, fmt.Errorf("internal error: missing payloadless node with hash %s", hex.EncodeToString(h[:]))
			}
			lchildIndex = idx
		}
		if rchild := n.RightChild(); rchild != nil {
			idx, found := visitedNodes[rchild]
			if !found {
				h := rchild.Hash()
				return 0, fmt.Errorf("internal error: missing payloadless node with hash %s", hex.EncodeToString(h[:]))
			}
			rchildIndex = idx
		}

		encNode := payloadless.EncodeNode(n, lchildIndex, rchildIndex, scratch)
		if _, err := writer.Write(encNode); err != nil {
			return 0, fmt.Errorf("cannot serialize payloadless node: %w", err)
		}
	}
	return nodeCounter, nil
}

// getPayloadlessNodesAtLevel returns the 2^level nodes at depth `level` of a
// payloadless trie in breadth-first order. Positions with no node are nil; the
// returned slice always has length 2^level.
func getPayloadlessNodesAtLevel(root *payloadless.Node, level uint) []*payloadless.Node {
	nodes := []*payloadless.Node{root}
	nodesLevel := uint(0)
	for nodesLevel < level {
		nextLevel := nodesLevel + 1
		nodesAtNextLevel := make([]*payloadless.Node, 1<<nextLevel)
		for i, n := range nodes {
			if n != nil {
				nodesAtNextLevel[i*2] = n.LeftChild()
				nodesAtNextLevel[i*2+1] = n.RightChild()
			}
		}
		nodes = nodesAtNextLevel
		nodesLevel = nextLevel
	}
	return nodes
}
