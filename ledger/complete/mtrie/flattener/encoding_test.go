package flattener_test

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
)

func TestLeafNodeEncodingDecoding(t *testing.T) {

	// Leaf node with nil payload
	path1 := utils.PathByUint8(0)
	payload1 := (*ledger.Payload)(nil)
	hashValue1 := hash.Hash([32]byte{1, 1, 1})
	leafNodeNilPayload := node.NewNode(255, nil, nil, ledger.Path(path1), payload1, hashValue1, 0, 1)

	// Leaf node with empty payload (not nil)
	// EmptyPayload() not used because decoded playload's value is empty slice (not nil)
	path2 := utils.PathByUint8(1)
	payload2 := &ledger.Payload{Value: []byte{}}
	hashValue2 := hash.Hash([32]byte{2, 2, 2})
	leafNodeEmptyPayload := node.NewNode(255, nil, nil, ledger.Path(path2), payload2, hashValue2, 0, 1)

	// Leaf node with payload
	path3 := utils.PathByUint8(2)
	payload3 := utils.LightPayload8('A', 'a')
	hashValue3 := hash.Hash([32]byte{3, 3, 3})
	leafNodePayload := node.NewNode(255, nil, nil, ledger.Path(path3), payload3, hashValue3, 0, 1)

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
		node        *node.Node
		encodedNode []byte
	}{
		{"nil payload", leafNodeNilPayload, encodedLeafNodeNilPayload},
		{"empty payload", leafNodeEmptyPayload, encodedLeafNodeEmptyPayload},
		{"payload", leafNodePayload, encodedLeafNodePayload},
	}

	for _, tc := range testCases {
		t.Run("encode "+tc.name, func(t *testing.T) {
			scratchBuffers := [][]byte{
				nil,
				make([]byte, 0),
				make([]byte, 16),
				make([]byte, 1024),
			}

			for _, scratch := range scratchBuffers {
				encodedNode := flattener.EncodeNode(tc.node, 0, 0, scratch)
				assert.Equal(t, tc.encodedNode, encodedNode)

				if len(scratch) > 0 {
					if len(scratch) >= len(encodedNode) {
						// reuse scratch buffer
						require.True(t, &scratch[0] == &encodedNode[0])
					} else {
						// new alloc
						require.True(t, &scratch[0] != &encodedNode[0])
					}
				}
			}
		})

		t.Run("decode "+tc.name, func(t *testing.T) {
			scratchBuffers := [][]byte{
				nil,
				make([]byte, 0),
				make([]byte, 16),
				make([]byte, 1024),
			}

			for _, scratch := range scratchBuffers {
				reader := bytes.NewReader(tc.encodedNode)
				newNode, err := flattener.ReadNode(reader, scratch, func(nodeIndex uint64) (*node.Node, error) {
					return nil, fmt.Errorf("no call expected")
				})
				require.NoError(t, err)
				assert.Equal(t, tc.node, newNode)
				assert.Equal(t, 0, reader.Len())
			}
		})
	}
}

func TestRandomLeafNodeEncodingDecoding(t *testing.T) {
	const count = 1000
	const minPayloadSize = 40
	const maxPayloadSize = 1024 * 2
	// scratchBufferSize is intentionally small here to test
	// when encoded node size is sometimes larger than scratch buffer.
	const scratchBufferSize = 512

	paths := utils.RandomPaths(count)
	payloads := utils.RandomPayloads(count, minPayloadSize, maxPayloadSize)

	writeScratch := make([]byte, scratchBufferSize)
	readScratch := make([]byte, scratchBufferSize)

	for i := 0; i < count; i++ {
		height := rand.Intn(257)

		var hashValue hash.Hash
		rand.Read(hashValue[:])

		n := node.NewNode(height, nil, nil, paths[i], payloads[i], hashValue, 0, 1)

		encodedNode := flattener.EncodeNode(n, 0, 0, writeScratch)

		if len(writeScratch) >= len(encodedNode) {
			// reuse scratch buffer
			require.True(t, &writeScratch[0] == &encodedNode[0])
		} else {
			// new alloc because scratch buffer isn't big enough
			require.True(t, &writeScratch[0] != &encodedNode[0])
		}

		reader := bytes.NewReader(encodedNode)
		newNode, err := flattener.ReadNode(reader, readScratch, func(nodeIndex uint64) (*node.Node, error) {
			return nil, fmt.Errorf("no call expected")
		})
		require.NoError(t, err)
		assert.Equal(t, n, newNode)
		assert.Equal(t, 0, reader.Len())
	}
}

func TestInterimNodeEncodingDecoding(t *testing.T) {

	const lchildIndex = 1
	const rchildIndex = 2

	// Child node
	path1 := utils.PathByUint8(0)
	payload1 := utils.LightPayload8('A', 'a')
	hashValue1 := hash.Hash([32]byte{1, 1, 1})
	leafNode1 := node.NewNode(255, nil, nil, ledger.Path(path1), payload1, hashValue1, 0, 1)

	// Child node
	path2 := utils.PathByUint8(1)
	payload2 := utils.LightPayload8('B', 'b')
	hashValue2 := hash.Hash([32]byte{2, 2, 2})
	leafNode2 := node.NewNode(255, nil, nil, ledger.Path(path2), payload2, hashValue2, 0, 1)

	// Interim node
	hashValue3 := hash.Hash([32]byte{3, 3, 3})
	interimNode := node.NewNode(256, leafNode1, leafNode2, ledger.DummyPath, nil, hashValue3, 1, 2)

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

	t.Run("encode", func(t *testing.T) {
		scratchBuffers := [][]byte{
			nil,
			make([]byte, 0),
			make([]byte, 16),
			make([]byte, 1024),
		}

		for _, scratch := range scratchBuffers {
			data := flattener.EncodeNode(interimNode, lchildIndex, rchildIndex, scratch)
			assert.Equal(t, encodedInterimNode, data)
		}
	})

	t.Run("decode", func(t *testing.T) {
		scratchBuffers := [][]byte{
			nil,
			make([]byte, 0),
			make([]byte, 16),
			make([]byte, 1024),
		}

		for _, scratch := range scratchBuffers {
			reader := bytes.NewReader(encodedInterimNode)
			newNode, err := flattener.ReadNode(reader, scratch, func(nodeIndex uint64) (*node.Node, error) {
				switch nodeIndex {
				case lchildIndex:
					return leafNode1, nil
				case rchildIndex:
					return leafNode2, nil
				default:
					return nil, fmt.Errorf("unexpected child node index %d ", nodeIndex)
				}
			})
			require.NoError(t, err)
			assert.Equal(t, interimNode, newNode)
			assert.Equal(t, 0, reader.Len())
		}
	})

	t.Run("decode child node not found error", func(t *testing.T) {
		nodeNotFoundError := errors.New("failed to find node by index")
		scratch := make([]byte, 1024)

		reader := bytes.NewReader(encodedInterimNode)
		newNode, err := flattener.ReadNode(reader, scratch, func(nodeIndex uint64) (*node.Node, error) {
			return nil, nodeNotFoundError
		})
		require.Nil(t, newNode)
		require.ErrorIs(t, err, nodeNotFoundError)
	})
}

func TestTrieEncodingDecoding(t *testing.T) {
	// Nil root node
	rootNodeNil := (*node.Node)(nil)
	rootNodeNilIndex := uint64(20)

	// Not nil root node
	hashValue := hash.Hash([32]byte{2, 2, 2})
	rootNode := node.NewNode(256, nil, nil, ledger.DummyPath, nil, hashValue, 7, 5000)
	rootNodeIndex := uint64(21)

	encodedNilTrie := []byte{
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
		rootNode      *node.Node
		rootNodeIndex uint64
		encodedTrie   []byte
	}{
		{"nil trie", rootNodeNil, rootNodeNilIndex, encodedNilTrie},
		{"trie", rootNode, rootNodeIndex, encodedTrie},
	}

	for _, tc := range testCases {

		t.Run("encode "+tc.name, func(t *testing.T) {
			scratchBuffers := [][]byte{
				nil,
				make([]byte, 0),
				make([]byte, 16),
				make([]byte, 1024),
			}

			for _, scratch := range scratchBuffers {
				encodedTrie := flattener.EncodeTrie(tc.rootNode, tc.rootNodeIndex, scratch)
				assert.Equal(t, tc.encodedTrie, encodedTrie)

				if len(scratch) > 0 {
					if len(scratch) >= len(encodedTrie) {
						// reuse scratch buffer
						require.True(t, &scratch[0] == &encodedTrie[0])
					} else {
						// new alloc
						require.True(t, &scratch[0] != &encodedTrie[0])
					}
				}
			}
		})

		t.Run("decode "+tc.name, func(t *testing.T) {
			scratchBuffers := [][]byte{
				nil,
				make([]byte, 0),
				make([]byte, 16),
				make([]byte, 1024),
			}

			for _, scratch := range scratchBuffers {
				reader := bytes.NewReader(tc.encodedTrie)
				trie, err := flattener.ReadTrie(reader, scratch, func(nodeIndex uint64) (*node.Node, error) {
					if nodeIndex != tc.rootNodeIndex {
						return nil, fmt.Errorf("unexpected root node index %d ", nodeIndex)
					}
					return tc.rootNode, nil
				})
				require.NoError(t, err)
				assert.Equal(t, tc.rootNode, trie.RootNode())
				assert.Equal(t, 0, reader.Len())
			}
		})

		t.Run("decode "+tc.name+" node not found error", func(t *testing.T) {
			nodeNotFoundError := errors.New("failed to find node by index")
			scratch := make([]byte, 1024)

			reader := bytes.NewReader(tc.encodedTrie)
			newNode, err := flattener.ReadTrie(reader, scratch, func(nodeIndex uint64) (*node.Node, error) {
				return nil, nodeNotFoundError
			})
			require.Nil(t, newNode)
			require.ErrorIs(t, err, nodeNotFoundError)
		})
	}
}
