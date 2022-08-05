package complete

import (
	"fmt"
	"io/ioutil"
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
	"github.com/onflow/flow-go/ledger/common/testutils"
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

// TestCompactor tests creation of WAL segments and checkpoints, and
// checks if the rebuilt ledger state matches previous ledger state.
func TestCompactor(t *testing.T) {
	const (
		numInsPerStep      = 2
		pathByteSize       = 32
		minPayloadByteSize = 2 << 15
		maxPayloadByteSize = 2 << 16
		size               = 10
		checkpointDistance = 3
		checkpointsToKeep  = 1
		forestCapacity     = size * 10
	)

	metricsCollector := &metrics.NoopCollector{}

	unittest.RunWithTempDir(t, func(dir string) {

		var l *Ledger

		// saved data after updates
		savedData := make(map[ledger.RootHash]map[string]*ledger.Payload)

		t.Run("creates checkpoints", func(t *testing.T) {

			wal, err := realWAL.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dir, forestCapacity, pathByteSize, 32*1024)
			require.NoError(t, err)

			l, err = NewLedger(wal, size*10, metricsCollector, zerolog.Logger{}, DefaultPathFinderVersion)
			require.NoError(t, err)

			// WAL segments are 32kB, so here we generate 2 keys 64kB each, times `size`
			// so we should get at least `size` segments

			compactor, err := NewCompactor(l, wal, zerolog.Nop(), forestCapacity, checkpointDistance, checkpointsToKeep)
			require.NoError(t, err)

			co := CompactorObserver{fromBound: 8, done: make(chan struct{})}
			compactor.Subscribe(&co)

			// Run Compactor in background.
			<-compactor.Ready()

			rootState := l.InitialState()

			// Generate the tree and create WAL
			for i := 0; i < size; i++ {

				payloads := testutils.RandomPayloads(numInsPerStep, minPayloadByteSize, maxPayloadByteSize)

				keys := make([]ledger.Key, len(payloads))
				values := make([]ledger.Value, len(payloads))
				for i, p := range payloads {
					k, err := p.Key()
					require.NoError(t, err)
					keys[i] = k
					values[i] = p.Value()
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
				// Log segment and checkpoint files
				files, err := ioutil.ReadDir(dir)
				require.NoError(t, err)

				for _, file := range files {
					fmt.Printf("%s, size %d\n", file.Name(), file.Size())
				}

				assert.FailNow(t, "timed out")
			}

			checkpointer, err := wal.NewCheckpointer()
			require.NoError(t, err)

			from, to, err := checkpointer.NotCheckpointedSegments()
			require.NoError(t, err)

			assert.True(t, from == 9 && to == 10, "from: %v, to: %v", from, to) // Make sure there is no leftover

			require.NoFileExists(t, path.Join(dir, "checkpoint.00000000"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000001"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000002"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000003"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000004"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000005"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000006"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000007"))
			require.FileExists(t, path.Join(dir, "checkpoint.00000008"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000009"))

			<-l.Done()
			<-compactor.Done()
		})

		time.Sleep(2 * time.Second)

		t.Run("remove unnecessary files", func(t *testing.T) {
			// Remove all files apart from target checkpoint and WAL segments ahead of it
			// We know their names, so just hardcode them
			dirF, _ := os.Open(dir)
			files, _ := dirF.Readdir(0)

			for _, fileInfo := range files {

				name := fileInfo.Name()

				if name != "checkpoint.00000008" &&
					name != "00000009" &&
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

			l2, err = NewLedger(wal2, size*10, metricsCollector, zerolog.Logger{}, DefaultPathFinderVersion)
			require.NoError(t, err)

			<-wal2.Done()
		})

		t.Run("make sure forests are equal", func(t *testing.T) {

			// Check for same data
			for rootHash, data := range savedData {

				keys := make([]ledger.Key, 0, len(data))
				for _, p := range data {
					k, err := p.Key()
					require.NoError(t, err)
					keys = append(keys, k)
				}

				q, err := ledger.NewQuery(ledger.State(rootHash), keys)
				require.NoError(t, err)

				values, err := l.Get(q)
				require.NoError(t, err)

				values2, err := l2.Get(q)
				require.NoError(t, err)

				for i, k := range keys {
					ks := k.CanonicalForm()
					require.Equal(t, data[string(ks)].Value(), values[i])
					require.Equal(t, data[string(ks)].Value(), values2[i])
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

// TestCompactorSkipCheckpointing tests that only one
// checkpointing is running at a time.
func TestCompactorSkipCheckpointing(t *testing.T) {
	const (
		numInsPerStep      = 2
		pathByteSize       = 32
		minPayloadByteSize = 2 << 15
		maxPayloadByteSize = 2 << 16
		size               = 10
		checkpointDistance = 1 // checkpointDistance 1 triggers checkpointing for every segment.
		checkpointsToKeep  = 0
		forestCapacity     = size * 10
	)

	metricsCollector := &metrics.NoopCollector{}

	unittest.RunWithTempDir(t, func(dir string) {

		var l *Ledger

		// saved data after updates
		savedData := make(map[ledger.RootHash]map[string]*ledger.Payload)

		wal, err := realWAL.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dir, forestCapacity, pathByteSize, 32*1024)
		require.NoError(t, err)

		l, err = NewLedger(wal, size*10, metricsCollector, zerolog.Logger{}, DefaultPathFinderVersion)
		require.NoError(t, err)

		// WAL segments are 32kB, so here we generate 2 keys 64kB each, times `size`
		// so we should get at least `size` segments

		compactor, err := NewCompactor(l, wal, zerolog.Nop(), forestCapacity, checkpointDistance, checkpointsToKeep)
		require.NoError(t, err)

		co := CompactorObserver{fromBound: 8, done: make(chan struct{})}
		compactor.Subscribe(&co)

		// Run Compactor in background.
		<-compactor.Ready()

		rootState := l.InitialState()

		// Generate the tree and create WAL
		for i := 0; i < size; i++ {

			payloads := testutils.RandomPayloads(numInsPerStep, minPayloadByteSize, maxPayloadByteSize)

			keys := make([]ledger.Key, len(payloads))
			values := make([]ledger.Value, len(payloads))
			for i, p := range payloads {
				k, err := p.Key()
				require.NoError(t, err)
				keys[i] = k
				values[i] = p.Value()
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
			// Log segment and checkpoint files
			files, err := ioutil.ReadDir(dir)
			require.NoError(t, err)

			for _, file := range files {
				fmt.Printf("%s, size %d\n", file.Name(), file.Size())
			}

			// This assert can be flaky because of speed fluctuations (GitHub CI slowdowns, etc.).
			// Because this test only cares about number of created checkpoint files,
			// we don't need to fail the test here and keeping commented out for documentation.
			// assert.FailNow(t, "timed out")
		}

		<-l.Done()
		<-compactor.Done()

		first, last, err := wal.Segments()
		require.NoError(t, err)

		segmentCount := last - first + 1

		checkpointer, err := wal.NewCheckpointer()
		require.NoError(t, err)

		nums, err := checkpointer.Checkpoints()
		require.NoError(t, err)

		// Check that there are gaps between checkpoints (some checkpoints are skipped)
		firstNum, lastNum := nums[0], nums[len(nums)-1]
		require.True(t, (len(nums) < lastNum-firstNum+1) || (len(nums) < segmentCount))
	})
}

// TestCompactorAccuracy expects checkpointed tries to match replayed tries.
// Replayed tries are tries updated by replaying all WAL segments
// (from segment 0, ignoring prior checkpoints) to the checkpoint number.
// This verifies that checkpointed tries are snapshopt of segments and at segment boundary.
func TestCompactorAccuracy(t *testing.T) {

	const (
		numInsPerStep      = 2
		pathByteSize       = 32
		minPayloadByteSize = 2<<11 - 256 // 3840 bytes
		maxPayloadByteSize = 2 << 11     // 4096 bytes
		size               = 20
		checkpointDistance = 5
		checkpointsToKeep  = 0 // keep all
		forestCapacity     = 500
	)

	metricsCollector := &metrics.NoopCollector{}

	unittest.RunWithTempDir(t, func(dir string) {

		// There appears to be 1-2 records per segment (according to logs), so
		// generate size/2 segments.

		lastCheckpointNum := -1

		rootHash := trie.EmptyTrieRootHash()

		// Create DiskWAL and Ledger repeatedly to test rebuilding ledger state at restart.
		for i := 0; i < 3; i++ {

			wal, err := realWAL.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dir, forestCapacity, pathByteSize, 32*1024)
			require.NoError(t, err)

			l, err := NewLedger(wal, forestCapacity, metricsCollector, zerolog.Logger{}, DefaultPathFinderVersion)
			require.NoError(t, err)

			compactor, err := NewCompactor(l, wal, zerolog.Nop(), forestCapacity, checkpointDistance, checkpointsToKeep)
			require.NoError(t, err)

			fromBound := lastCheckpointNum + (size / 2)

			co := CompactorObserver{fromBound: fromBound, done: make(chan struct{})}
			compactor.Subscribe(&co)

			// Run Compactor in background.
			<-compactor.Ready()

			// Generate the tree and create WAL
			// size+2 is used to ensure that size/2 segments are finalized.
			for i := 0; i < size+2; i++ {

				payloads := testutils.RandomPayloads(numInsPerStep, minPayloadByteSize, maxPayloadByteSize)

				keys := make([]ledger.Key, len(payloads))
				values := make([]ledger.Value, len(payloads))
				for i, p := range payloads {
					k, err := p.Key()
					require.NoError(t, err)
					keys[i] = k
					values[i] = p.Value()
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
			<-l.Done()
			<-compactor.Done()

			checkpointer, err := wal.NewCheckpointer()
			require.NoError(t, err)

			nums, err := checkpointer.Checkpoints()
			require.NoError(t, err)

			for _, n := range nums {
				// TODO:  After the LRU Cache (outside of checkpointing code) is replaced
				// by a queue, etc. we should make sure the insertion order is preserved.

				checkSequence := false
				if i == 0 {
					// Sequence check only works when initial state is blank.
					// When initial state is from ledger's forest (LRU cache),
					// its sequence is altered by reads when replaying segment records.
					// Insertion order is not preserved (which is the way
					// it is currently on mainnet).  However, with PR 2792, only the
					// initial values are affected and those would likely
					// get into insertion order for the next checkpoint.  Once
					// the LRU Cache (outside of checkpointing code) is replaced,
					// then we can verify insertion order.
					checkSequence = true
				}
				testCheckpointedTriesMatchReplayedTriesFromSegments(t, checkpointer, n, dir, checkSequence)
			}

			lastCheckpointNum = nums[len(nums)-1]
		}
	})
}

// TestCompactorConcurrency expects checkpointed tries to
// match replayed tries in sequence with concurrent updates.
// Replayed tries are tries updated by replaying all WAL segments
// (from segment 0, ignoring prior checkpoints) to the checkpoint number.
// This verifies that checkpointed tries are snapshopt of segments
// and at segment boundary.
// Note: sequence check only works when initial state is blank.
// When initial state is from ledger's forest (LRU cache), its
// sequence is altered by reads when replaying segment records.
func TestCompactorConcurrency(t *testing.T) {
	const (
		numInsPerStep      = 2
		pathByteSize       = 32
		minPayloadByteSize = 2<<11 - 256 // 3840 bytes
		maxPayloadByteSize = 2 << 11     // 4096 bytes
		size               = 20
		checkpointDistance = 5
		checkpointsToKeep  = 0 // keep all
		forestCapacity     = 500
		numGoroutine       = 4
		lastCheckpointNum  = -1
	)

	rootState := ledger.State(trie.EmptyTrieRootHash())

	metricsCollector := &metrics.NoopCollector{}

	unittest.RunWithTempDir(t, func(dir string) {

		// There are 1-2 records per segment (according to logs), so
		// generate size/2 segments.

		wal, err := realWAL.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dir, forestCapacity, pathByteSize, 32*1024)
		require.NoError(t, err)

		l, err := NewLedger(wal, forestCapacity, metricsCollector, zerolog.Logger{}, DefaultPathFinderVersion)
		require.NoError(t, err)

		compactor, err := NewCompactor(l, wal, zerolog.Nop(), forestCapacity, checkpointDistance, checkpointsToKeep)
		require.NoError(t, err)

		fromBound := lastCheckpointNum + (size / 2 * numGoroutine)

		co := CompactorObserver{fromBound: fromBound, done: make(chan struct{})}
		compactor.Subscribe(&co)

		// Run Compactor in background.
		<-compactor.Ready()

		// Run 4 goroutines and each goroutine updates size+1 tries.
		for j := 0; j < numGoroutine; j++ {
			go func(parentState ledger.State) {
				// size+1 is used to ensure that size/2*numGoroutine segments are finalized.
				for i := 0; i < size+1; i++ {
					payloads := testutils.RandomPayloads(numInsPerStep, minPayloadByteSize, maxPayloadByteSize)

					keys := make([]ledger.Key, len(payloads))
					values := make([]ledger.Value, len(payloads))
					for i, p := range payloads {
						k, err := p.Key()
						require.NoError(t, err)
						keys[i] = k
						values[i] = p.Value()
					}

					update, err := ledger.NewUpdate(parentState, keys, values)
					require.NoError(t, err)

					newState, _, err := l.Set(update)
					require.NoError(t, err)

					parentState = newState
				}
			}(rootState)
		}

		// wait for the bound-checking observer to confirm checkpoints have been made
		select {
		case <-co.done:
			// continue
		case <-time.After(120 * time.Second):
			assert.FailNow(t, "timed out")
		}

		// Shutdown ledger and compactor
		<-l.Done()
		<-compactor.Done()

		checkpointer, err := wal.NewCheckpointer()
		require.NoError(t, err)

		nums, err := checkpointer.Checkpoints()
		require.NoError(t, err)

		for _, n := range nums {
			// For each created checkpoint:
			// - get tries by loading checkpoint
			// - get tries by replaying segments up to checkpoint number (ignoring all prior checkpoints)
			// - test that these 2 sets of tries match in content and sequence (insertion order).
			testCheckpointedTriesMatchReplayedTriesFromSegments(t, checkpointer, n, dir, true)
		}
	})
}

func testCheckpointedTriesMatchReplayedTriesFromSegments(
	t *testing.T,
	checkpointer *realWAL.Checkpointer,
	checkpointNum int,
	dir string,
	inSequence bool,
) {
	// Get tries by loading checkpoint
	triesFromLoadingCheckpoint, err := checkpointer.LoadCheckpoint(checkpointNum)
	require.NoError(t, err)

	// Get tries by replaying segments up to checkpoint number (ignoring checkpoints)
	triesFromReplayingSegments, err := triesUpToSegment(dir, checkpointNum, len(triesFromLoadingCheckpoint))
	require.NoError(t, err)

	if inSequence {
		// Test that checkpointed tries match replayed tries in content and sequence (insertion order).
		require.Equal(t, len(triesFromReplayingSegments), len(triesFromLoadingCheckpoint))
		for i := 0; i < len(triesFromReplayingSegments); i++ {
			require.Equal(t, triesFromReplayingSegments[i].RootHash(), triesFromLoadingCheckpoint[i].RootHash())
		}
		return
	}

	// Test that checkpointed tries match replayed tries in content (ignore order).
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

func triesUpToSegment(dir string, to int, capacity int) ([]*trie.MTrie, error) {

	// forest is used to create new trie.
	forest, err := mtrie.NewForest(capacity, &metrics.NoopCollector{}, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create Forest: %w", err)
	}

	initialTries, err := forest.GetTries()
	if err != nil {
		return nil, fmt.Errorf("cannot get tries from forest: %w", err)
	}

	// TrieQueue is used to store last n tries from segment files in order (n = capacity)
	tries := realWAL.NewTrieQueueWithValues(uint(capacity), initialTries)

	err = replaySegments(
		dir,
		to,
		func(update *ledger.TrieUpdate) error {
			t, err := forest.NewTrie(update)
			if err == nil {
				err = forest.AddTrie(t)
				if err != nil {
					return err
				}
				tries.Push(t)
			}
			return err
		}, func(rootHash ledger.RootHash) error {
			return nil
		})
	if err != nil {
		return nil, err
	}

	return tries.Tries(), nil
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
