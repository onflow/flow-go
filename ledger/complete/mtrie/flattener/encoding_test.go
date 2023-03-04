package flattener_test

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
)

func TestLeafNodeEncodingDecoding(t *testing.T) {

	// Leaf node with nil payload
	paths := testutils.RandomPaths(1)
	payload := testutils.RandomPayload(1, 100)
	leafNode := node.NewLeaf(paths[0], payload, 255)

	t.Run("encode", func(t *testing.T) {
		scratchBuffers := [][]byte{
			nil,
			make([]byte, 0),
			make([]byte, 16),
			make([]byte, 1024),
		}

		for _, scratch := range scratchBuffers {
			encodedNode := flattener.EncodeNode(leafNode, 0, 0, scratch)
			require.Equal(t, 1+2+32+32, len(encodedNode), "encode leaf node should alway have 67 bytes")

			reader := bytes.NewReader(encodedNode)
			newNode, err := flattener.ReadNode(reader, scratch, func(nodeIndex uint64) (*node.Node, error) {
				return nil, fmt.Errorf("no call expected")
			})
			require.NoError(t, err)
			require.Equal(t, leafNode.Height(), newNode.Height())
			require.Equal(t, leafNode.Hash(), newNode.Hash())
			require.Equal(t, leafNode.ExpandedLeafHash(), newNode.ExpandedLeafHash())
			require.Equal(t, 0, reader.Len())
		}
	})
}

func TestRandomLeafNodeEncodingDecoding(t *testing.T) {
	const count = 1000
	const minPayloadSize = 40
	const maxPayloadSize = 1024 * 2
	// scratchBufferSize is intentionally small here to test
	// when encoded node size is sometimes larger than scratch buffer.
	const scratchBufferSize = 512

	writeScratch := make([]byte, scratchBufferSize)
	readScratch := make([]byte, scratchBufferSize)

	for i := 0; i < count; i++ {
		height := rand.Intn(257)

		paths := testutils.RandomPaths(1)
		payload := testutils.RandomPayload(1, 100)
		n := node.NewLeaf(paths[0], payload, height)

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
		require.Equal(t, n, newNode)
		require.Equal(t, 0, reader.Len())
	}
}

func TestInterimNodeEncodingDecoding(t *testing.T) {

	const lchildIndex = 1
	const rchildIndex = 2

	// Child node
	paths := testutils.RandomPaths(1)
	payload := testutils.RandomPayload(1, 100)
	leafNode1 := node.NewLeaf(paths[0], payload, 255)

	// Child node

	paths = testutils.RandomPaths(1)
	payload = testutils.RandomPayload(1, 100)
	leafNode2 := node.NewLeaf(paths[0], payload, 255)

	// Interim node
	interimNode := node.NewInterimNode(254, leafNode1, leafNode2)

	t.Run("decode", func(t *testing.T) {
		scratchBuffers := [][]byte{
			nil,
			make([]byte, 0),
			make([]byte, 16),
			make([]byte, 1024),
		}

		for _, scratch := range scratchBuffers {
			encodedInterimNode := flattener.EncodeNode(interimNode, lchildIndex, rchildIndex, scratch)
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
			require.Equal(t, interimNode, newNode)
			require.Equal(t, 0, reader.Len())
		}
	})

	t.Run("decode child node not found error", func(t *testing.T) {
		nodeNotFoundError := errors.New("failed to find node by index")
		scratch := make([]byte, 1024)

		encodedInterimNode := flattener.EncodeNode(interimNode, lchildIndex, rchildIndex, scratch)
		reader := bytes.NewReader(encodedInterimNode)
		newNode, err := flattener.ReadNode(reader, scratch, func(nodeIndex uint64) (*node.Node, error) {
			return nil, nodeNotFoundError
		})
		require.Nil(t, newNode)
		require.ErrorIs(t, err, nodeNotFoundError)
	})
}

func TestTrieEncodingDecoding(t *testing.T) {
	rootNodeNilIndex := uint64(20)

	hashValue := hash.Hash([32]byte{2, 2, 2})
	rootNode := node.NewNode(256, nil, nil, hashValue, hash.DummyHash)
	rootNodeIndex := uint64(21)

	mtrie, err := trie.NewMTrie(rootNode, 7, 1234)
	require.NoError(t, err)

	encodedNilTrie := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x14, // root index
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // reg count
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // reg size
		0x56, 0x8f, 0x4e, 0xc7, 0x40, 0xfe, 0x3b, 0x5d,
		0xe8, 0x80, 0x34, 0xcb, 0x7b, 0x1f, 0xbd, 0xdb,
		0x41, 0x54, 0x8b, 0x06, 0x8f, 0x31, 0xae, 0xbc,
		0x8a, 0xe9, 0x18, 0x9e, 0x42, 0x9c, 0x57, 0x49, // hash data
	}

	encodedTrie := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x15, // root index
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x07, // reg count
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x04, 0xd2, // reg size
		0x02, 0x02, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // hash data
	}

	testCases := []struct {
		name          string
		trie          *trie.MTrie
		rootNodeIndex uint64
		encodedTrie   []byte
	}{
		{"empty trie", trie.NewEmptyMTrie(), rootNodeNilIndex, encodedNilTrie},
		{"trie", mtrie, rootNodeIndex, encodedTrie},
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
				encodedTrie := flattener.EncodeTrie(tc.trie, tc.rootNodeIndex, scratch)
				require.Equal(t, tc.encodedTrie, encodedTrie)

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
					return tc.trie.RootNode(), nil
				})
				require.NoError(t, err)
				require.Equal(t, tc.trie, trie)
				require.Equal(t, tc.trie.AllocatedRegCount(), trie.AllocatedRegCount())
				require.Equal(t, tc.trie.AllocatedRegSize(), trie.AllocatedRegSize())
				require.Equal(t, 0, reader.Len())
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
