package complete_test

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal/fixtures"
	"github.com/onflow/flow-go/module/metrics"
)

// newPayloadlessLedger constructs a default payloadless ledger for tests.
// It uses a nil WAL, which keeps Set synchronous and avoids the need for a
// compactor in these tests.
func newPayloadlessLedger(t *testing.T) *complete.PayloadlessLedger {
	t.Helper()
	l, err := complete.NewPayloadlessLedger(nil, 100, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
	require.NoError(t, err)
	return l
}

// newFullLedger constructs a full ledger backed by a NoopWAL for tests.
// A NoopCompactor is started and stopped via t.Cleanup so the ledger's
// trie update channel is drained.
func newFullLedger(t *testing.T) *complete.Ledger {
	t.Helper()
	wal := &fixtures.NoopWAL{}
	l, err := complete.NewLedger(wal, 100, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
	require.NoError(t, err)
	compactor := fixtures.NewNoopCompactor(l)
	<-compactor.Ready()
	t.Cleanup(func() {
		<-l.Done()
		<-compactor.Done()
	})
	return l
}

// expectedLeafHash returns HashLeaf(path(key), value). This is the height-0
// commitment that the payloadless trie stores at the leaf for the given key.
func expectedLeafHash(t *testing.T, key ledger.Key, value ledger.Value) hash.Hash {
	t.Helper()
	path, err := pathfinder.KeyToPath(key, complete.DefaultPathFinderVersion)
	require.NoError(t, err)
	return hash.HashLeaf(hash.Hash(path), []byte(value))
}

// TestNewPayloadlessLedger verifies the constructor succeeds and the
// resulting ledger is immediately ready (no WAL replay phase).
func TestNewPayloadlessLedger(t *testing.T) {
	l := newPayloadlessLedger(t)

	// Ready and Done are no-ops; both channels should already be closed.
	select {
	case <-l.Ready():
	default:
		t.Fatal("Ready() channel should be closed immediately")
	}
	select {
	case <-l.Done():
	default:
		t.Fatal("Done() channel should be closed immediately")
	}
}

func TestPayloadlessLedger_Set(t *testing.T) {
	t.Run("empty update returns same state", func(t *testing.T) {
		l := newPayloadlessLedger(t)

		state := l.InitialState()
		up, err := ledger.NewEmptyUpdate(state)
		require.NoError(t, err)

		newState, trieUpdate, err := l.Set(up)
		require.NoError(t, err)
		require.True(t, trieUpdate.IsEmpty())
		assert.Equal(t, state, newState)
	})

	t.Run("non-empty update advances state", func(t *testing.T) {
		l := newPayloadlessLedger(t)
		state := l.InitialState()

		u := testutils.UpdateFixture()
		u.SetState(state)

		newState, trieUpdate, err := l.Set(u)
		require.NoError(t, err)
		assert.NotEqual(t, state, newState)
		assert.False(t, trieUpdate.IsEmpty())
		assert.True(t, l.HasState(newState))
	})
}

func TestPayloadlessLedger_HasPaths(t *testing.T) {
	l := newPayloadlessLedger(t)
	state := l.InitialState()

	u := testutils.UpdateFixture()
	u.SetState(state)
	newState, _, err := l.Set(u)
	require.NoError(t, err)

	// Allocated keys report true.
	q, err := ledger.NewQuery(newState, u.Keys())
	require.NoError(t, err)
	exists, err := l.HasPaths(q)
	require.NoError(t, err)
	require.Len(t, exists, len(u.Keys()))
	for i, has := range exists {
		assert.Truef(t, has, "key %d should report allocated", i)
	}

	// Random unrelated keys report false.
	unallocated := testutils.RandomUniqueKeys(5, 2, 1, 10)
	q2, err := ledger.NewQuery(newState, unallocated)
	require.NoError(t, err)
	exists2, err := l.HasPaths(q2)
	require.NoError(t, err)
	for i, has := range exists2 {
		assert.Falsef(t, has, "unallocated key %d should report unallocated", i)
	}
}

func TestPayloadlessLedger_GetSingleLeafHash(t *testing.T) {
	l := newPayloadlessLedger(t)
	state := l.InitialState()

	u := testutils.UpdateFixture()
	u.SetState(state)
	newState, _, err := l.Set(u)
	require.NoError(t, err)

	t.Run("allocated key returns expected leaf hash", func(t *testing.T) {
		for i, k := range u.Keys() {
			q, err := ledger.NewQuerySingleValue(newState, k)
			require.NoError(t, err)
			got, err := l.GetSingleLeafHash(q)
			require.NoError(t, err)
			require.NotNilf(t, got, "key %d should have a leaf hash", i)

			expected := expectedLeafHash(t, k, u.Values()[i])
			assert.Equal(t, expected, *got)
		}
	})

	t.Run("unallocated key returns nil", func(t *testing.T) {
		unallocated := testutils.RandomUniqueKeys(3, 2, 1, 10)
		for _, k := range unallocated {
			q, err := ledger.NewQuerySingleValue(newState, k)
			require.NoError(t, err)
			got, err := l.GetSingleLeafHash(q)
			require.NoError(t, err)
			assert.Nil(t, got)
		}
	})
}

func TestPayloadlessLedger_GetLeafHashes(t *testing.T) {
	l := newPayloadlessLedger(t)
	state := l.InitialState()

	u := testutils.UpdateFixture()
	u.SetState(state)
	newState, _, err := l.Set(u)
	require.NoError(t, err)

	t.Run("allocated keys return expected leaf hashes in order", func(t *testing.T) {
		q, err := ledger.NewQuery(newState, u.Keys())
		require.NoError(t, err)
		got, err := l.GetLeafHashes(q)
		require.NoError(t, err)
		require.Len(t, got, len(u.Keys()))

		for i, k := range u.Keys() {
			require.NotNilf(t, got[i], "key %d should have a leaf hash", i)
			expected := expectedLeafHash(t, k, u.Values()[i])
			assert.Equal(t, expected, *got[i])
		}
	})

	t.Run("unallocated keys return nil entries", func(t *testing.T) {
		unallocated := testutils.RandomUniqueKeys(3, 2, 1, 10)
		q, err := ledger.NewQuery(newState, unallocated)
		require.NoError(t, err)
		got, err := l.GetLeafHashes(q)
		require.NoError(t, err)
		require.Len(t, got, len(unallocated))
		for i, h := range got {
			assert.Nilf(t, h, "unallocated key %d should produce nil", i)
		}
	})
}

func TestPayloadlessLedger_Prove(t *testing.T) {
	l := newPayloadlessLedger(t)
	state := l.InitialState()

	u := testutils.UpdateFixture()
	u.SetState(state)
	newState, _, err := l.Set(u)
	require.NoError(t, err)

	q, err := ledger.NewQuery(newState, u.Keys())
	require.NoError(t, err)
	batch, err := l.Prove(q)
	require.NoError(t, err)
	require.NotNil(t, batch)
	assert.Equal(t, len(u.Keys()), batch.Size())

	// Each inclusion proof should carry a leaf hash that matches HashLeaf(path, value).
	expectedByPath := make(map[ledger.Path]hash.Hash, len(u.Keys()))
	for i, k := range u.Keys() {
		path, err := pathfinder.KeyToPath(k, complete.DefaultPathFinderVersion)
		require.NoError(t, err)
		expectedByPath[path] = expectedLeafHash(t, k, u.Values()[i])
	}
	for i, p := range batch.Proofs {
		require.Truef(t, p.Inclusion, "proof %d should be inclusion", i)
		require.NotNilf(t, p.LeafHash, "proof %d should carry a leaf hash", i)
		expected, ok := expectedByPath[p.Path]
		require.Truef(t, ok, "proof %d path not in expected set", i)
		assert.Equalf(t, expected, *p.LeafHash, "proof %d leaf hash mismatch", i)
	}
}

// Equivalence tests: drive complete.Ledger and complete.PayloadlessLedger
// through identical inputs and verify their observable outputs agree.

// TestPayloadlessLedger_Equivalence_EmptyState verifies both implementations
// report the same initial root hash.
func TestPayloadlessLedger_Equivalence_EmptyState(t *testing.T) {
	full := newFullLedger(t)
	pl := newPayloadlessLedger(t)

	assert.Equal(t, full.InitialState(), pl.InitialState())
}

// TestPayloadlessLedger_Equivalence_Set verifies a Set with the same Update
// produces the same resulting state on both ledgers.
func TestPayloadlessLedger_Equivalence_Set(t *testing.T) {
	full := newFullLedger(t)
	pl := newPayloadlessLedger(t)

	state := full.InitialState()
	uFull := testutils.UpdateFixture()
	uFull.SetState(state)
	uPL := testutils.UpdateFixture()
	uPL.SetState(state)

	fullNew, _, err := full.Set(uFull)
	require.NoError(t, err)

	plNew, _, err := pl.Set(uPL)
	require.NoError(t, err)

	assert.Equal(t, fullNew, plNew, "states should agree after identical update")
	assert.True(t, full.HasState(fullNew))
	assert.True(t, pl.HasState(plNew))
}

// TestPayloadlessLedger_Equivalence_Reads verifies that for every allocated
// key, the payloadless leaf hash equals HashLeaf(path, fullLedgerValue).
func TestPayloadlessLedger_Equivalence_Reads(t *testing.T) {
	full := newFullLedger(t)
	pl := newPayloadlessLedger(t)

	state := full.InitialState()
	uFull := testutils.UpdateFixture()
	uFull.SetState(state)
	uPL := testutils.UpdateFixture()
	uPL.SetState(state)

	fullNew, _, err := full.Set(uFull)
	require.NoError(t, err)
	plNew, _, err := pl.Set(uPL)
	require.NoError(t, err)
	require.Equal(t, fullNew, plNew)

	// Compare each allocated key.
	fullQ, err := ledger.NewQuery(fullNew, uFull.Keys())
	require.NoError(t, err)
	values, err := full.Get(fullQ)
	require.NoError(t, err)

	plQ, err := ledger.NewQuery(plNew, uFull.Keys())
	require.NoError(t, err)
	leafHashes, err := pl.GetLeafHashes(plQ)
	require.NoError(t, err)

	require.Equal(t, len(uFull.Keys()), len(values))
	require.Equal(t, len(uFull.Keys()), len(leafHashes))
	for i, k := range uFull.Keys() {
		require.NotNil(t, leafHashes[i])
		expected := expectedLeafHash(t, k, values[i])
		assert.Equalf(t, expected, *leafHashes[i], "key %d: payloadless leaf hash must equal HashLeaf(path, fullValue)", i)
	}
}

// TestPayloadlessLedger_Equivalence_HasPaths verifies that HasPaths agrees
// with the full ledger's ValueSizes>0 for the same query.
func TestPayloadlessLedger_Equivalence_HasPaths(t *testing.T) {
	full := newFullLedger(t)
	pl := newPayloadlessLedger(t)

	state := full.InitialState()
	uFull := testutils.UpdateFixture()
	uFull.SetState(state)
	uPL := testutils.UpdateFixture()
	uPL.SetState(state)

	fullNew, _, err := full.Set(uFull)
	require.NoError(t, err)
	plNew, _, err := pl.Set(uPL)
	require.NoError(t, err)
	require.Equal(t, fullNew, plNew)

	// Mix of allocated and unallocated keys.
	queryKeys := append([]ledger.Key{}, uFull.Keys()...)
	queryKeys = append(queryKeys, testutils.RandomUniqueKeys(5, 2, 1, 10)...)

	fullQ, err := ledger.NewQuery(fullNew, queryKeys)
	require.NoError(t, err)
	sizes, err := full.ValueSizes(fullQ)
	require.NoError(t, err)

	plQ, err := ledger.NewQuery(plNew, queryKeys)
	require.NoError(t, err)
	exists, err := pl.HasPaths(plQ)
	require.NoError(t, err)

	require.Equal(t, len(queryKeys), len(sizes))
	require.Equal(t, len(queryKeys), len(exists))
	for i := range queryKeys {
		assert.Equalf(t, sizes[i] > 0, exists[i], "key %d: HasPaths should agree with ValueSizes>0", i)
	}
}

// TestPayloadlessLedger_Equivalence_IncrementalUpdates verifies state
// agreement across multiple rounds of updates.
func TestPayloadlessLedger_Equivalence_IncrementalUpdates(t *testing.T) {
	full := newFullLedger(t)
	pl := newPayloadlessLedger(t)

	fullState := full.InitialState()
	plState := pl.InitialState()
	require.Equal(t, fullState, plState)

	for round := 1; round <= 5; round++ {
		uFull := testutils.UpdateFixture()
		uFull.SetState(fullState)
		uPL := testutils.UpdateFixture()
		uPL.SetState(plState)

		var err error
		fullState, _, err = full.Set(uFull)
		require.NoErrorf(t, err, "round %d full.Set", round)
		plState, _, err = pl.Set(uPL)
		require.NoErrorf(t, err, "round %d pl.Set", round)

		require.Equalf(t, fullState, plState, "round %d: states diverged", round)
	}
}
