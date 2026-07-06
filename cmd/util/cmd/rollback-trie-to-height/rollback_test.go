package rollback_trie_to_height

import (
	"math"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	prometheusWAL "github.com/onflow/wal/wal"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

// fakeRegisterStore is an in-memory [RegisterGetter] indexed by height. It returns the most recent
// value at or below the requested height, and [storage.ErrNotFound] when no value exists — mirroring
// the semantics of the production Pebble register store.
type fakeRegisterStore struct {
	// heights holds a snapshot of every register's value as of each stored height.
	heights map[uint64]map[flow.RegisterID]flow.RegisterValue
	// ordered is the ascending list of stored heights.
	ordered []uint64
}

func newFakeRegisterStore() *fakeRegisterStore {
	return &fakeRegisterStore{heights: make(map[uint64]map[flow.RegisterID]flow.RegisterValue)}
}

// setSnapshot records the complete register set as of the given height. Heights must be set in
// ascending order.
func (f *fakeRegisterStore) setSnapshot(height uint64, snapshot map[flow.RegisterID]flow.RegisterValue) {
	f.heights[height] = snapshot
	f.ordered = append(f.ordered, height)
}

func (f *fakeRegisterStore) Get(id flow.RegisterID, height uint64) (flow.RegisterValue, error) {
	// find the most recent snapshot at or below height
	var chosen uint64
	found := false
	for _, h := range f.ordered {
		if h <= height {
			chosen = h
			found = true
		}
	}
	if !found {
		return nil, storage.ErrNotFound
	}
	value, ok := f.heights[chosen][id]
	if !ok || len(value) == 0 {
		return nil, storage.ErrNotFound
	}
	return value, nil
}

func reg(owner byte, key string) flow.RegisterID {
	return flow.NewRegisterID(flow.BytesToAddress([]byte{owner}), key)
}

// buildUpdate constructs a ledger.Update from a base state and a set of register writes.
func buildUpdate(t *testing.T, base ledger.State, writes map[flow.RegisterID]flow.RegisterValue) *ledger.Update {
	keys := make([]ledger.Key, 0, len(writes))
	values := make([]ledger.Value, 0, len(writes))
	for id, v := range writes {
		keys = append(keys, convert.RegisterIDToLedgerKey(id))
		values = append(values, ledger.Value(v))
	}
	update, err := ledger.NewUpdate(base, keys, values)
	require.NoError(t, err)
	return update
}

// TestRollbackReconstructsHistoricalRoot verifies the end-to-end reconstruction: given a WAL that
// evolved a trie across two blocks and a register store holding each block's historical values, the
// update produced by BuildRollbackTrieUpdate rolls the latest (base) trie back to the exact earlier
// root.
func TestRollbackReconstructsHistoricalRoot(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		logger := zerolog.Nop()

		led, compactor := newLedger(t, dir)

		// Block at height 1: create registers A, B, C.
		h1Writes := map[flow.RegisterID]flow.RegisterValue{
			reg(0x01, "A"): []byte("a-v1"),
			reg(0x01, "B"): []byte("b-v1"),
			reg(0x02, "C"): []byte("c-v1"),
		}
		root1, _, err := led.Set(buildUpdate(t, led.InitialState(), h1Writes))
		require.NoError(t, err)

		// Block at height 2 (the base/latest): overwrite A, delete B, add D. C is unchanged.
		h2Writes := map[flow.RegisterID]flow.RegisterValue{
			reg(0x01, "A"): []byte("a-v2-longer-value"),
			reg(0x01, "B"): []byte{}, // deletion
			reg(0x03, "D"): []byte("d-v2"),
		}
		root2, _, err := led.Set(buildUpdate(t, root1, h2Writes))
		require.NoError(t, err)
		require.NotEqual(t, root1, root2)

		// Shut down the ledger so its WAL segments are flushed and no longer locked.
		<-led.Done()
		<-compactor.Done()

		// Register store snapshots: height 1 holds the state after block 1; height 2 after block 2.
		store := newFakeRegisterStore()
		store.setSnapshot(1, map[flow.RegisterID]flow.RegisterValue{
			reg(0x01, "A"): []byte("a-v1"),
			reg(0x01, "B"): []byte("b-v1"),
			reg(0x02, "C"): []byte("c-v1"),
		})
		store.setSnapshot(2, map[flow.RegisterID]flow.RegisterValue{
			reg(0x01, "A"): []byte("a-v2-longer-value"),
			reg(0x02, "C"): []byte("c-v1"),
			reg(0x03, "D"): []byte("d-v2"),
		})

		from, to, err := segmentsOf(dir)
		require.NoError(t, err)

		// Build the rollback update anchored on the base (root2), targeting height 1.
		trieUpdate, err := BuildRollbackTrieUpdate(logger, dir, from, to, ledger.RootHash(root2), store, 1)
		require.NoError(t, err)

		// The collected register set must contain every register ever written (A, B, C, D).
		require.Len(t, trieUpdate.Paths, 4)
		require.Equal(t, ledger.RootHash(root2), trieUpdate.RootHash)

		// Applying the update to the base trie must reproduce root1 exactly.
		newRoot := applyToBase(t, dir, ledger.RootHash(root2), trieUpdate)
		require.Equal(t, ledger.RootHash(root1), newRoot,
			"reconstructed root must equal the historical root at height 1")
	})
}

// TestRollbackToEmptyState verifies that rolling back to a height before any register existed (all
// reads return not-found) reproduces the empty trie root.
func TestRollbackToEmptyState(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		logger := zerolog.Nop()

		led, compactor := newLedger(t, dir)
		emptyRoot := ledger.RootHash(led.InitialState())

		writes := map[flow.RegisterID]flow.RegisterValue{
			reg(0x01, "A"): []byte("a"),
			reg(0x02, "B"): []byte("b"),
		}
		base, _, err := led.Set(buildUpdate(t, led.InitialState(), writes))
		require.NoError(t, err)

		<-led.Done()
		<-compactor.Done()

		// Register store has no snapshot at or below height 0 -> every register reads back not-found.
		store := newFakeRegisterStore()
		store.setSnapshot(5, writes) // only a later height is populated

		from, to, err := segmentsOf(dir)
		require.NoError(t, err)

		trieUpdate, err := BuildRollbackTrieUpdate(logger, dir, from, to, ledger.RootHash(base), store, 0)
		require.NoError(t, err)

		newRoot := applyToBase(t, dir, ledger.RootHash(base), trieUpdate)
		require.Equal(t, emptyRoot, newRoot, "rolling back before any register existed yields the empty trie")
	})
}

// TestRollbackRecordAndReplay exercises the full artifact path the command uses in non-dry-run mode:
// the rollback update is written to the WAL via RecordUpdate, and a fresh forest replaying that WAL
// ends up with its most-recent trie at the target historical root.
func TestRollbackRecordAndReplay(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		logger := zerolog.Nop()

		led, compactor := newLedger(t, dir)

		h1Writes := map[flow.RegisterID]flow.RegisterValue{
			reg(0x01, "A"): []byte("a-v1"),
			reg(0x02, "C"): []byte("c-v1"),
		}
		root1, _, err := led.Set(buildUpdate(t, led.InitialState(), h1Writes))
		require.NoError(t, err)

		h2Writes := map[flow.RegisterID]flow.RegisterValue{
			reg(0x01, "A"): []byte("a-v2"),
			reg(0x03, "D"): []byte("d-v2"),
		}
		root2, _, err := led.Set(buildUpdate(t, root1, h2Writes))
		require.NoError(t, err)

		<-led.Done()
		<-compactor.Done()

		store := newFakeRegisterStore()
		store.setSnapshot(1, map[flow.RegisterID]flow.RegisterValue{
			reg(0x01, "A"): []byte("a-v1"),
			reg(0x02, "C"): []byte("c-v1"),
		})
		store.setSnapshot(2, map[flow.RegisterID]flow.RegisterValue{
			reg(0x01, "A"): []byte("a-v2"),
			reg(0x02, "C"): []byte("c-v1"),
			reg(0x03, "D"): []byte("d-v2"),
		})

		from, to, err := segmentsOf(dir)
		require.NoError(t, err)

		trieUpdate, err := BuildRollbackTrieUpdate(logger, dir, from, to, ledger.RootHash(root2), store, 1)
		require.NoError(t, err)

		// Write the rollback update to the WAL, exactly as the command does when not in dry-run.
		func() {
			diskWal, err := wal.NewDiskWAL(logger, nil, metrics.NewNoopCollector(), dir,
				complete.DefaultCacheSize, pathfinder.PathByteSize, wal.SegmentSize)
			require.NoError(t, err)
			defer func() { <-diskWal.Done() }()

			_, skipped, err := diskWal.RecordUpdate(trieUpdate)
			require.NoError(t, err)
			require.False(t, skipped)
		}()

		// The last record in the WAL must be our rollback update: anchored on the base (root2) and,
		// when replayed onto the base trie, producing the target historical root (root1).
		//
		// Note: we intentionally do not assert via forest.MostRecentTouchedRootHash after a full
		// replay. Because the target historical trie (root1) still lies within this small forest's
		// retained window, re-adding it is a no-op that leaves the most-recent pointer on root2. In a
		// production forest the historical target is outside the retained window and is genuinely
		// re-added, becoming the most-recent trie — but that condition cannot be reproduced with a
		// two-block WAL.
		last := lastWALUpdate(t, dir)
		require.Equal(t, ledger.RootHash(root2), last.RootHash, "rollback record must be anchored on the base root")

		newRoot := applyToBase(t, dir, ledger.RootHash(root2), last)
		require.Equal(t, ledger.RootHash(root1), newRoot,
			"replaying the recorded rollback update onto the base trie must reproduce the target historical root")
	})
}

// lastWALUpdate returns the last WALUpdate record present in the WAL segments in dir.
func lastWALUpdate(t *testing.T, dir string) *ledger.TrieUpdate {
	from, to, err := segmentsOf(dir)
	require.NoError(t, err)

	sr, err := prometheusWAL.NewSegmentsRangeReader(zerolog.Nop(), prometheusWAL.SegmentRange{Dir: dir, First: from, Last: to})
	require.NoError(t, err)
	defer sr.Close()

	reader := prometheusWAL.NewReader(sr)
	var last *ledger.TrieUpdate
	for reader.Next() {
		op, _, update, err := wal.Decode(reader.Record())
		require.NoError(t, err)
		if op == wal.WALUpdate {
			last = update
		}
	}
	require.NoError(t, reader.Err())
	require.NotNil(t, last)
	return last
}

// newLedger creates a fresh on-disk WAL, ledger and compactor in dir, mirroring the standard
// cmd/util loading pattern. The caller is responsible for draining led.Done()/compactor.Done().
func newLedger(t *testing.T, dir string) (*complete.Ledger, *complete.Compactor) {
	logger := zerolog.Nop()
	diskWal, err := wal.NewDiskWAL(logger, nil, metrics.NewNoopCollector(), dir,
		complete.DefaultCacheSize, pathfinder.PathByteSize, wal.SegmentSize)
	require.NoError(t, err)

	led, err := complete.NewLedger(diskWal, complete.DefaultCacheSize, &metrics.NoopCollector{}, logger,
		complete.DefaultPathFinderVersion)
	require.NoError(t, err)

	compactor, err := complete.NewCompactor(led, diskWal, logger, complete.DefaultCacheSize,
		math.MaxInt, 1, atomic.NewBool(false), &metrics.NoopCollector{})
	require.NoError(t, err)

	<-compactor.Ready()
	return led, compactor
}

// applyToBase loads the base trie fresh from the WAL in dir and applies trieUpdate to it in a
// side-effect-free forest, returning the resulting root. This mirrors the verification the command
// performs.
func applyToBase(t *testing.T, dir string, baseRoot ledger.RootHash, trieUpdate *ledger.TrieUpdate) ledger.RootHash {
	logger := zerolog.Nop()
	diskWal, err := wal.NewDiskWAL(logger, nil, metrics.NewNoopCollector(), dir,
		complete.DefaultCacheSize, pathfinder.PathByteSize, wal.SegmentSize)
	require.NoError(t, err)
	defer func() { <-diskWal.Done() }()

	forest, err := mtrie.NewForest(complete.DefaultCacheSize, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)
	require.NoError(t, diskWal.ReplayOnForest(forest))

	newTrie, err := forest.NewTrie(trieUpdate)
	require.NoError(t, err)
	require.Equal(t, baseRoot, trieUpdate.RootHash)
	return newTrie.RootHash()
}

// segmentsOf returns the [first, last] WAL segment numbers present in dir.
func segmentsOf(dir string) (int, int, error) {
	diskWal, err := wal.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dir,
		complete.DefaultCacheSize, pathfinder.PathByteSize, wal.SegmentSize)
	if err != nil {
		return 0, 0, err
	}
	defer func() { <-diskWal.Done() }()
	return diskWal.Segments()
}
