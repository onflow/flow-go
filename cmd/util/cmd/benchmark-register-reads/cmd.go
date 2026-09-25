// Package benchmark_register_reads provides the benchmark-register-reads utility command.
//
// It benchmarks reading registers from the two states an execution node can execute against: the
// merkle trie (fpm the checkpoint of the state, held in memory) and the pebble register store
// (the storehouse's on-disk register store).
package benchmark_register_reads

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/ledger"
	completeLedger "github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/wal"
	pebblestorage "github.com/onflow/flow-go/storage/pebble"
)

// patterns of the registers to read.
const (
	// patternRandom reads registers drawn uniformly from a sample of the checkpoint's registers.
	patternRandom = "random"

	// patternHotSet reads registers drawn uniformly from a small working set of the checkpoint's
	// registers, the way execution reads hot accounts over and over.
	patternHotSet = "hot-set"

	// patternReplay reads the registers blocks touched, taken from the execution data of the
	// given execution data datastore.
	patternReplay = "replay"
)

var (
	flagCheckpointFile   string
	flagRegisterDir      string
	flagExecutionDataDir string
	flagPattern          string
	flagHeight           uint64
	flagKeys             int
	flagHotKeys          int
	flagReads            int
	flagMissPercent      int
	flagBlocks           int
	flagWorkers          int
	flagCacheSize        int64
	flagLatencyReads     int
	flagCompareKeys      int
	flagSeed             int64
)

var Cmd = &cobra.Command{
	Use:   "benchmark-register-reads",
	Short: "Benchmark reading registers from an in-memory merkle trie and from a storehouse register store",
	Long: `benchmark-register-reads benchmarks reading registers from the two states an execution node
can execute against, and reports the throughput and the read latencies of both:

  - the merkle trie of the state, loaded from a checkpoint into memory (the command needs enough
    memory for the trie of the checkpoint),
  - the pebble register store of the storehouse, opened with --cache-size.

The registers to read are drawn with --pattern:

  - "hot-set": a small working set of the checkpoint's registers (--hot-keys out of --keys
    registers sampled from the checkpoint), which is how execution reads hot accounts block after
    block,
  - "random": registers drawn uniformly from all sampled registers, which is the worst case for
    the register store (no locality, no reuse) and the best case for the merkle trie,
  - "replay": the registers that blocks touched, read from blocks of the execution data datastore
    given with --execution-data-dir, which is the closest to the reads of real execution (minus
    the per-snapshot read cache that both states have).

A share of the reads can be on registers that do not exist in the state (--miss-percent).

The command also reads a sample of the registers from both states (--compare-keys) and fails if
they do not return the same values, which checks the two states hold the same registers.`,
	RunE: runE,
}

// init registers the benchmark-register-reads command flags.
func init() {
	Cmd.Flags().StringVar(&flagCheckpointFile, "checkpoint-file", "",
		"compacted single-trie checkpoint file of the state to read from")
	_ = Cmd.MarkFlagRequired("checkpoint-file")

	Cmd.Flags().StringVar(&flagRegisterDir, "register-dir", "",
		"directory of the storehouse register store (Pebble) to read from")
	_ = Cmd.MarkFlagRequired("register-dir")

	Cmd.Flags().StringVar(&flagExecutionDataDir, "execution-data-dir", "",
		"directory of the execution data datastore, required by the 'replay' pattern")

	Cmd.Flags().StringVar(&flagPattern, "pattern", patternHotSet,
		fmt.Sprintf("registers to read: %q, %q or %q", patternHotSet, patternRandom, patternReplay))

	Cmd.Flags().Uint64Var(&flagHeight, "height", 0,
		"height to read the register store at, 0 for the store's latest height")

	Cmd.Flags().IntVar(&flagKeys, "keys", 100_000,
		"number of distinct registers to sample from the checkpoint ('hot-set' and 'random' patterns)")

	Cmd.Flags().IntVar(&flagHotKeys, "hot-keys", 10_000,
		"number of registers of the working set the 'hot-set' pattern reads from")

	Cmd.Flags().IntVar(&flagReads, "reads", 1_000_000,
		"number of reads to perform ('hot-set' and 'random' patterns)")

	Cmd.Flags().IntVar(&flagMissPercent, "miss-percent", 1,
		"roughly the percentage of reads on registers that do not exist in the state")

	Cmd.Flags().IntVar(&flagBlocks, "blocks", 100,
		"number of blocks of execution data to replay ('replay' pattern)")

	Cmd.Flags().IntVar(&flagWorkers, "workers", min(runtime.NumCPU(), 16),
		"number of readers reading concurrently")

	Cmd.Flags().Int64Var(&flagCacheSize, "cache-size", pebblestorage.DefaultPebbleCacheSize,
		"size of the register store's block cache in bytes; the execution node uses "+
			"DefaultPebbleCacheSize (1 MiB), raise it to see the effect of a cache that holds the "+
			"working set")

	Cmd.Flags().IntVar(&flagLatencyReads, "latency-reads", 100_000,
		"number of reads to measure the per-read latency of; the latencies are measured under "+
			"--workers concurrency and instrumented with two clock reads per read, so the "+
			"instrumented part is a subset of the reads")

	Cmd.Flags().IntVar(&flagCompareKeys, "compare-keys", 10_000,
		"number of registers to read from both states to check they return the same values; 0 disables the check")

	Cmd.Flags().Int64Var(&flagSeed, "seed", 1, "seed of the sampling of the registers to read")
}

// runE implements the benchmark-register-reads command.
func runE(*cobra.Command, []string) error {
	switch flagPattern {
	case patternRandom, patternHotSet, patternReplay:
	default:
		return fmt.Errorf("invalid pattern %q, supported patterns are %q, %q and %q",
			flagPattern, patternHotSet, patternRandom, patternReplay)
	}
	if flagPattern == patternReplay && flagExecutionDataDir == "" {
		return fmt.Errorf("the %q pattern requires --execution-data-dir", patternReplay)
	}
	if flagKeys < 1 || flagReads < 1 || flagWorkers < 1 || flagBlocks < 1 {
		return fmt.Errorf("--keys, --reads, --blocks and --workers must be at least 1, got %d, %d, %d and %d",
			flagKeys, flagReads, flagBlocks, flagWorkers)
	}
	if flagHotKeys < 1 || flagHotKeys > flagKeys {
		return fmt.Errorf("--hot-keys must be between 1 and --keys (%d), got %d", flagKeys, flagHotKeys)
	}
	if flagCacheSize < 1 {
		return fmt.Errorf("--cache-size must be at least 1 byte, got %d", flagCacheSize)
	}

	ctx := context.Background()
	pathFinderVersion := uint8(completeLedger.DefaultPathFinderVersion)

	// ── load the state from the checkpoint ────────────────────────────────────
	log.Info().Msgf("loading the trie of checkpoint %s (this needs memory for the whole trie)", flagCheckpointFile)
	loadStart := time.Now()
	tries, err := wal.LoadCheckpoint(flagCheckpointFile, log.Logger)
	if err != nil {
		return fmt.Errorf("could not load checkpoint %s: %w", flagCheckpointFile, err)
	}
	if len(tries) != 1 {
		return fmt.Errorf("checkpoint %s holds %d tries, expected a compacted single-trie checkpoint",
			flagCheckpointFile, len(tries))
	}
	checkpointTrie := tries[0]
	rootHash := checkpointTrie.RootHash()
	log.Info().
		Uint64("register_count", checkpointTrie.AllocatedRegCount()).
		Hex("root_hash", rootHash[:]).
		Str("duration", fmt.Sprintf("%v", time.Since(loadStart))).
		Msg("trie loaded")

	// ── open the register store ───────────────────────────────────────────────
	db, registerStore, err := openRegisterStore(log.Logger, flagRegisterDir, flagCacheSize)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("could not close register store")
		}
	}()

	height := flagHeight
	if height == 0 {
		height = registerStore.LatestHeight()
	}
	log.Info().
		Str("register_dir", flagRegisterDir).
		Uint64("height", height).
		Int64("cache_size", flagCacheSize).
		Msg("register store opened")

	// ── collect the registers to read ─────────────────────────────────────────
	keys, err := collectKeys(ctx, checkpointTrie, rootHash, pathFinderVersion)
	if err != nil {
		return err
	}

	// ── measure the readers ───────────────────────────────────────────────────
	readers := []registerReader{
		triePathReader(checkpointTrie),
		trieHashReader(checkpointTrie, pathFinderVersion),
		storeReader(registerStore, height),
	}

	measurements := make([]measurement, 0, len(readers))
	for _, reader := range readers {
		start := time.Now()
		result, err := measureReads(reader, keys, flagWorkers, flagLatencyReads)
		if err != nil {
			return err
		}
		log.Info().Msgf("measured reader %s in %v", reader.name, time.Since(start))
		measurements = append(measurements, result)
	}

	log.Info().
		Str("pattern", flagPattern).
		Int("reads", len(keys)).
		Int("workers", flagWorkers).
		Msg("benchmark finished")
	printMeasurements(measurements)

	// ── check that both states hold the same registers ────────────────────────
	if flagCompareKeys > 0 {
		compareKeys := sampleEvery(keys, min(flagCompareKeys, len(keys)))
		mismatches, firstMismatch, err := compareReaders(compareKeys, readers)
		if err != nil {
			return err
		}
		if mismatches > 0 {
			return fmt.Errorf("the states hold different registers: %d of %d registers differ, first difference: %s",
				mismatches, len(compareKeys), firstMismatch)
		}
		log.Info().Msgf("read %d registers from both states, they hold the same values", len(compareKeys))
	}

	return nil
}

// collectKeys returns the registers to read, according to the pattern.
//
// Expected error returns during normal operations:
//   - [context.Canceled], [context.DeadlineExceeded]: if the context is cancelled
func collectKeys(
	ctx context.Context,
	checkpointTrie *trie.MTrie,
	rootHash ledger.RootHash,
	pathFinderVersion uint8,
) ([]registerKey, error) {
	switch flagPattern {
	case patternReplay:
		blocks, err := replayKeys(ctx, flagExecutionDataDir, flagBlocks, log.Logger)
		if err != nil {
			return nil, err
		}

		keys := flattenBlocks(blocks)
		log.Info().
			Int("blocks", len(blocks)).
			Int("reads", len(keys)).
			Msg("collected the registers the blocks touched")
		return keys, nil

	case patternRandom:
		keys, err := sampleCheckpointKeys(ctx, flagCheckpointFile, rootHash, flagKeys, flagWorkers, log.Logger)
		if err != nil {
			return nil, err
		}
		return keyStream(keys, flagReads, flagMissPercent, flagSeed, pathFinderVersion)

	case patternHotSet:
		keys, err := sampleCheckpointKeys(ctx, flagCheckpointFile, rootHash, flagKeys, flagWorkers, log.Logger)
		if err != nil {
			return nil, err
		}

		hotKeys := keys[:min(flagHotKeys, len(keys))]
		log.Info().
			Int("hot_keys", len(hotKeys)).
			Int("sampled_keys", len(keys)).
			Msg("collected the working set of registers")
		return keyStream(hotKeys, flagReads, flagMissPercent, flagSeed, pathFinderVersion)
	}

	return nil, fmt.Errorf("invalid pattern %q", flagPattern)
}
