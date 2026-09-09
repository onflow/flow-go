package ptrie

import (
	"math/rand"
	"slices"
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

// These tests guard against fabricated register reads in NewPSMT. A byzantine source of proofs
// (e.g. an execution node's ChunkDataPack) must not be able to make GetSinglePayload/Get serve a
// value that differs from the committed state while still passing NewPSMT's root check.
//
// The proofs a verifier receives are untrusted: the Inclusion flag and Payload are fully
// attacker-controlled. Before the fix, NewPSMT computed a node's hash from the payload only for
// inclusion proofs, so a non-inclusion proof carrying a fabricated payload (or a duplicate proof
// for an already-proven path) left the node's hash at the honest default and slipped past the
// root check. NewPSMT now binds the payload to the node hash unconditionally, so any fabricated
// payload changes the node hash and the root check rejects it. That binding only holds for proof
// terminals that remain leaves: forceComputeHash recomputes interior nodes from their children and
// discards the payload-derived hash, while pathLookUp still serves their payload. NewPSMT therefore
// also rejects any proof whose terminal node has children, defeating truncated-Steps proofs that
// land on the root or on an interior ancestor of another proof's path.

var (
	// secPathP holds a committed register; secPathP2 is a sibling so the trie has a real branch.
	secPathP  = testutils.PathByUint16(1)     // bit 0 = 0
	secPathP2 = testutils.PathByUint16(3)     // bit 0 = 0
	secPathQ  = testutils.PathByUint16(32768) // bit 0 = 1, EMPTY register on the opposite subtree
	secPayVP  = testutils.LightPayload('A', 'a')
	secPayVP2 = testutils.LightPayload('B', 'b')
)

// secBuildCommittedState inserts the two honest registers and returns the committed root hash.
func secBuildCommittedState(t *testing.T, f *mtrie.Forest) ledger.RootHash {
	u := &ledger.TrieUpdate{
		RootHash: f.GetEmptyRootHash(),
		Paths:    []ledger.Path{secPathP, secPathP2},
		Payloads: []*ledger.Payload{secPayVP, secPayVP2},
	}
	rootHash, err := f.Update(u)
	require.NoError(t, err, "error updating trie")
	return rootHash
}

// secHonestProofs returns the honest batch proof for [secPathP, secPathQ]:
//   - honestP: inclusion proof for the committed register secPathP (payload secPayVP)
//   - honestQ: proof for the EMPTY register secPathQ (carries an empty payload)
//
// f.Proofs permutes its input in place, so proofs are matched by their Path field.
func secHonestProofs(t *testing.T, f *mtrie.Forest, rootHash ledger.RootHash) (honestP, honestQ *ledger.TrieProof) {
	r := &ledger.TrieRead{RootHash: rootHash, Paths: []ledger.Path{secPathP, secPathQ}}
	bp, err := f.Proofs(r)
	require.NoError(t, err, "error getting batch proof")
	require.Len(t, bp.Proofs, 2)
	for _, pr := range bp.Proofs {
		switch pr.Path {
		case secPathP:
			honestP = pr
		case secPathQ:
			honestQ = pr
		}
	}
	require.NotNil(t, honestP, "expected a proof for secPathP")
	require.NotNil(t, honestQ, "expected a proof for secPathQ")
	require.True(t, honestP.Payload.Equals(secPayVP))
	require.True(t, honestQ.Payload.IsEmpty(), "proof for the empty register Q carries an empty payload")
	return honestP, honestQ
}

// secCloneProof deep-copies a proof so a crafted variant can mutate fields without aliasing.
func secCloneProof(pr *ledger.TrieProof) *ledger.TrieProof {
	return &ledger.TrieProof{
		Path:      pr.Path,
		Payload:   pr.Payload,
		Interims:  slices.Clone(pr.Interims),
		Inclusion: pr.Inclusion,
		Flags:     slices.Clone(pr.Flags),
		Steps:     pr.Steps,
	}
}

// TestNewPSMT_HonestBatchProof is the honest baseline: an untampered batch proof builds a PSMT
// whose root check passes and serves the committed value for secPathP and an empty payload for the
// empty register secPathQ. This pins down that the fix does not reject legitimate proofs.
func TestNewPSMT_HonestBatchProof(t *testing.T) {
	withForest(t, 32, 10, func(t *testing.T, f *mtrie.Forest) {
		rootHash := secBuildCommittedState(t, f)
		honestP, honestQ := secHonestProofs(t, f, rootHash)

		bp := &ledger.TrieBatchProof{Proofs: []*ledger.TrieProof{honestP, honestQ}}
		psmt, err := NewPSMT(rootHash, bp)
		require.NoError(t, err, "honest batch proof must build the partial trie")
		ensureRootHash(t, rootHash, psmt)

		gotP, err := psmt.GetSinglePayload(secPathP)
		require.NoError(t, err)
		require.True(t, gotP.Equals(secPayVP), "secPathP serves the committed value")

		gotQ, err := psmt.GetSinglePayload(secPathQ)
		require.NoError(t, err)
		require.True(t, gotQ.IsEmpty(), "empty register Q serves an empty payload")
	})
}

// TestNewPSMT_RejectsStuffedNonInclusionProof covers attack variant A: a non-inclusion proof for
// an EMPTY register with a fabricated (non-empty) payload. Because the payload now determines the
// node hash unconditionally, the fabricated payload changes the reconstructed root and NewPSMT
// rejects the batch.
func TestNewPSMT_RejectsStuffedNonInclusionProof(t *testing.T) {
	withForest(t, 32, 10, func(t *testing.T, f *mtrie.Forest) {
		rootHash := secBuildCommittedState(t, f)
		honestP, honestQ := secHonestProofs(t, f, rootHash)

		// Craft the malicious non-inclusion proof: honest Path/Steps/Flags/Interims, but with a
		// fabricated payload and Inclusion flipped to false.
		stuffed := testutils.LightPayload('X', 'x')
		require.False(t, stuffed.IsEmpty())
		craftedQ := secCloneProof(honestQ)
		craftedQ.Inclusion = false
		craftedQ.Payload = stuffed

		// The crafted proof still round-trips through the wire format unchanged (the decoder does
		// no semantic validation); the defense lives in NewPSMT.
		attackBatch := &ledger.TrieBatchProof{Proofs: []*ledger.TrieProof{honestP, craftedQ}}
		decoded, err := ledger.DecodeTrieBatchProof(ledger.EncodeTrieBatchProof(attackBatch))
		require.NoError(t, err)
		require.True(t, decoded.Proofs[1].Equals(craftedQ))

		_, err = NewPSMT(rootHash, decoded)
		require.Error(t, err, "NewPSMT must reject a non-inclusion proof carrying a fabricated payload")
	})
}

// TestNewPSMT_RejectsDuplicatePathOverwrite covers attack variant B: an honest inclusion proof for
// secPathP plus a duplicate proof for the same path carrying a fabricated payload. The duplicate's
// payload now necessarily changes P's node hash, so the reconstructed root no longer matches and
// NewPSMT rejects the batch.
func TestNewPSMT_RejectsDuplicatePathOverwrite(t *testing.T) {
	withForest(t, 32, 10, func(t *testing.T, f *mtrie.Forest) {
		rootHash := secBuildCommittedState(t, f)
		honestP, _ := secHonestProofs(t, f, rootHash)

		// Duplicate proof for secPathP: identical Path/Steps/Flags/Interims, but a fabricated payload.
		crafted := testutils.LightPayload('Z', 'z')
		duplicateP := secCloneProof(honestP)
		duplicateP.Inclusion = false
		duplicateP.Payload = crafted
		require.Equal(t, honestP.Path, duplicateP.Path)

		attackBatch := &ledger.TrieBatchProof{Proofs: []*ledger.TrieProof{honestP, duplicateP}}
		decoded, err := ledger.DecodeTrieBatchProof(ledger.EncodeTrieBatchProof(attackBatch))
		require.NoError(t, err)
		require.Len(t, decoded.Proofs, 2)

		_, err = NewPSMT(rootHash, decoded)
		require.Error(t, err, "NewPSMT must reject a duplicate proof that overwrites a committed register's payload")
	})
}

// TestNewPSMT_RejectsInclusionProofWithForgedPayload is a companion check: even an inclusion proof
// whose payload was swapped to a fabricated value is rejected, since the fabricated payload no
// longer hashes to the committed root.
func TestNewPSMT_RejectsInclusionProofWithForgedPayload(t *testing.T) {
	withForest(t, 32, 10, func(t *testing.T, f *mtrie.Forest) {
		rootHash := secBuildCommittedState(t, f)
		honestP, honestQ := secHonestProofs(t, f, rootHash)

		forgedP := secCloneProof(honestP)
		forgedP.Payload = testutils.LightPayload('Y', 'y')
		require.False(t, forgedP.Payload.Equals(secPayVP))

		bp := &ledger.TrieBatchProof{Proofs: []*ledger.TrieProof{forgedP, honestQ}}
		_, err := NewPSMT(rootHash, bp)
		require.Error(t, err, "NewPSMT must reject an inclusion proof carrying a forged payload")
	})
}

// TestNewPSMT_RejectsTruncatedProofOnRoot covers attack variant C: a proof with Steps truncated
// to 0, so its terminal node is the root itself. forceComputeHash recomputes the root from its
// children (supplied by the honest proof) and discards the payload-derived hash, so the root
// check alone cannot detect the fabricated payload. NewPSMT must instead reject the batch
// because the crafted proof's terminal node is not a leaf.
func TestNewPSMT_RejectsTruncatedProofOnRoot(t *testing.T) {
	withForest(t, 32, 10, func(t *testing.T, f *mtrie.Forest) {
		rootHash := secBuildCommittedState(t, f)
		honestP, _ := secHonestProofs(t, f, rootHash)

		// Crafted proof for the distinct path secPathQ: Steps=0 makes the walk stop at the root,
		// so the fabricated payload is attached to the root node and its payload-derived hash is
		// discarded when the root is recomputed from its children.
		craftedQ := secCloneProof(honestP)
		craftedQ.Path = secPathQ
		craftedQ.Inclusion = false
		craftedQ.Steps = 0
		craftedQ.Flags = nil
		craftedQ.Interims = nil
		craftedQ.Payload = testutils.LightPayload('X', 'x')

		attackBatch := &ledger.TrieBatchProof{Proofs: []*ledger.TrieProof{honestP, craftedQ}}
		decoded, err := ledger.DecodeTrieBatchProof(ledger.EncodeTrieBatchProof(attackBatch))
		require.NoError(t, err)

		_, err = NewPSMT(rootHash, decoded)
		require.Error(t, err, "NewPSMT must reject a proof whose terminal node is the root")
	})
}

// TestNewPSMT_RejectsTruncatedDuplicateOnInteriorNode covers attack variant D: a duplicate proof
// for an honestly proven path, but with Steps truncated so its terminal node is an interior
// ancestor of the honest leaf. The interior node has children (built by the honest proof), so
// forceComputeHash discards the payload-derived hash and the root check passes. NewPSMT must
// instead reject the batch because the duplicate's terminal node is not a leaf.
func TestNewPSMT_RejectsTruncatedDuplicateOnInteriorNode(t *testing.T) {
	withForest(t, 32, 10, func(t *testing.T, f *mtrie.Forest) {
		rootHash := secBuildCommittedState(t, f)
		honestP, honestQ := secHonestProofs(t, f, rootHash)

		// Crafted duplicate of honestP: same Path, but Steps truncated to 1 so the walk stops at
		// the depth-1 interior node on P's path. Placed after honestP so it wins the pathLookUp
		// entry for secPathP. Flags carry a single zero byte so the one consumed flag reads as 0
		// (sibling subtree at the default hash).
		duplicateP := secCloneProof(honestP)
		duplicateP.Inclusion = false
		duplicateP.Steps = 1
		duplicateP.Flags = []byte{0}
		duplicateP.Interims = nil
		duplicateP.Payload = testutils.LightPayload('Z', 'z')

		attackBatch := &ledger.TrieBatchProof{Proofs: []*ledger.TrieProof{honestP, honestQ, duplicateP}}
		decoded, err := ledger.DecodeTrieBatchProof(ledger.EncodeTrieBatchProof(attackBatch))
		require.NoError(t, err)

		_, err = NewPSMT(rootHash, decoded)
		require.Error(t, err, "NewPSMT must reject a proof whose terminal node is an interior node")
	})
}

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
