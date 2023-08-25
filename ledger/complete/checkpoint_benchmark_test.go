package complete_test

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger/complete/wal"
)

// To benchmark with local data, using this command:
// $ go test -c -o benchmark
// $ GOARCH=amd64 GOOS=linux ./benchmark -test.bench=BenchmarkStoreCheckpointV6Concurrently -test.benchmem --checkpointFile ./root.checkpoint
var checkpointFile = flag.String("checkpointFile", "", "input checkpoint filename")

func BenchmarkStoreCheckpointV5(b *testing.B) {
	benchmarkStoreCheckpoint(b, 5, false)
}

func BenchmarkStoreCheckpointV6(b *testing.B) {
	benchmarkStoreCheckpoint(b, 6, false)
}

func BenchmarkStoreCheckpointV6Concurrently(b *testing.B) {
	benchmarkStoreCheckpoint(b, 6, true)
}

func benchmarkStoreCheckpoint(b *testing.B, version int, concurrent bool) {
	if version != 5 && version != 6 {
		b.Fatalf("checkpoint file version must be 5 or 6, version %d isn't supported", version)
	}

	log := zerolog.Nop()

	// Check if checkpoint file exists
	_, err := os.Stat(*checkpointFile)
	if errors.Is(err, os.ErrNotExist) {
		b.Fatalf("input checkpoint file %s doesn't exist", *checkpointFile)
	}

	dir, fileName := filepath.Split(*checkpointFile)
	subdir := strconv.FormatInt(time.Now().UnixNano(), 10)
	outputDir := filepath.Join(dir, subdir)
	err = os.Mkdir(outputDir, 0755)
	if err != nil {
		b.Fatalf("cannot create output dir %s: %s", outputDir, err)
	}
	defer func() {
		// Remove output directory and its contents.
		os.RemoveAll(outputDir)
	}()

	// Load checkpoint
	tries, err := wal.LoadCheckpoint(*checkpointFile, log)
	if err != nil {
		b.Fatalf("cannot load checkpoint: %s", err)
	}

	start := time.Now()
	b.ResetTimer()

	// Serialize checkpoint V5.
	switch version {
	case 5:
		err = wal.StoreCheckpointV5(outputDir, fileName, log, tries...)
	case 6:
		if concurrent {
			err = wal.StoreCheckpointV6Concurrently(tries, outputDir, fileName, log)
		} else {
			err = wal.StoreCheckpointV6SingleThread(tries, outputDir, fileName, log)
		}
	}

	b.StopTimer()
	elapsed := time.Since(start)

	if err != nil {
		b.Fatalf("cannot store checkpoint: %s", err)
	}

	b.ReportMetric(float64(elapsed/time.Millisecond), fmt.Sprintf("storecheckpoint_v%d_time_(ms)", version))
	b.ReportAllocs()
}

func BenchmarkLoadCheckpoint(b *testing.B) {
	// Check if input checkpoint file exists
	_, err := os.Stat(*checkpointFile)
	if errors.Is(err, os.ErrNotExist) {
		b.Fatalf("input checkpoint file %s doesn't exist", *checkpointFile)
	}

	log := zerolog.Nop()

	start := time.Now()
	b.ResetTimer()

	// Load checkpoint
	_, err = wal.LoadCheckpoint(*checkpointFile, log)

	b.StopTimer()
	elapsed := time.Since(start)

	if err != nil {
		b.Fatalf("cannot load checkpoint : %s", err)
	}

	b.ReportMetric(float64(elapsed/time.Millisecond), "loadcheckpoint_time_(ms)")
	b.ReportAllocs()
}
