package node_test

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
)

// Test_ProperLeaf verifies that the hash value of a proper leaf (at height 0) is computed correctly
func Test_ProperLeaf(t *testing.T) {
	path := utils.PathByUint16(56809)
	payload := utils.LightPayload(56810, 59656)
	n := node.NewLeaf(path, payload, 0)
	expectedRootHashHex := "0ee164bc69981088186b5ceeb666e90e8e11bb15a1427aa56f47a484aedf73b4"
	require.Equal(t, expectedRootHashHex, hashToString(n.Hash()))
	require.True(t, n.VerifyCachedHash())
}

// Test_ProperLeaf verifies that the hash value of a compactified leaf (at height > 0) is computed correctly.
// Here, we test with 16bit keys. Hence, the max height of a compactified leaf can be 16.
// We test the hash at the lowest-possible height (1), for the leaf to be still compactified,
// at an interim height (9) and the max possible height (256)
func Test_CompactifiedLeaf(t *testing.T) {
	path := utils.PathByUint16(56809)
	payload := utils.LightPayload(56810, 59656)
	n := node.NewLeaf(path, payload, 1)
	expectedRootHashHex := "aa496f68adbbf43197f7e4b6ba1a63a47b9ce19b1587ca9ce587a7f29cad57d5"
	require.Equal(t, expectedRootHashHex, hashToString(n.Hash()))

	n = node.NewLeaf(path, payload, 9)
	expectedRootHashHex = "606aa23fdc40443de85b75768b847f94ff1d726e0bafde037833fe27543bb988"
	require.Equal(t, expectedRootHashHex, hashToString(n.Hash()))

	n = node.NewLeaf(path, payload, 256)
	expectedRootHashHex = "d2536303495a9325037d247cbb2b9be4d6cb3465986ea2c4481d8770ff16b6b0"
	require.Equal(t, expectedRootHashHex, hashToString(n.Hash()))
}

// Test_InterimNode verifies that the hash value of an interim node without children is computed correctly.
// We test the hash at the lowest-possible height (0), at an interim height (9) and the max possible height (256)
func Test_InterimNodeWithoutChildren(t *testing.T) {
	n := node.NewInterimNode(0, nil, nil)
	expectedRootHashHex := "18373b4b038cbbf37456c33941a7e346e752acd8fafa896933d4859002b62619"
	require.Equal(t, expectedRootHashHex, hashToString(n.Hash()))

	n = node.NewInterimNode(9, nil, nil)
	expectedRootHashHex = "a37f98dbac56e315fbd4b9f9bc85fbd1b138ed4ae453b128c22c99401495af6d"
	require.Equal(t, expectedRootHashHex, hashToString(n.Hash()))

	n = node.NewInterimNode(16, nil, nil)
	expectedRootHashHex = "6e24e2397f130d9d17bef32b19a77b8f5bcf03fb7e9e75fd89b8a455675d574a"
	require.Equal(t, expectedRootHashHex, hashToString(n.Hash()))
}

// Test_InterimNodeWithOneChild verifies that the hash value of an interim node with
// only one child (left or right) is computed correctly.
func Test_InterimNodeWithOneChild(t *testing.T) {
	path := utils.PathByUint16(56809)
	payload := utils.LightPayload(56810, 59656)
	c := node.NewLeaf(path, payload, 0)

	n := node.NewInterimNode(1, c, nil)
	expectedRootHashHex := "aa496f68adbbf43197f7e4b6ba1a63a47b9ce19b1587ca9ce587a7f29cad57d5"
	require.Equal(t, expectedRootHashHex, hashToString(n.Hash()))

	n = node.NewInterimNode(1, nil, c)
	expectedRootHashHex = "9845f2c9e9c067ec6efba06ffb7c1be387b2a893ae979b1f6cb091bda1b7e12d"
	require.Equal(t, expectedRootHashHex, hashToString(n.Hash()))
}

// Test_InterimNodeWithBothChildren verifies that the hash value of an interim node with
// both children (left and right) is computed correctly.
func Test_InterimNodeWithBothChildren(t *testing.T) {
	leftPath := utils.PathByUint16(56809)
	leftPayload := utils.LightPayload(56810, 59656)
	leftChild := node.NewLeaf(leftPath, leftPayload, 0)

	rightPath := utils.PathByUint16(2)
	rightPayload := utils.LightPayload(11, 22)
	rightChild := node.NewLeaf(rightPath, rightPayload, 0)

	n := node.NewInterimNode(1, leftChild, rightChild)
	expectedRootHashHex := "1e4754fb35ec011b6192e205de403c1031d8ce64bd3d1ff8f534a20595af90c3"
	require.Equal(t, expectedRootHashHex, hashToString(n.Hash()))
}

func Test_MaxDepth(t *testing.T) {
	path := utils.PathByUint16(1)
	payload := utils.LightPayload(2, 3)

	n1 := node.NewLeaf(path, payload, 0)
	n2 := node.NewLeaf(path, payload, 0)
	n3 := node.NewLeaf(path, payload, 0)

	n4 := node.NewInterimNode(1, n1, n2)
	n5 := node.NewInterimNode(1, n4, n3)
	require.Equal(t, n5.MaxDepth(), uint16(2))
}

func Test_RegCount(t *testing.T) {
	path := utils.PathByUint16(1)
	payload := utils.LightPayload(2, 3)
	n1 := node.NewLeaf(path, payload, 0)
	n2 := node.NewLeaf(path, payload, 0)
	n3 := node.NewLeaf(path, payload, 0)

	n4 := node.NewInterimNode(1, n1, n2)
	n5 := node.NewInterimNode(1, n4, n3)
	require.Equal(t, n5.RegCount(), uint64(3))
}
func Test_AllPayloads(t *testing.T) {
	path := utils.PathByUint16(1)
	payload := utils.LightPayload(2, 3)
	n1 := node.NewLeaf(path, payload, 0)
	n2 := node.NewLeaf(path, payload, 0)
	n3 := node.NewLeaf(path, payload, 0)
	n4 := node.NewInterimNode(1, n1, n2)
	n5 := node.NewInterimNode(1, n4, n3)
	require.Equal(t, 3, len(n5.AllPayloads()))
}

func Test_VerifyCachedHash(t *testing.T) {
	path := utils.PathByUint16(1)
	payload := utils.LightPayload(2, 3)
	n1 := node.NewLeaf(path, payload, 0)
	n2 := node.NewLeaf(path, payload, 0)
	n3 := node.NewLeaf(path, payload, 0)
	n4 := node.NewInterimNode(1, n1, n2)
	n5 := node.NewInterimNode(1, n4, n3)
	require.True(t, n5.VerifyCachedHash())
}

// Test_Compactify_EmptySubtrie tests constructing an interim node
// with pruning/compactification, where both children are empty. We expect
// the compactified node to be nil, as it represents a completely empty subtrie
func Test_Compactify_EmptySubtrie(t *testing.T) {
	//      n3
	//    /   \
	// n1(-)  n2(-)
	n1 := node.NewLeaf(utils.PathByUint16LeftPadded(0), &ledger.Payload{}, 4)    // path: ...0000 0000
	n2 := node.NewLeaf(utils.PathByUint16LeftPadded(1<<4), &ledger.Payload{}, 4) // path: ...0001 0000

	t.Run("both children empty", func(t *testing.T) {
		require.Nil(t, node.NewInterimCompactifiedNode(5, n1, n2))
	})

	t.Run("one child nil and one child empty", func(t *testing.T) {
		require.Nil(t, node.NewInterimCompactifiedNode(5, nil, n2))
		require.Nil(t, node.NewInterimCompactifiedNode(5, n1, nil))
	})

	t.Run("both children nil", func(t *testing.T) {
		require.Nil(t, node.NewInterimCompactifiedNode(5, nil, n2))
		require.Nil(t, node.NewInterimCompactifiedNode(5, n1, nil))
	})
}

// Test_Compactify_ToLeaf tests constructing an interim node with pruning/compactification,
// where one child is empty and the other child is a leaf. We expect the compactified node
// to be a leaf, as it only contains a single allocated register.
func Test_Compactify_ToLeaf(t *testing.T) {
	path1 := utils.PathByUint16LeftPadded(0)      // ...0000 0000
	path2 := utils.PathByUint16LeftPadded(1 << 4) // ...0001 0000
	emptyPayload := &ledger.Payload{}
	payloadA := utils.LightPayload(2, 2)

	t.Run("left child empty", func(t *testing.T) {
		// constructing an un-pruned tree first as reference:
		//      n3
		//    /   \
		// n1(-)  n2(A)
		n1 := node.NewLeaf(path1, emptyPayload, 4)
		n2 := node.NewLeaf(path2, payloadA, 4)
		n3 := node.NewInterimNode(5, n1, n2)

		// Constructing a trie with pruning/compactification should result in
		//       nn3(A)
		// while keeping the root hash invariant
		nn3 := node.NewInterimCompactifiedNode(5, n1, n2)
		requireIsLeafWithHash(t, nn3, n3.Hash())

		nn3 = node.NewInterimCompactifiedNode(5, nil, n2)
		requireIsLeafWithHash(t, nn3, n3.Hash())
	})

	t.Run("right child empty", func(t *testing.T) {
		// constructing an un-pruned tree first as reference:
		//      n3
		//    /   \
		// n1(A)  n2(-)
		n1 := node.NewLeaf(path1, payloadA, 4)
		n2 := node.NewLeaf(path2, emptyPayload, 4)
		n3 := node.NewInterimNode(5, n1, n2)

		// Constructing a trie with pruning/compactification should result in
		//       nn3(A)
		// while keeping the root hash invariant
		nn3 := node.NewInterimCompactifiedNode(5, n1, n2)
		requireIsLeafWithHash(t, nn3, n3.Hash())

		nn3 = node.NewInterimCompactifiedNode(5, n1, nil)
		requireIsLeafWithHash(t, nn3, n3.Hash())
	})
}

// Test_Compactify_EmptyChild tests constructing an interim node with pruning/compactification,
// where one child is empty and the other child holds _multiple_ allocated registers (more than one).
// We expect in the compactified node, the empty subtrie is completely removed and replaced by nil.
func Test_Compactify_EmptyChild(t *testing.T) {
	payloadA := utils.LightPayload(2, 2)
	payloadB := utils.LightPayload(4, 4)
	emptyPayload := &ledger.Payload{}

	t.Run("right child empty", func(t *testing.T) {
		// constructing an un-pruned tree first as reference:
		//          n5
		//       /     \
		//      n3      n4(-)
		//   /    \
		// n1(A)  n2(B)
		n1 := node.NewLeaf(utils.PathByUint16LeftPadded(0), payloadA, 4)    // path: ...0000 0000
		n2 := node.NewLeaf(utils.PathByUint16LeftPadded(1<<4), payloadB, 4) // path: ...0001 0000
		n3 := node.NewInterimNode(5, n1, n2)
		n4 := node.NewLeaf(utils.PathByUint16LeftPadded(3<<4), emptyPayload, 5) // path: ...0011 0000
		n5 := node.NewInterimNode(6, n3, n4)

		// Constructing a trie with pruning/compactification should result
		// in n4 being replaced with nil, while keeping the root hash invariant.
		nn5 := node.NewInterimCompactifiedNode(6, n3, n4)
		require.Equal(t, n3, nn5.LeftChild())
		require.Nil(t, nn5.RightChild())
		require.True(t, nn5.VerifyCachedHash())
		require.Equal(t, nn5.Hash(), n5.Hash())
		require.Equal(t, uint16(2), nn5.MaxDepth())
		require.Equal(t, uint64(2), nn5.RegCount())
	})

	t.Run("left child empty", func(t *testing.T) {
		// constructing an un-pruned tree first as reference:
		//          n5
		//       /     \
		//    n3(-)    n4
		//           /   \
		//        n1(A)  n2(B)
		n1 := node.NewLeaf(utils.PathByUint16LeftPadded(2<<4), payloadA, 4)  // path: ...0010 0000
		n2 := node.NewLeaf(utils.PathByUint16LeftPadded(3<<4), payloadB, 4)  // path: ...0011 0000
		n3 := node.NewLeaf(utils.PathByUint16LeftPadded(0), emptyPayload, 5) // path: ...0000 0000
		n4 := node.NewInterimNode(5, n1, n2)
		n5 := node.NewInterimNode(6, n3, n4)

		// Constructing a trie with pruning/compactification should result
		// in n4 being replaced with nil, while keeping the root hash invariant.
		nn5 := node.NewInterimCompactifiedNode(6, n3, n4)
		require.Nil(t, nn5.LeftChild())
		require.Equal(t, n4, nn5.RightChild())
		require.True(t, nn5.VerifyCachedHash())
		require.Equal(t, nn5.Hash(), n5.Hash())
		require.Equal(t, uint16(2), nn5.MaxDepth())
		require.Equal(t, uint64(2), nn5.RegCount())
	})

}

// Test_Compactify_BothChildrenPopulated tests some cases, where both children are populated
func Test_Compactify_BothChildrenPopulated(t *testing.T) {
	//          n5
	//       /     \
	//      n3      n4(C)
	//   /    \
	// n1(A)  n2(B)
	path1 := utils.PathByUint16LeftPadded(0)      // ...0000 0000
	path2 := utils.PathByUint16LeftPadded(1 << 4) // ...0001 0000
	path4 := utils.PathByUint16LeftPadded(3 << 4) // ...0011 0000
	payloadA := utils.LightPayload(2, 2)
	payloadB := utils.LightPayload(3, 3)
	payloadC := utils.LightPayload(4, 4)

	// constructing an un-pruned tree first as reference:
	n1 := node.NewLeaf(path1, payloadA, 4)
	n2 := node.NewLeaf(path2, payloadB, 4)
	n3 := node.NewInterimNode(5, n1, n2)
	n4 := node.NewLeaf(path4, payloadC, 5)
	n5 := node.NewInterimNode(6, n3, n4)

	// Constructing a trie with pruning/compactification should result
	// reproduce exactly the same trie as no pruning/compactification is possible
	nn3 := node.NewInterimCompactifiedNode(5, n1, n2)
	require.Equal(t, n1, nn3.LeftChild())
	require.Equal(t, n2, nn3.RightChild())
	require.True(t, nn3.VerifyCachedHash())
	require.Equal(t, n3.Hash(), nn3.Hash())
	require.Equal(t, uint16(1), nn3.MaxDepth())
	require.Equal(t, uint64(2), nn3.RegCount())

	nn5 := node.NewInterimCompactifiedNode(6, nn3, n4)
	require.Equal(t, nn3, nn5.LeftChild())
	require.Equal(t, n4, nn5.RightChild())
	require.True(t, nn5.VerifyCachedHash())
	require.Equal(t, nn5.Hash(), n5.Hash())
	require.Equal(t, uint16(2), nn5.MaxDepth())
	require.Equal(t, uint64(3), nn5.RegCount())
}

func hashToString(hash hash.Hash) string {
	return hex.EncodeToString(hash[:])
}

// requireIsLeafWithHash verifies that `node` is a leaf node, whose hash equals `expectedHash`.
// We perform the following checks:
// * both children must be nil
// * depth is zero
// * number of registers in the sub-trie is 1
// * pre-computed hash matches the `expectedHash`
// * re-computing the hash from the children yields the pre-computed value
// * node reports itself as a leaf
func requireIsLeafWithHash(t *testing.T, node *node.Node, expectedHash hash.Hash) {
	require.Nil(t, node.LeftChild())
	require.Nil(t, node.RightChild())
	require.Equal(t, uint16(0), node.MaxDepth())
	require.Equal(t, uint64(1), node.RegCount())
	require.Equal(t, node.Hash(), expectedHash)
	require.True(t, node.VerifyCachedHash())
	require.True(t, node.IsLeaf())
}
