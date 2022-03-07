package complete_test

import (
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/module/metrics"
)

var dir = flag.String("dir", ".", "dir containing checkpoint and wal files")

// BenchmarkNewCheckpoint benchmarks checkpoint file creation from existing checkpoint and wal segments.
// This requires a checkpoint file and one or more segments following the checkpoint file.
// This benchmark will create a checkpoint file.
func BenchmarkNewCheckpoint(b *testing.B) {
	// Check if there is any segment in specified dir
	foundSeg, err := hasSegmentInDir(*dir)
	if err != nil {
		b.Fatal(err)
	}
	if !foundSeg {
		b.Fatalf("failed to find segment in %s.  Use -dir to specify dir containing segments and checkpoint files.", *dir)
	}

	// Check if there is any checkpoint file in specified dir
	foundCheckpoint, err := hasCheckpointInDir(*dir)
	if err != nil {
		b.Fatal(err)
	}
	if !foundCheckpoint {
		b.Fatalf("failed to find checkpoint in %s.  Use -dir to specify dir containing segments and checkpoint files.", *dir)
	}

	diskwal, err := wal.NewDiskWAL(
		zerolog.Nop(),
		nil,
		metrics.NewNoopCollector(),
		*dir,
		500,
		pathfinder.PathByteSize,
		wal.SegmentSize,
	)
	if err != nil {
		b.Fatal(err)
	}

	_, to, err := diskwal.Segments()
	if err != nil {
		b.Fatal(err)
	}

	checkpointer, err := diskwal.NewCheckpointer()
	if err != nil {
		b.Fatal(err)
	}

	start := time.Now()
	b.ResetTimer()

	err = checkpointer.Checkpoint(to-1, func() (io.WriteCloser, error) {
		return checkpointer.CheckpointWriter(to - 1)
	})

	b.StopTimer()
	elapsed := time.Since(start)

	if err != nil {
		b.Fatal(err)
	}

	b.ReportMetric(float64(elapsed/time.Millisecond), "newcheckpoint_time_(ms)")
	b.ReportAllocs()
}

// BenchmarkLoadCheckpointAndWALs benchmarks checkpoint file loading and wal segments replaying.
// This requires a checkpoint file and one or more segments following the checkpoint file.
// This mimics rebuliding mtrie at EN startup.
func BenchmarkLoadCheckpointAndWALs(b *testing.B) {
	// Check if there is any segment in specified dir
	foundSeg, err := hasSegmentInDir(*dir)
	if err != nil {
		b.Fatal(err)
	}
	if !foundSeg {
		b.Fatalf("failed to find segment in %s.  Use -dir to specify dir containing segments and checkpoint files.", *dir)
	}

	// Check if there is any checkpoint file in specified dir
	foundCheckpoint, err := hasCheckpointInDir(*dir)
	if err != nil {
		b.Fatal(err)
	}
	if !foundCheckpoint {
		b.Fatalf("failed to find checkpoint in %s.  Use -dir to specify dir containing segments and checkpoint files.", *dir)
	}

	forest, err := mtrie.NewForest(500, metrics.NewNoopCollector(), nil)
	if err != nil {
		b.Fatal(err)
	}

	diskwal, err := wal.NewDiskWAL(
		zerolog.Nop(),
		nil,
		metrics.NewNoopCollector(),
		*dir,
		500,
		pathfinder.PathByteSize,
		wal.SegmentSize,
	)
	if err != nil {
		b.Fatal(err)
	}

	// pause records to prevent double logging trie removals
	diskwal.PauseRecord()
	defer diskwal.UnpauseRecord()

	start := time.Now()
	b.ResetTimer()

	err = diskwal.Replay(
		func(tries []*trie.MTrie) error {
			err := forest.AddTries(tries)
			if err != nil {
				return fmt.Errorf("adding rebuilt tries to forest failed: %w", err)
			}
			return nil
		},
		func(update *ledger.TrieUpdate) error {
			_, err := forest.Update(update)
			return err
		},
		func(rootHash ledger.RootHash) error {
			forest.RemoveTrie(rootHash)
			return nil
		},
	)
	if err != nil {
		b.Fatal(err)
	}

	b.StopTimer()
	elapsed := time.Since(start)

	b.ReportMetric(float64(elapsed/time.Millisecond), "loadcheckpointandwals_time_(ms)")
	b.ReportAllocs()
}

func hasSegmentInDir(dir string) (bool, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}

	for _, fn := range files {
		fname := fn.Name()
		_, err := strconv.Atoi(fname)
		if err != nil {
			continue
		}
		return true, nil
	}
	return false, nil
}

func hasCheckpointInDir(dir string) (bool, error) {
	const checkpointFilenamePrefix = "checkpoint."

	files, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}

	for _, fn := range files {
		fname := fn.Name()
		if !strings.HasPrefix(fname, checkpointFilenamePrefix) {
			continue
		}
		justNumber := fname[len(checkpointFilenamePrefix):]
		_, err := strconv.Atoi(justNumber)
		if err != nil {
			continue
		}
		return true, nil
	}

	return false, nil
}

func BenchmarkNewCheckpointRandom5Seg(b *testing.B) { benchmarkNewCheckpointRandomData(b, 5) }

func BenchmarkNewCheckpointRandom10Seg(b *testing.B) { benchmarkNewCheckpointRandomData(b, 10) }

func BenchmarkNewCheckpointRandom20Seg(b *testing.B) { benchmarkNewCheckpointRandomData(b, 20) }

func BenchmarkNewCheckpointRandom30Seg(b *testing.B) { benchmarkNewCheckpointRandomData(b, 30) }

func BenchmarkNewCheckpointRandom40Seg(b *testing.B) { benchmarkNewCheckpointRandomData(b, 40) }

// benchmarkCheckpointCreate benchmarks checkpoint file creation.
// This benchmark creates segmentCount+1 WAL segments.  It also creates two checkpoint files:
// - checkpoint file A from segment 0, and
// - checkpoint file B from checkpoint file A and all segments after segment 0.
// This benchmark measures the creation of checkpoint file B:
// - loading checkpoint file A
// - replaying all segments after segment 0
// - creating checkpoint file B
// Because payload data is random, number of segments created can differ from segmentCount.
func benchmarkNewCheckpointRandomData(b *testing.B, segmentCount int) {

	const (
		updatePerSegment = 75  // 75 updates for 1 segment by approximation.
		kvBatchCount     = 500 // Each update has 500 new payloads.
	)

	if segmentCount < 1 {
		segmentCount = 1
	}

	kvOpts := randKeyValueOptions{
		keyNumberOfParts:   3,
		keyPartMinByteSize: 1,
		keyPartMaxByteSize: 50,
		valueMinByteSize:   50,
		valueMaxByteSize:   1024 * 1.5,
	}
	updateCount := (segmentCount + 1) * updatePerSegment

	seed := uint64(0x9E3779B97F4A7C15) // golden ratio
	rand.Seed(int64(seed))

	dir, err := os.MkdirTemp("", "test-mtrie-")
	defer os.RemoveAll(dir)
	if err != nil {
		b.Fatal(err)
	}

	wal1, err := wal.NewDiskWAL(
		zerolog.Nop(),
		nil,
		metrics.NewNoopCollector(),
		dir,
		500,
		pathfinder.PathByteSize,
		wal.SegmentSize)
	if err != nil {
		b.Fatal(err)
	}

	led, err := complete.NewLedger(
		wal1,
		500,
		&metrics.NoopCollector{},
		zerolog.Logger{},
		complete.DefaultPathFinderVersion,
	)
	if err != nil {
		b.Fatal(err)
	}

	state := led.InitialState()

	_, err = updateLedgerWithRandomData(led, state, updateCount, kvBatchCount, kvOpts)
	if err != nil {
		b.Fatal(err)
	}

	<-wal1.Done()
	<-led.Done()

	wal2, err := wal.NewDiskWAL(
		zerolog.Nop(),
		nil,
		metrics.NewNoopCollector(),
		dir,
		500,
		pathfinder.PathByteSize,
		wal.SegmentSize,
	)
	if err != nil {
		b.Fatal(err)
	}

	checkpointer, err := wal2.NewCheckpointer()
	if err != nil {
		b.Fatal(err)
	}

	// Create checkpoint with only one segment as the base checkpoint for the next step.
	err = checkpointer.Checkpoint(0, func() (io.WriteCloser, error) {
		return checkpointer.CheckpointWriter(0)
	})
	require.NoError(b, err)

	// Create checkpoint with remaining segments
	_, to, err := wal2.Segments()
	require.NoError(b, err)

	if to == 1 {
		fmt.Printf("skip creating second checkpoint file because to segment is 1\n")
		return
	}

	start := time.Now()
	b.ResetTimer()

	err = checkpointer.Checkpoint(to-1, func() (io.WriteCloser, error) {
		return checkpointer.CheckpointWriter(to)
	})

	b.StopTimer()
	elapsed := time.Since(start)

	if err != nil {
		b.Fatal(err)
	}

	b.ReportMetric(float64(elapsed/time.Millisecond), "newcheckpoint_rand_time_(ms)")
	b.ReportAllocs()
}

type randKeyValueOptions struct {
	keyNumberOfParts   int
	keyPartMinByteSize int
	keyPartMaxByteSize int
	valueMinByteSize   int
	valueMaxByteSize   int
}

func updateLedgerWithRandomData(
	led ledger.Ledger,
	state ledger.State,
	updateCount int,
	kvBatchCount int,
	kvOpts randKeyValueOptions,
) (ledger.State, error) {

	for i := 0; i < updateCount; i++ {
		keys := utils.RandomUniqueKeys(kvBatchCount, kvOpts.keyNumberOfParts, kvOpts.keyPartMinByteSize, kvOpts.keyPartMaxByteSize)
		values := utils.RandomValues(kvBatchCount, kvOpts.valueMinByteSize, kvOpts.valueMaxByteSize)

		update, err := ledger.NewUpdate(state, keys, values)
		if err != nil {
			return ledger.State(hash.DummyHash), err
		}

		newState, _, err := led.Set(update)
		if err != nil {
			return ledger.State(hash.DummyHash), err
		}

		state = newState
	}

	return state, nil
}
