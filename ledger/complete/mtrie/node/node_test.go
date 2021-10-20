package node_test

import (
	"encoding/hex"
	"fmt"
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

func Test_BubbleUp(t *testing.T) {
	path := utils.PathByUint16(1)
	payload1 := utils.LightPayload(2, 3)
	payload2 := utils.LightPayload(2, 4)
	emptyPayload := &ledger.Payload{}
	//       n5
	//     /    \
	//    n4    n3
	//   / \
	//  n1 n2
	n1 := node.NewLeaf(path, payload1, 0)
	n2 := node.NewLeaf(path, emptyPayload, 0)
	n3 := node.NewLeaf(path, payload2, 1)
	n4 := node.NewInterimNode(1, n1, n2)
	n5 := node.NewInterimNode(2, n4, n3)
	fmt.Println(n5.FmtStr(" ", " "))
	// require.True(t, n5.VerifyCachedHash())
	nn5, _ := n5.Prunned()
	fmt.Println(nn5.FmtStr(" ", " "))

	// it should be no change

	t.Fatal("XXX")
}

func hashToString(hash hash.Hash) string {
	return hex.EncodeToString(hash[:])
}
