package wal

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
)

func Test_LoadingV1Checkpoint(t *testing.T) {

	expectedRootHash := [4]ledger.RootHash{
		mustToHash("568f4ec740fe3b5de88034cb7b1fbddb41548b068f31aebc8ae9189e429c5749"), // empty trie root hash
		mustToHash("f53f9696b85b7428227f1b39f40b2ce07c175f58dea2b86cb6f84dc7c9fbeabd"),
		mustToHash("7ac8daf34733cce3d5d03b5a1afde33a572249f81c45da91106412e94661e109"),
		mustToHash("63df641430e5e0745c3d99ece6ac209467ccfdb77e362e7490a830db8e8803ae"),
	}

	tries, err := LoadCheckpoint("test_data/checkpoint.v1")
	require.NoError(t, err)
	require.Equal(t, len(expectedRootHash), len(tries))

	for i, trie := range tries {
		require.Equal(t, expectedRootHash[i], trie.RootHash())
		require.True(t, trie.RootNode().VerifyCachedHash())
	}
}

func Test_LoadingV3Checkpoint(t *testing.T) {

	expectedRootHash := [4]ledger.RootHash{
		mustToHash("568f4ec740fe3b5de88034cb7b1fbddb41548b068f31aebc8ae9189e429c5749"), // empty trie root hash
		mustToHash("f53f9696b85b7428227f1b39f40b2ce07c175f58dea2b86cb6f84dc7c9fbeabd"),
		mustToHash("7ac8daf34733cce3d5d03b5a1afde33a572249f81c45da91106412e94661e109"),
		mustToHash("63df641430e5e0745c3d99ece6ac209467ccfdb77e362e7490a830db8e8803ae"),
	}

	tries, err := LoadCheckpoint("test_data/checkpoint.v3")
	require.NoError(t, err)
	require.Equal(t, len(expectedRootHash), len(tries))

	for i, trie := range tries {
		require.Equal(t, expectedRootHash[i], trie.RootHash())
		require.True(t, trie.RootNode().VerifyCachedHash())
	}
}

func mustToHash(s string) ledger.RootHash {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	h, err := ledger.ToRootHash(b)
	if err != nil {
		panic(err)
	}
	return h
}

/*
// CreateCheckpointV3 is used to create checkpoint.v3 test file used by Test_LoadingV3Checkpoint.
func CreateCheckpointV3() {

	f, err := mtrie.NewForest(size*10, metricsCollector, func(tree *trie.MTrie) error { return nil })
	require.NoError(t, err)

	emptyTrie := trie.NewEmptyMTrie()

	// key: 0000...
	p1 := utils.PathByUint8(1)
	v1 := utils.LightPayload8('A', 'a')

	// key: 0100....
	p2 := utils.PathByUint8(64)
	v2 := utils.LightPayload8('B', 'b')

	paths := []ledger.Path{p1, p2}
	payloads := []ledger.Payload{*v1, *v2}

	trie1, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads, true)
	require.NoError(t, err)

	// trie1
	//              n4
	//             /
	//            /
	//          n3
	//        /     \
	//      /         \
	//   n1 (p1/v1)     n2 (p2/v2)
	//

	f.AddTrie(trie1)

	// New trie reuses its parent's left sub-trie.

	// key: 1000...
	p3 := utils.PathByUint8(128)
	v3 := utils.LightPayload8('C', 'c')

	// key: 1100....
	p4 := utils.PathByUint8(192)
	v4 := utils.LightPayload8('D', 'd')

	paths = []ledger.Path{p3, p4}
	payloads = []ledger.Payload{*v3, *v4}

	trie2, err := trie.NewTrieWithUpdatedRegisters(trie1, paths, payloads, true)
	require.NoError(t, err)

	// trie2
	//              n8
	//             /   \
	//            /      \
	//          n3       n7
	//       (shared)   /   \
	//                /       \
	//              n5         n6
	//            (p3/v3)    (p4/v4)

	f.AddTrie(trie2)

	// New trie reuses its parent's right sub-trie, and left sub-trie's leaf node.

	// key: 0000...
	v5 := utils.LightPayload8('E', 'e')

	paths = []ledger.Path{p1}
	payloads = []ledger.Payload{*v5}

	trie3, err := trie.NewTrieWithUpdatedRegisters(trie2, paths, payloads, true)
	require.NoError(t, err)

	// trie3
	//              n11
	//             /   \
	//            /      \
	//          n10       n7
	//         /   \    (shared)
	//       /       \
	//     n9         n2
	//  (p1/v5)    (shared)

	f.AddTrie(trie3)

	flattenedForest, err := flattener.FlattenForest(f)
	require.NoError(t, err)

	file, err := os.OpenFile("checkpoint.v3", os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	require.NoError(t, err)

	err = realWAL.StoreCheckpoint(flattenedForest, file)
	require.NoError(t, err)

	file.Close()
}
*/
