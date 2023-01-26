package storage

import (
	"fmt"
	"testing"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecodePayload(t *testing.T) {
	path := testutils.PathByUint8(0)
	payload := testutils.LightPayload8('A', 'a')
	scratch := make([]byte, 1024*4)

	// encode the payload
	encoded, err := EncodePayload(path, payload, scratch)
	require.NoError(t, err, "can not encode payload")

	// decode the payload
	decodedPath, decodedPayload, err := DecodePayload(encoded)
	require.NoError(t, err)

	// decoded should be the same as original
	require.Equal(t, path, decodedPath)
	require.True(t, payload.Equals(decodedPayload))

	// encode again the decoded leaf node
	encodedAgain, err := EncodePayload(decodedPath, decodedPayload, scratch)
	require.NoError(t, err)

	require.Equal(t, encoded, encodedAgain)
}

func TestEncodeDecodeLeafNode(t *testing.T) {
	leafNode := leafNodeFixture()
	scratch := make([]byte, 1024*4)

	// encode the leaf node
	nodeHash, encoded, err := EncodeLeafNode(leafNode, scratch)
	require.NoError(t, err)

	// decode the leaf node
	decoded, err := DecodeLeafNode(nodeHash, encoded)
	require.NoError(t, err)

	// decoded should be the same as original
	require.NoError(t, leafNodeEqual(leafNode, decoded))

	// encode again the decoded leaf node
	fromDecodedHash, encodedFromDecoded, err := EncodeLeafNode(decoded, scratch)
	require.NoError(t, err)

	// encode again from the decoded leaf node should be the same as originally encoded
	require.Equal(t, nodeHash, fromDecodedHash)
	require.Equal(t, encoded, encodedFromDecoded)
}

func leafNodeEqual(n1 *node.Node, n2 *node.Node) error {
	if !n1.IsLeaf() {
		return fmt.Errorf("n1 is not leaf")
	}

	if !n2.IsLeaf() {
		return fmt.Errorf("n2 is not leaf")
	}

	if n1.Height() != n2.Height() {
		return fmt.Errorf("different height (%v != %v)", n1.Height(), n2.Height())
	}

	if *n1.Path() != *n2.Path() {
		return fmt.Errorf("different path (%v != %v)", n1.Path(), n2.Path())
	}

	if n1.Hash() != n2.Hash() {
		return fmt.Errorf("different hash (%v != %v)", n1.Hash(), n2.Hash())
	}

	if !n1.Payload().Equals(n2.Payload()) {
		return fmt.Errorf("different payload (%v != %v)", n1.Payload(), n2.Payload())
	}

	return nil
}

func leafNodeFixture() *node.Node {
	height := 255
	path := testutils.PathByUint8(0)
	payload := testutils.LightPayload8('A', 'a')
	nodeHash := hash.Hash([32]byte{1, 2, 3})
	return node.NewNode(height, nil, nil, ledger.Path(path), payload, nodeHash)
}

func TestImportAndRead(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		// create a trie for testing
		tries, err := createTrie()
		require.NoError(t, err, "could not create trie")
		logger := unittest.Logger()
		fileName := "root.checkpoint"
		store := createMockStore()

		// create a checkfile from the trie
		require.NoError(t, wal.StoreCheckpointV6Concurrently(tries, dir, fileName, &logger),
			"fail to store checkpoint")

		// import the checkpoint into storage mock
		require.NoError(t, ImportLeafNodesFromCheckpoint(dir, fileName, &logger, store),
			"fail to import leaf nodes from checkpoint")

		// check all leaf nodes are stored in the storage mock
		leafNodes := tries[0].AllLeafNodes()
		for _, n := range leafNodes {
			encoded, err := store.Get(n.Hash())
			require.NoError(t, err, "could not get node from storage")

			decoded, err := DecodeLeafNode(n.Hash(), encoded)
			require.NoError(t, leafNodeEqual(n, decoded))
		}

		// check didn't store extra nodes
		require.Equal(t, len(leafNodes), store.Count())
	})
}

func createTrie() ([]*trie.MTrie, error) {
	emptyTrie := trie.NewEmptyMTrie()

	p1 := testutils.PathByUint8(0)
	v1 := testutils.LightPayload8('A', 'a')

	p2 := testutils.PathByUint8(1)
	v2 := testutils.LightPayload8('B', 'b')

	paths := []ledger.Path{p1, p2}
	payloads := []ledger.Payload{*v1, *v2}

	updatedTrie, _, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads, true)
	if err != nil {
		return nil, err
	}
	tries := []*trie.MTrie{emptyTrie, updatedTrie}
	return tries, nil
}

func createMockStore() *store {
	return &store{
		nodes: make(map[hash.Hash][]byte),
	}
}

// a mock key-value storage
type store struct {
	nodes map[hash.Hash][]byte
}

func (s *store) Get(hash hash.Hash) ([]byte, error) {
	node, found := s.nodes[hash]
	if !found {
		return nil, fmt.Errorf("key not found: %v", hash)
	}

	return node, nil
}

func (s *store) Set(hash hash.Hash, value []byte) error {
	s.nodes[hash] = value
	return nil
}

func (s *store) Count() int {
	return len(s.nodes)
}
