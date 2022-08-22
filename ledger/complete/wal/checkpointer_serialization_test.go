package wal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
)

func TestGetNodesAtLevel(t *testing.T) {

	t.Run("nil node", func(t *testing.T) {
		n := (*node.Node)(nil)
		for level := uint(0); level < 6; level++ {
			nodes := getNodesAtLevel(n, level)
			assert.Equal(t, 1<<level, len(nodes))
			for _, nAtLevel := range nodes {
				require.Nil(t, nAtLevel)
			}
		}
	})

	t.Run("leaf node", func(t *testing.T) {
		path := testutils.PathByUint8(1)
		payload := testutils.LightPayload8('A', 'a')
		hashValue := hash.Hash([32]byte{1, 1, 1})
		leafNode := node.NewNode(255, nil, nil, path, payload, hashValue)

		nodes := getNodesAtLevel(leafNode, 0)
		require.Equal(t, []*node.Node{leafNode}, nodes)

		for level := uint(1); level < 6; level++ {
			nodes := getNodesAtLevel(leafNode, level)
			require.Equal(t, 1<<level, len(nodes))
			for _, nAtLevel := range nodes {
				require.Nil(t, nAtLevel)
			}
		}
	})

	t.Run("interim node with left child", func(t *testing.T) {
		emptyTrie := trie.NewEmptyMTrie()

		// key: 0000...
		p1 := testutils.PathByUint8(0)
		v1 := testutils.LightPayload8('A', 'a')

		// key: 0100....
		p2 := testutils.PathByUint8(64)
		v2 := testutils.LightPayload8('B', 'b')

		paths := []ledger.Path{p1, p2}
		payloads := []ledger.Payload{*v1, *v2}

		updatedTrie, _, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads, true)
		require.NoError(t, err)

		//              n4
		//             /
		//            /
		//          n3
		//        /     \
		//      /         \
		//   n1 (p1/v1)     n2 (p2/v2)

		rootNode := updatedTrie.RootNode()

		nodes := getNodesAtLevel(rootNode, 0)
		require.Equal(t, []*node.Node{updatedTrie.RootNode()}, nodes)

		nodes = getNodesAtLevel(rootNode, 1)
		require.Equal(t, []*node.Node{rootNode.LeftChild(), nil}, nodes)

		nodes = getNodesAtLevel(rootNode, 2)
		require.Equal(t, []*node.Node{rootNode.LeftChild().LeftChild(), rootNode.LeftChild().RightChild(), nil, nil}, nodes)

		for level := uint(3); level < 6; level++ {
			nodes := getNodesAtLevel(rootNode, level)
			require.Equal(t, 1<<level, len(nodes))
			for _, nAtLevel := range nodes {
				require.Nil(t, nAtLevel)
			}
		}
	})

	t.Run("interim node with right child", func(t *testing.T) {
		emptyTrie := trie.NewEmptyMTrie()

		// key: 1000...
		p3 := testutils.PathByUint8(128)
		v3 := testutils.LightPayload8('C', 'c')

		// key: 1100....
		p4 := testutils.PathByUint8(192)
		v4 := testutils.LightPayload8('D', 'd')

		paths := []ledger.Path{p3, p4}
		payloads := []ledger.Payload{*v3, *v4}

		updatedTrie, _, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads, true)
		require.NoError(t, err)

		//              n8
		//                 \
		//                   \
		//                   n7
		//                  /   \
		//                /       \
		//              n5         n6
		//            (p3/v3)    (p4/v4)

		rootNode := updatedTrie.RootNode()

		nodes := getNodesAtLevel(rootNode, 0)
		require.Equal(t, []*node.Node{rootNode}, nodes)

		nodes = getNodesAtLevel(rootNode, 1)
		require.Equal(t, []*node.Node{nil, rootNode.RightChild()}, nodes)

		nodes = getNodesAtLevel(rootNode, 2)
		require.Equal(t, []*node.Node{nil, nil, rootNode.RightChild().LeftChild(), rootNode.RightChild().RightChild()}, nodes)

		for level := uint(3); level < 6; level++ {
			nodes := getNodesAtLevel(rootNode, level)
			require.Equal(t, 1<<level, len(nodes))
			for _, nAtLevel := range nodes {
				require.Nil(t, nAtLevel)
			}
		}
	})

	t.Run("interim node with 2 children", func(t *testing.T) {
		emptyTrie := trie.NewEmptyMTrie()

		// key: 0000...
		p1 := testutils.PathByUint8(0)
		v1 := testutils.LightPayload8('A', 'a')

		// key: 0100....
		p2 := testutils.PathByUint8(64)
		v2 := testutils.LightPayload8('B', 'b')

		// key: 1000...
		p3 := testutils.PathByUint8(128)
		v3 := testutils.LightPayload8('C', 'c')

		// key: 1100....
		p4 := testutils.PathByUint8(192)
		v4 := testutils.LightPayload8('D', 'd')

		paths := []ledger.Path{p1, p2, p3, p4}
		payloads := []ledger.Payload{*v1, *v2, *v3, *v4}

		updatedTrie, _, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads, true)
		require.NoError(t, err)

		//                n4
		//             /      \
		//            /        \
		//          n3           n5
		//        /   \          /  \
		//      /      \        /     \
		//   n1        n2      n6      n7
		//  (p1)      (p2)    (p3)    (p4)

		rootNode := updatedTrie.RootNode()

		nodes := getNodesAtLevel(rootNode, 0)
		require.Equal(t, []*node.Node{rootNode}, nodes)

		nodes = getNodesAtLevel(rootNode, 1)
		require.Equal(t, []*node.Node{rootNode.LeftChild(), rootNode.RightChild()}, nodes)

		nodes = getNodesAtLevel(rootNode, 2)
		require.Equal(t, []*node.Node{rootNode.LeftChild().LeftChild(), rootNode.LeftChild().RightChild(), rootNode.RightChild().LeftChild(), rootNode.RightChild().RightChild()}, nodes)

		for level := uint(3); level < 6; level++ {
			nodes := getNodesAtLevel(rootNode, level)
			require.Equal(t, 1<<level, len(nodes))
			for _, nAtLevel := range nodes {
				require.Nil(t, nAtLevel)
			}
		}
	})

	t.Run("sparse trie with 8 levels", func(t *testing.T) {

		emptyTrie := trie.NewEmptyMTrie()

		// key: 0000 0000
		p1 := testutils.PathByUint8(0)
		v1 := testutils.LightPayload8('A', 'a')

		// key: 0000 0001
		p2 := testutils.PathByUint8(1)
		v2 := testutils.LightPayload8('B', 'b')

		// key: 1111 1110
		p3 := testutils.PathByUint8(254)
		v3 := testutils.LightPayload8('C', 'c')

		// key: 1111 1111
		p4 := testutils.PathByUint8(255)
		v4 := testutils.LightPayload8('D', 'd')

		paths := []ledger.Path{p1, p2, p3, p4}
		payloads := []ledger.Payload{*v1, *v2, *v3, *v4}

		updatedTrie, _, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads, true)
		require.NoError(t, err)

		//                             n0
		//                           /    \
		//                         n1a    n1b
		//                         /        \
		//                       n2a        n2b
		//                       /            \
		//                      n3a           n3b
		//                      /               \
		//                     n4a              n4b
		//                     /                  \
		//                    n5a                 n5b
		//                    /                     \
		//                   n6a                    n6b
		//                   /                        \
		//                  n7a                       n7b
		//                 /   \                      /  \
		//               n8a   n8b                  n8d   n8d
		//              (p1)   (p2)                (p3)   (p4)

		n0 := updatedTrie.RootNode()

		n1a := n0.LeftChild()
		require.NotNil(t, n1a)
		n1b := n0.RightChild()
		require.NotNil(t, n1b)

		n2a := n1a.LeftChild()
		require.NotNil(t, n2a)
		n2b := n1b.RightChild()
		require.NotNil(t, n2b)

		n3a := n2a.LeftChild()
		require.NotNil(t, n3a)
		n3b := n2b.RightChild()
		require.NotNil(t, n3b)

		n4a := n3a.LeftChild()
		require.NotNil(t, n4a)
		n4b := n3b.RightChild()
		require.NotNil(t, n4b)

		n5a := n4a.LeftChild()
		require.NotNil(t, n5a)
		n5b := n4b.RightChild()
		require.NotNil(t, n5b)

		n6a := n5a.LeftChild()
		require.NotNil(t, n6a)
		n6b := n5b.RightChild()
		require.NotNil(t, n6b)

		n7a := n6a.LeftChild()
		require.NotNil(t, n7a)
		n7b := n6b.RightChild()
		require.NotNil(t, n7b)

		n8a := n7a.LeftChild()
		require.NotNil(t, n8a)
		n8b := n7a.RightChild()
		require.NotNil(t, n8b)
		n8c := n7b.LeftChild()
		require.NotNil(t, n8c)
		n8d := n7b.RightChild()
		require.NotNil(t, n8d)

		testcases := []struct {
			level         uint
			expectedNodes []*node.Node
		}{
			{0, []*node.Node{n0}},
			{1, []*node.Node{n1a, n1b}},
			{2, []*node.Node{0: n2a, 3: n2b}},
			{3, []*node.Node{0: n3a, 7: n3b}},
			{4, []*node.Node{0: n4a, 15: n4b}},
			{5, []*node.Node{0: n5a, 31: n5b}},
			{6, []*node.Node{0: n6a, 63: n6b}},
			{7, []*node.Node{0: n7a, 127: n7b}},
			{8, []*node.Node{0: n8a, 1: n8b, 254: n8c, 255: n8d}},
		}

		rootNode := updatedTrie.RootNode()
		for _, tc := range testcases {
			nodes := getNodesAtLevel(rootNode, tc.level)
			require.Equal(t, len(tc.expectedNodes), len(nodes))
			require.Equal(t, tc.expectedNodes, nodes)
		}
	})
}
