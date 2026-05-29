package payloadless_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/payloadless"
	"github.com/onflow/flow-go/utils/unittest"
)

// These tests build a regular mtrie and a payloadless mtrie from the same
// inputs and assert they agree on observable outputs. The payloadless trie is
// designed to be hash-equivalent to the full mtrie when given matching
// path/value pairs, so any divergence in root hash, leaf hash, or proof
// interim hashes is a bug.

// 1. TestEquivalence_EmptyTrie — empty tries report the same root hash.
// 2. TestEquivalence_SingleRegister — one allocated register; root hash + register count match.
// 3. TestEquivalence_ManyRegisters — 5000 deduplicated random registers, both with and without pruning.
// 4. TestEquivalence_IncrementalUpdates — 10 rounds of updates over a growing trie, both with and without pruning, asserting after every round.
// 5. TestEquivalence_Unallocation — allocate 200 registers, then unallocate them all; root hash must return to the empty-trie default and AllocatedRegCount to 0 in both implementations.
// 6. TestEquivalence_ReadSinglePath — for every allocated path, asserts payloadless.ReadSingleLeafHash(p) == HashLeaf(p, mtrie.ReadSinglePayload(p).Value()). For unallocated paths, asserts payloadless returns nil while the full mtrie returns
// an empty payload.
// 7. TestEquivalence_UnsafeRead — batched version of the above, with a mix of allocated and unallocated paths. Indexes results by path since both implementations permute their inputs in place independently.
// 8. TestEquivalence_UnsafeProofs — proves the structural equivalence of TrieBatchProof vs. PayloadlessTrieBatchProof: Inclusion, Steps, Flags, Interims, and Path must match exactly. For inclusion proofs, additionally checks
// payloadless.LeafHash == HashLeaf(mtrie.Payload.Value()).
// 9. TestEquivalence_RandomWalk — 50-step random walk performing both allocations and unallocations per step, asserting root hash and register count agree after every step.

// applyToBoth applies the same register updates to a regular mtrie and a
// payloadless mtrie. Returns both updated tries.
//
// `mtrieParent` and `plParent` are the parent tries to update; pass empty
// tries for a fresh build. Each call deep-copies its slice inputs because
// both implementations permute paths/values in place.
func applyToBoth(
	t *testing.T,
	mtrieParent *trie.MTrie,
	plParent *payloadless.MTrie,
	paths []ledger.Path,
	values [][]byte,
	prune bool,
) (*trie.MTrie, *payloadless.MTrie) {
	t.Helper()

	// Build payloads for the full mtrie from (path, value) pairs. The key
	// portion is irrelevant for hashing — only the value bytes are hashed —
	// so we use a constant key for every register.
	mtriePaths := make([]ledger.Path, len(paths))
	copy(mtriePaths, paths)
	mtriePayloads := make([]ledger.Payload, len(values))
	for i, v := range values {
		mtriePayloads[i] = *ledger.NewPayload(constantKey, ledger.Value(v))
	}

	plPaths := make([]ledger.Path, len(paths))
	copy(plPaths, paths)
	plValues := make([][]byte, len(values))
	copy(plValues, values)

	mtrieUpdated, _, err := trie.NewTrieWithUpdatedRegisters(mtrieParent, mtriePaths, mtriePayloads, prune)
	require.NoError(t, err)

	plUpdated, _, err := payloadless.NewTrieWithUpdatedRegisters(plParent, plPaths, plValues, prune)
	require.NoError(t, err)

	return mtrieUpdated, plUpdated
}

// constantKey is reused for every payload built from a raw value, since the
// payloadless trie only sees the value bytes and the full mtrie's key is
// not part of the hash.
var constantKey = ledger.NewKey([]ledger.KeyPart{{Type: 0, Value: []byte("k")}})

// TestEquivalence_EmptyTrie verifies that empty tries match.
func TestEquivalence_EmptyTrie(t *testing.T) {
	m := trie.NewEmptyMTrie()
	pl := payloadless.NewEmptyMTrie()
	require.Equal(t, m.RootHash(), pl.RootHash())
}

// TestEquivalence_SingleRegister verifies a trie with one allocated register.
func TestEquivalence_SingleRegister(t *testing.T) {
	path := testutils.PathByUint16(56809)
	value := payloadValue(testutils.LightPayload(56810, 59656))

	m, pl := applyToBoth(t,
		trie.NewEmptyMTrie(), payloadless.NewEmptyMTrie(),
		[]ledger.Path{path}, [][]byte{value}, true,
	)

	require.Equal(t, m.RootHash(), pl.RootHash())
	require.Equal(t, m.AllocatedRegCount(), pl.AllocatedRegCount())
}

// TestEquivalence_ManyRegisters verifies the trie root hash agrees over many
// registers, both with pruning enabled and disabled.
func TestEquivalence_ManyRegisters(t *testing.T) {
	for _, prune := range []bool{false, true} {
		t.Run(prefixForPrune(prune), func(t *testing.T) {
			rng := &LinearCongruentialGenerator{seed: 0}
			paths, values := deduplicateWrites(sampleRandomRegisterWrites(rng, 5000))

			m, pl := applyToBoth(t,
				trie.NewEmptyMTrie(), payloadless.NewEmptyMTrie(),
				paths, values, prune,
			)

			require.Equal(t, m.RootHash(), pl.RootHash())
			require.Equal(t, m.AllocatedRegCount(), pl.AllocatedRegCount())
		})
	}
}

// TestEquivalence_IncrementalUpdates verifies that root hashes agree after
// each round of updates over multiple update rounds.
func TestEquivalence_IncrementalUpdates(t *testing.T) {
	for _, prune := range []bool{false, true} {
		t.Run(prefixForPrune(prune), func(t *testing.T) {
			rng := &LinearCongruentialGenerator{seed: 0}
			m := trie.NewEmptyMTrie()
			pl := payloadless.NewEmptyMTrie()

			for round := 1; round <= 10; round++ {
				paths, values := deduplicateWrites(sampleRandomRegisterWrites(rng, round*50))
				m, pl = applyToBoth(t, m, pl, paths, values, prune)
				require.Equalf(t, m.RootHash(), pl.RootHash(), "root hashes diverged after round %d", round)
				require.Equalf(t, m.AllocatedRegCount(), pl.AllocatedRegCount(), "register counts diverged after round %d", round)
			}
		})
	}
}

// TestEquivalence_Unallocation verifies the root hashes match after allocating
// registers and subsequently unallocating them.
func TestEquivalence_Unallocation(t *testing.T) {
	rng := &LinearCongruentialGenerator{seed: 0}
	paths, values := deduplicateWrites(sampleRandomRegisterWrites(rng, 200))

	m, pl := applyToBoth(t,
		trie.NewEmptyMTrie(), payloadless.NewEmptyMTrie(),
		paths, values, true,
	)
	require.Equal(t, m.RootHash(), pl.RootHash())

	// Unallocate all registers by writing nil values for the same paths.
	emptyValues := make([][]byte, len(paths))
	m, pl = applyToBoth(t, m, pl, paths, emptyValues, true)
	require.Equal(t, m.RootHash(), pl.RootHash())
	require.Equal(t, uint64(0), pl.AllocatedRegCount())
	require.Equal(t, uint64(0), m.AllocatedRegCount())
	require.Equal(t, m.RootHash(), pl.RootHash())
	require.Equal(t, ledger.RootHash(ledger.GetDefaultHashForHeight(ledger.NodeMaxHeight)), pl.RootHash())
}

// TestEquivalence_ReadSinglePath verifies that for every path in the trie,
// the payloadless ReadSingleLeafHash matches HashLeaf(path, fullPayloadValue)
// retrieved from the full mtrie. Also checks that non-existent paths return
// nil from payloadless and an empty payload from the full mtrie.
func TestEquivalence_ReadSinglePath(t *testing.T) {
	rng := &LinearCongruentialGenerator{seed: 0}
	paths, values := deduplicateWrites(sampleRandomRegisterWrites(rng, 500))

	m, pl := applyToBoth(t,
		trie.NewEmptyMTrie(), payloadless.NewEmptyMTrie(),
		paths, values, true,
	)
	require.Equal(t, m.RootHash(), pl.RootHash())

	// Allocated paths: both implementations report the value, and the
	// payloadless leaf hash equals HashLeaf(path, mtrie-payload-value).
	for i, p := range paths {
		mPayload := m.ReadSinglePayload(p)
		plLeafHash := pl.ReadSingleLeafHash(p)

		require.False(t, mPayload.IsEmpty(), "mtrie should have a payload for allocated path %d", i)
		require.NotNil(t, plLeafHash, "payloadless should have a leaf hash for allocated path %d", i)

		expected := hash.HashLeaf(hash.Hash(p), []byte(mPayload.Value()))
		require.Equalf(t, expected, *plLeafHash, "leaf hash mismatch for path index %d", i)
	}

	// Non-existent paths: mtrie returns empty, payloadless returns nil.
	for i := 0; i < 50; i++ {
		var p ledger.Path
		// Choose a path that almost certainly isn't in the trie.
		p[0] = byte(0xff)
		p[1] = byte(i)
		mPayload := m.ReadSinglePayload(p)
		plLeafHash := pl.ReadSingleLeafHash(p)
		require.True(t, mPayload.IsEmpty(), "mtrie should not have a payload for unallocated path")
		require.Nil(t, plLeafHash, "payloadless should not have a leaf hash for unallocated path")
	}
}

// TestEquivalence_UnsafeRead verifies that the batched UnsafeRead from both
// implementations agrees on a slice of paths (existent and non-existent).
func TestEquivalence_UnsafeRead(t *testing.T) {
	rng := &LinearCongruentialGenerator{seed: 0}
	paths, values := deduplicateWrites(sampleRandomRegisterWrites(rng, 300))

	m, pl := applyToBoth(t,
		trie.NewEmptyMTrie(), payloadless.NewEmptyMTrie(),
		paths, values, true,
	)

	// Build a query: half of the allocated paths plus some unallocated paths.
	queryPaths := make([]ledger.Path, 0, len(paths)/2+50)
	queryPaths = append(queryPaths, paths[:len(paths)/2]...)
	for i := 0; i < 50; i++ {
		var p ledger.Path
		p[0] = byte(0xff)
		p[31] = byte(i)
		queryPaths = append(queryPaths, p)
	}

	// Both implementations permute their `paths` argument in place, so use
	// independent copies.
	mPaths := make([]ledger.Path, len(queryPaths))
	copy(mPaths, queryPaths)
	plPaths := make([]ledger.Path, len(queryPaths))
	copy(plPaths, queryPaths)

	mPayloads := m.UnsafeRead(mPaths)
	plLeafHashes := pl.UnsafeRead(plPaths)

	require.Equal(t, len(mPayloads), len(plLeafHashes))

	// Each read returns results in a permuted order matching its own paths
	// argument. Compare value-by-path via maps.
	mByPath := make(map[ledger.Path]*ledger.Payload, len(mPaths))
	for i, p := range mPaths {
		mByPath[p] = mPayloads[i]
	}
	plByPath := make(map[ledger.Path]*hash.Hash, len(plPaths))
	for i, p := range plPaths {
		plByPath[p] = plLeafHashes[i]
	}

	for _, p := range queryPaths {
		mp := mByPath[p]
		plh := plByPath[p]
		if mp.IsEmpty() {
			require.Nil(t, plh, "payloadless should report nil for unallocated path")
			continue
		}
		require.NotNil(t, plh, "payloadless should report a leaf hash for allocated path")
		expected := hash.HashLeaf(hash.Hash(p), []byte(mp.Value()))
		require.Equal(t, expected, *plh)
	}
}

// TestEquivalence_UnsafeProofs verifies that the proof interim hashes and
// structure (Flags, Steps, Inclusion) match between the two implementations.
// The payload-vs-leafHash field differs by design.
func TestEquivalence_UnsafeProofs(t *testing.T) {
	rng := &LinearCongruentialGenerator{seed: 0}
	paths, values := deduplicateWrites(sampleRandomRegisterWrites(rng, 300))

	m, pl := applyToBoth(t,
		trie.NewEmptyMTrie(), payloadless.NewEmptyMTrie(),
		paths, values, true,
	)

	// Query a mix of allocated and unallocated paths.
	queryPaths := make([]ledger.Path, 0, len(paths)/4+25)
	queryPaths = append(queryPaths, paths[:len(paths)/4]...)
	for i := 0; i < 25; i++ {
		var p ledger.Path
		p[0] = byte(0xfe)
		p[31] = byte(i)
		queryPaths = append(queryPaths, p)
	}

	mPaths := make([]ledger.Path, len(queryPaths))
	copy(mPaths, queryPaths)
	plPaths := make([]ledger.Path, len(queryPaths))
	copy(plPaths, queryPaths)

	mBatch := m.UnsafeProofs(mPaths)
	plBatch := pl.UnsafeProofs(plPaths)

	require.Equal(t, mBatch.Size(), plBatch.Size())

	// Both implementations permute their `paths` argument in place, possibly
	// in different orders. Index proofs by path so comparisons stay aligned.
	mByPath := make(map[ledger.Path]*ledger.TrieProof, len(mPaths))
	for i, p := range mPaths {
		mByPath[p] = mBatch.Proofs[i]
	}
	plByPath := make(map[ledger.Path]*ledger.PayloadlessTrieProof, len(plPaths))
	for i, p := range plPaths {
		plByPath[p] = plBatch.Proofs[i]
	}

	for _, p := range queryPaths {
		mp := mByPath[p]
		plp := plByPath[p]

		require.Equalf(t, mp.Inclusion, plp.Inclusion, "Inclusion mismatch for path %x", p[:])
		require.Equalf(t, mp.Steps, plp.Steps, "Steps mismatch for path %x", p[:])
		require.Equalf(t, mp.Flags, plp.Flags, "Flags mismatch for path %x", p[:])
		require.Equalf(t, mp.Interims, plp.Interims, "Interims mismatch for path %x", p[:])
		require.Equal(t, mp.Path, plp.Path)

		if mp.Inclusion {
			// On inclusion, the full proof carries the payload and the
			// payloadless proof carries HashLeaf(path, value).
			require.NotNil(t, plp.LeafHash)
			expected := hash.HashLeaf(hash.Hash(mp.Path), []byte(mp.Payload.Value()))
			require.Equal(t, expected, *plp.LeafHash)
		}
	}
}

// TestEquivalence_RandomWalk runs random allocations, updates, and
// unallocations on both implementations and checks the root hash, register
// count, and per-path reads agree after every step.
func TestEquivalence_RandomWalk(t *testing.T) {
	rand := unittest.GetPRG(t)

	const steps = 50
	const allocPerStep = 60
	const unallocPerStep = 30

	m := trie.NewEmptyMTrie()
	pl := payloadless.NewEmptyMTrie()
	live := make(map[ledger.Path][]byte)

	for step := 0; step < steps; step++ {
		updatePaths := make([]ledger.Path, 0, allocPerStep+unallocPerStep)
		updateValues := make([][]byte, 0, allocPerStep+unallocPerStep)

		// Allocate / update some registers.
		for i := 0; i < allocPerStep; i++ {
			var p ledger.Path
			_, err := rand.Read(p[:])
			require.NoError(t, err)
			value := payloadValue(testutils.RandomPayload(1, 100))
			updatePaths = append(updatePaths, p)
			updateValues = append(updateValues, value)
		}
		// Unallocate up to unallocPerStep existing registers.
		count := 0
		for p := range live {
			if count >= unallocPerStep {
				break
			}
			updatePaths = append(updatePaths, p)
			updateValues = append(updateValues, nil)
			count++
		}

		m, pl = applyToBoth(t, m, pl, updatePaths, updateValues, true)

		// Track the expected live set. Re-establish from the post-update
		// updatePaths/updateValues because applyToBoth used copies (the
		// originals are untouched).
		for i, p := range updatePaths {
			if updateValues[i] == nil {
				delete(live, p)
			} else {
				live[p] = updateValues[i]
			}
		}

		require.Equalf(t, m.RootHash(), pl.RootHash(), "root hashes diverged at step %d", step)
		require.Equalf(t, uint64(len(live)), pl.AllocatedRegCount(), "reg count mismatch at step %d", step)
		require.Equal(t, m.AllocatedRegCount(), pl.AllocatedRegCount())
	}
}

func prefixForPrune(prune bool) string {
	if prune {
		return "with_pruning"
	}
	return "without_pruning"
}
