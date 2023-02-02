package storage

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/testutils"
)

func TestEncodeDecodePayload(t *testing.T) {
	path := testutils.PathByUint8(0)
	scratch := make([]byte, 1024*4)
	payload := testutils.LightPayload8('A', 'a')

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

func TestEncodeDecodeEmptyPayload(t *testing.T) {
	path := testutils.PathByUint8(0)
	scratch := make([]byte, 1024*4)
	payload := ledger.EmptyPayload()

	// encode the payload
	encoded, err := EncodePayload(path, payload, scratch)
	require.NoError(t, err, "can not encode payload")

	// decode the payload
	decodedPath, decodedPayload, err := DecodePayload(encoded)
	require.NoError(t, err)

	// decoded should be the same as original
	require.Equal(t, path, decodedPath)
	require.True(t, payload.Equals(decodedPayload))
}

// func TestImportAndRead(t *testing.T) {
// 	unittest.RunWithTempDir(t, func(dir string) {
// 		// create a trie for testing
// 		tries, err := createTrie()
// 		require.NoError(t, err, "could not create trie")
// 		logger := unittest.Logger()
// 		fileName := "root.checkpoint"
// 		store := createMockStore()
//
// 		// create a checkfile from the trie
// 		require.NoError(t, wal.StoreCheckpointV6Concurrently(tries, dir, fileName, &logger),
// 			"fail to store checkpoint")
//
// 		// import the checkpoint into storage mock
// 		require.NoError(t, ImportLeafNodesFromCheckpoint(dir, fileName, &logger, store),
// 			"fail to import leaf nodes from checkpoint")
//
// 		// check all leaf nodes are stored in the storage mock
// 		leafNodes := tries[0].AllLeafNodes()
// 		for _, n := range leafNodes {
// 			encoded, err := store.Get(n.Hash())
// 			require.NoError(t, err, "could not get node from storage")
//
// 			decoded, err := DecodeLeafNode(n.Hash(), encoded)
// 			require.NoError(t, leafNodeEqual(n, decoded))
// 		}
//
// 		// check didn't store extra nodes
// 		require.Equal(t, len(leafNodes), store.Count())
// 	})
// }
//
// func createTrie() ([]*trie.MTrie, error) {
// 	emptyTrie := trie.NewEmptyMTrie()
//
// 	p1 := testutils.PathByUint8(0)
// 	v1 := testutils.LightPayload8('A', 'a')
//
// 	p2 := testutils.PathByUint8(1)
// 	v2 := testutils.LightPayload8('B', 'b')
//
// 	paths := []ledger.Path{p1, p2}
// 	payloads := []ledger.Payload{*v1, *v2}
//
// 	updatedTrie, _, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads, true)
// 	if err != nil {
// 		return nil, err
// 	}
// 	tries := []*trie.MTrie{emptyTrie, updatedTrie}
// 	return tries, nil
// }

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
