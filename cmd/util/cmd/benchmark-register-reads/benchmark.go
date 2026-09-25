package benchmark_register_reads

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/storage"
	pebblestorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/storage/pebble/registers"
)

// registerReader reads registers from one state: either the merkle trie of a checkpoint, or a
// storehouse register store.
type registerReader struct {
	name string
	read func(key registerKey) (value []byte, found bool, err error)
}

// triePathReader reads registers from the given trie by their trie path.
func triePathReader(checkpointTrie *trie.MTrie) registerReader {
	return registerReader{
		name: "mtrie (read by path)",
		read: func(key registerKey) ([]byte, bool, error) {
			return trieValue(checkpointTrie.ReadSinglePayload(key.path))
		},
	}
}

// trieValue returns the value of the given payload of an mtrie read. An empty payload means the
// register does not exist in the trie ([mtrie.MTrie.ReadSinglePayload] returns
// [ledger.EmptyPayload] for registers it does not hold), which is also how the register store
// represents a register that does not exist, so both states treat an empty value as absent.
func trieValue(payload *ledger.Payload) ([]byte, bool, error) {
	if payload == nil {
		return nil, false, nil
	}

	value := payload.Value()
	if len(value) == 0 {
		return nil, false, nil
	}
	return value, true, nil
}

// trieHashReader reads registers from the given trie the way an execution node reads them: it
// derives the trie path from the register ID first, and then reads the trie.
func trieHashReader(checkpointTrie *trie.MTrie, pathFinderVersion uint8) registerReader {
	return registerReader{
		name: "mtrie (hash + read by path)",
		read: func(key registerKey) ([]byte, bool, error) {
			path, err := pathfinder.KeyToPath(convert.RegisterIDToLedgerKey(key.id), pathFinderVersion)
			if err != nil {
				return nil, false, fmt.Errorf("could not compute path of register %s: %w", key.id, err)
			}

			return trieValue(checkpointTrie.ReadSinglePayload(path))
		},
	}
}

// storeReader reads registers from the given register store at the given height.
func storeReader(registerStore *pebblestorage.Registers, height uint64) registerReader {
	return registerReader{
		name: "storehouse",
		read: func(key registerKey) ([]byte, bool, error) {
			value, err := registerStore.Get(key.id, height)
			if err != nil {
				if errors.Is(err, storage.ErrNotFound) {
					return nil, false, nil
				}
				return nil, false, err
			}
			if len(value) == 0 {
				// the register was removed, so it does not exist in the state
				return nil, false, nil
			}
			return value, true, nil
		},
	}
}

// openRegisterStore opens the register store in the given directory with the given block cache
// size. The store uses the same pebble options and comparer as the execution node, except for the
// cache size, which is a benchmark input (the execution node opens it with
// [pebblestorage.DefaultPebbleCacheSize]).
//
// No error returns are expected during normal operation.
func openRegisterStore(log zerolog.Logger, dir string, cacheSize int64) (*pebble.DB, *pebblestorage.Registers, error) {
	cache := pebble.NewCache(cacheSize)
	defer cache.Unref()

	opts := pebblestorage.DefaultPebbleOptions(log, cache, registers.NewMVCCComparer())
	db, err := pebble.Open(dir, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("could not open register store %s: %w", dir, err)
	}

	registerStore, err := pebblestorage.NewRegisters(db, pebblestorage.PruningDisabled)
	if err != nil {
		if closeErr := db.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("could not close register store")
		}
		return nil, nil, fmt.Errorf("could not open registers of %s: %w", dir, err)
	}

	return db, registerStore, nil
}

// measurement is the result of reading a set of registers with one reader.
type measurement struct {
	name        string
	reads       int
	found       int
	bytesRead   int64
	duration    time.Duration
	readsPerSec float64
	bytesPerSec float64
	p50         time.Duration
	p95         time.Duration
	p99         time.Duration
	max         time.Duration
}

// readResult is the result of one worker reading its share of the registers.
type readResult struct {
	found     int
	bytesRead int64
	durations []time.Duration
}

// measureReads reads the given registers with the given reader and worker count: first all
// registers without instrumentation to measure the throughput, and then a subset of them while
// recording the per-read latency. The latencies are recorded across all workers, so they are the
// latencies under the given concurrency.
//
// No error returns are expected during normal operation.
func measureReads(reader registerReader, keys []registerKey, workerCount int, latencyReads int) (measurement, error) {
	result := measurement{name: reader.name, reads: len(keys)}

	throughputResults, duration, err := readKeys(keys, workerCount, 0, reader.read)
	if err != nil {
		return measurement{}, fmt.Errorf("could not read registers with reader %s: %w", reader.name, err)
	}
	result.duration = duration
	result.readsPerSec = float64(len(keys)) / duration.Seconds()
	for _, workerResult := range throughputResults {
		result.found += workerResult.found
		result.bytesRead += workerResult.bytesRead
	}
	result.bytesPerSec = float64(result.bytesRead) / duration.Seconds()

	if latencyReads > 0 && latencyReads <= len(keys) {
		latencyKeys := sampleEvery(keys, latencyReads)
		latencyResults, _, err := readKeys(latencyKeys, workerCount, latencyReads, reader.read)
		if err != nil {
			return measurement{}, fmt.Errorf("could not read registers with reader %s: %w", reader.name, err)
		}

		durations := make([]time.Duration, 0, latencyReads)
		for _, workerResult := range latencyResults {
			durations = append(durations, workerResult.durations...)
		}
		sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
		result.p50 = percentile(durations, 0.50)
		result.p95 = percentile(durations, 0.95)
		result.p99 = percentile(durations, 0.99)
		result.max = durations[len(durations)-1]
	}

	return result, nil
}

// readKeys reads the given registers with workerCount workers, each reading one share of the
// registers, and returns the results of every worker. If latencyLimit is greater than zero, the
// per-read latencies are recorded (up to latencyLimit reads per worker).
//
// No error returns are expected during normal operation.
func readKeys(
	keys []registerKey,
	workerCount int,
	latencyLimit int,
	read func(registerKey) ([]byte, bool, error),
) ([]readResult, time.Duration, error) {
	if workerCount < 1 {
		return nil, 0, fmt.Errorf("worker count must be at least 1, got %d", workerCount)
	}

	shares := splitKeys(keys, workerCount)
	results := make([]readResult, workerCount)
	errs := make([]error, workerCount)

	start := time.Now()
	var workers sync.WaitGroup
	for worker, share := range shares {
		workers.Add(1)
		go func() {
			defer workers.Done()

			result := &results[worker]
			recordLatencies := latencyLimit > 0
			if recordLatencies {
				result.durations = make([]time.Duration, 0, min(len(share), latencyLimit))
			}

			for i, key := range share {
				var readStart time.Time
				if recordLatencies && i < latencyLimit {
					readStart = time.Now()
				}

				value, found, err := read(key)

				if recordLatencies && i < latencyLimit {
					result.durations = append(result.durations, time.Since(readStart))
				}
				if err != nil {
					errs[worker] = err
					return
				}
				if found {
					result.found++
					result.bytesRead += int64(len(value))
				}
			}
		}()
	}
	workers.Wait()

	duration := time.Since(start)
	for _, err := range errs {
		if err != nil {
			return nil, 0, err
		}
	}

	return results, duration, nil
}

// splitKeys splits the given registers into up to workerCount shares of similar size.
func splitKeys(keys []registerKey, workerCount int) [][]registerKey {
	shares := make([][]registerKey, 0, workerCount)
	shareSize := (len(keys) + workerCount - 1) / workerCount
	for start := 0; start < len(keys); start += shareSize {
		shares = append(shares, keys[start:min(start+shareSize, len(keys))])
	}
	return shares
}

// sampleEvery returns up to count registers of the given registers, evenly spread over them.
func sampleEvery(keys []registerKey, count int) []registerKey {
	if count >= len(keys) {
		return keys
	}

	sampled := make([]registerKey, 0, count)
	for i := range count {
		sampled = append(sampled, keys[i*len(keys)/count])
	}
	return sampled
}

// percentile returns the value at the given percentile of the sorted durations.
func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	return sorted[int(float64(len(sorted)-1)*p)]
}

// compareReaders reads the given registers with all readers and checks that they return the same
// values: the reader counts, the number of registers found, the number of registers whose value
// differs, and a description of the first difference.
func compareReaders(keys []registerKey, readers []registerReader) (int, string, error) {
	if len(readers) < 2 {
		return 0, "", nil
	}

	mismatches := 0
	firstMismatch := ""
	for _, key := range keys {
		reference, referenceFound, err := readers[0].read(key)
		if err != nil {
			return 0, "", fmt.Errorf("reader %s could not read register %s: %w", readers[0].name, key.id, err)
		}

		for _, reader := range readers[1:] {
			value, found, err := reader.read(key)
			if err != nil {
				return 0, "", fmt.Errorf("reader %s could not read register %s: %w", reader.name, key.id, err)
			}
			if found != referenceFound || !equalValues(value, reference) {
				mismatches++
				if firstMismatch == "" {
					firstMismatch = fmt.Sprintf("register %s: %s found=%t (length %d), %s found=%t (length %d)",
						key.id, readers[0].name, referenceFound, len(reference), reader.name, found, len(value))
				}
			}
		}
	}
	return mismatches, firstMismatch, nil
}

// equalValues returns true if both register values are equal.
func equalValues(a []byte, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// printMeasurements prints the measurements of the readers as a table.
func printMeasurements(measurements []measurement) {
	fmt.Printf("\n%-28s %10s %8s %12s %9s %9s %9s %9s %9s\n",
		"reader", "reads", "found", "reads/s", "MB/s", "p50", "p95", "p99", "max")
	for _, m := range measurements {
		fmt.Printf("%-28s %10d %8d %12.0f %9.1f %9s %9s %9s %9s\n",
			m.name, m.reads, m.found, m.readsPerSec, m.bytesPerSec/1e6,
			m.p50.Round(time.Nanosecond), m.p95.Round(time.Nanosecond),
			m.p99.Round(time.Nanosecond), m.max.Round(time.Nanosecond))
	}
	fmt.Println()
}
