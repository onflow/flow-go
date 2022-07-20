package complete

import (
	"fmt"
	"os"
	"path"
	"reflect"
	"testing"
	"time"

	prometheusWAL "github.com/m4ksio/wal/wal"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	realWAL "github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

// Compactor observer that waits until it gets notified of a
// latest checkpoint larger than fromBound
type CompactorObserver struct {
	fromBound int
	done      chan struct{}
}

func (co *CompactorObserver) OnNext(val interface{}) {
	res, ok := val.(int)
	if ok {
		new := res
		fmt.Printf("Compactor observer received checkpoint num %d\n", new)
		if new >= co.fromBound {
			co.done <- struct{}{}
		}
	}
}
func (co *CompactorObserver) OnError(err error) {}
func (co *CompactorObserver) OnComplete() {
	close(co.done)
}

func TestCompactor(t *testing.T) {
	numInsPerStep := 2
	pathByteSize := 32
	minPayloadByteSize := 2 << 15
	maxPayloadByteSize := 2 << 16
	size := 10
	metricsCollector := &metrics.NoopCollector{}
	checkpointDistance := uint(2)
	checkpointsToKeep := uint(1)

	unittest.RunWithTempDir(t, func(dir string) {

		var l *Ledger

		// saved data after updates
		savedData := make(map[ledger.RootHash]map[string]*ledger.Payload)

		t.Run("creates checkpoints", func(t *testing.T) {

			wal, err := realWAL.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dir, size*10, pathByteSize, 32*1024)
			require.NoError(t, err)

			l, err = NewSyncLedger(wal, size*10, metricsCollector, zerolog.Logger{}, DefaultPathFinderVersion)
			require.NoError(t, err)

			// WAL segments are 32kB, so here we generate 2 keys 64kB each, times `size`
			// so we should get at least `size` segments

			compactor, err := NewCompactor(l, wal, checkpointDistance, checkpointsToKeep, zerolog.Nop())
			require.NoError(t, err)

			co := CompactorObserver{fromBound: 9, done: make(chan struct{})}
			compactor.Subscribe(&co)

			// Run Compactor in background.
			<-compactor.Ready()

			rootState := l.InitialState()

			// Generate the tree and create WAL
			for i := 0; i < size; i++ {

				payloads := utils.RandomPayloads(numInsPerStep, minPayloadByteSize, maxPayloadByteSize)

				keys := make([]ledger.Key, len(payloads))
				values := make([]ledger.Value, len(payloads))
				for i, p := range payloads {
					keys[i] = p.Key
					values[i] = p.Value
				}

				update, err := ledger.NewUpdate(rootState, keys, values)
				require.NoError(t, err)

				newState, _, err := l.Set(update)
				require.NoError(t, err)

				require.FileExists(t, path.Join(dir, realWAL.NumberToFilenamePart(i)))

				data := make(map[string]*ledger.Payload, len(keys))
				for j, k := range keys {
					ks := string(k.CanonicalForm())
					data[ks] = payloads[j]
				}

				savedData[ledger.RootHash(newState)] = data

				rootState = newState
			}

			// wait for the bound-checking observer to confirm checkpoints have been made
			select {
			case <-co.done:
				// continue
			case <-time.After(60 * time.Second):
				assert.FailNow(t, "timed out")
			}

			checkpointer, err := wal.NewCheckpointer()
			require.NoError(t, err)

			from, to, err := checkpointer.NotCheckpointedSegments()
			require.NoError(t, err)

			assert.True(t, from == 10 && to == 10, "from: %v, to: %v", from, to) // Make sure there is no leftover

			require.NoFileExists(t, path.Join(dir, "checkpoint.00000000"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000001"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000002"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000003"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000004"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000005"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000006"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000007"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000008"))
			require.FileExists(t, path.Join(dir, "checkpoint.00000009"))

			ledgerDone := l.Done()
			compactorDone := compactor.Done()

			<-ledgerDone
			<-compactorDone
		})

		time.Sleep(2 * time.Second)

		t.Run("remove unnecessary files", func(t *testing.T) {
			// Remove all files apart from target checkpoint and WAL segments ahead of it
			// We know their names, so just hardcode them
			dirF, _ := os.Open(dir)
			files, _ := dirF.Readdir(0)

			for _, fileInfo := range files {

				name := fileInfo.Name()

				if name != "checkpoint.00000009" &&
					name != "00000010" {
					err := os.Remove(path.Join(dir, name))
					require.NoError(t, err)
				}
			}
		})

		var l2 *Ledger

		time.Sleep(2 * time.Second)

		t.Run("load data from checkpoint and WAL", func(t *testing.T) {

			wal2, err := realWAL.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dir, size*10, pathByteSize, 32*1024)
			require.NoError(t, err)

			l2, err = NewSyncLedger(wal2, size*10, metricsCollector, zerolog.Logger{}, DefaultPathFinderVersion)
			require.NoError(t, err)

			<-wal2.Done()
		})

		t.Run("make sure forests are equal", func(t *testing.T) {

			// Check for same data
			for rootHash, data := range savedData {

				keys := make([]ledger.Key, 0, len(data))
				for _, p := range data {
					keys = append(keys, p.Key)
				}

				q, err := ledger.NewQuery(ledger.State(rootHash), keys)
				require.NoError(t, err)

				values, err := l.Get(q)
				require.NoError(t, err)

				values2, err := l2.Get(q)
				require.NoError(t, err)

				for i, k := range keys {
					ks := k.CanonicalForm()
					require.Equal(t, data[string(ks)].Value, values[i])
					require.Equal(t, data[string(ks)].Value, values2[i])
				}
			}

			forestTries, err := l.Tries()
			require.NoError(t, err)

			forestTriesSet := make(map[ledger.RootHash]struct{})
			for _, trie := range forestTries {
				forestTriesSet[trie.RootHash()] = struct{}{}
			}

			forestTries2, err := l.Tries()
			require.NoError(t, err)

			forestTries2Set := make(map[ledger.RootHash]struct{})
			for _, trie := range forestTries2 {
				forestTries2Set[trie.RootHash()] = struct{}{}
			}

			require.Equal(t, forestTriesSet, forestTries2Set)
		})

	})
}

// TestCompactorAccuracy expects checkpointed tries to match replayed tries.
// Replayed tries are tries updated by replaying all WAL segments
// (from segment 0, ignoring prior checkpoints) to the checkpoint number.
// This verifies that checkpointed tries are snapshopt of segments and at segment boundary.
func TestCompactorAccuracy(t *testing.T) {
	numInsPerStep := 2
	pathByteSize := 32
	minPayloadByteSize := 2 << 15
	maxPayloadByteSize := 2 << 16
	size := 10
	metricsCollector := &metrics.NoopCollector{}
	checkpointDistance := uint(5)
	checkpointsToKeep := uint(0)

	unittest.RunWithTempDir(t, func(dir string) {

		lastCheckpointNum := -1

		rootHash := trie.EmptyTrieRootHash()

		// Create DiskWAL and Ledger repeatedly to test rebuilding ledger state at restart.
		for i := 0; i < 3; i++ {

			wal, err := realWAL.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dir, size*10, pathByteSize, 32*1024)
			require.NoError(t, err)

			l, err := NewSyncLedger(wal, size*10, metricsCollector, zerolog.Logger{}, DefaultPathFinderVersion)
			require.NoError(t, err)

			checkpointer, err := wal.NewCheckpointer()
			require.NoError(t, err)

			// WAL segments are 32kB, so here we generate 2 keys 64kB each, times `size`
			// so we should get at least `size` segments

			compactor, err := NewCompactor(l, wal, checkpointDistance, checkpointsToKeep, zerolog.Nop())
			require.NoError(t, err)

			fromBound := lastCheckpointNum + int(checkpointDistance)*2

			co := CompactorObserver{fromBound: fromBound, done: make(chan struct{})}
			compactor.Subscribe(&co)

			// Run Compactor in background.
			<-compactor.Ready()

			// Generate the tree and create WAL
			for i := 0; i < size; i++ {

				payloads := utils.RandomPayloads(numInsPerStep, minPayloadByteSize, maxPayloadByteSize)

				keys := make([]ledger.Key, len(payloads))
				values := make([]ledger.Value, len(payloads))
				for i, p := range payloads {
					keys[i] = p.Key
					values[i] = p.Value
				}

				update, err := ledger.NewUpdate(ledger.State(rootHash), keys, values)
				require.NoError(t, err)

				newState, _, err := l.Set(update)
				require.NoError(t, err)

				rootHash = ledger.RootHash(newState)
			}

			// wait for the bound-checking observer to confirm checkpoints have been made
			select {
			case <-co.done:
				// continue
			case <-time.After(60 * time.Second):
				assert.FailNow(t, "timed out")
			}

			// Shutdown ledger and compactor
			ledgerDone := l.Done()
			compactorDone := compactor.Done()

			<-ledgerDone
			<-compactorDone

			nums, err := checkpointer.Checkpoints()
			require.NoError(t, err)

			for _, n := range nums {
				testCheckpointedTriesMatchReplayedTriesFromSegments(t, checkpointer, n, dir)
			}
		}
	})
}

func testCheckpointedTriesMatchReplayedTriesFromSegments(t *testing.T, checkpointer *realWAL.Checkpointer, checkpointNum int, dir string) {

	// Get tries by replaying segments up to checkpoint number
	triesFromReplayingSegments, err := triesUpToSegment(dir, checkpointNum)
	require.NoError(t, err)

	// Get tries by loading checkpoint
	triesFromLoadingCheckpoint, err := checkpointer.LoadCheckpoint(checkpointNum)
	require.NoError(t, err)

	// Test that checkpointed tries match replayed tries.
	triesSetFromReplayingSegments := make(map[ledger.RootHash]struct{})
	for _, t := range triesFromReplayingSegments {
		triesSetFromReplayingSegments[t.RootHash()] = struct{}{}
	}

	triesSetFromLoadingCheckpoint := make(map[ledger.RootHash]struct{})
	for _, t := range triesFromLoadingCheckpoint {
		triesSetFromLoadingCheckpoint[t.RootHash()] = struct{}{}
	}

	require.True(t, reflect.DeepEqual(triesSetFromReplayingSegments, triesSetFromLoadingCheckpoint))
}

func triesUpToSegment(dir string, to int) ([]*trie.MTrie, error) {

	forest, err := mtrie.NewForest(500, &metrics.NoopCollector{}, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create Forest: %w", err)
	}

	err = replaySegments(
		dir,
		to,
		func(update *ledger.TrieUpdate) error {
			_, err := forest.Update(update)
			return err
		}, func(rootHash ledger.RootHash) error {
			return nil
		})
	if err != nil {
		return nil, err
	}

	return forest.GetTries()
}

func replaySegments(
	dir string,
	to int,
	updateFn func(update *ledger.TrieUpdate) error,
	deleteFn func(rootHash ledger.RootHash) error,
) error {
	sr, err := prometheusWAL.NewSegmentsRangeReader(prometheusWAL.SegmentRange{
		Dir:   dir,
		First: 0,
		Last:  to,
	})
	if err != nil {
		return fmt.Errorf("cannot create segment reader: %w", err)
	}

	reader := prometheusWAL.NewReader(sr)

	defer sr.Close()

	for reader.Next() {
		record := reader.Record()
		operation, rootHash, update, err := realWAL.Decode(record)
		if err != nil {
			return fmt.Errorf("cannot decode LedgerWAL record: %w", err)
		}

		switch operation {
		case realWAL.WALUpdate:
			err = updateFn(update)
			if err != nil {
				return fmt.Errorf("error while processing LedgerWAL update: %w", err)
			}
		case realWAL.WALDelete:
			err = deleteFn(rootHash)
			if err != nil {
				return fmt.Errorf("error while processing LedgerWAL deletion: %w", err)
			}
		}

		err = reader.Err()
		if err != nil {
			return fmt.Errorf("cannot read LedgerWAL: %w", err)
		}
	}

	return nil
}
