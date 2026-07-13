package ptrie

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/module/metrics"
)

func withForest(
	t *testing.T,
	pathByteSize int,
	numberOfActiveTries int, f func(t *testing.T, f *mtrie.Forest)) {

	forest, err := mtrie.NewForest(numberOfActiveTries, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	f(t, forest)
}

func TestPartialTrieEmptyTrie(t *testing.T) {

	pathByteSize := 32
	withForest(t, pathByteSize, 10, func(t *testing.T, f *mtrie.Forest) {

		// add path1 to the empty trie
		// 00000000...0 (0)
		path1 := testutils.PathByUint16(0)
		payload1 := testutils.LightPayload('A', 'a')

		paths := []ledger.Path{path1}
		payloads := []*ledger.Payload{payload1}

		rootHash := f.GetEmptyRootHash()
		r := &ledger.TrieRead{RootHash: rootHash, Paths: paths}
		bp, err := f.Proofs(r)
		require.NoError(t, err, "error getting proofs values")

		psmt, err := NewPSMT(rootHash, bp)
		require.NoError(t, err, "error building partial trie")
		ensureRootHash(t, rootHash, psmt)

		u := &ledger.TrieUpdate{RootHash: rootHash, Paths: paths, Payloads: payloads}
		rootHash, err = f.Update(u)
		require.NoError(t, err, "error updating trie")

		_, err = psmt.Update(paths, payloads)
		require.NoError(t, err, "error updating psmt")
		ensureRootHash(t, rootHash, psmt)

		updatedPayload1 := testutils.LightPayload('B', 'b')
		payloads = []*ledger.Payload{updatedPayload1}

		u = &ledger.TrieUpdate{RootHash: rootHash, Paths: paths, Payloads: payloads}
		rootHash, err = f.Update(u)
		require.NoError(t, err, "error updating trie")

		_, err = psmt.Update(paths, payloads)
		require.NoError(t, err, "error updating psmt")
		ensureRootHash(t, rootHash, psmt)
	})
}

// TestPartialTrieGet gets payloads from existent and non-existent paths.
func TestPartialTrieGet(t *testing.T) {

	pathByteSize := 32
	withForest(t, pathByteSize, 10, func(t *testing.T, f *mtrie.Forest) {

		path1 := testutils.PathByUint16(0)
		payload1 := testutils.LightPayload('A', 'a')

		path2 := testutils.PathByUint16(1)
		payload2 := testutils.LightPayload('B', 'b')

		paths := []ledger.Path{path1, path2}
		payloads := []*ledger.Payload{payload1, payload2}

		u := &ledger.TrieUpdate{RootHash: f.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
		rootHash, err := f.Update(u)
		require.NoError(t, err, "error updating trie")

		r := &ledger.TrieRead{RootHash: rootHash, Paths: paths}
		bp, err := f.Proofs(r)
		require.NoError(t, err, "error getting batch proof")

		psmt, err := NewPSMT(rootHash, bp)
		require.NoError(t, err, "error building partial trie")
		ensureRootHash(t, rootHash, psmt)

		t.Run("non-existent key", func(t *testing.T) {
			path3 := testutils.PathByUint16(2)
			path4 := testutils.PathByUint16(4)

			nonExistentPaths := []ledger.Path{path3, path4}
			retPayloads, err := psmt.Get(nonExistentPaths)
			require.Nil(t, retPayloads)

			e, ok := err.(*ErrMissingPath)
			require.True(t, ok)
			assert.Equal(t, 2, len(e.Paths))
			require.Equal(t, path3, e.Paths[0])
			require.Equal(t, path4, e.Paths[1])
		})

		t.Run("existent key", func(t *testing.T) {
			retPayloads, err := psmt.Get(paths)
			require.NoError(t, err)
			require.Equal(t, len(paths), len(retPayloads))
			require.Equal(t, payload1, retPayloads[0])
			require.Equal(t, payload2, retPayloads[1])
		})

		t.Run("mix of existent and non-existent keys", func(t *testing.T) {
			path3 := testutils.PathByUint16(2)
			path4 := testutils.PathByUint16(4)

			retPayloads, err := psmt.Get([]ledger.Path{path1, path2, path3, path4})
			require.Nil(t, retPayloads)

			e, ok := err.(*ErrMissingPath)
			require.True(t, ok)
			assert.Equal(t, 2, len(e.Paths))
			require.Equal(t, path3, e.Paths[0])
			require.Equal(t, path4, e.Paths[1])
		})
	})
}

// TestPartialTrieGetSinglePayload gets single payload from existent/non-existent path.
func TestPartialTrieGetSinglePayload(t *testing.T) {

	pathByteSize := 32
	withForest(t, pathByteSize, 10, func(t *testing.T, f *mtrie.Forest) {

		path1 := testutils.PathByUint16(0)
		payload1 := testutils.LightPayload('A', 'a')

		path2 := testutils.PathByUint16(1)
		payload2 := testutils.LightPayload('B', 'b')

		paths := []ledger.Path{path1, path2}
		payloads := []*ledger.Payload{payload1, payload2}

		u := &ledger.TrieUpdate{RootHash: f.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
		rootHash, err := f.Update(u)
		require.NoError(t, err, "error updating trie")

		r := &ledger.TrieRead{RootHash: rootHash, Paths: paths}
		bp, err := f.Proofs(r)
		require.NoError(t, err, "error getting batch proof")

		psmt, err := NewPSMT(rootHash, bp)
		require.NoError(t, err, "error building partial trie")
		ensureRootHash(t, rootHash, psmt)

		retPayload, err := psmt.GetSinglePayload(path1)
		require.NoError(t, err)
		require.Equal(t, payload1, retPayload)

		retPayload, err = psmt.GetSinglePayload(path2)
		require.NoError(t, err)
		require.Equal(t, payload2, retPayload)

		path3 := testutils.PathByUint16(2)

		retPayload, err = psmt.GetSinglePayload(path3)
		require.Nil(t, retPayload)

		var errMissingPath *ErrMissingPath
		require.ErrorAs(t, err, &errMissingPath)
		missingPath := err.(*ErrMissingPath)
		require.Equal(t, 1, len(missingPath.Paths))
		require.Equal(t, path3, missingPath.Paths[0])
	})
}

func TestPartialTrieLeafUpdates(t *testing.T) {

	pathByteSize := 32
	withForest(t, pathByteSize, 10, func(t *testing.T, f *mtrie.Forest) {

		path1 := testutils.PathByUint16(0)
		payload1 := testutils.LightPayload('A', 'a')
		updatedPayload1 := testutils.LightPayload('B', 'b')

		path2 := testutils.PathByUint16(1)
		payload2 := testutils.LightPayload('C', 'c')
		updatedPayload2 := testutils.LightPayload('D', 'd')

		path3 := testutils.PathByUint16(2)
		payload3 := testutils.LightPayload('E', 'e')

		paths := []ledger.Path{path1, path2}
		payloads := []*ledger.Payload{payload1, payload2}

		u := &ledger.TrieUpdate{RootHash: f.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
		rootHash, err := f.Update(u)
		require.NoError(t, err, "error updating trie")

		r := &ledger.TrieRead{RootHash: rootHash, Paths: paths}
		bp, err := f.Proofs(r)
		require.NoError(t, err, "error getting batch proof")

		psmt, err := NewPSMT(rootHash, bp)
		require.NoError(t, err, "error building partial trie")
		ensureRootHash(t, rootHash, psmt)

		payloads = []*ledger.Payload{updatedPayload1, updatedPayload2}
		rootHash, err = f.Update(&ledger.TrieUpdate{RootHash: rootHash, Paths: paths, Payloads: payloads})
		require.NoError(t, err, "error updating trie")

		_, err = psmt.Update(paths, payloads)
		require.NoError(t, err, "error updating psmt")
		ensureRootHash(t, rootHash, psmt)

		// Update on non-existent leafs
		_, err = psmt.Update([]ledger.Path{path3}, []*ledger.Payload{payload3})
		missingPathErr, ok := err.(*ErrMissingPath)
		require.True(t, ok)
		require.Equal(t, 1, len(missingPathErr.Paths))
		require.Equal(t, path3, missingPathErr.Paths[0])
	})

}

func TestPartialTrieMiddleBranching(t *testing.T) {

	pathByteSize := 32
	withForest(t, pathByteSize, 10, func(t *testing.T, f *mtrie.Forest) {

		path1 := testutils.PathByUint16(0)
		payload1 := testutils.LightPayload('A', 'a')
		updatedPayload1 := testutils.LightPayload('B', 'b')

		path2 := testutils.PathByUint16(2)
		payload2 := testutils.LightPayload('C', 'c')
		updatedPayload2 := testutils.LightPayload('D', 'd')

		path3 := testutils.PathByUint16(8)
		payload3 := testutils.LightPayload('E', 'e')
		updatedPayload3 := testutils.LightPayload('F', 'f')

		paths := []ledger.Path{path1, path2, path3}
		payloads := []*ledger.Payload{payload1, payload2, payload3}

		rootHash := f.GetEmptyRootHash()
		bp, err := f.Proofs(&ledger.TrieRead{RootHash: rootHash, Paths: paths})
		require.NoError(t, err, "error getting batch proof")

		psmt, err := NewPSMT(rootHash, bp)
		require.NoError(t, err, "error building partial trie")
		ensureRootHash(t, f.GetEmptyRootHash(), psmt)

		// first update
		rootHash, err = f.Update(&ledger.TrieUpdate{RootHash: rootHash, Paths: paths, Payloads: payloads})
		require.NoError(t, err, "error updating trie")

		_, err = psmt.Update(paths, payloads)
		require.NoError(t, err, "error updating psmt")
		ensureRootHash(t, rootHash, psmt)

		// second update
		payloads = []*ledger.Payload{updatedPayload1, updatedPayload2, updatedPayload3}
		rootHash, err = f.Update(&ledger.TrieUpdate{RootHash: rootHash, Paths: paths, Payloads: payloads})
		require.NoError(t, err, "error updating trie")

		_, err = psmt.Update(paths, payloads)
		require.NoError(t, err, "error updating psmt")
		ensureRootHash(t, rootHash, psmt)
	})

}

func TestPartialTrieRootUpdates(t *testing.T) {

	pathByteSize := 32
	withForest(t, pathByteSize, 10, func(t *testing.T, f *mtrie.Forest) {

		path1 := testutils.PathByUint16(0)
		payload1 := testutils.LightPayload('A', 'a')
		updatedPayload1 := testutils.LightPayload('B', 'b')
		//  10000....0
		path2 := testutils.PathByUint16(32768)
		payload2 := testutils.LightPayload('C', 'c')
		updatedPayload2 := testutils.LightPayload('D', 'd')

		paths := []ledger.Path{path1, path2}
		payloads := []*ledger.Payload{payload1, payload2}

		rootHash := f.GetEmptyRootHash()
		bp, err := f.Proofs(&ledger.TrieRead{RootHash: rootHash, Paths: paths})
		require.NoError(t, err, "error getting batch proof")

		psmt, err := NewPSMT(rootHash, bp)
		require.NoError(t, err, "error building partial trie")
		ensureRootHash(t, rootHash, psmt)

		// first update
		rootHash, err = f.Update(&ledger.TrieUpdate{RootHash: rootHash, Paths: paths, Payloads: payloads})
		require.NoError(t, err, "error updating trie")

		pRootHash, err := psmt.Update(paths, payloads)
		require.NoError(t, err, "error updating psmt")
		assert.Equal(t, rootHash, pRootHash, "rootNode hash doesn't match [after update]")

		// second update
		payloads = []*ledger.Payload{updatedPayload1, updatedPayload2}
		rootHash, err = f.Update(&ledger.TrieUpdate{RootHash: rootHash, Paths: paths, Payloads: payloads})
		require.NoError(t, err, "error updating trie")

		pRootHash, err = psmt.Update(paths, payloads)
		require.NoError(t, err, "error updating psmt")
		assert.Equal(t, rootHash, pRootHash, "rootNode hash doesn't match [after second update]")
	})

}

func TestMixProof(t *testing.T) {
	pathByteSize := 32
	withForest(t, pathByteSize, 10, func(t *testing.T, f *mtrie.Forest) {

		path1 := testutils.PathByUint16(0)
		payload1 := testutils.LightPayload('A', 'a')

		path2 := testutils.PathByUint16(2)
		updatedPayload2 := testutils.LightPayload('D', 'd')

		path3 := testutils.PathByUint16(8)
		payload3 := testutils.LightPayload('E', 'e')

		paths := []ledger.Path{path1, path3}
		payloads := []*ledger.Payload{payload1, payload3}

		rootHash := f.GetEmptyRootHash()
		rootHash, err := f.Update(&ledger.TrieUpdate{RootHash: rootHash, Paths: paths, Payloads: payloads})
		require.NoError(t, err, "error updating trie")

		paths = []ledger.Path{path1, path2, path3}

		bp, err := f.Proofs(&ledger.TrieRead{RootHash: rootHash, Paths: paths})
		require.NoError(t, err, "error getting batch proof")

		psmt, err := NewPSMT(rootHash, bp)
		require.NoError(t, err, "error building partial trie")
		ensureRootHash(t, rootHash, psmt)

		paths = []ledger.Path{path2, path3}
		payloads = []*ledger.Payload{updatedPayload2, updatedPayload2}

		rootHash, err = f.Update(&ledger.TrieUpdate{RootHash: rootHash, Paths: paths, Payloads: payloads})
		require.NoError(t, err, "error updating trie")

		pRootHash, err := psmt.Update(paths, payloads)
		require.NoError(t, err, "error updating partial trie")
		ensureRootHash(t, rootHash, psmt)
		assert.Equal(t, rootHash, pRootHash, "root2 hash doesn't match [%x] != [%x]", rootHash, pRootHash)
	})

}

func TestRandomProofs(t *testing.T) {
	pathByteSize := 32 // key size of 16 bits
	minPayloadSize := 2
	maxPayloadSize := 10
	experimentRep := 20
	for range experimentRep {
		withForest(t, pathByteSize, experimentRep+1, func(t *testing.T, f *mtrie.Forest) {

			// generate some random paths and payloads
			numberOfPaths := rand.Intn(256) + 1
			paths := testutils.RandomPaths(numberOfPaths)
			payloads := testutils.RandomPayloads(numberOfPaths, minPayloadSize, maxPayloadSize)
			// keep a subset as initial insert and keep the rest for reading default values
			split := rand.Intn(numberOfPaths)
			insertPaths := paths[:split]
			insertPayloads := payloads[:split]

			rootHash, err := f.Update(&ledger.TrieUpdate{RootHash: f.GetEmptyRootHash(), Paths: insertPaths, Payloads: insertPayloads})
			require.NoError(t, err, "error updating trie")

			// shuffle paths for read
			rand.Shuffle(len(paths), func(i, j int) {
				paths[i], paths[j] = paths[j], paths[i]
				payloads[i], payloads[j] = payloads[j], payloads[i]
			})

			bp, err := f.Proofs(&ledger.TrieRead{RootHash: rootHash, Paths: paths})
			require.NoError(t, err, "error getting batch proof")

			psmt, err := NewPSMT(rootHash, bp)
			require.NoError(t, err, "error building partial trie")
			ensureRootHash(t, rootHash, psmt)

			// select a subset of shuffled paths for random updates
			split = rand.Intn(numberOfPaths)
			updatePaths := paths[:split]
			updatePayloads := payloads[:split]
			// random updates
			rand.Shuffle(len(updatePayloads), func(i, j int) {
				updatePayloads[i], updatePayloads[j] = updatePayloads[j], updatePayloads[i]
			})

			rootHash2, err := f.Update(&ledger.TrieUpdate{RootHash: rootHash, Paths: updatePaths, Payloads: updatePayloads})
			require.NoError(t, err, "error updating trie")

			pRootHash2, err := psmt.Update(updatePaths, updatePayloads)
			require.NoError(t, err, "error updating partial trie")
			assert.Equal(t, pRootHash2, rootHash2, "root2 hash doesn't match [%x] != [%x]", rootHash2, pRootHash2)
		})
	}
}

// TODO add test for incompatible proofs [Byzantine milestone]
// TODO add test key not exist [Byzantine milestone]

func ensureRootHash(t *testing.T, expectedRootHash ledger.RootHash, psmt *PSMT) {
	if expectedRootHash != ledger.RootHash(psmt.root.Hash()) {
		t.Fatal("rootNode hash doesn't match")
	}
	if expectedRootHash != psmt.RootHash() {
		t.Fatal("rootNode hash doesn't match")
	}
	if expectedRootHash != ledger.RootHash(psmt.root.forceComputeHash()) {
		t.Fatal("rootNode hash doesn't match")
	}
}
