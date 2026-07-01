package committer_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/computation/committer"
	"github.com/onflow/flow-go/engine/execution/storehouse"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/unittest"
)

// mockPayloadlessLedger is a hand-rolled implementation of
// [ledger.PayloadlessLedger] backed by closures. The committer only invokes
// Set and Prove; the remaining methods are present to satisfy the interface
// and return zero values.
type mockPayloadlessLedger struct {
	setFn   func(update *ledger.Update) (ledger.State, *ledger.TrieUpdate, error)
	proveFn func(query *ledger.Query) (*ledger.PayloadlessTrieBatchProof, error)
}

func (m *mockPayloadlessLedger) Ready() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (m *mockPayloadlessLedger) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (m *mockPayloadlessLedger) InitialState() ledger.State { return ledger.State{} }

func (m *mockPayloadlessLedger) HasState(ledger.State) bool { return false }

func (m *mockPayloadlessLedger) HasPaths(*ledger.Query) ([]bool, error) { return nil, nil }

func (m *mockPayloadlessLedger) GetSingleLeafHash(*ledger.QuerySingleValue) (*hash.Hash, error) {
	return nil, nil
}

func (m *mockPayloadlessLedger) GetLeafHashes(*ledger.Query) ([]*hash.Hash, error) {
	return nil, nil
}

func (m *mockPayloadlessLedger) Set(update *ledger.Update) (ledger.State, *ledger.TrieUpdate, error) {
	return m.setFn(update)
}

func (m *mockPayloadlessLedger) Prove(query *ledger.Query) (*ledger.PayloadlessTrieBatchProof, error) {
	return m.proveFn(query)
}

func TestPayloadlessLedgerViewCommitter(t *testing.T) {

	t.Run("CommitView returns reconstructed full proof and statecommitment", func(t *testing.T) {
		reg := unittest.MakeOwnerReg("key1", "val1")
		startState := unittest.StateCommitmentFixture()
		endState := unittest.StateCommitmentFixture()
		require.NotEqual(t, startState, endState)

		ledgerKey := convert.RegisterIDToLedgerKey(reg.Key)
		path, err := pathfinder.KeyToPath(ledgerKey, complete.DefaultPathFinderVersion)
		require.NoError(t, err)

		update, err := ledger.NewUpdate(ledger.State(startState), []ledger.Key{ledgerKey}, []ledger.Value{reg.Value})
		require.NoError(t, err)
		expectedTrieUpdate, err := pathfinder.UpdateToTrieUpdate(update, complete.DefaultPathFinderVersion)
		require.NoError(t, err)

		// Construct a payloadless proof whose leaf hash is HashLeaf(path, reg.Value).
		// The committer's reconstruction step will read reg.Value from the
		// storage snapshot, recompute HashLeaf, and verify they match.
		leafHash := hash.HashLeaf(hash.Hash(path), reg.Value)
		expectedBatch := ledger.NewPayloadlessTrieBatchProofWithEmptyProofs(1)
		expectedBatch.Proofs[0].Path = path
		expectedBatch.Proofs[0].LeafHash = &leafHash
		expectedBatch.Proofs[0].Inclusion = true
		expectedBatch.Proofs[0].Steps = 1
		expectedBatch.Proofs[0].Flags[0] = 0x80
		expectedBatch.Proofs[0].Interims = []hash.Hash{hash.DummyHash}

		setCalled := false
		proveCalled := false
		ledgerMock := &mockPayloadlessLedger{
			setFn: func(u *ledger.Update) (ledger.State, *ledger.TrieUpdate, error) {
				setCalled = true
				require.True(t, u.State().Equals(ledger.State(startState)))
				return ledger.State(endState), expectedTrieUpdate, nil
			},
			proveFn: func(q *ledger.Query) (*ledger.PayloadlessTrieBatchProof, error) {
				proveCalled = true
				require.Equal(t, 1, q.Size())
				require.True(t, q.Keys()[0].Equals(&ledgerKey))
				require.True(t, ledger.State(startState).Equals(ledger.State(q.State())))
				return expectedBatch, nil
			},
		}

		c := committer.NewPayloadlessLedgerViewCommitter(
			ledgerMock,
			trace.NewNoopTracer(),
			complete.DefaultPathFinderVersion,
		)

		// baseStorageSnapshot holds the pre-execution value (reg.Value) that
		// reconstruction will read.
		previousBlockSnapshot := storehouse.NewExecutingBlockSnapshot(
			snapshot.MapStorageSnapshot{
				reg.Key: reg.Value,
			},
			flow.StateCommitment(update.State()),
		)

		blockUpdates := &snapshot.ExecutionSnapshot{
			WriteSet: map[flow.RegisterID]flow.RegisterValue{
				reg.Key: reg.Value,
			},
		}

		newCommit, proofBytes, trieUpdate, newStorageSnapshot, err := c.CommitView(
			blockUpdates,
			previousBlockSnapshot,
		)
		require.NoError(t, err)
		require.True(t, setCalled, "Set should have been invoked")
		require.True(t, proveCalled, "Prove should have been invoked")

		// state-side assertions
		require.Equal(t, previousBlockSnapshot.Commitment(), flow.StateCommitment(trieUpdate.RootHash))
		require.Equal(t, newCommit, newStorageSnapshot.Commitment())
		require.Equal(t, endState, newCommit)
		require.True(t, expectedTrieUpdate.Equals(trieUpdate))

		// proof-side assertions: bytes decode as a full *TrieBatchProof with
		// the original payload (key + value) attached.
		require.NotEmpty(t, proofBytes)
		fullBatch, err := ledger.DecodeTrieBatchProof(proofBytes)
		require.NoError(t, err)
		require.Equal(t, 1, fullBatch.Size())

		got := fullBatch.Proofs[0]
		require.Equal(t, path, got.Path)
		require.True(t, got.Inclusion)
		require.Equal(t, expectedBatch.Proofs[0].Steps, got.Steps)
		require.Equal(t, expectedBatch.Proofs[0].Flags, got.Flags)
		require.Equal(t, expectedBatch.Proofs[0].Interims, got.Interims)
		// The reconstructed payload carries the actual value, not a hash.
		require.Equal(t, ledger.Value(reg.Value), got.Payload.Value())
	})

	t.Run("Set error is propagated", func(t *testing.T) {
		reg := unittest.MakeOwnerReg("key1", "val1")
		startState := unittest.StateCommitmentFixture()

		setErr := errors.New("boom-set")
		ledgerMock := &mockPayloadlessLedger{
			setFn: func(*ledger.Update) (ledger.State, *ledger.TrieUpdate, error) {
				return ledger.DummyState, nil, setErr
			},
			proveFn: func(*ledger.Query) (*ledger.PayloadlessTrieBatchProof, error) {
				return ledger.NewPayloadlessTrieBatchProof(), nil
			},
		}

		c := committer.NewPayloadlessLedgerViewCommitter(
			ledgerMock,
			trace.NewNoopTracer(),
			complete.DefaultPathFinderVersion,
		)

		oldReg := unittest.MakeOwnerReg("key1", "oldvalue")
		previousBlockSnapshot := storehouse.NewExecutingBlockSnapshot(
			snapshot.MapStorageSnapshot{
				oldReg.Key: oldReg.Value,
			},
			flow.StateCommitment(startState),
		)
		blockUpdates := &snapshot.ExecutionSnapshot{
			WriteSet: map[flow.RegisterID]flow.RegisterValue{
				reg.Key: oldReg.Value,
			},
		}

		_, _, _, _, err := c.CommitView(blockUpdates, previousBlockSnapshot)
		require.Error(t, err)
		require.ErrorIs(t, err, setErr)
	})

	t.Run("Prove error is propagated", func(t *testing.T) {
		reg := unittest.MakeOwnerReg("key1", "val1")
		startState := unittest.StateCommitmentFixture()
		endState := unittest.StateCommitmentFixture()

		update, err := ledger.NewUpdate(ledger.State(startState), []ledger.Key{convert.RegisterIDToLedgerKey(reg.Key)}, []ledger.Value{reg.Value})
		require.NoError(t, err)
		expectedTrieUpdate, err := pathfinder.UpdateToTrieUpdate(update, complete.DefaultPathFinderVersion)
		require.NoError(t, err)

		proveErr := errors.New("boom-prove")
		ledgerMock := &mockPayloadlessLedger{
			setFn: func(u *ledger.Update) (ledger.State, *ledger.TrieUpdate, error) {
				return ledger.State(endState), expectedTrieUpdate, nil
			},
			proveFn: func(*ledger.Query) (*ledger.PayloadlessTrieBatchProof, error) {
				return nil, proveErr
			},
		}

		c := committer.NewPayloadlessLedgerViewCommitter(
			ledgerMock,
			trace.NewNoopTracer(),
			complete.DefaultPathFinderVersion,
		)

		oldReg := unittest.MakeOwnerReg("key1", "oldvalue")
		previousBlockSnapshot := storehouse.NewExecutingBlockSnapshot(
			snapshot.MapStorageSnapshot{
				oldReg.Key: oldReg.Value,
			},
			flow.StateCommitment(startState),
		)
		blockUpdates := &snapshot.ExecutionSnapshot{
			WriteSet: map[flow.RegisterID]flow.RegisterValue{
				reg.Key: oldReg.Value,
			},
		}

		_, _, _, _, err = c.CommitView(blockUpdates, previousBlockSnapshot)
		require.Error(t, err)
		require.ErrorIs(t, err, proveErr)
	})

	t.Run("storage value mismatch surfaces ErrPayloadHashMismatch", func(t *testing.T) {
		// The proof's leafHash is built from the truthful value; the storage
		// snapshot returns a different value. Reconstruction must fail.
		reg := unittest.MakeOwnerReg("key1", "real-value")
		startState := unittest.StateCommitmentFixture()
		endState := unittest.StateCommitmentFixture()

		ledgerKey := convert.RegisterIDToLedgerKey(reg.Key)
		path, err := pathfinder.KeyToPath(ledgerKey, complete.DefaultPathFinderVersion)
		require.NoError(t, err)
		truthfulLeafHash := hash.HashLeaf(hash.Hash(path), reg.Value)

		update, err := ledger.NewUpdate(ledger.State(startState), []ledger.Key{ledgerKey}, []ledger.Value{reg.Value})
		require.NoError(t, err)
		expectedTrieUpdate, err := pathfinder.UpdateToTrieUpdate(update, complete.DefaultPathFinderVersion)
		require.NoError(t, err)

		batch := ledger.NewPayloadlessTrieBatchProofWithEmptyProofs(1)
		batch.Proofs[0].Path = path
		batch.Proofs[0].LeafHash = &truthfulLeafHash
		batch.Proofs[0].Inclusion = true

		ledgerMock := &mockPayloadlessLedger{
			setFn: func(*ledger.Update) (ledger.State, *ledger.TrieUpdate, error) {
				return ledger.State(endState), expectedTrieUpdate, nil
			},
			proveFn: func(*ledger.Query) (*ledger.PayloadlessTrieBatchProof, error) {
				return batch, nil
			},
		}

		c := committer.NewPayloadlessLedgerViewCommitter(
			ledgerMock,
			trace.NewNoopTracer(),
			complete.DefaultPathFinderVersion,
		)

		// Storage snapshot returns a wrong value for the same key.
		previousBlockSnapshot := storehouse.NewExecutingBlockSnapshot(
			snapshot.MapStorageSnapshot{
				reg.Key: []byte("lying-value"),
			},
			flow.StateCommitment(update.State()),
		)
		blockUpdates := &snapshot.ExecutionSnapshot{
			WriteSet: map[flow.RegisterID]flow.RegisterValue{
				reg.Key: reg.Value,
			},
		}

		_, _, _, _, err = c.CommitView(blockUpdates, previousBlockSnapshot)
		require.Error(t, err)
	})

	t.Run("query is built from AllRegisterIDs including both reads and writes", func(t *testing.T) {
		readReg := unittest.MakeOwnerReg("read", "rv")
		writeReg := unittest.MakeOwnerReg("write", "wv")
		startState := unittest.StateCommitmentFixture()
		endState := unittest.StateCommitmentFixture()

		// Capture the query rather than asserting inside the closure; a failed
		// require inside a goroutine would leave the committer's WaitGroup
		// unsignalled and the test would hang instead of failing.
		var capturedKeys []ledger.Key
		ledgerMock := &mockPayloadlessLedger{
			setFn: func(*ledger.Update) (ledger.State, *ledger.TrieUpdate, error) {
				return ledger.State(endState), &ledger.TrieUpdate{RootHash: ledger.RootHash(endState)}, nil
			},
			proveFn: func(q *ledger.Query) (*ledger.PayloadlessTrieBatchProof, error) {
				capturedKeys = append(capturedKeys, q.Keys()...)
				return ledger.NewPayloadlessTrieBatchProof(), nil
			},
		}
		c := committer.NewPayloadlessLedgerViewCommitter(
			ledgerMock,
			trace.NewNoopTracer(),
			complete.DefaultPathFinderVersion,
		)

		previousBlockSnapshot := storehouse.NewExecutingBlockSnapshot(
			snapshot.MapStorageSnapshot{readReg.Key: readReg.Value},
			flow.StateCommitment(startState),
		)
		blockUpdates := &snapshot.ExecutionSnapshot{
			ReadSet:  map[flow.RegisterID]struct{}{readReg.Key: {}},
			WriteSet: map[flow.RegisterID]flow.RegisterValue{writeReg.Key: writeReg.Value},
		}

		_, _, _, _, err := c.CommitView(blockUpdates, previousBlockSnapshot)
		require.NoError(t, err)

		// Both read and write register IDs must appear in the query. Compare
		// LedgerKey directly to avoid owner-padding differences between the
		// pre-conversion RegisterID (raw "owner" string) and the
		// round-tripped one (RegisterIDToLedgerKey pads the owner).
		require.Equal(t, 2, len(capturedKeys))
		readKey := convert.RegisterIDToLedgerKey(readReg.Key)
		writeKey := convert.RegisterIDToLedgerKey(writeReg.Key)
		seen := map[string]bool{
			ledgerKeyString(readKey):  false,
			ledgerKeyString(writeKey): false,
		}
		for _, k := range capturedKeys {
			s := ledgerKeyString(k)
			_, ok := seen[s]
			require.True(t, ok, "unexpected key in query: %s", s)
			seen[s] = true
		}
		for s, ok := range seen {
			require.True(t, ok, "missing key in query: %s", s)
		}
	})
}

func ledgerKeyString(k ledger.Key) string {
	parts := make([]string, 0, len(k.KeyParts))
	for _, p := range k.KeyParts {
		parts = append(parts, fmt.Sprintf("%d:%x", p.Type, p.Value))
	}
	return fmt.Sprintf("%v", parts)
}
