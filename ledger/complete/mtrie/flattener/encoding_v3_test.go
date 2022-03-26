package flattener_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
)

// This file contains node/trie decoding tests for checkpoint v3 and earlier versions.
// These tests are based on TestStorableNode and TestStorableTrie.

func TestNodeV3Decoding(t *testing.T) {

	const leafNode1Index = 1
	const leafNode2Index = 2

	leafNode1 := node.NewNode(255, nil, nil, utils.PathByUint8(0), utils.LightPayload8('A', 'a'), hash.Hash([32]byte{1, 1, 1}))
	leafNode2 := node.NewNode(255, nil, nil, utils.PathByUint8(1), utils.LightPayload8('B', 'b'), hash.Hash([32]byte{2, 2, 2}))

	interimNode := node.NewNode(256, leafNode1, leafNode2, ledger.DummyPath, nil, hash.Hash([32]byte{3, 3, 3}))

	encodedLeafNode1 := []byte{
		0x00, 0x00, // encoding version
		0x00, 0xff, // height
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // LIndex
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // RIndex
		0x00, 0x00, // max depth
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // reg count
		0x00, 0x20, // path data len
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // path data
		0x00, 0x00, 0x00, 0x19, // payload data len
		0x00, 0x00, 0x06, 0x00, 0x00, 0x00, 0x09, 0x00,
		0x01, 0x00, 0x00, 0x00, 0x03, 0x00, 0x00, 0x41,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0x61,       // payload data
		0x00, 0x20, // hashValue length
		0x01, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // hash value
	}

	encodedInterimNode := []byte{
		0x00, 0x00, // encoding version
		0x01, 0x00, // height
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // LIndex
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, // RIndex
		0x00, 0x01, // max depth
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, // reg count
		0x00, 0x00, // path data len
		0x00, 0x00, 0x00, 0x00, // payload data len
		0x00, 0x20, // hashValue length
		0x03, 0x03, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // hash value
	}

	t.Run("leaf node", func(t *testing.T) {
		reader := bytes.NewReader(encodedLeafNode1)
		newNode, regCount, regSize, err := flattener.ReadNodeFromCheckpointV3AndEarlier(reader, func(nodeIndex uint64) (*node.Node, uint64, uint64, error) {
			return nil, 0, 0, fmt.Errorf("no call expected")
		})
		require.NoError(t, err)
		require.Equal(t, leafNode1, newNode)
		require.Equal(t, uint64(1), regCount)
		require.Equal(t, uint64(leafNode1.Payload().Size()), regSize)
	})

	t.Run("interim node", func(t *testing.T) {
		reader := bytes.NewReader(encodedInterimNode)
		newNode, regCount, regSize, err := flattener.ReadNodeFromCheckpointV3AndEarlier(reader, func(nodeIndex uint64) (*node.Node, uint64, uint64, error) {
			switch nodeIndex {
			case leafNode1Index:
				return leafNode1, 1, uint64(leafNode1.Payload().Size()), nil
			case leafNode2Index:
				return leafNode2, 1, uint64(leafNode2.Payload().Size()), nil
			default:
				return nil, 0, 0, fmt.Errorf("unexpected child node index %d ", nodeIndex)
			}
		})
		require.NoError(t, err)
		require.Equal(t, interimNode, newNode)
		require.Equal(t, uint64(2), regCount)
		require.Equal(t, uint64(leafNode1.Payload().Size()+leafNode2.Payload().Size()), regSize)
	})
}

func TestTrieV3Decoding(t *testing.T) {

	const rootNodeIndex = 21
	const rootNodeRegCount = 5000
	const rootNodeRegSize = 1000000

	hashValue := hash.Hash([32]byte{2, 2, 2})
	rootNode := node.NewNode(256, nil, nil, ledger.DummyPath, nil, hashValue)

	expected := []byte{
		0x00, 0x00, // encoding version
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 21, // RootIndex
		0x00, 0x20, // hashValue length
		0x02, 0x02, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // hash value
	}

	reader := bytes.NewReader(expected)

	trie, err := flattener.ReadTrieFromCheckpointV3AndEarlier(reader, func(nodeIndex uint64) (*node.Node, uint64, uint64, error) {
		switch nodeIndex {
		case rootNodeIndex:
			return rootNode, rootNodeRegCount, rootNodeRegSize, nil
		default:
			return nil, 0, 0, fmt.Errorf("unexpected root node index %d ", nodeIndex)
		}
	})
	require.NoError(t, err)
	require.Equal(t, rootNode, trie.RootNode())
	require.Equal(t, uint64(rootNodeRegCount), trie.AllocatedRegCount())
	require.Equal(t, uint64(rootNodeRegSize), trie.AllocatedRegSize())
}
