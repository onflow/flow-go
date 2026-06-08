package payloadless_test

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/payloadless"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// mockProofLedger satisfies ledger.PayloadlessLedger. The proof tests only
// exercise Prove; the other methods are present to make the interface
// satisfaction check happy and return zero values.
type mockProofLedger struct {
	proveFn func(*ledger.Query) (*ledger.PayloadlessTrieBatchProof, error)
}

func (m *mockProofLedger) Ready() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (m *mockProofLedger) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (m *mockProofLedger) InitialState() ledger.State             { return ledger.State{} }
func (m *mockProofLedger) HasState(ledger.State) bool             { return false }
func (m *mockProofLedger) HasPaths(*ledger.Query) ([]bool, error) { return nil, nil }
func (m *mockProofLedger) GetSingleLeafHash(*ledger.QuerySingleValue) (*hash.Hash, error) {
	return nil, nil
}
func (m *mockProofLedger) GetLeafHashes(*ledger.Query) ([]*hash.Hash, error) { return nil, nil }
func (m *mockProofLedger) Set(*ledger.Update) (ledger.State, *ledger.TrieUpdate, error) {
	return ledger.State{}, nil, nil
}

func (m *mockProofLedger) Prove(q *ledger.Query) (*ledger.PayloadlessTrieBatchProof, error) {
	return m.proveFn(q)
}

// payloadlessLeaf builds a single inclusion-proof leaf at the given path with
// leafHash = HashLeaf(path, value) and minimal structural fields. Used as the
// standard fixture shape for reconstruction tests.
func payloadlessLeaf(t *testing.T, path ledger.Path, value flow.RegisterValue) *ledger.PayloadlessTrieProof {
	t.Helper()
	leafHash := hash.HashLeaf(hash.Hash(path), value)
	p := ledger.NewPayloadlessTrieProof()
	p.Path = path
	p.LeafHash = &leafHash
	p.Inclusion = true
	p.Steps = 1
	p.Flags[0] = 0x80
	p.Interims = []hash.Hash{hash.DummyHash}
	return p
}

// pathFor derives the trie path for a register ID using the production
// pathfinder version. Convenient for setting up proofs whose paths agree with
// what `complete.PayloadlessLedger` would compute internally.
func pathFor(t *testing.T, registerID flow.RegisterID) ledger.Path {
	t.Helper()
	path, err := pathfinder.KeyToPath(
		convert.RegisterIDToLedgerKey(registerID),
		complete.DefaultPathFinderVersion,
	)
	require.NoError(t, err)
	return path
}

// mockedLedger returns a mockProofLedger whose Prove() returns the supplied
// batch verbatim.
func mockedLedger(batch *ledger.PayloadlessTrieBatchProof) *mockProofLedger {
	return &mockProofLedger{
		proveFn: func(*ledger.Query) (*ledger.PayloadlessTrieBatchProof, error) {
			return batch, nil
		},
	}
}

func TestProveAndReconstruct_HappyPath(t *testing.T) {
	reg := unittest.MakeOwnerReg("k", "v")
	path := pathFor(t, reg.Key)
	leaf := payloadlessLeaf(t, path, reg.Value)
	batch := ledger.NewPayloadlessTrieBatchProof()
	batch.AppendProof(leaf)

	state := unittest.StateCommitmentFixture()
	expectedKey := convert.RegisterIDToLedgerKey(reg.Key)

	proveCalled := false
	l := &mockProofLedger{
		proveFn: func(q *ledger.Query) (*ledger.PayloadlessTrieBatchProof, error) {
			proveCalled = true
			require.Equal(t, 1, q.Size())
			require.True(t, q.Keys()[0].Equals(&expectedKey))
			require.True(t, ledger.State(state).Equals(ledger.State(q.State())))
			return batch, nil
		},
	}

	reader := func(id flow.RegisterID) (flow.RegisterValue, error) {
		require.Equal(t, reg.Key, id)
		return reg.Value, nil
	}

	bytes, err := payloadless.ProveAndReconstruct(
		l, ledger.State(state), []flow.RegisterID{reg.Key}, reader, complete.DefaultPathFinderVersion,
	)
	require.NoError(t, err)
	require.True(t, proveCalled)

	full, err := ledger.DecodeTrieBatchProof(bytes)
	require.NoError(t, err)
	require.Equal(t, 1, full.Size())

	got := full.Proofs[0]
	require.Equal(t, path, got.Path)
	require.True(t, got.Inclusion)
	require.Equal(t, leaf.Steps, got.Steps)
	require.Equal(t, leaf.Flags, got.Flags)
	require.Equal(t, leaf.Interims, got.Interims)
	require.Equal(t, ledger.Value(reg.Value), got.Payload.Value())
}

func TestProveAndReconstruct_ProveError(t *testing.T) {
	reg := unittest.MakeOwnerReg("k", "v")
	proveErr := errors.New("prove blew up")
	l := &mockProofLedger{
		proveFn: func(*ledger.Query) (*ledger.PayloadlessTrieBatchProof, error) {
			return nil, proveErr
		},
	}

	_, err := payloadless.ProveAndReconstruct(
		l,
		ledger.State(unittest.StateCommitmentFixture()),
		[]flow.RegisterID{reg.Key},
		func(flow.RegisterID) (flow.RegisterValue, error) { return nil, nil },
		complete.DefaultPathFinderVersion,
	)
	require.Error(t, err)
	require.ErrorIs(t, err, proveErr)
}

func TestProveAndReconstruct_MultipleRegisters(t *testing.T) {
	regA := unittest.MakeOwnerReg("a", "va")
	regB := unittest.MakeOwnerReg("b", "vb")
	pathA := pathFor(t, regA.Key)
	pathB := pathFor(t, regB.Key)

	// Mocked ledger returns both proofs (order doesn't matter — the function
	// looks up each by path).
	batch := ledger.NewPayloadlessTrieBatchProof()
	batch.AppendProof(payloadlessLeaf(t, pathA, regA.Value))
	batch.AppendProof(payloadlessLeaf(t, pathB, regB.Value))

	l := &mockProofLedger{
		proveFn: func(q *ledger.Query) (*ledger.PayloadlessTrieBatchProof, error) {
			require.Equal(t, 2, q.Size())
			return batch, nil
		},
	}

	values := map[flow.RegisterID]flow.RegisterValue{
		regA.Key: regA.Value,
		regB.Key: regB.Value,
	}
	reader := func(id flow.RegisterID) (flow.RegisterValue, error) {
		v, ok := values[id]
		require.Truef(t, ok, "reader called for unknown register %s", id)
		return v, nil
	}

	bytes, err := payloadless.ProveAndReconstruct(
		l,
		ledger.State(unittest.StateCommitmentFixture()),
		[]flow.RegisterID{regA.Key, regB.Key},
		reader,
		complete.DefaultPathFinderVersion,
	)
	require.NoError(t, err)

	full, err := ledger.DecodeTrieBatchProof(bytes)
	require.NoError(t, err)
	require.Equal(t, 2, full.Size())

	gotByPath := map[ledger.Path]ledger.Value{}
	for _, p := range full.Proofs {
		gotByPath[p.Path] = p.Payload.Value()
	}
	require.Equal(t, ledger.Value(regA.Value), gotByPath[pathA])
	require.Equal(t, ledger.Value(regB.Value), gotByPath[pathB])
}

func TestProveAndReconstruct_NonInclusion(t *testing.T) {
	// Non-inclusion proof: Inclusion = false, no LeafHash. The reader must
	// not be called; the reconstructed leaf carries an empty payload.
	reg := unittest.MakeOwnerReg("k", "v")
	path := pathFor(t, reg.Key)

	leaf := ledger.NewPayloadlessTrieProof()
	leaf.Path = path
	leaf.Inclusion = false
	leaf.Steps = 2
	leaf.Flags[0] = 0x40
	leaf.Interims = []hash.Hash{hash.DummyHash}

	batch := ledger.NewPayloadlessTrieBatchProof()
	batch.AppendProof(leaf)

	readerNotCalled := func(flow.RegisterID) (flow.RegisterValue, error) {
		t.Fatalf("valueReader must not be called for non-inclusion proofs")
		return nil, nil
	}

	bytes, err := payloadless.ProveAndReconstruct(
		mockedLedger(batch),
		ledger.State(unittest.StateCommitmentFixture()),
		[]flow.RegisterID{reg.Key},
		readerNotCalled,
		complete.DefaultPathFinderVersion,
	)
	require.NoError(t, err)

	full, err := ledger.DecodeTrieBatchProof(bytes)
	require.NoError(t, err)
	require.Equal(t, 1, full.Size())

	got := full.Proofs[0]
	require.False(t, got.Inclusion)
	require.Equal(t, leaf.Steps, got.Steps)
	require.Equal(t, leaf.Flags, got.Flags)
	require.Equal(t, leaf.Interims, got.Interims)
	require.True(t, got.Payload.IsEmpty(), "non-inclusion → empty payload on the reconstructed side")
}

func TestProveAndReconstruct_EmptyLeafInclusion(t *testing.T) {
	// Inclusion = true but LeafHash = nil. The forest pads non-inclusion
	// proofs with empty inclusions for non-existent paths; reconstruction
	// must collapse those to empty payloads, not reach for a value.
	reg := unittest.MakeOwnerReg("k", "v")
	path := pathFor(t, reg.Key)

	leaf := ledger.NewPayloadlessTrieProof()
	leaf.Path = path
	leaf.LeafHash = nil
	leaf.Inclusion = true

	batch := ledger.NewPayloadlessTrieBatchProof()
	batch.AppendProof(leaf)

	readerNotCalled := func(flow.RegisterID) (flow.RegisterValue, error) {
		t.Fatalf("valueReader must not be called for empty-leaf inclusion proofs")
		return nil, nil
	}

	bytes, err := payloadless.ProveAndReconstruct(
		mockedLedger(batch),
		ledger.State(unittest.StateCommitmentFixture()),
		[]flow.RegisterID{reg.Key},
		readerNotCalled,
		complete.DefaultPathFinderVersion,
	)
	require.NoError(t, err)

	full, err := ledger.DecodeTrieBatchProof(bytes)
	require.NoError(t, err)
	require.True(t, full.Proofs[0].Inclusion)
	require.True(t, full.Proofs[0].Payload.IsEmpty())
}

func TestProveAndReconstruct_MixedProofs(t *testing.T) {
	// Mix three proofs: a real inclusion, an empty-leaf inclusion, and a
	// non-inclusion. Verify each branch is handled independently and that
	// the reader is invoked only for the real inclusion.
	regA := unittest.MakeOwnerReg("a", "va")
	regB := unittest.MakeOwnerReg("b", "vb")
	regC := unittest.MakeOwnerReg("c", "vc")
	pathA := pathFor(t, regA.Key)
	pathB := pathFor(t, regB.Key)
	pathC := pathFor(t, regC.Key)

	inclusion := payloadlessLeaf(t, pathA, regA.Value)

	empty := ledger.NewPayloadlessTrieProof()
	empty.Path = pathB
	empty.Inclusion = true // empty leaf, but inclusion proof shape

	noninclusion := ledger.NewPayloadlessTrieProof()
	noninclusion.Path = pathC
	noninclusion.Inclusion = false

	batch := ledger.NewPayloadlessTrieBatchProof()
	batch.AppendProof(inclusion)
	batch.AppendProof(empty)
	batch.AppendProof(noninclusion)

	// Atomic counter so this assertion stays valid if the reader is later
	// invoked from worker goroutines.
	var called atomic.Int32
	reader := func(id flow.RegisterID) (flow.RegisterValue, error) {
		called.Add(1)
		require.Equal(t, regA.Key, id, "only the real inclusion path should reach the reader")
		return regA.Value, nil
	}

	bytes, err := payloadless.ProveAndReconstruct(
		mockedLedger(batch),
		ledger.State(unittest.StateCommitmentFixture()),
		[]flow.RegisterID{regA.Key, regB.Key, regC.Key},
		reader,
		complete.DefaultPathFinderVersion,
	)
	require.NoError(t, err)
	require.Equal(t, int32(1), called.Load(), "reader should be invoked exactly once (for the real inclusion)")

	full, err := ledger.DecodeTrieBatchProof(bytes)
	require.NoError(t, err)
	require.Equal(t, 3, full.Size())

	gotByPath := map[ledger.Path]*ledger.TrieProof{}
	for _, p := range full.Proofs {
		gotByPath[p.Path] = p
	}
	require.True(t, gotByPath[pathA].Inclusion)
	require.Equal(t, ledger.Value(regA.Value), gotByPath[pathA].Payload.Value())
	require.True(t, gotByPath[pathB].Inclusion)
	require.True(t, gotByPath[pathB].Payload.IsEmpty())
	require.False(t, gotByPath[pathC].Inclusion)
	require.True(t, gotByPath[pathC].Payload.IsEmpty())
}

func TestProveAndReconstruct_ValueMismatch(t *testing.T) {
	// Reader returns a value that does not hash to the leafHash carried in
	// the proof. Expect ErrPayloadHashMismatch.
	reg := unittest.MakeOwnerReg("k", "real-value")
	path := pathFor(t, reg.Key)
	leaf := payloadlessLeaf(t, path, reg.Value)

	batch := ledger.NewPayloadlessTrieBatchProof()
	batch.AppendProof(leaf)

	reader := func(flow.RegisterID) (flow.RegisterValue, error) {
		return flow.RegisterValue("lying-value"), nil
	}

	_, err := payloadless.ProveAndReconstruct(
		mockedLedger(batch),
		ledger.State(unittest.StateCommitmentFixture()),
		[]flow.RegisterID{reg.Key},
		reader,
		complete.DefaultPathFinderVersion,
	)
	require.Error(t, err)
	require.ErrorIs(t, err, payloadless.ErrPayloadHashMismatch)
}

func TestProveAndReconstruct_ValueReaderError(t *testing.T) {
	// Reader returns an error. Expect it to be propagated (wrapped, but
	// errors.Is should still find it).
	reg := unittest.MakeOwnerReg("k", "v")
	path := pathFor(t, reg.Key)
	leaf := payloadlessLeaf(t, path, reg.Value)

	batch := ledger.NewPayloadlessTrieBatchProof()
	batch.AppendProof(leaf)

	readerErr := errors.New("storehouse offline")
	reader := func(flow.RegisterID) (flow.RegisterValue, error) {
		return nil, readerErr
	}

	_, err := payloadless.ProveAndReconstruct(
		mockedLedger(batch),
		ledger.State(unittest.StateCommitmentFixture()),
		[]flow.RegisterID{reg.Key},
		reader,
		complete.DefaultPathFinderVersion,
	)
	require.Error(t, err)
	require.ErrorIs(t, err, readerErr)
}

func TestProveAndReconstruct_MissingTargetForProofPath(t *testing.T) {
	// The mock returns a proof for a path that does not correspond to any
	// of the queried register IDs. The path → target map (built from the
	// input) won't contain that path, so the lookup must surface a "no
	// register target provided" error rather than crash.
	queriedReg := unittest.MakeOwnerReg("queried", "v")
	foreignReg := unittest.MakeOwnerReg("foreign", "v")
	foreignPath := pathFor(t, foreignReg.Key)

	leaf := payloadlessLeaf(t, foreignPath, foreignReg.Value)
	batch := ledger.NewPayloadlessTrieBatchProof()
	batch.AppendProof(leaf)

	reader := func(flow.RegisterID) (flow.RegisterValue, error) {
		t.Fatalf("reader must not be called when path → target lookup fails")
		return nil, nil
	}

	_, err := payloadless.ProveAndReconstruct(
		mockedLedger(batch),
		ledger.State(unittest.StateCommitmentFixture()),
		[]flow.RegisterID{queriedReg.Key},
		reader,
		complete.DefaultPathFinderVersion,
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no register target provided")
}

func TestProveAndReconstruct_PathfinderVersionRejected(t *testing.T) {
	// An unsupported pathfinder version is rejected by KeysToPaths before
	// any proof is fetched. We don't assert on the specific message — just
	// that the error is surfaced cleanly.
	reg := unittest.MakeOwnerReg("k", "v")
	wrongVersion := uint8(complete.DefaultPathFinderVersion + 1)

	l := &mockProofLedger{
		proveFn: func(*ledger.Query) (*ledger.PayloadlessTrieBatchProof, error) {
			t.Fatalf("Prove must not be called when pathfinder version is rejected")
			return nil, nil
		},
	}

	_, err := payloadless.ProveAndReconstruct(
		l,
		ledger.State(unittest.StateCommitmentFixture()),
		[]flow.RegisterID{reg.Key},
		func(flow.RegisterID) (flow.RegisterValue, error) { return reg.Value, nil },
		wrongVersion,
	)
	require.Error(t, err)
}
