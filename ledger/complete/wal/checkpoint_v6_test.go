package wal

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestFileIndex(t *testing.T) {
	index, err := decodeFileIndex(encodeFileIndex(uint16(20)))
	require.NoError(t, err)
	require.Equal(t, 20, int(index))
}

func TestVersion(t *testing.T) {
	m, v, err := decodeVersion(encodeVersion(MagicBytes, VersionV6))
	require.NoError(t, err)
	require.Equal(t, MagicBytes, m)
	require.Equal(t, VersionV6, v)
}

func TestSubtrieLevel(t *testing.T) {
	l, err := decodeSubtrieLevel(encodeSubtrieLevel(subtrieLevel))
	require.NoError(t, err)
	require.Equal(t, uint16(subtrieLevel), l)
}

func TestCRC32SumEncoding(t *testing.T) {
	v := uint32(3)
	s, err := decodeCRC32Sum(encodeCRC32Sum(v))
	require.NoError(t, err)
	require.Equal(t, v, s)
}

func TestNodeCountEncoding(t *testing.T) {
	v := uint64(100)
	s, err := decodeNodeCount(encodeNodeCount(v))
	require.NoError(t, err)
	require.Equal(t, v, s)
}

func TestFooterEncoding(t *testing.T) {
	n1, r1 := uint64(40), uint16(500)
	n2, r2, err := decodeFooter(encodeFooter(n1, r1))
	require.NoError(t, err)
	require.Equal(t, n1, n2)
	require.Equal(t, r1, r2)
}

func requireTriesEqual(t *testing.T, tries1, tries2 []*trie.MTrie) {
	require.Equal(t, len(tries1), len(tries2), "tries have different length")
	for i, expect := range tries1 {
		actual := tries2[i]
		require.True(t, expect.Equals(actual), "%v-th trie is different", i)
	}
}

func createSimpleTrie(t *testing.T) []*trie.MTrie {
	emptyTrie := trie.NewEmptyMTrie()

	p1 := testutils.PathByUint8(0)
	v1 := testutils.LightPayload8('A', 'a')

	p2 := testutils.PathByUint8(1)
	v2 := testutils.LightPayload8('B', 'b')

	paths := []ledger.Path{p1, p2}
	payloads := []ledger.Payload{*v1, *v2}

	updatedTrie, _, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads, true)
	require.NoError(t, err)
	tries := []*trie.MTrie{updatedTrie}
	return tries
}

func TestEncodeSubTrie(t *testing.T) {
	file := "checkpoint"
	logger := unittest.Logger()
	tries := createSimpleTrie(t)
	estimatedSubtrieNodeCount := estimateSubtrieNodeCount(tries)
	subtrieRoots := createSubTrieRoots(tries)

	for index, roots := range subtrieRoots {
		unittest.RunWithTempDir(t, func(dir string) {
			indices, nodeCount, checksum, err := storeCheckpointSubTrie(
				index, roots, estimatedSubtrieNodeCount, dir, file, &logger)
			require.NoError(t, err)

			if len(indices) > 1 {
				require.Len(t, indices, len(roots)+1, // +1 means the default (nil: 0) is included
					"indices %v should include all roots %v", indices, roots)
			}
			// each root should be included in the indices
			for _, root := range roots {
				_, ok := indices[root]
				require.True(t, ok, "each root should be included in the indices")
			}

			logger.Info().Msgf("sub trie checkpoint stored, indices: %v, node count: %v, checksum: %v",
				indices, nodeCount, checksum)

			// all the nodes
			nodes, err := readCheckpointSubTrie(dir, file, index, checksum)
			require.NoError(t, err)

			for _, root := range roots {
				if root == nil {
					continue
				}
				index := indices[root]
				require.Equal(t, root.Hash(), nodes[index].Hash(),
					"readCheckpointSubTrie should return nodes where the root should be found "+
						"by the index specified by the indices returned by storeCheckpointSubTrie")
			}
		})
	}
}

func randomNode() *node.Node {
	var randomPath ledger.Path
	rand.Read(randomPath[:])

	var randomHashValue hash.Hash
	rand.Read(randomHashValue[:])

	return node.NewNode(256, nil, nil, randomPath, nil, randomHashValue)
}
func TestGetNodesByIndex(t *testing.T) {
	n := 10
	ns := make([]*node.Node, n)
	for i := 0; i < n; i++ {
		ns[i] = randomNode()
	}
	subtrieNodes := [][]*node.Node{
		[]*node.Node{nil, ns[0], ns[1]},
		[]*node.Node{nil, ns[2]},
		[]*node.Node{nil},
		[]*node.Node{nil},
	}
	topLevelNodes := []*node.Node{nil, ns[3]}

	for i := uint64(1); i <= 4; i++ {
		fmt.Println(i)
		node, err := getNodeByIndex(subtrieNodes, topLevelNodes, i)
		require.NoError(t, err, "cannot get node by index", i)
		require.Equal(t, ns[i-1], node, "got wrong node by index", i)
	}
}

func TestWriteAndReadCheckpointV6(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createSimpleTrie(t)
		fileName := "checkpoint"
		logger := unittest.Logger()
		require.NoErrorf(t, StoreCheckpointV6(tries, dir, fileName, &logger), "fail to store checkpoint")
		decoded, err := ReadCheckpointV6(dir, fileName, &logger)
		require.NoErrorf(t, err, "fail to read checkpoint %v/%v", dir, fileName)
		requireTriesEqual(t, tries, decoded)
	})
}
