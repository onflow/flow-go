package trie_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/partial/ptrie"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

// There are some steps to test a Merkle trie implementation:
//
// 1. Test basic insertion and lookup operations:
//    Verify that we can insert key-value pairs (path and payload) to the trie and
//    look them up using their key (path).
//    See:
// 				Test_Lookup_AddUntilFull
// 				Test_Lookup_RemoveUntilEmpty
//
// 2. Test hash computation:
//    Verify that the hash values of the nodes in the trie are correctly computed based on the
//    hash values of their children.
//		See:
//				Test_RootHash_AddUntilFull
//				Test_RootHash_RemoveUntilEmpty
//
// 3. Test proof generation:
//    Verify that the implementation correctly verifies the proof of key-value pairs given its key
//    and the root node hash.
// 		See:
//				Test_Prove_AddUntilFull
//
// For simplicity, we use the top 2-levels to test the above properties. With a 2-level tree, it can store
//  4 payloads on the leaf nodes [n1,n2,n3,n4] on the 4 paths: [00,01,01,11]
//         n7
//     0/     \1
//     n5     n6
//   0/ \1  0/ \1
//   n1 n2  n3 n4
//
// As the 256-levels mtrie implemented compaction, which allows a subtrie with only one payload to be compacted
// into a single compacted leaf node, we can use the 256 levels mtrie as a 2-levels tree, and it should have the
// same merkle trie properties. This allows us to write simplier tests to verify all the above properties, as
// if we are testing the implementation of a 2-levels tree.

// a full trie with 4 payloads at bottom
func Test_FullTrie4_Update(t *testing.T) {
	// prepare paths for a trie with only 2 levels, which has 4 payloads in total
	paths := []ledger.Path{
		testutils.PathByUint8(0),           // 00000000...
		testutils.PathByUint8(1 << 6),      // 01000000...
		testutils.PathByUint8(1 << 7),      // 10000000...
		testutils.PathByUint8(1<<7 + 1<<6), // 11000000...
	}

	// prepare payloads, the value doesn't matter
	payloads := make([]ledger.Payload, 0, len(paths))
	for i := range paths {
		payloads = append(payloads, *payloadFromInt16(uint16(i)))
	}

	t0 := trie.NewEmptyMTrie()
	payloadStorage := unittest.CreateMockPayloadStore()

	// create a full trie by adding payloads at all 4 paths to an empty trie
	t1, depth, err := trie.NewTrieWithUpdatedRegisters(t0, paths, payloads, true, payloadStorage)
	require.NoError(t, err)
	require.Equal(t, uint16(2), depth)

	// create a updated trie that is updated at one path,
	// so prepare the updated path first
	paths2 := []ledger.Path{
		paths[2], // 10000000...
	}
	payloads2 := []ledger.Payload{
		*payloadFromInt16(uint16(100)),
	}
	// create the updated trie at the given path
	t2, _, err := trie.NewTrieWithUpdatedRegisters(t1, paths2, payloads2, true, payloadStorage)
	require.NoError(t, err)

	// empty trie should have nothing stored at the path
	from0, err := t0.ReadSinglePayload(paths2[0], payloadStorage)
	require.NoError(t, err)
	require.True(t, from0.IsEmpty())

	// the full trie should have original payload at the path
	from1, err := t1.ReadSinglePayload(paths2[0], payloadStorage)
	require.NoError(t, err)
	require.True(t, from1.Equals(&payloads[2]))

	// the updated trie should have updated payload at the path
	from2, err := t2.ReadSinglePayload(paths2[0], payloadStorage)
	require.NoError(t, err)
	require.True(t, from2.Equals(&payloads2[0]))

}

func payloadFromInt16(i uint16) *ledger.Payload {
	return testutils.LightPayload(i, i)
}

func getAllPathsAtLevel2() []ledger.Path {
	return []ledger.Path{
		testutils.PathByUint8(0),           // 00000000...
		testutils.PathByUint8(1 << 6),      // 01000000...
		testutils.PathByUint8(1 << 7),      // 10000000...
		testutils.PathByUint8(1<<7 + 1<<6), // 11000000...
	}
}

// Verify simple insertaion and lookup operation
func Test_AddRemovePayload(t *testing.T) {
	// prepare paths for a 2-levels tree, which can store 4 payloads in total on different paths.
	paths := getAllPathsAtLevel2()

	// prepare payloads, the value doesn't matter
	payloads := make([]ledger.Payload, 0, len(paths))
	for i := range paths {
		payloads = append(payloads, *payloadFromInt16(uint16(i)))
	}

	t0 := trie.NewEmptyMTrie()
	payloadStorage := unittest.CreateMockPayloadStore()

	fmt.Println("=== adding 1 payload at 00000000")
	t1, depth, err := trie.NewTrieWithUpdatedRegisters(t0, paths[0:1], payloads[0:1], true, payloadStorage)
	require.NoError(t, err)
	require.Equal(t, uint16(0), depth)

	fmt.Println("=== adding 1 payload at 01000000")
	t2, depth, err := trie.NewTrieWithUpdatedRegisters(t1, paths[1:2], payloads[1:2], true, payloadStorage)
	require.NoError(t, err)
	require.Equal(t, uint16(2), depth)

	emptyPayloads := []ledger.Payload{
		*ledger.EmptyPayload(),
	}

	fmt.Println("=== removing 1 payload at 01000000")
	t3, depth, err := trie.NewTrieWithUpdatedRegisters(t2, paths[1:2], emptyPayloads, true, payloadStorage)
	require.NoError(t, err)
	require.Equal(t, uint16(2), depth)
	require.True(t, t1.Equals(t3))

	fmt.Println("=== removing 1 payload at 00000000")
	t4, depth, err := trie.NewTrieWithUpdatedRegisters(t3, paths[0:1], emptyPayloads, true, payloadStorage)
	require.NoError(t, err)
	require.Equal(t, uint16(0), depth)
	require.True(t, t0.Equals(t4))
}

type pathAndPayload struct {
	Paths    []ledger.Path
	Payloads []ledger.Payload
}

// Verify the basic insertaion and lookup operation.
// In order to verify we can lookup from a 2-levels tree in any state, we could enum all
// the possible 2-levels tree state by adding sets of payloads to different paths.
// For a 2-levels tree, there are (4 * 3 * 2 * 1 = 24) possibilities:
// For instance,
// adding 4 payloads will create a full tree:
//       n7
//      /  \
//    n5    n6
//   / \   / \
//  n1 n2 n3 n4
//
// adding 2 payloads at [00, 10], will create a tree:
//      n3
//		 / \
// 		n1  n2
func Test_Lookup_AddUntilFull(t *testing.T) {
	// prepare paths for a 2-levels tree, which can store 4 payloads in total on different paths.
	paths := getAllPathsAtLevel2()
	payloads := testutils.RandomPayloads(4, 10, 20)

	// enum the 24 possibilities for different paths set
	// each group contains a unique set of paths to be stored in a trie
	groups := getPermutationPathsPayloads(paths, payloads)

	// for each unique set of paths, create a tree using it, and verify the payload can be
	// retrieved by the paths, and verify retrieving payload for non existing path will return empty payload
	for _, group := range groups {
		payloadStorage := unittest.CreateMockPayloadStore()
		updated, _, err := trie.NewTrieWithUpdatedRegisters(
			trie.NewEmptyMTrie(), group.Paths, group.Payloads, true, payloadStorage)
		require.NoError(t, err)

		nonExistingPaths := findNonExistingPaths(paths, group.Paths)

		for i, path := range group.Paths {
			payload := group.Payloads[i]
			foundPayload, err := updated.ReadSinglePayload(path, payloadStorage)
			require.NoError(t, err)
			require.True(t, foundPayload.Equals(&payload))
		}

		for _, nonExistingPath := range nonExistingPaths {
			shouldBeEmpty, err := updated.ReadSinglePayload(nonExistingPath, payloadStorage)
			require.NoError(t, err)
			require.True(t, shouldBeEmpty.IsEmpty())
		}
	}
}

func getPermutationPathsPayloads(paths []ledger.Path, payloads []*ledger.Payload) []*pathAndPayload {
	permutation := make([]*pathAndPayload, 0)
	for i, path := range paths {
		payload := payloads[i]

		count := len(permutation)
		for i := 0; i < count; i++ {
			group := permutation[i]
			permutation = append(permutation, &pathAndPayload{
				Paths:    append(group.Paths, path),
				Payloads: append(group.Payloads, *payload),
			})
		}

		permutation = append(permutation, &pathAndPayload{
			Paths:    []ledger.Path{path},
			Payloads: []ledger.Payload{*payload},
		})
	}
	return permutation
}

func findNonExistingPaths(all []ledger.Path, paths []ledger.Path) []ledger.Path {
	lookup := make(map[ledger.Path]struct{})
	for _, path := range paths {
		lookup[path] = struct{}{}
	}

	missing := make([]ledger.Path, 0, len(all))
	for _, path := range all {
		_, ok := lookup[path]
		if !ok {
			missing = append(missing, path)
		}
	}
	return missing
}

func nEmptyPayloads(n int) []ledger.Payload {
	emptyPayloads := make([]ledger.Payload, 0, n)
	for i := 0; i < n; i++ {
		emptyPayloads = append(emptyPayloads, *ledger.EmptyPayload())
	}
	return emptyPayloads
}

func toPayloads(pointers []*ledger.Payload) []ledger.Payload {
	payloads := make([]ledger.Payload, 0, len(pointers))
	for _, p := range pointers {
		payloads = append(payloads, *p)
	}
	return payloads
}

// Verify that existing payload can be removed from the trie, and looking up removed payload should
// return empty payload
func Test_Lookup_RemoveUntilEmpty(t *testing.T) {
	// prepare paths for a 2-levels tree, which can store 4 payloads in total on different paths.
	paths := getAllPathsAtLevel2()
	randoms := testutils.RandomPayloads(4, 10, 20)
	payloads := toPayloads(randoms)

	groups := getPermutationPathsPayloads(paths, randoms)

	// enum 24 different sets of paths to be removed from a full 2-levels tree,
	// and verify the lookup operation.
	for _, group := range groups {
		payloadStorage := unittest.CreateMockPayloadStore()

		// create a full trie with all 4 payloads added
		fullTrie, _, err := trie.NewTrieWithUpdatedRegisters(
			trie.NewEmptyMTrie(), paths, payloads, true, payloadStorage)
		require.NoError(t, err)

		// remove some payloads at the given paths
		updated, _, err := trie.NewTrieWithUpdatedRegisters(
			fullTrie, group.Paths, nEmptyPayloads(len(group.Paths)), true, payloadStorage)
		require.NoError(t, err)

		// there are 2 cases to verify here:
		// 1. deleting an existing payload, the deleted payload should no longer exists
		// 2. deleting an non-existing payload, the non-existing payload should still not exist

		// build a lookup to check if which payloads exist and which are not.
		lookup := toLookup(group.Paths, group.Payloads)

		// verify the 2 cases
		for _, path := range group.Paths {
			payloadRead, err := updated.ReadSinglePayload(path, payloadStorage)
			require.NoError(t, err)

			deletedPayload, isDeleted := lookup[path]
			if isDeleted {
				require.True(t, payloadRead.IsEmpty())
			} else {
				require.True(t, payloadRead.Equals(&deletedPayload))
			}
		}
	}
}

// verify the root hash can be correctly computed
func Test_RootHash_AddUntilFull(t *testing.T) {
	paths := getAllPathsAtLevel2()
	payloads := testutils.RandomPayloads(4, 10, 20)

	// enum 24 different sets of payloads, and create different trees using each set of payloads
	// verify the root hash is correct
	groups := getPermutationPathsPayloads(paths, payloads)
	for _, group := range groups {
		payloadStorage := unittest.CreateMockPayloadStore()
		updated, _, err := trie.NewTrieWithUpdatedRegisters(
			trie.NewEmptyMTrie(), group.Paths, group.Payloads, true, payloadStorage)
		require.NoError(t, err)

		expectedRootHash := computeRootHash(paths, group.Paths, group.Payloads)
		require.Equal(t, expectedRootHash, updated.RootHash())
	}
}

// verify the root hash can be correctly computed when tree nodes are removed.
func Test_RootHash_RemoveUntilEmpty(t *testing.T) {
	// prepare paths for a trie with only 2 levels, which has 4 payloads in total
	paths := getAllPathsAtLevel2()
	randoms := testutils.RandomPayloads(4, 10, 20)
	payloads := toPayloads(randoms)

	groups := getPermutationPathsPayloads(paths, randoms)

	for _, group := range groups {
		payloadStorage := unittest.CreateMockPayloadStore()

		// create a full trie with all 4 payloads added
		fullTrie, _, err := trie.NewTrieWithUpdatedRegisters(
			trie.NewEmptyMTrie(), paths, payloads, true, payloadStorage)
		require.NoError(t, err)

		// remove some payloads at the given paths
		updated, _, err := trie.NewTrieWithUpdatedRegisters(
			fullTrie, group.Paths, nEmptyPayloads(len(group.Paths)), true, payloadStorage)
		require.NoError(t, err)

		// lookup for deleted payload
		deletedLookup := toLookup(group.Paths, group.Payloads)

		// find the remaining payloads
		remaining := &pathAndPayload{}
		for i, path := range paths {
			_, isDeleted := deletedLookup[path]
			if !isDeleted {
				remaining.Paths = append(remaining.Paths, path)
				remaining.Payloads = append(remaining.Payloads, payloads[i])
			}
		}

		// compute the expected root hash with remaining payloads stored in a trie,
		// and use it to verify
		expectedRootHash := computeRootHash(paths, remaining.Paths, remaining.Payloads)
		require.Equal(t, expectedRootHash, updated.RootHash(),
			fmt.Sprintf("root hash mismatch after deleting paths [%v] from a full trie", printPaths(group.Paths)))
	}
}

func printPaths(paths []ledger.Path) string {
	all := getAllPathsAtLevel2()
	lookup := map[ledger.Path]string{
		all[0]: "00",
		all[1]: "01",
		all[2]: "10",
		all[3]: "11",
	}
	shorts := make([]string, 0)
	for _, path := range paths {
		shorts = append(shorts, lookup[path])
	}
	return strings.Join(shorts, ",")
}

// verify the generation of proof.for existing paths
// enum all different sets of payloads and create all unique 2-level trees, and verify
// it can generate inclusion proof for existing payloads, and non-inclusion proof for non-existing payloads
func Test_Prove_PartialPathsFromFullTree(t *testing.T) {
	paths := getAllPathsAtLevel2()
	randoms := testutils.RandomPayloads(4, 10, 20)
	payloads := toPayloads(randoms)

	groups := getPermutationPathsPayloads(paths, randoms)

	// for each unique set of payloads, using them to create unique 2 level tree.
	// verify that a batch proof can be created for existing payloads
	// verify that a partial trie can be created by a light client from a batch proof and the trie root hash
	// verify that the same payloads can be retrieved from the partial trie
	// verify that retrieving non existing payload from the partial trie would get ErrMissingPath error
	for _, group := range groups {

		payloadStorage := unittest.CreateMockPayloadStore()
		fullTrie, _, err := trie.NewTrieWithUpdatedRegisters(
			trie.NewEmptyMTrie(), paths, payloads, true, payloadStorage)
		require.NoError(t, err)

		proof, err := fullTrie.UnsafeProofs(group.Paths, payloadStorage)
		require.NoError(t, err)

		proofTrie, err := ptrie.NewPSMT(fullTrie.RootHash(), proof)
		require.NoError(t, err)

		for i, path := range group.Paths {
			payload := group.Payloads[i]
			foundPayload, err := proofTrie.GetSinglePayload(path)
			require.NoError(t, err)
			require.False(t, foundPayload.IsEmpty(), fmt.Sprintf("an added payload at path %v should be found, but not",
				path))
			require.True(t, foundPayload.Equals(&payload),
				fmt.Sprintf("expect the payload from proof to equal to the one from the trie, %v != %v",
					foundPayload, payload))
		}

		nonExistingPaths := findNonExistingPaths(paths, group.Paths)
		if len(nonExistingPaths) > 0 {
			_, err = proofTrie.Get(nonExistingPaths)
			require.NotNil(t, err, fmt.Sprintf("paths to add %v, paths not exist %v", group.Paths, nonExistingPaths))

			errMissing, ok := err.(*ptrie.ErrMissingPath)
			require.True(t, ok, err)
			require.Equal(t, nonExistingPaths, errMissing.Paths)
		}
	}
}

// verify the generation of proof for non-existing paths
// enum all different sets of payloads and create 24 unique 2-level trees, and verify
// it can generate inclusion proof for existing payloads, and non-inclusion proof for non-existing payloads
func Test_Prove_NonExistingPaths(t *testing.T) {
	paths := getAllPathsAtLevel2()
	randoms := testutils.RandomPayloads(4, 10, 20)

	groups := getPermutationPathsPayloads(paths, randoms)

	// for each unique set of payloads, using them to create unique 2 level tree.
	// verify that a batch proof can be created for both existing payloads and non-existing paths
	// verify that a partial trie can be created by a light client from a batch proof and the trie root hash
	// verify that the same payloads can be retrieved from the partial trie
	// verify that retrieving non existing payload from the partial trie would get ErrMissingPath error
	for _, group := range groups {

		payloadStorage := unittest.CreateMockPayloadStore()
		updatedTrie, _, err := trie.NewTrieWithUpdatedRegisters(
			trie.NewEmptyMTrie(), group.Paths, group.Payloads, true, payloadStorage)
		require.NoError(t, err)

		nonExistingPaths := findNonExistingPaths(paths, group.Paths)

		// before generating the proof, the trie needs to be updated with empty payloads
		// on the paths to be used for generating proofs
		// note: prune: false
		withEmptyPayloads, _, err := trie.NewTrieWithUpdatedRegisters(
			updatedTrie, nonExistingPaths, nEmptyPayloads(len(nonExistingPaths)), false, payloadStorage)
		require.NoError(t, err)

		// proof can be generated for all paths including existing and non-existing paths
		proof, err := withEmptyPayloads.UnsafeProofs(paths, payloadStorage)
		require.NoError(t, err)

		proofTrie, err := ptrie.NewPSMT(updatedTrie.RootHash(), proof)
		require.NoError(t, err)

		// for existing paths, the light client can retrieve the payload
		for i, path := range group.Paths {
			payload := group.Payloads[i]
			foundPayload, err := proofTrie.GetSinglePayload(path)
			require.NoError(t, err)
			require.False(t, foundPayload.IsEmpty(), fmt.Sprintf("an added payload at path %v to trie with paths %v should be found, but not",
				path, group.Paths))
			require.True(t, foundPayload.Equals(&payload),
				fmt.Sprintf("expect the payload from proof to equal to the one from the trie, %v != %v",
					foundPayload, payload))
		}

		// for nonExistingPaths, the light client will get empty payload for them
		if len(nonExistingPaths) > 0 {
			emptyPayloads, err := proofTrie.Get(nonExistingPaths)
			require.NoError(t, err)

			for _, empty := range emptyPayloads {
				require.True(t, empty.IsEmpty(), empty)
			}
		}
	}
}

func toLookup(paths []ledger.Path, payloads []ledger.Payload) map[ledger.Path]ledger.Payload {
	lookup := make(map[ledger.Path]ledger.Payload)

	for i, path := range paths {
		lookup[path] = payloads[i]
	}
	return lookup
}

func computeRootHash(paths []ledger.Path, groupPath []ledger.Path, groupPayloads []ledger.Payload) ledger.RootHash {
	path1, path2, path3, path4 := paths[0], paths[1], paths[2], paths[3]
	lookup := toLookup(groupPath, groupPayloads)

	//       h7
	//   0/     \1
	//   h5     h6
	// 0/ \1  0/ \1
	// h1 h2  h3 h4
	h1 := getLeafHash(path1, lookup) // 00
	h2 := getLeafHash(path2, lookup) // 01
	h3 := getLeafHash(path3, lookup) // 10
	h4 := getLeafHash(path4, lookup) // 11
	h5 := hash.HashInterNode(h1, h2)
	h6 := hash.HashInterNode(h3, h4)
	h7 := hash.HashInterNode(h5, h6)
	return ledger.RootHash(h7)
}

func getLeafHash(path ledger.Path, lookup map[ledger.Path]ledger.Payload) hash.Hash {
	payload, ok := lookup[path]
	height := 256 - 2
	if !ok {
		return ledger.GetDefaultHashForHeight(height)
	}

	leaf := node.NewLeaf(path, &payload, height)
	return leaf.Hash()
}
