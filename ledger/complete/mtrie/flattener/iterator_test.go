package flattener_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
)

func TestEmptyTrie(t *testing.T) {
	emptyTrie := trie.NewEmptyMTrie()

	itr := flattener.NewNodeIterator(emptyTrie.RootNode())
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
	p1 := testutils.PathByUint8(0)
	v1 := testutils.LightPayload8('A', 'a')

	// key: 0100....
	p2 := testutils.PathByUint8(64)
	v2 := testutils.LightPayload8('B', 'b')

	paths := []ledger.Path{p1, p2}
	payloads := []ledger.Payload{*v1, *v2}

	testTrie, _, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads, true)
	require.NoError(t, err)

	for itr := flattener.NewNodeIterator(testTrie.RootNode()); itr.Next(); {
		fmt.Println(itr.Value().FmtStr("", ""))
		fmt.Println()
	}

	itr := flattener.NewNodeIterator(testTrie.RootNode())

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

func TestUniqueNodeIterator(t *testing.T) {
	t.Run("empty trie", func(t *testing.T) {
		emptyTrie := trie.NewEmptyMTrie()

		// visitedNodes is nil
		itr := flattener.NewUniqueNodeIterator(emptyTrie.RootNode(), nil)
		require.False(t, itr.Next())
		require.True(t, nil == itr.Value()) // initial iterator should return nil

		// visitedNodes is empty map
		visitedNodes := make(map[*node.Node]uint64)
		itr = flattener.NewUniqueNodeIterator(emptyTrie.RootNode(), visitedNodes)
		require.False(t, itr.Next())
		require.True(t, nil == itr.Value()) // initial iterator should return nil
	})

	t.Run("trie", func(t *testing.T) {
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
		//

		expectedNodes := []*node.Node{
			updatedTrie.RootNode().LeftChild().LeftChild(),  // n1
			updatedTrie.RootNode().LeftChild().RightChild(), // n2
			updatedTrie.RootNode().LeftChild(),              // n3
			updatedTrie.RootNode(),                          // n4
		}

		// visitedNodes is nil
		i := 0
		for itr := flattener.NewUniqueNodeIterator(updatedTrie.RootNode(), nil); itr.Next(); {
			n := itr.Value()
			require.True(t, i < len(expectedNodes))
			require.Equal(t, expectedNodes[i], n)
			i++
		}
		require.Equal(t, i, len(expectedNodes))

		// visitedNodes is not nil, but it's pointless for iterating a single trie because
		// there isn't any shared sub-trie.
		visitedNodes := make(map[*node.Node]uint64)
		i = 0
		for itr := flattener.NewUniqueNodeIterator(updatedTrie.RootNode(), visitedNodes); itr.Next(); {
			n := itr.Value()
			visitedNodes[n] = uint64(i)

			require.True(t, i < len(expectedNodes))
			require.Equal(t, expectedNodes[i], n)
			i++
		}
		require.Equal(t, i, len(expectedNodes))
	})

	t.Run("forest", func(t *testing.T) {

		// tries is a slice of mtries to guarantee order.
		var tries []*trie.MTrie

		emptyTrie := trie.NewEmptyMTrie()

		// key: 0000...
		p1 := testutils.PathByUint8(0)
		v1 := testutils.LightPayload8('A', 'a')

		// key: 0100....
		p2 := testutils.PathByUint8(64)
		v2 := testutils.LightPayload8('B', 'b')

		paths := []ledger.Path{p1, p2}
		payloads := []ledger.Payload{*v1, *v2}

		trie1, _, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads, true)
		require.NoError(t, err)

		// trie1
		//              n4
		//             /
		//            /
		//          n3
		//        /     \
		//      /         \
		//   n1 (p1/v1)     n2 (p2/v2)
		//

		tries = append(tries, trie1)

		// New trie reuses its parent's left sub-trie.

		// key: 1000...
		p3 := testutils.PathByUint8(128)
		v3 := testutils.LightPayload8('C', 'c')

		// key: 1100....
		p4 := testutils.PathByUint8(192)
		v4 := testutils.LightPayload8('D', 'd')

		paths = []ledger.Path{p3, p4}
		payloads = []ledger.Payload{*v3, *v4}

		trie2, _, err := trie.NewTrieWithUpdatedRegisters(trie1, paths, payloads, true)
		require.NoError(t, err)

		// trie2
		//              n8
		//             /   \
		//            /      \
		//          n3       n7
		//       (shared)   /   \
		//                /       \
		//              n5         n6
		//            (p3/v3)    (p4/v4)

		tries = append(tries, trie2)

		// New trie reuses its parent's right sub-trie, and left sub-trie's leaf node.

		// key: 0000...
		v5 := testutils.LightPayload8('E', 'e')

		paths = []ledger.Path{p1}
		payloads = []ledger.Payload{*v5}

		trie3, _, err := trie.NewTrieWithUpdatedRegisters(trie2, paths, payloads, true)
		require.NoError(t, err)

		// trie3
		//              n11
		//             /   \
		//            /      \
		//          n10       n7
		//         /   \    (shared)
		//       /       \
		//     n9         n2
		//  (p1/v5)    (shared)

		tries = append(tries, trie3)

		expectedNodes := []*node.Node{
			// unique nodes from trie1
			trie1.RootNode().LeftChild().LeftChild(),  // n1
			trie1.RootNode().LeftChild().RightChild(), // n2
			trie1.RootNode().LeftChild(),              // n3
			trie1.RootNode(),                          // n4
			// unique nodes from trie2
			trie2.RootNode().RightChild().LeftChild(),  // n5
			trie2.RootNode().RightChild().RightChild(), // n6
			trie2.RootNode().RightChild(),              // n7
			trie2.RootNode(),                           // n8
			// unique nodes from trie3
			trie3.RootNode().LeftChild().LeftChild(), // n9
			trie3.RootNode().LeftChild(),             // n10
			trie3.RootNode(),                         // n11
		}

		// Use visitedNodes to prevent revisiting shared sub-tries.
		visitedNodes := make(map[*node.Node]uint64)
		i := 0
		for _, trie := range tries {
			for itr := flattener.NewUniqueNodeIterator(trie.RootNode(), visitedNodes); itr.Next(); {
				n := itr.Value()
				visitedNodes[n] = uint64(i)

				require.True(t, i < len(expectedNodes))
				require.Equal(t, expectedNodes[i], n)
				i++
			}
		}
		require.Equal(t, i, len(expectedNodes))
	})

	t.Run("subtries", func(t *testing.T) {

		emptyTrie := trie.NewEmptyMTrie()

		// key: 0000...
		p1 := testutils.PathByUint8(0)
		v1 := testutils.LightPayload8('A', 'a')

		// key: 0100....
		p2 := testutils.PathByUint8(64)
		v2 := testutils.LightPayload8('B', 'b')

		paths := []ledger.Path{p1, p2}
		payloads := []ledger.Payload{*v1, *v2}

		trie1, _, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads, true)
		require.NoError(t, err)

		// trie1
		//              n4
		//             /
		//            /
		//          n3
		//        /     \
		//      /         \
		//   n1 (p1/v1)     n2 (p2/v2)
		//

		// New trie reuses its parent's left sub-trie.

		// key: 1000...
		p3 := testutils.PathByUint8(128)
		v3 := testutils.LightPayload8('C', 'c')

		// key: 1100....
		p4 := testutils.PathByUint8(192)
		v4 := testutils.LightPayload8('D', 'd')

		paths = []ledger.Path{p3, p4}
		payloads = []ledger.Payload{*v3, *v4}

		trie2, _, err := trie.NewTrieWithUpdatedRegisters(trie1, paths, payloads, true)
		require.NoError(t, err)

		// trie2
		//              n8
		//             /   \
		//            /      \
		//          n3       n7
		//       (shared)   /   \
		//                /       \
		//              n5         n6
		//            (p3/v3)    (p4/v4)

		// New trie reuses its parent's right sub-trie, and left sub-trie's leaf node.

		// key: 0000...
		v5 := testutils.LightPayload8('E', 'e')

		paths = []ledger.Path{p1}
		payloads = []ledger.Payload{*v5}

		trie3, _, err := trie.NewTrieWithUpdatedRegisters(trie2, paths, payloads, true)
		require.NoError(t, err)

		// trie3
		//              n11
		//             /   \
		//            /      \
		//          n10       n7
		//         /   \    (shared)
		//       /       \
		//     n9         n2
		//  (p1/v5)    (shared)

		leftSubtries := []*node.Node{
			trie1.RootNode().LeftChild(),
			trie2.RootNode().LeftChild(),
			trie3.RootNode().LeftChild(),
		}

		expectedNodesInLeftSubtries := []*node.Node{
			// unique nodes from trie1
			trie1.RootNode().LeftChild().LeftChild(),  // n1
			trie1.RootNode().LeftChild().RightChild(), // n2
			trie1.RootNode().LeftChild(),              // n3
			// unique nodes from trie3
			trie3.RootNode().LeftChild().LeftChild(), // n9
			trie3.RootNode().LeftChild(),             // n10
		}

		rightSubtries := []*node.Node{
			trie1.RootNode().RightChild(),
			trie2.RootNode().RightChild(),
			trie3.RootNode().RightChild(),
		}

		expectedNodesInRightSubtries := []*node.Node{
			// unique nodes from trie2
			trie2.RootNode().RightChild().LeftChild(),  // n5
			trie2.RootNode().RightChild().RightChild(), // n6
			trie2.RootNode().RightChild(),              // n7
		}

		testcases := []struct {
			roots         []*node.Node
			expectedNodes []*node.Node
		}{
			{leftSubtries, expectedNodesInLeftSubtries},
			{rightSubtries, expectedNodesInRightSubtries},
		}

		for _, tc := range testcases {
			visitedNodes := make(map[*node.Node]uint64)
			i := 0
			for _, n := range tc.roots {
				for itr := flattener.NewUniqueNodeIterator(n, visitedNodes); itr.Next(); {
					n := itr.Value()
					visitedNodes[n] = uint64(i)

					require.True(t, i < len(tc.expectedNodes))
					require.Equal(t, tc.expectedNodes[i], n)
					i++
				}
			}
			require.Equal(t, i, len(tc.expectedNodes))
		}
	})
}
