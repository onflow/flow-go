package flattener_test

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
)

func TestLeafNodeV4Decoding(t *testing.T) {

	// Leaf node with nil payload
	path1 := utils.PathByUint8(0)
	payload1 := (*ledger.Payload)(nil)
	hashValue1 := hash.Hash([32]byte{1, 1, 1})
	leafNodeNilPayload := node.NewLeafNodeWithHash(ledger.Path(path1), payload1, 255, hashValue1)

	// Leaf node with empty payload (not nil)
	// EmptyPayload() not used because decoded playload's value is empty slice (not nil)
	path2 := utils.PathByUint8(1)
	payload2 := &ledger.Payload{Value: []byte{}}
	hashValue2 := hash.Hash([32]byte{2, 2, 2})
	leafNodeEmptyPayload := node.NewLeafNodeWithHash(ledger.Path(path2), payload2, 255, hashValue2)

	// Leaf node with payload
	path3 := utils.PathByUint8(2)
	payload3 := utils.LightPayload8('A', 'a')
	hashValue3 := hash.Hash([32]byte{3, 3, 3})
	leafNodePayload := node.NewLeafNodeWithHash(ledger.Path(path3), payload3, 255, hashValue3)

	encodedLeafNodeNilPayload := []byte{
		0x00,       // node type
		0x00, 0xff, // height
		0x00, 0x00, // max depth
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // reg count
		0x01, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // hash data
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // path data
		0x00, 0x00, 0x00, 0x00, // payload data len
	}

	encodedLeafNodeEmptyPayload := []byte{
		0x00,       // node type
		0x00, 0xff, // height
		0x00, 0x00, // max depth
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // reg count
		0x02, 0x02, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // hash data
		0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // path data
		0x00, 0x00, 0x00, 0x0e, // payload data len
		0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // payload data
	}

	encodedLeafNodePayload := []byte{
		0x00,       // node type
		0x00, 0xff, // height
		0x00, 0x00, // max depth
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // reg count
		0x03, 0x03, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // hash data
		0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // path data
		0x00, 0x00, 0x00, 0x16, // payload data len
		0x00, 0x00, 0x00, 0x09, 0x00, 0x01, 0x00, 0x00,
		0x00, 0x03, 0x00, 0x00, 0x41, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x01, 0x61, // payload data
	}

	testCases := []struct {
		name        string
		node        node.Node
		encodedNode []byte
	}{
		{"nil payload", leafNodeNilPayload, encodedLeafNodeNilPayload},
		{"empty payload", leafNodeEmptyPayload, encodedLeafNodeEmptyPayload},
		{"payload", leafNodePayload, encodedLeafNodePayload},
	}

	for _, tc := range testCases {
		t.Run("decode "+tc.name, func(t *testing.T) {
			scratchBuffers := [][]byte{
				nil,
				make([]byte, 0),
				make([]byte, 16),
				make([]byte, 1024),
			}

			for _, scratch := range scratchBuffers {
				reader := bytes.NewReader(tc.encodedNode)
				newNode, regCount, regSize, err := flattener.ReadNodeFromCheckpointV4(reader, scratch, func(nodeIndex uint64) (node.Node, uint64, uint64, error) {
					return nil, 0, 0, fmt.Errorf("no call expected")
				})
				require.NoError(t, err)
				assert.Equal(t, tc.node, newNode)
				assert.Equal(t, 0, reader.Len())
				require.Equal(t, uint64(1), regCount)
				require.Equal(t, uint64(tc.node.Payload().Size()), regSize)
			}
		})
	}
}

func TestInterimNodeV4Decoding(t *testing.T) {

	const lchildIndex = 1
	const rchildIndex = 2

	// Child node
	path1 := utils.PathByUint8(0)
	payload1 := utils.LightPayload8('A', 'a')
	hashValue1 := hash.Hash([32]byte{1, 1, 1})
	leafNode1 := node.NewLeafNodeWithHash(ledger.Path(path1), payload1, 255, hashValue1)

	// Child node
	path2 := utils.PathByUint8(1)
	payload2 := utils.LightPayload8('B', 'b')
	hashValue2 := hash.Hash([32]byte{2, 2, 2})
	leafNode2 := node.NewLeafNodeWithHash(ledger.Path(path2), payload2, 255, hashValue2)

	// Interim node
	hashValue3 := hash.Hash([32]byte{3, 3, 3})
	interimNode := node.NewInterimNodeWithHash(256, leafNode1, leafNode2, hashValue3)

	encodedInterimNode := []byte{
		0x01,       // node type
		0x01, 0x00, // height
		0x00, 0x01, // max depth
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, // reg count
		0x03, 0x03, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // hash data
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // LIndex
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, // RIndex
	}

	t.Run("decode", func(t *testing.T) {
		scratchBuffers := [][]byte{
			nil,
			make([]byte, 0),
			make([]byte, 16),
			make([]byte, 1024),
		}

		for _, scratch := range scratchBuffers {
			reader := bytes.NewReader(encodedInterimNode)
			newNode, regCount, regSize, err := flattener.ReadNodeFromCheckpointV4(reader, scratch, func(nodeIndex uint64) (node.Node, uint64, uint64, error) {
				switch nodeIndex {
				case lchildIndex:
					return leafNode1, 1, uint64(leafNode1.Payload().Size()), nil
				case rchildIndex:
					return leafNode2, 1, uint64(leafNode2.Payload().Size()), nil
				default:
					return nil, 0, 0, fmt.Errorf("unexpected child node index %d ", nodeIndex)
				}
			})
			require.NoError(t, err)
			assert.Equal(t, interimNode, newNode)
			assert.Equal(t, 0, reader.Len())
			require.Equal(t, uint64(2), regCount)
			require.Equal(t, uint64(leafNode1.Payload().Size()+leafNode2.Payload().Size()), regSize)
		}
	})

	t.Run("decode child node not found error", func(t *testing.T) {
		nodeNotFoundError := errors.New("failed to find node by index")
		scratch := make([]byte, 1024)

		reader := bytes.NewReader(encodedInterimNode)
		newNode, regCount, regSize, err := flattener.ReadNodeFromCheckpointV4(reader, scratch, func(nodeIndex uint64) (node.Node, uint64, uint64, error) {
			return nil, 0, 0, nodeNotFoundError
		})
		require.Nil(t, newNode)
		require.ErrorIs(t, err, nodeNotFoundError)
		require.Equal(t, uint64(0), regCount)
		require.Equal(t, uint64(0), regSize)
	})
}

func TestTrieV4Decoding(t *testing.T) {
	// Trie with nil root node
	emptyTrieRootNodeIndex := uint64(20)
	emptyTrie := trie.NewEmptyMTrie()

	// Trie with not nil root node
	hashValue := hash.Hash([32]byte{2, 2, 2})
	rootNode := node.NewLeafNodeWithHash(ledger.DummyPath, nil, 256, hashValue)
	notEmptyTrieRootNodeIndex := uint64(21)
	notEmptyTrie, err := trie.NewMTrie(rootNode, 7, 5000)
	require.NoError(t, err)

	encodedEmptyTrie := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x14, // RootIndex
		0x56, 0x8f, 0x4e, 0xc7, 0x40, 0xfe, 0x3b, 0x5d,
		0xe8, 0x80, 0x34, 0xcb, 0x7b, 0x1f, 0xbd, 0xdb,
		0x41, 0x54, 0x8b, 0x06, 0x8f, 0x31, 0xae, 0xbc,
		0x8a, 0xe9, 0x18, 0x9e, 0x42, 0x9c, 0x57, 0x49, // RootHash data
	}

	encodedTrie := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x15, // RootIndex
		0x02, 0x02, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // hash data
	}

	testCases := []struct {
		name          string
		tr            *trie.MTrie
		rootNodeIndex uint64
		encodedTrie   []byte
	}{
		{"empty trie", emptyTrie, emptyTrieRootNodeIndex, encodedEmptyTrie},
		{"trie", notEmptyTrie, notEmptyTrieRootNodeIndex, encodedTrie},
	}

	for _, tc := range testCases {
		t.Run("decode "+tc.name, func(t *testing.T) {
			scratchBuffers := [][]byte{
				nil,
				make([]byte, 0),
				make([]byte, 16),
				make([]byte, 1024),
			}

			for _, scratch := range scratchBuffers {
				reader := bytes.NewReader(tc.encodedTrie)
				tr, err := flattener.ReadTrieFromCheckpointV4(reader, scratch, func(nodeIndex uint64) (node.Node, uint64, uint64, error) {
					switch nodeIndex {
					case emptyTrieRootNodeIndex, notEmptyTrieRootNodeIndex:
						return tc.tr.RootNode(), tc.tr.AllocatedRegCount(), tc.tr.AllocatedRegSize(), nil
					default:
						return nil, 0, 0, fmt.Errorf("unexpected root node index %d ", nodeIndex)
					}
				})

				require.NoError(t, err)
				assert.Equal(t, tc.tr.RootNode(), tr.RootNode())
				assert.Equal(t, tc.tr.AllocatedRegCount(), tr.AllocatedRegCount())
				assert.Equal(t, tc.tr.AllocatedRegSize(), tr.AllocatedRegSize())
				assert.Equal(t, 0, reader.Len())
			}
		})

		t.Run("decode "+tc.name+" node not found error", func(t *testing.T) {
			nodeNotFoundError := errors.New("failed to find node by index")
			scratch := make([]byte, 1024)

			reader := bytes.NewReader(tc.encodedTrie)
			tr, err := flattener.ReadTrieFromCheckpointV4(reader, scratch, func(nodeIndex uint64) (node.Node, uint64, uint64, error) {
				return nil, 0, 0, nodeNotFoundError
			})
			require.Nil(t, tr)
			require.ErrorIs(t, err, nodeNotFoundError)
		})
	}
}
