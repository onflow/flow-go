package complete_test

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
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
