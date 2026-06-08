package payloadless_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/ledger/complete/payloadless"
	"github.com/onflow/flow-go/module/metrics"
)

// These tests build a regular mtrie.Forest and a payloadless.Forest from the same
// inputs and assert they agree on observable outputs. The payloadless forest is
// designed to be hash-equivalent to the full forest when given identical
// TrieUpdates, so any divergence in root hash, leaf reads, existence checks, or
// proof interim hashes is a bug.

// forestPair holds a regular and a payloadless forest that are kept in lockstep.
type forestPair struct {
	m  *mtrie.Forest
	pl *payloadless.Forest
}

func newForestPair(t *testing.T, capacity int) *forestPair {
	t.Helper()
	noop := &metrics.NoopCollector{}
	m, err := mtrie.NewForest(capacity, noop, nil)
	require.NoError(t, err)
	pl, err := payloadless.NewForest(capacity, noop, nil)
	require.NoError(t, err)
	return &forestPair{m: m, pl: pl}
}

// applyUpdate sends the same TrieUpdate to both forests. The same root hash
// must be returned by both. Each forest internally re-uses or permutes the
// slices in its input, so we hand each a defensive deep copy.
func (fp *forestPair) applyUpdate(t *testing.T, u *ledger.TrieUpdate) (ledger.RootHash, ledger.RootHash) {
	t.Helper()

	mRoot, err := fp.m.Update(cloneUpdate(u))
	require.NoError(t, err)
	plRoot, err := fp.pl.Update(cloneUpdate(u))
	require.NoError(t, err)
	require.Equal(t, mRoot, plRoot, "root hash mismatch between full and payloadless forest")
	return mRoot, plRoot
}

// cloneUpdate returns a deep copy of u suitable for passing to a Forest that
// permutes its inputs in place.
func cloneUpdate(u *ledger.TrieUpdate) *ledger.TrieUpdate {
	paths := make([]ledger.Path, len(u.Paths))
	copy(paths, u.Paths)
	payloads := make([]*ledger.Payload, len(u.Payloads))
	copy(payloads, u.Payloads)
	return &ledger.TrieUpdate{RootHash: u.RootHash, Paths: paths, Payloads: payloads}
}

// TestForestEquivalence_Empty verifies the empty root hashes match.
func TestForestEquivalence_Empty(t *testing.T) {
	fp := newForestPair(t, 5)
	require.Equal(t, fp.m.GetEmptyRootHash(), fp.pl.GetEmptyRootHash())
}

// TestForestEquivalence_SingleUpdate verifies a single TrieUpdate produces the
// same root hash in both forests.
func TestForestEquivalence_SingleUpdate(t *testing.T) {
	fp := newForestPair(t, 5)

	path := testutils.PathByUint16(56809)
	payload := testutils.LightPayload(56810, 59656)
	update := &ledger.TrieUpdate{
		RootHash: fp.m.GetEmptyRootHash(),
		Paths:    []ledger.Path{path},
		Payloads: []*ledger.Payload{payload},
	}

	fp.applyUpdate(t, update)
}

// TestForestEquivalence_IncrementalUpdates verifies root hashes agree after each
// round of updates over many rounds.
func TestForestEquivalence_IncrementalUpdates(t *testing.T) {
	fp := newForestPair(t, 100)

	rootHash := fp.m.GetEmptyRootHash()
	rng := &payloadlessRNG{seed: 0}

	for round := 1; round <= 10; round++ {
		paths, payloads := randomUpdate(rng, round*40)
		update := &ledger.TrieUpdate{RootHash: rootHash, Paths: paths, Payloads: payloads}
		newRoot, _ := fp.applyUpdate(t, update)
		rootHash = newRoot
	}
}

// TestForestEquivalence_Forking verifies that forking a base trie into two
// children yields matching root hashes in both forests.
func TestForestEquivalence_Forking(t *testing.T) {
	fp := newForestPair(t, 10)

	rng := &payloadlessRNG{seed: 0}
	basePaths, basePayloads := randomUpdate(rng, 50)
	baseUpdate := &ledger.TrieUpdate{
		RootHash: fp.m.GetEmptyRootHash(),
		Paths:    basePaths,
		Payloads: basePayloads,
	}
	baseRoot, _ := fp.applyUpdate(t, baseUpdate)

	// fork A
	pathsA, payloadsA := randomUpdate(rng, 30)
	updateA := &ledger.TrieUpdate{RootHash: baseRoot, Paths: pathsA, Payloads: payloadsA}
	fp.applyUpdate(t, updateA)

	// fork B (independent from A)
	pathsB, payloadsB := randomUpdate(rng, 30)
	updateB := &ledger.TrieUpdate{RootHash: baseRoot, Paths: pathsB, Payloads: payloadsB}
	fp.applyUpdate(t, updateB)
}

// TestForestEquivalence_Reads verifies that for every path in the trie, the
// payloadless ReadLeafHashes returns HashLeaf(path, value) where value is what
// the full Read returns.
func TestForestEquivalence_Reads(t *testing.T) {
	fp := newForestPair(t, 5)

	rng := &payloadlessRNG{seed: 0}
	paths, payloads := randomUpdate(rng, 200)
	update := &ledger.TrieUpdate{
		RootHash: fp.m.GetEmptyRootHash(),
		Paths:    paths,
		Payloads: payloads,
	}
	root, _ := fp.applyUpdate(t, update)

	// Mix of allocated and unallocated paths.
	queryPaths := make([]ledger.Path, 0, len(paths)+30)
	queryPaths = append(queryPaths, paths...)
	for i := 0; i < 30; i++ {
		var p ledger.Path
		p[0] = 0xff
		p[31] = byte(i)
		queryPaths = append(queryPaths, p)
	}

	mPaths := append([]ledger.Path(nil), queryPaths...)
	plPaths := append([]ledger.Path(nil), queryPaths...)

	mValues, err := fp.m.Read(&ledger.TrieRead{RootHash: root, Paths: mPaths})
	require.NoError(t, err)
	plLeafHashes, err := fp.pl.ReadLeafHashes(&ledger.TrieRead{RootHash: root, Paths: plPaths})
	require.NoError(t, err)

	require.Equal(t, len(queryPaths), len(mValues))
	require.Equal(t, len(queryPaths), len(plLeafHashes))

	for i, p := range queryPaths {
		if len(mValues[i]) == 0 {
			// full forest returned empty value → payloadless must report nil
			require.Nilf(t, plLeafHashes[i], "expected nil leaf hash at index %d for unallocated path", i)
			continue
		}
		require.NotNilf(t, plLeafHashes[i], "expected non-nil leaf hash at index %d for allocated path", i)
		expected := hash.HashLeaf(hash.Hash(p), []byte(mValues[i]))
		require.Equalf(t, expected, *plLeafHashes[i], "leaf hash mismatch at index %d", i)
	}
}

// TestForestEquivalence_ReadSingle verifies the single-path read APIs agree.
func TestForestEquivalence_ReadSingle(t *testing.T) {
	fp := newForestPair(t, 5)

	rng := &payloadlessRNG{seed: 0}
	paths, payloads := randomUpdate(rng, 50)
	update := &ledger.TrieUpdate{
		RootHash: fp.m.GetEmptyRootHash(),
		Paths:    paths,
		Payloads: payloads,
	}
	root, _ := fp.applyUpdate(t, update)

	// allocated paths
	for i, p := range paths {
		mValue, err := fp.m.ReadSingleValue(&ledger.TrieReadSingleValue{RootHash: root, Path: p})
		require.NoError(t, err)
		plLeafHash, err := fp.pl.ReadSingleLeafHash(&ledger.TrieReadSingleValue{RootHash: root, Path: p})
		require.NoError(t, err)

		if len(mValue) == 0 {
			require.Nil(t, plLeafHash)
			continue
		}
		require.NotNil(t, plLeafHash)
		expected := hash.HashLeaf(hash.Hash(p), []byte(mValue))
		require.Equalf(t, expected, *plLeafHash, "leaf hash mismatch for path index %d", i)
	}

	// unallocated paths
	for i := 0; i < 20; i++ {
		var p ledger.Path
		p[0] = 0xfe
		p[31] = byte(i)

		mValue, err := fp.m.ReadSingleValue(&ledger.TrieReadSingleValue{RootHash: root, Path: p})
		require.NoError(t, err)
		plLeafHash, err := fp.pl.ReadSingleLeafHash(&ledger.TrieReadSingleValue{RootHash: root, Path: p})
		require.NoError(t, err)

		require.Equal(t, 0, len(mValue))
		require.Nil(t, plLeafHash)
	}
}

// TestForestEquivalence_HasPathsVsValueSizes verifies that for every path,
// payloadless.HasPaths reports true iff the full forest's ValueSizes is > 0.
func TestForestEquivalence_HasPathsVsValueSizes(t *testing.T) {
	fp := newForestPair(t, 5)

	rng := &payloadlessRNG{seed: 0}
	paths, payloads := randomUpdate(rng, 150)
	update := &ledger.TrieUpdate{
		RootHash: fp.m.GetEmptyRootHash(),
		Paths:    paths,
		Payloads: payloads,
	}
	root, _ := fp.applyUpdate(t, update)

	// Mix allocated paths with some unallocated and duplicate entries.
	queryPaths := make([]ledger.Path, 0, len(paths)+25)
	queryPaths = append(queryPaths, paths...)
	for i := 0; i < 20; i++ {
		var p ledger.Path
		p[0] = 0xfd
		p[31] = byte(i)
		queryPaths = append(queryPaths, p)
	}
	// add a few duplicates
	queryPaths = append(queryPaths, paths[0], paths[1], paths[0])

	mPaths := append([]ledger.Path(nil), queryPaths...)
	plPaths := append([]ledger.Path(nil), queryPaths...)

	sizes, err := fp.m.ValueSizes(&ledger.TrieRead{RootHash: root, Paths: mPaths})
	require.NoError(t, err)
	exists, err := fp.pl.HasPaths(&ledger.TrieRead{RootHash: root, Paths: plPaths})
	require.NoError(t, err)

	require.Equal(t, len(queryPaths), len(sizes))
	require.Equal(t, len(queryPaths), len(exists))

	for i := range queryPaths {
		require.Equalf(t, sizes[i] > 0, exists[i], "existence mismatch at index %d (size=%d)", i, sizes[i])
	}
}

// TestForestEquivalence_Proofs verifies that the proof interim hashes and
// structural fields (Flags, Steps, Inclusion, Path) match between the two
// forests. Inclusion proofs additionally carry HashLeaf(payload.Value()) in
// the payloadless variant.
func TestForestEquivalence_Proofs(t *testing.T) {
	fp := newForestPair(t, 5)

	rng := &payloadlessRNG{seed: 0}
	paths, payloads := randomUpdate(rng, 150)
	update := &ledger.TrieUpdate{
		RootHash: fp.m.GetEmptyRootHash(),
		Paths:    paths,
		Payloads: payloads,
	}
	root, _ := fp.applyUpdate(t, update)

	// Query a mix of allocated and unallocated paths.
	queryPaths := make([]ledger.Path, 0, 60)
	queryPaths = append(queryPaths, paths[:40]...)
	for i := 0; i < 20; i++ {
		var p ledger.Path
		p[0] = 0xfc
		p[31] = byte(i)
		queryPaths = append(queryPaths, p)
	}

	mPaths := append([]ledger.Path(nil), queryPaths...)
	plPaths := append([]ledger.Path(nil), queryPaths...)

	mBatch, err := fp.m.Proofs(&ledger.TrieRead{RootHash: root, Paths: mPaths})
	require.NoError(t, err)
	plBatch, err := fp.pl.Proofs(&ledger.TrieRead{RootHash: root, Paths: plPaths})
	require.NoError(t, err)

	require.Equal(t, mBatch.Size(), plBatch.Size())

	// Both forests sort the input paths before generating proofs and return
	// proofs indexed by the resulting order. Index by path so we compare the
	// same proof regardless of internal ordering choices.
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
			// `forest.Proofs` pre-expands the trie with EmptyPayloads for
			// previously-unallocated paths, turning them into inclusion
			// proofs of empty leaves. In the payloadless variant, those
			// empty leaves carry a nil leafHash.
			if mp.Payload.IsEmpty() {
				require.Nilf(t, plp.LeafHash, "payloadless leaf hash should be nil for empty inclusion (path %x)", mp.Path[:])
			} else {
				require.NotNilf(t, plp.LeafHash, "payloadless leaf hash should be non-nil for non-empty inclusion (path %x)", mp.Path[:])
				expected := hash.HashLeaf(hash.Hash(mp.Path), []byte(mp.Payload.Value()))
				require.Equal(t, expected, *plp.LeafHash)
			}
		}
	}
}

// payloadlessRNG is a self-contained LCG so the equivalence test doesn't depend
// on internals of either forest_test.go or trie_test.go (those are in
// different test packages).
type payloadlessRNG struct {
	seed uint64
}

func (r *payloadlessRNG) next() uint16 {
	r.seed = (r.seed*1140671485 + 12820163) % 65536
	return uint16(r.seed)
}

// randomUpdate generates deduplicated path/payload pairs for a TrieUpdate.
func randomUpdate(rng *payloadlessRNG, n int) ([]ledger.Path, []*ledger.Payload) {
	seen := make(map[ledger.Path]int, n)
	paths := make([]ledger.Path, 0, n)
	payloads := make([]*ledger.Payload, 0, n)
	for i := 0; i < n; i++ {
		path := testutils.PathByUint16LeftPadded(rng.next())
		v := rng.next()
		payload := testutils.LightPayload(v, v)
		if idx, ok := seen[path]; ok {
			payloads[idx] = payload
			continue
		}
		seen[path] = len(paths)
		paths = append(paths, path)
		payloads = append(payloads, payload)
	}
	return paths, payloads
}
