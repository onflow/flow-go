package node_test

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/ledger/common"
	"github.com/dapperlabs/flow-go/ledger/complete/mtrie/node"
)

const (
	ReferenceImplKeyByteSize   = 2
	ReferenceImplValueByteSize = 2
)

// Test_ProperLeaf verifies that the hash value of a proper leaf (at height 0) is computed correctly
func Test_ProperLeaf(t *testing.T) {
	path := common.TwoBytesPath(56809)
	payload := common.LightPayload(56810, 59656)
	n := node.NewLeaf(path, payload, 0)
	expectedRootHashHex := "aa7693d498e9a087b1cadf5bfe9a1ff07829badc1915c210e482f369f9a00a70"
	require.Equal(t, expectedRootHashHex, hex.EncodeToString(n.Hash()))
}

// Test_ProperLeaf verifies that the hash value of a compactified leaf (at height > 0) is computed correctly.
// Here, we test with 16bit keys. Hence, the max height of a compactified leaf can be 16.
// We test the hash at the lowest-possible height (1), for the leaf to be still compactified,
// at an interim height (9) and the max possible height (16)
func Test_CompactifiedLeaf(t *testing.T) {
	path := common.TwoBytesPath(56809)
	payload := common.LightPayload(56810, 59656)
	n := node.NewLeaf(path, payload, 1)
	expectedRootHashHex := "34ee03b8ca7d5cc8638d28b7cf2d70641efd5dfa428333863904a0fd19930700"
	require.Equal(t, expectedRootHashHex, hex.EncodeToString(n.Hash()))

	n = node.NewLeaf(path, payload, 9)
	expectedRootHashHex = "1e726af2a11191dfaf03de45408955a114817872dbf063d161c3669c530f26f5"
	require.Equal(t, expectedRootHashHex, hex.EncodeToString(n.Hash()))

	n = node.NewLeaf(path, payload, 16)
	expectedRootHashHex = "b44a9a00c182ba2203fca6886c4c99b854f9f8279a9978b180ad10e82362e412"
	require.Equal(t, expectedRootHashHex, hex.EncodeToString(n.Hash()))
}

// Test_InterimNode verifies that the hash value of an interim node without children is computed correctly.
// We test the hash at the lowest-possible height (0), at an interim height (9) and the max possible height (16)
func Test_InterimNodeWithoutChildren(t *testing.T) {
	n := node.NewInterimNode(0, nil, nil)
	expectedRootHashHex := "18373b4b038cbbf37456c33941a7e346e752acd8fafa896933d4859002b62619"
	require.Equal(t, expectedRootHashHex, hex.EncodeToString(n.Hash()))

	n = node.NewInterimNode(9, nil, nil)
	expectedRootHashHex = "a37f98dbac56e315fbd4b9f9bc85fbd1b138ed4ae453b128c22c99401495af6d"
	require.Equal(t, expectedRootHashHex, hex.EncodeToString(n.Hash()))

	n = node.NewInterimNode(16, nil, nil)
	expectedRootHashHex = "6e24e2397f130d9d17bef32b19a77b8f5bcf03fb7e9e75fd89b8a455675d574a"
	require.Equal(t, expectedRootHashHex, hex.EncodeToString(n.Hash()))
}

// Test_InterimNodeWithOneChild verifies that the hash value of an interim node with
// only one child (left or right) is computed correctly.
func Test_InterimNodeWithOneChild(t *testing.T) {
	path := common.TwoBytesPath(56809)
	payload := common.LightPayload(56810, 59656)
	c := node.NewLeaf(path, payload, 0)

	n := node.NewInterimNode(1, c, nil)
	expectedRootHashHex := "87768f75da797362be04fbe4d30291f94ed416cc5f336fb17dd430791f93a661"
	require.Equal(t, expectedRootHashHex, hex.EncodeToString(n.Hash()))

	n = node.NewInterimNode(1, nil, c)
	expectedRootHashHex = "34ee03b8ca7d5cc8638d28b7cf2d70641efd5dfa428333863904a0fd19930700"
	require.Equal(t, expectedRootHashHex, hex.EncodeToString(n.Hash()))
}

// Test_InterimNodeWithBothChildren verifies that the hash value of an interim node with
// both children (left and right) is computed correctly.
func Test_InterimNodeWithBothChildren(t *testing.T) {
	leftPath := common.TwoBytesPath(56809)
	leftPayload := common.LightPayload(56810, 59656)
	leftChild := node.NewLeaf(leftPath, leftPayload, 0)

	rightPath := common.TwoBytesPath(2)
	rightPayload := common.LightPayload(11, 22)
	rightChild := node.NewLeaf(rightPath, rightPayload, 0)

	n := node.NewInterimNode(1, leftChild, rightChild)
	expectedRootHashHex := "77ae9ef2993849e70476c2dac2abc947cce92ca326fcafa74e912223a0b1a2ed"
	require.Equal(t, expectedRootHashHex, hex.EncodeToString(n.Hash()))
}

func Test_MaxDepth(t *testing.T) {
	path := common.TwoBytesPath(1)
	payload := common.LightPayload(2, 3)

	n1 := node.NewLeaf(path, payload, 0)
	n2 := node.NewLeaf(path, payload, 0)
	n3 := node.NewLeaf(path, payload, 0)

	n4 := node.NewInterimNode(1, n1, n2)
	n5 := node.NewInterimNode(1, n4, n3)
	require.Equal(t, n5.MaxDepth(), uint16(2))
}

func Test_RegCount(t *testing.T) {
	path := common.TwoBytesPath(1)
	payload := common.LightPayload(2, 3)
	n1 := node.NewLeaf(path, payload, 0)
	n2 := node.NewLeaf(path, payload, 0)
	n3 := node.NewLeaf(path, payload, 0)

	n4 := node.NewInterimNode(1, n1, n2)
	n5 := node.NewInterimNode(1, n4, n3)
	require.Equal(t, n5.RegCount(), uint64(3))
}
