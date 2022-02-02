package flattener_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
)

func TestNodeSerialization(t *testing.T) {

	path1 := utils.PathByUint8(0)
	payload1 := utils.LightPayload8('A', 'a')
	hashValue1 := hash.Hash([32]byte{1, 1, 1})

	path2 := utils.PathByUint8(1)
	payload2 := utils.LightPayload8('B', 'b')
	hashValue2 := hash.Hash([32]byte{2, 2, 2})

	hashValue3 := hash.Hash([32]byte{3, 3, 3})

	leafNode1 := node.NewNode(255, nil, nil, ledger.Path(path1), payload1, hashValue1, 0, 1)
	leafNode2 := node.NewNode(255, nil, nil, ledger.Path(path2), payload2, hashValue2, 0, 1)
	rootNode := node.NewNode(256, leafNode1, leafNode2, ledger.DummyPath, nil, hashValue3, 1, 2)

	// Version 0
	expectedLeafNode1 := []byte{
		0, 0, // encoding version
		0, 255, // height
		0, 0, 0, 0, 0, 0, 0, 0, // LIndex
		0, 0, 0, 0, 0, 0, 0, 0, // RIndex
		0, 0, // max depth
		0, 0, 0, 0, 0, 0, 0, 1, // reg count
		0, 32, // path data len
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // path data
		0, 0, 0, 25, // payload data len
		0, 0, 6, 0, 0, 0, 9, 0, 1, 0, 0, 0, 3, 0, 0, 65, 0, 0, 0, 0, 0, 0, 0, 1, 97, // payload data
		0, 32, // hashValue length
		1, 1, 1, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, // hashValue
	}

	// Version 0
	expectedRootNode := []byte{
		0, 0, // encoding version
		1, 0, // height
		0, 0, 0, 0, 0, 0, 0, 1, // LIndex
		0, 0, 0, 0, 0, 0, 0, 2, // RIndex
		0, 1, // max depth
		0, 0, 0, 0, 0, 0, 0, 2, // reg count
		0, 0, // path data len
		0, 0, 0, 0, // payload data len
		0, 32, // hashValue length
		3, 3, 3, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, // hashValue
	}

	t.Run("encode leaf node", func(t *testing.T) {
		data := flattener.EncodeNode(leafNode1, 0, 0)
		assert.Equal(t, expectedLeafNode1, data)
	})

	t.Run("encode interim node", func(t *testing.T) {
		data := flattener.EncodeNode(rootNode, 1, 2)
		assert.Equal(t, expectedRootNode, data)
	})

	t.Run("decode leaf node", func(t *testing.T) {
		reader := bytes.NewReader(expectedLeafNode1)
		newNode, err := flattener.ReadNode(reader, func(nodeIndex uint64) (*node.Node, error) {
			if nodeIndex != 0 {
				return nil, fmt.Errorf("expect child node index 0, got %d", nodeIndex)
			}
			return nil, nil
		})
		require.NoError(t, err)
		assert.Equal(t, leafNode1, newNode)
	})

	t.Run("decode interim node", func(t *testing.T) {
		reader := bytes.NewReader(expectedRootNode)
		newNode, err := flattener.ReadNode(reader, func(nodeIndex uint64) (*node.Node, error) {
			switch nodeIndex {
			case 1:
				return leafNode1, nil
			case 2:
				return leafNode2, nil
			default:
				return nil, fmt.Errorf("unexpected child node index %d ", nodeIndex)
			}
		})
		require.NoError(t, err)
		assert.Equal(t, rootNode, newNode)
	})
}

func TestTrieSerialization(t *testing.T) {
	hashValue := hash.Hash([32]byte{2, 2, 2})
	rootNode := node.NewNode(256, nil, nil, ledger.DummyPath, nil, hashValue, 7, 5000)
	rootNodeIndex := uint64(21)

	// Version 0
	expected := []byte{
		0, 0, // encoding version
		0, 0, 0, 0, 0, 0, 0, 21, // RootIndex
		0, 32, // RootHash length
		2, 2, 2, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, // RootHash data
	}

	t.Run("encode", func(t *testing.T) {
		data := flattener.EncodeTrie(rootNode, rootNodeIndex)
		assert.Equal(t, expected, data)
	})

	t.Run("decode", func(t *testing.T) {
		reader := bytes.NewReader(expected)
		trie, err := flattener.ReadTrie(reader, func(nodeIndex uint64) (*node.Node, error) {
			if nodeIndex != rootNodeIndex {
				return nil, fmt.Errorf("unexpected root node index %d ", nodeIndex)
			}
			return rootNode, nil
		})
		require.NoError(t, err)
		assert.Equal(t, rootNode, trie.RootNode())
	})
}
