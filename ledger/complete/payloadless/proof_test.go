package payloadless_test

import (
	"errors"
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

// payloadlessLeaf is a helper that builds a single inclusion-proof leaf at the
// given path with leafHash = HashLeaf(path, value) and minimal structural
// fields. Used by the reconstruction tests as the standard fixture shape.
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
// pathfinder version. Convenient for setting up proofs whose paths agree
// with what `complete.PayloadlessLedger` would compute internally.
func pathFor(t *testing.T, registerID flow.RegisterID) ledger.Path {
	t.Helper()
	path, err := pathfinder.KeyToPath(
		convert.RegisterIDToLedgerKey(registerID),
		complete.DefaultPathFinderVersion,
	)
	require.NoError(t, err)
	return path
}

// =====================================================================
// ReconstructPayloadlessProof
// =====================================================================

func TestReconstructPayloadlessProof_HappyPath(t *testing.T) {
	reg := unittest.MakeOwnerReg("k", "v")
	path := pathFor(t, reg.Key)
	leaf := payloadlessLeaf(t, path, reg.Value)

	batch := ledger.NewPayloadlessTrieBatchProof()
	batch.AppendProof(leaf)

	pathToRegisterID := map[ledger.Path]flow.RegisterID{path: reg.Key}
	reader := func(id flow.RegisterID) (flow.RegisterValue, error) {
		require.Equal(t, reg.Key, id)
		return reg.Value, nil
	}

	bytes, err := payloadless.ReconstructPayloadlessProof(batch, pathToRegisterID, reader)
	require.NoError(t, err)
	require.NotEmpty(t, bytes)

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

func TestReconstructPayloadlessProof_NonInclusion(t *testing.T) {
	reg := unittest.MakeOwnerReg("k", "v")
	path := pathFor(t, reg.Key)

	// Non-inclusion proof: Inclusion = false, no LeafHash.
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

	bytes, err := payloadless.ReconstructPayloadlessProof(batch, nil, readerNotCalled)
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

func TestReconstructPayloadlessProof_EmptyLeafInclusion(t *testing.T) {
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

	bytes, err := payloadless.ReconstructPayloadlessProof(batch, nil, readerNotCalled)
	require.NoError(t, err)

	full, err := ledger.DecodeTrieBatchProof(bytes)
	require.NoError(t, err)
	require.True(t, full.Proofs[0].Inclusion)
	require.True(t, full.Proofs[0].Payload.IsEmpty())
}

func TestReconstructPayloadlessProof_MultipleProofs(t *testing.T) {
	// Mix three proofs: a real inclusion, an empty-leaf inclusion, and a
	// non-inclusion. Verify each branch is handled independently.
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

	pathToRegisterID := map[ledger.Path]flow.RegisterID{pathA: regA.Key}

	called := 0
	reader := func(id flow.RegisterID) (flow.RegisterValue, error) {
		called++
		require.Equal(t, regA.Key, id, "only the real inclusion path should reach the reader")
		return regA.Value, nil
	}

	bytes, err := payloadless.ReconstructPayloadlessProof(batch, pathToRegisterID, reader)
	require.NoError(t, err)
	require.Equal(t, 1, called, "reader should be invoked exactly once (for the real inclusion)")

	full, err := ledger.DecodeTrieBatchProof(bytes)
	require.NoError(t, err)
	require.Equal(t, 3, full.Size())

	require.True(t, full.Proofs[0].Inclusion)
	require.Equal(t, ledger.Value(regA.Value), full.Proofs[0].Payload.Value())

	require.True(t, full.Proofs[1].Inclusion)
	require.True(t, full.Proofs[1].Payload.IsEmpty())

	require.False(t, full.Proofs[2].Inclusion)
	require.True(t, full.Proofs[2].Payload.IsEmpty())
}

func TestReconstructPayloadlessProof_ValueMismatch(t *testing.T) {
	// Reader returns a value that does not hash to the leafHash carried in
	// the proof. Expect ErrPayloadHashMismatch.
	reg := unittest.MakeOwnerReg("k", "real-value")
	path := pathFor(t, reg.Key)
	leaf := payloadlessLeaf(t, path, reg.Value)

	batch := ledger.NewPayloadlessTrieBatchProof()
	batch.AppendProof(leaf)

	pathToRegisterID := map[ledger.Path]flow.RegisterID{path: reg.Key}
	reader := func(flow.RegisterID) (flow.RegisterValue, error) {
		return flow.RegisterValue("lying-value"), nil
	}

	_, err := payloadless.ReconstructPayloadlessProof(batch, pathToRegisterID, reader)
	require.Error(t, err)
	require.ErrorIs(t, err, payloadless.ErrPayloadHashMismatch)
}

func TestReconstructPayloadlessProof_ValueReaderError(t *testing.T) {
	// Reader returns an error. Expect it to be propagated (wrapped, but
	// errors.Is should still find it).
	reg := unittest.MakeOwnerReg("k", "v")
	path := pathFor(t, reg.Key)
	leaf := payloadlessLeaf(t, path, reg.Value)

	batch := ledger.NewPayloadlessTrieBatchProof()
	batch.AppendProof(leaf)

	pathToRegisterID := map[ledger.Path]flow.RegisterID{path: reg.Key}
	readerErr := errors.New("storehouse offline")
	reader := func(flow.RegisterID) (flow.RegisterValue, error) {
		return nil, readerErr
	}

	_, err := payloadless.ReconstructPayloadlessProof(batch, pathToRegisterID, reader)
	require.Error(t, err)
	require.ErrorIs(t, err, readerErr)
}

func TestReconstructPayloadlessProof_MissingPath(t *testing.T) {
	// pathToRegisterID does not contain the proof's path. Expect a
	// descriptive error rather than a panic or a silent miss.
	reg := unittest.MakeOwnerReg("k", "v")
	path := pathFor(t, reg.Key)
	leaf := payloadlessLeaf(t, path, reg.Value)

	batch := ledger.NewPayloadlessTrieBatchProof()
	batch.AppendProof(leaf)

	reader := func(flow.RegisterID) (flow.RegisterValue, error) {
		t.Fatalf("reader must not be called when path → registerID lookup fails")
		return nil, nil
	}

	_, err := payloadless.ReconstructPayloadlessProof(batch, map[ledger.Path]flow.RegisterID{}, reader)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no register ID provided")
}

func TestReconstructPayloadlessProof_EmptyBatch(t *testing.T) {
	// An empty batch round-trips to a decoded empty batch.
	batch := ledger.NewPayloadlessTrieBatchProof()
	require.Equal(t, 0, batch.Size())

	bytes, err := payloadless.ReconstructPayloadlessProof(batch, nil, nil)
	require.NoError(t, err)

	full, err := ledger.DecodeTrieBatchProof(bytes)
	require.NoError(t, err)
	require.Equal(t, 0, full.Size())
}

// =====================================================================
// ProveAndReconstruct
// =====================================================================

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
	require.True(t, full.Proofs[0].Inclusion)
	require.Equal(t, ledger.Value(reg.Value), full.Proofs[0].Payload.Value())
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

func TestProveAndReconstruct_PathfinderVersionMismatch(t *testing.T) {
	// If the caller passes a pathfinder version that disagrees with the
	// version implied by the proof's path, the path → registerID map will
	// not contain the proof's path. The function should surface this as a
	// "no register ID provided" error rather than crash.
	reg := unittest.MakeOwnerReg("k", "v")
	correctPath := pathFor(t, reg.Key)
	leaf := payloadlessLeaf(t, correctPath, reg.Value)
	batch := ledger.NewPayloadlessTrieBatchProof()
	batch.AppendProof(leaf)

	l := &mockProofLedger{
		proveFn: func(*ledger.Query) (*ledger.PayloadlessTrieBatchProof, error) {
			return batch, nil
		},
	}

	// Pass a different pathfinder version. KeysToPaths will produce a path
	// that is not correctPath, so the lookup fails.
	wrongVersion := uint8(complete.DefaultPathFinderVersion + 1)

	_, err := payloadless.ProveAndReconstruct(
		l,
		ledger.State(unittest.StateCommitmentFixture()),
		[]flow.RegisterID{reg.Key},
		func(flow.RegisterID) (flow.RegisterValue, error) { return reg.Value, nil },
		wrongVersion,
	)
	require.Error(t, err)
}
