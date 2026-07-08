package reexecute_block

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/engine/execution/computation/committer"
	"github.com/onflow/flow-go/engine/execution/computation/computer"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete"
	ledgerfactory "github.com/onflow/flow-go/ledger/factory"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/storage"
	pebblestorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/storage/store"
)

// committerMode selects how the re-execution loop produces state commitments and proofs. It maps
// directly onto the execution node's committer choice (see cmd/execution_builder.go).
const (
	// committerNoop uses committer.NewNoopViewCommitter: no trie required, no proofs produced.
	// This is the compute-only mode for benchmarking pure execution cost.
	committerNoop = "noop"
	// committerPayloadless uses committer.NewPayloadlessLedgerViewCommitter over a payloadless
	// ledger, mirroring an execution node started with --payloadless.
	committerPayloadless = "payloadless"
	// committerFull uses committer.NewLedgerViewCommitter over a full (V6) ledger.
	committerFull = "full"
)

// reexecStores bundles the read-only storage modules the re-execution loop needs. Some of them
// (Commits, Events) are not part of [store.All] and are constructed directly.
type reexecStores struct {
	blocks      storage.Blocks
	collections storage.Collections
	commits     storage.Commits
	results     storage.ExecutionResults
	events      storage.Events
}

var (
	flagDataDir                      string
	flagRegisterDir                  string
	flagChain                        string
	flagFromHeight                   uint64
	flagToHeight                     uint64
	flagVerify                       bool
	flagStopOnMismatch               bool
	flagTransactionFeesDisabled      bool
	flagScheduledTransactionsEnabled bool
	flagCommitter                    string
	flagTrieDir                      string
	flagMTrieCacheSize               uint32
	flagCheckpointDistance           uint
	flagCheckpointsToKeep            uint
	flagRepeat                       uint
	flagWarmup                       uint
)

// Cmd re-executes a historical block (or a range of blocks) in memory without persisting anything.
//
// Registers are sourced from the Storehouse register store at the parent height (an open-world source
// that works across execution versions). Execution reuses the production computation stack via
// ComputeBlock and never invokes any persistence step: it computes results in memory and writes
// nothing to the node's databases (no last-executed-height update, no results/chunk-data-packs/
// events/receipts/registers). It is safe to run against a stopped node's data directory.
var Cmd = &cobra.Command{
	Use:   "reexecute-block",
	Short: "Re-execute a historical block or range in memory, without persistence",
	Long: `Re-execute a historical block (or a range of blocks) in memory, without persisting anything.

Registers are read from the Storehouse register store at each block's parent height, and execution
reuses the production computation layer (ComputeBlock). No persistence step is ever invoked, so the
node's protocol/register databases are untouched.

--committer selects how state commitments and proofs are produced:
  - noop (default): no trie required, no proofs produced. The compute-only mode from
    doc/re-execute-block.md, for benchmarking pure execution cost/time and deterministic replay.
  - payloadless: drives a payloadless ledger and collects proofs exactly as an execution node started
    with --payloadless does. Requires --triedir. Use this to benchmark the commit/proof-collection path.
  - full: drives a full (V6) ledger via committer.NewLedgerViewCommitter. Requires --triedir.

For payloadless/full, --triedir must contain a trie (checkpoint + WAL) that covers the requested
height range: the parent trie of each re-executed block must be present in the ledger forest. Because
re-execution calls the ledger's Set, the WAL under --triedir is appended to (and may be checkpointed);
point --triedir at a COPY of the node's execution state directory, not the live one.

With --verify, each re-executed block's events are checked against the events recorded for that block
in the protocol database, confirming the re-execution is faithful.`,
	RunE: runE,
}

func init() {
	Cmd.Flags().StringVar(&flagDataDir, "datadir", "/var/flow/data/protocol",
		"directory containing the protocol/execution database (blocks, collections, results, events)")

	Cmd.Flags().StringVar(&flagRegisterDir, "register-dir", "",
		"directory containing the Pebble Storehouse register store")
	_ = Cmd.MarkFlagRequired("register-dir")

	Cmd.Flags().StringVar(&flagChain, "chain", "", "chain ID (e.g. flow-mainnet, flow-testnet)")
	_ = Cmd.MarkFlagRequired("chain")

	Cmd.Flags().Uint64Var(&flagFromHeight, "from-height", 0, "first block height to re-execute")
	_ = Cmd.MarkFlagRequired("from-height")

	Cmd.Flags().Uint64Var(&flagToHeight, "to-height", 0,
		"last block height to re-execute (defaults to --from-height when zero)")

	Cmd.Flags().BoolVar(&flagVerify, "verify", false,
		"verify each re-executed block's events against the events stored in the database")

	Cmd.Flags().BoolVar(&flagStopOnMismatch, "stop-on-mismatch", false,
		"when verifying, stop at the first block whose events do not match")

	Cmd.Flags().BoolVar(&flagTransactionFeesDisabled, "transaction-fees-disabled", false,
		"disable transaction fees in the FVM (must match how the blocks were originally executed)")

	Cmd.Flags().BoolVar(&flagScheduledTransactionsEnabled, "scheduled-transactions-enabled", true,
		"enable scheduled transactions in the FVM (must match how the blocks were originally executed)")

	Cmd.Flags().StringVar(&flagCommitter, "committer", committerNoop,
		"view committer to use: 'noop' (compute-only, no proofs), 'payloadless', or 'full'")

	Cmd.Flags().StringVar(&flagTrieDir, "triedir", "",
		"directory containing the ledger trie (checkpoint + WAL); required for --committer=payloadless|full. "+
			"Point at a COPY: re-execution appends to the WAL and may write checkpoints")

	Cmd.Flags().Uint32Var(&flagMTrieCacheSize, "mtrie-cache-size", 1000,
		"number of tries to keep in the ledger forest (payloadless/full committer only)")

	Cmd.Flags().UintVar(&flagCheckpointDistance, "checkpoint-distance", 20,
		"segments between checkpoints (payloadless/full committer only)")

	Cmd.Flags().UintVar(&flagCheckpointsToKeep, "checkpoints-to-keep", 5,
		"number of checkpoints to retain (payloadless/full committer only)")

	Cmd.Flags().UintVar(&flagRepeat, "repeat", 1,
		"number of times to re-execute the height range in this single process, for benchmarking. "+
			"All iterations share the same warm process (Pebble block cache, page cache, seeded ledger, "+
			"steady-state GC), so per-height min/median/p90 timing is reported at the end")

	Cmd.Flags().UintVar(&flagWarmup, "warmup", 0,
		"number of leading --repeat iterations to execute but exclude from the timing summary, so cold-"+
			"cache and GC-warmup effects do not skew the reported statistics. Must be less than --repeat")
}

func runE(*cobra.Command, []string) error {
	logger := log.Logger

	toHeight := flagToHeight
	if toHeight == 0 {
		toHeight = flagFromHeight
	}
	if toHeight < flagFromHeight {
		return fmt.Errorf("to-height %d is less than from-height %d", toHeight, flagFromHeight)
	}

	if flagRepeat == 0 {
		return fmt.Errorf("--repeat must be at least 1")
	}
	if flagWarmup >= flagRepeat {
		return fmt.Errorf("--warmup %d must be less than --repeat %d", flagWarmup, flagRepeat)
	}

	chainID := flow.ChainID(flagChain)

	logger.Info().
		Str("datadir", flagDataDir).
		Str("register-dir", flagRegisterDir).
		Str("chain", flagChain).
		Uint64("from", flagFromHeight).
		Uint64("to", toHeight).
		Bool("verify", flagVerify).
		Msg("starting persistence-free re-execution")

	// Open the protocol/execution database read-only and derive the protocol state.
	db, err := common.InitStorage(flagDataDir)
	if err != nil {
		return fmt.Errorf("could not open protocol database at %s: %w", flagDataDir, err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error().Err(err).Msg("could not close protocol database")
		}
	}()

	storages := common.InitStorages(db)
	state, err := common.OpenProtocolState(storage.MakeSingletonLockManager(), db, storages)
	if err != nil {
		return fmt.Errorf("could not open protocol state: %w", err)
	}

	stores := &reexecStores{
		blocks:      storages.Blocks,
		collections: storages.Collections,
		commits:     store.NewCommits(metrics.NewNoopCollector(), db),
		results:     storages.Results,
		events:      store.NewEvents(metrics.NewNoopCollector(), db),
	}

	// Open the Storehouse register store.
	registers, closeRegisters, err := openRegisterStore(flagRegisterDir)
	if err != nil {
		return err
	}
	defer closeRegisters()

	logger.Info().
		Uint64("first_height", registers.FirstHeight()).
		Uint64("latest_height", registers.LatestHeight()).
		Msg("opened register store")

	// Build the view committer selected by --committer, and (for a trie-backed committer) the ledger
	// state checker used to confirm the parent trie is present before executing each height.
	viewCommitter, stateChecker, closeCommitter, err := buildViewCommitter(logger)
	if err != nil {
		return err
	}
	defer closeCommitter()

	// Wrap the committer so per-block CommitView cost can be reported. For the payloadless/full
	// committer this is dominated by proof collection, which is what benchmarking targets. The wrapper
	// is reset before each block, so it is shared across all repeat iterations.
	timedCommitter := newTimingViewCommitter(viewCommitter)

	ctx := context.Background()
	mismatches := 0

	// durations collects, per height, the block-level ComputeBlock duration from every measured (non-
	// warmup) repeat iteration. All iterations run in this one warm process, so aggregating them into a
	// per-height min/median/p90 yields a benchmark far more stable than any single run.
	// proofDurations collects the corresponding per-block proof-collection time (0 for committers that
	// do not collect proofs), so proof collection can be benchmarked in isolation.
	durations := make(map[uint64][]time.Duration)
	proofDurations := make(map[uint64][]time.Duration)

	for iter := uint(0); iter < flagRepeat; iter++ {
		warmup := iter < flagWarmup

		// Build a fresh computation manager for each iteration. The manager owns the derived-data cache
		// (parsed/checked Cadence programs), which advances a per-block monotonic logical clock on commit;
		// reusing it to re-execute the same height twice fails validation ("non-increasing time"). A fresh
		// manager gives every iteration an identical cold programs cache — matching the original single-run
		// behavior — while the register store and (payloadless/full) ledger stay warm and shared across
		// iterations.
		manager, err := NewComputationManager(
			logger, chainID, storages.Headers, state, timedCommitter,
			flagTransactionFeesDisabled, flagScheduledTransactionsEnabled,
		)
		if err != nil {
			return fmt.Errorf("could not create computation manager (iteration %d): %w", iter, err)
		}

		for height := flagFromHeight; height <= toHeight; height++ {
			elapsed, proofElapsed, mismatch, err := reExecuteHeight(
				ctx, logger, manager, timedCommitter, stateChecker, stores, registers, height, flagVerify, iter, warmup)
			if err != nil {
				return fmt.Errorf("could not re-execute height %d (iteration %d): %w", height, iter, err)
			}
			if !warmup {
				durations[height] = append(durations[height], elapsed)
				proofDurations[height] = append(proofDurations[height], proofElapsed)
			}
			if mismatch {
				mismatches++
				if flagStopOnMismatch {
					return fmt.Errorf("events mismatch at height %d", height)
				}
			}
		}
	}

	if flagVerify && mismatches > 0 {
		return fmt.Errorf("re-execution finished with %d block(s) whose events did not match", mismatches)
	}

	// Report the per-height timing summary over all measured iterations. This is the reliable benchmark
	// signal: the minimum is the least noise-perturbed sample and best for A/B comparison, while
	// median/p90 characterize the run-to-run spread.
	if flagRepeat > 1 {
		for height := flagFromHeight; height <= toHeight; height++ {
			s := computeDurationStats(durations[height])
			logger.Info().
				Uint64("height", height).
				Int("samples", s.count).
				Uint("warmup_discarded", flagWarmup).
				Float64("min_ms", msFloat(s.min)).
				Float64("median_ms", msFloat(s.median)).
				Float64("p90_ms", msFloat(s.p90)).
				Float64("max_ms", msFloat(s.max)).
				Float64("mean_ms", msFloat(s.mean)).
				Msg("re-execution timing summary")

			// Proof-collection timing summary. All-zero for the noop committer, which collects no proofs.
			ps := computeDurationStats(proofDurations[height])
			logger.Info().
				Uint64("height", height).
				Int("samples", ps.count).
				Uint("warmup_discarded", flagWarmup).
				Float64("min_ms", msFloat(ps.min)).
				Float64("median_ms", msFloat(ps.median)).
				Float64("p90_ms", msFloat(ps.p90)).
				Float64("max_ms", msFloat(ps.max)).
				Float64("mean_ms", msFloat(ps.mean)).
				Msg("collect proof timing summary")
		}
	}

	logger.Info().Msg("re-execution finished")
	return nil
}

// durationStats holds summary statistics over a set of measured durations.
type durationStats struct {
	count  int
	min    time.Duration
	median time.Duration
	p90    time.Duration
	max    time.Duration
	mean   time.Duration
}

// computeDurationStats returns summary statistics over the given durations. The input slice is copied
// before sorting, so the caller's ordering is preserved. Returns the zero value for an empty input.
func computeDurationStats(ds []time.Duration) durationStats {
	if len(ds) == 0 {
		return durationStats{}
	}
	sorted := make([]time.Duration, len(ds))
	copy(sorted, ds)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	var sum time.Duration
	for _, d := range sorted {
		sum += d
	}

	return durationStats{
		count:  len(sorted),
		min:    sorted[0],
		median: percentile(sorted, 50),
		p90:    percentile(sorted, 90),
		max:    sorted[len(sorted)-1],
		mean:   sum / time.Duration(len(sorted)),
	}
}

// percentile returns the p-th percentile (0..100) of the pre-sorted, ascending, non-empty slice using
// the nearest-rank method. Returns 0 for an empty slice.
func percentile(sorted []time.Duration, p int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	rank := (p*len(sorted) + 99) / 100 // ceil(p/100 * n), 1-indexed
	if rank < 1 {
		rank = 1
	}
	if rank > len(sorted) {
		rank = len(sorted)
	}
	return sorted[rank-1]
}

// msFloat converts a duration to milliseconds as a float, preserving sub-millisecond precision.
func msFloat(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000.0
}

// reExecuteHeight re-executes the block at the given height and, when verify is set, compares its
// events against the stored events. It returns the wall-clock duration of the timed ComputeBlock call
// and whether a verification mismatch occurred.
//
// `iteration` is the zero-based --repeat iteration index and `warmup` indicates whether this iteration
// is a discarded warmup pass; both are attached to the per-block log so lines are attributable across
// repeated runs. They do not alter execution.
//
// `stateChecker` may be nil (the no-op committer needs no trie); when non-nil it is used to confirm
// the parent trie is present in the ledger forest before execution.
//
// No error returns are expected during normal operation.
func reExecuteHeight(
	ctx context.Context,
	logger zerolog.Logger,
	manager *computation.Manager,
	timedCommitter *timingViewCommitter,
	stateChecker ledgerStateChecker,
	stores *reexecStores,
	registers RegisterGetter,
	height uint64,
	verify bool,
	iteration uint,
	warmup bool,
) (elapsed time.Duration, proofElapsed time.Duration, mismatch bool, err error) {
	block, err := stores.blocks.ByHeight(height)
	if err != nil {
		return 0, 0, false, fmt.Errorf("could not get block at height %d: %w", height, err)
	}
	blockID := block.ID()
	parentID := block.ParentID

	// The block executes against its parent's final state. Registers as of the parent height feed the
	// open-world snapshot, and the parent state commitment is the trie the block starts from.
	if height == 0 {
		return 0, 0, false, fmt.Errorf("cannot re-execute the root block (height 0)")
	}
	parentHeight := height - 1

	parentCommit, err := stores.commits.ByBlockID(parentID)
	if err != nil {
		return 0, 0, false, fmt.Errorf("could not get parent state commitment for block %s: %w", parentID, err)
	}

	// A trie-backed committer can only commit/prove against a trie that is present in its forest. The
	// block executes on top of the parent's final state, so that trie must be loadable from --triedir.
	if stateChecker != nil && !stateChecker.HasState(ledger.State(parentCommit)) {
		return 0, 0, false, fmt.Errorf(
			"parent trie for height %d (state %x) is not present in the ledger forest; "+
				"--triedir must contain a checkpoint/WAL covering this height",
			height, parentCommit)
	}

	// The parent block's execution result ID links the new result's PreviousResultID. It is not
	// required for execution; fall back to the zero ID if the parent result is not stored.
	var parentResultID flow.Identifier
	if parentResult, err := stores.results.ByBlockID(parentID); err == nil {
		parentResultID = parentResult.ID()
	} else if !errors.Is(err, storage.ErrNotFound) {
		return 0, 0, false, fmt.Errorf("could not get parent execution result for block %s: %w", parentID, err)
	}

	executableBlock, err := AssembleExecutableBlock(block, stores.collections, parentCommit)
	if err != nil {
		return 0, 0, false, fmt.Errorf("could not assemble executable block %s: %w", blockID, err)
	}

	registerSnapshot := StorehouseSnapshotAtHeight(registers, parentHeight)

	timedCommitter.reset()
	start := time.Now()
	result, err := manager.ComputeBlock(ctx, parentResultID, executableBlock, registerSnapshot)
	if err != nil {
		return 0, 0, false, fmt.Errorf("could not compute block %s: %w", blockID, err)
	}
	elapsed = time.Since(start)
	commitCalls, commitDuration := timedCommitter.stats()
	proofCalls, proofDuration := timedCommitter.proofStats()
	proofElapsed = proofDuration

	txResults := result.AllTransactionResults()
	events := result.AllEvents()
	var totalComputation uint64
	failedTxs := 0
	for _, r := range txResults {
		totalComputation += r.ComputationUsed
		if r.ErrorMessage != "" {
			failedTxs++
		}
	}

	endState := result.CurrentEndState()
	resultID := result.ExecutionReceipt.ExecutionResult.ID()

	logger.Info().
		Uint64("height", height).
		Uint("iteration", iteration).
		Bool("warmup", warmup).
		Hex("block_id", blockID[:]).
		Int("collections", len(executableBlock.CompleteCollections)).
		Int("transactions", len(txResults)).
		Int("failed_transactions", failedTxs).
		Uint64("total_computation_used", totalComputation).
		Int("events", len(events)).
		Hex("state_commitment", endState[:]).
		Hex("result_id", resultID[:]).
		Dur("duration", elapsed).
		Int64("commit_view_calls", commitCalls).
		Dur("commit_view_duration", commitDuration).
		Int64("collect_proof_calls", proofCalls).
		Dur("collect_proof_duration", proofDuration).
		Msg("re-executed block")

	if !verify {
		return elapsed, proofElapsed, false, nil
	}

	mismatch, err = verifyEvents(logger, stores.events, blockID, height, result)
	return elapsed, proofElapsed, mismatch, err
}

// verifyEvents compares the events produced by re-execution against those recorded for the block in
// the database, using their Merkle root. It returns whether the events did not match.
//
// No error returns are expected during normal operation.
func verifyEvents(
	logger zerolog.Logger,
	eventsStore storage.Events,
	blockID flow.Identifier,
	height uint64,
	result *execution.ComputationResult,
) (mismatch bool, err error) {
	storedEvents, err := eventsStore.ByBlockID(blockID)
	if err != nil {
		return false, fmt.Errorf("could not get stored events for block %s: %w", blockID, err)
	}

	computedRoot, err := flow.EventsMerkleRootHash(result.AllEvents())
	if err != nil {
		return false, fmt.Errorf("could not compute events root for re-executed block %s: %w", blockID, err)
	}
	storedRoot, err := flow.EventsMerkleRootHash(storedEvents)
	if err != nil {
		return false, fmt.Errorf("could not compute events root for stored block %s: %w", blockID, err)
	}

	if computedRoot != storedRoot {
		logger.Error().
			Uint64("height", height).
			Int("computed_events", len(result.AllEvents())).
			Int("stored_events", len(storedEvents)).
			Str("computed_events_root", computedRoot.String()).
			Str("stored_events_root", storedRoot.String()).
			Msg("events mismatch: re-execution did not reproduce the recorded events")
		return true, nil
	}

	logger.Info().
		Uint64("height", height).
		Str("events_root", computedRoot.String()).
		Msg("verified: re-executed events match the recorded events")
	return false, nil
}

// openRegisterStore opens the Pebble-backed Storehouse register store at dir with pruning disabled so
// the full retained register history is queryable, returning it together with a close function.
//
// No error returns are expected during normal operation.
func openRegisterStore(dir string) (*pebblestorage.Registers, func(), error) {
	pebbleDB, err := pebblestorage.OpenRegisterPebbleDB(log.Logger, dir)
	if err != nil {
		return nil, nil, fmt.Errorf("could not open register Pebble DB at %s: %w", dir, err)
	}

	registers, err := pebblestorage.NewRegisters(pebbleDB, pebblestorage.PruningDisabled)
	if err != nil {
		_ = pebbleDB.Close()
		return nil, nil, fmt.Errorf("could not initialize register store: %w", err)
	}

	closeFn := func() {
		if err := pebbleDB.Close(); err != nil {
			log.Error().Err(err).Msg("could not close register Pebble DB")
		}
	}
	return registers, closeFn, nil
}

// ledgerStateChecker reports whether a given trie state is present in a ledger's forest. Both the full
// [ledger.Ledger] and the [ledger.PayloadlessLedger] satisfy it via their HasState method.
type ledgerStateChecker interface {
	HasState(state ledger.State) bool
}

// buildViewCommitter constructs the view committer selected by --committer. It returns the committer,
// a ledger state checker (nil for the no-op committer, which needs no trie), and a close function that
// shuts down the backing ledger (a no-op for the no-op committer).
//
// For the payloadless and full committers a trie-backed ledger is opened from --triedir via the same
// factory the execution node uses; the ledger's WAL is appended to as blocks are re-executed, so
// --triedir must point at a copy of the node's execution state.
//
// Expected error returns during normal operation:
//   - error if --committer is not one of noop, payloadless, full
//   - error if --triedir is empty for a trie-backed committer
func buildViewCommitter(logger zerolog.Logger) (computer.ViewCommitter, ledgerStateChecker, func(), error) {
	noopClose := func() {}

	switch flagCommitter {
	case committerNoop:
		return committer.NewNoopViewCommitter(), nil, noopClose, nil

	case committerPayloadless:
		if flagTrieDir == "" {
			return nil, nil, nil, fmt.Errorf("--triedir is required for --committer=payloadless")
		}
		pl, err := ledgerfactory.NewPayloadlessLedger(ledgerConfig(logger), nil)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("could not open payloadless ledger at %s: %w", flagTrieDir, err)
		}
		<-pl.Ready()
		viewCommitter := committer.NewPayloadlessLedgerViewCommitter(pl, trace.NewNoopTracer(), complete.DefaultPathFinderVersion)
		closeFn := func() { <-pl.Done() }
		return viewCommitter, pl, closeFn, nil

	case committerFull:
		if flagTrieDir == "" {
			return nil, nil, nil, fmt.Errorf("--triedir is required for --committer=full")
		}
		led, err := ledgerfactory.NewLedger(ledgerConfig(logger), nil)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("could not open full ledger at %s: %w", flagTrieDir, err)
		}
		<-led.Ready()
		viewCommitter := committer.NewLedgerViewCommitter(led, trace.NewNoopTracer())
		// The concrete full ledger implements HasState; guard the type assertion so an
		// implementation that does not (e.g. a future remote client) simply skips the check.
		stateChecker, _ := led.(ledgerStateChecker)
		closeFn := func() { <-led.Done() }
		return viewCommitter, stateChecker, closeFn, nil

	default:
		return nil, nil, nil, fmt.Errorf(
			"invalid --committer %q: must be one of %q, %q, %q",
			flagCommitter, committerNoop, committerPayloadless, committerFull)
	}
}

// ledgerConfig assembles the trie-backed ledger configuration from the command's flags. Metrics are
// no-ops and no Prometheus registerer is wired: this is a short-lived, single-process utility.
func ledgerConfig(logger zerolog.Logger) ledgerfactory.Config {
	return ledgerfactory.Config{
		Triedir:            flagTrieDir,
		MTrieCacheSize:     flagMTrieCacheSize,
		CheckpointDistance: flagCheckpointDistance,
		CheckpointsToKeep:  flagCheckpointsToKeep,
		MetricsRegisterer:  nil,
		WALMetrics:         metrics.NewNoopCollector(),
		LedgerMetrics:      metrics.NewNoopCollector(),
		Logger:             logger,
	}
}

// timingViewCommitter wraps a [computer.ViewCommitter] and accumulates the wall-clock time and call
// count of CommitView. The re-execution loop resets it before each block and reads the totals after,
// to report per-block committer cost — which, for the payloadless/full committer, is dominated by
// proof collection.
//
// Safe for concurrent access: the block computer invokes CommitView from multiple goroutines.
type timingViewCommitter struct {
	inner computer.ViewCommitter
	calls atomic.Int64
	nanos atomic.Int64
}

// newTimingViewCommitter wraps inner so its CommitView calls are timed.
func newTimingViewCommitter(inner computer.ViewCommitter) *timingViewCommitter {
	return &timingViewCommitter{inner: inner}
}

// CommitView delegates to the wrapped committer, recording the call's duration.
//
// No error returns are expected during normal operation beyond those of the wrapped committer.
func (c *timingViewCommitter) CommitView(
	execSnapshot *snapshot.ExecutionSnapshot,
	baseStorageSnapshot execution.ExtendableStorageSnapshot,
) (flow.StateCommitment, []byte, *ledger.TrieUpdate, execution.ExtendableStorageSnapshot, error) {
	start := time.Now()
	commit, proof, trieUpdate, newSnapshot, err := c.inner.CommitView(execSnapshot, baseStorageSnapshot)
	c.nanos.Add(int64(time.Since(start)))
	c.calls.Add(1)
	return commit, proof, trieUpdate, newSnapshot, err
}

// reset zeroes the accumulated CommitView stats, and the wrapped committer's proof-collection stats
// when it exposes them. Call before re-executing a block.
func (c *timingViewCommitter) reset() {
	c.calls.Store(0)
	c.nanos.Store(0)
	if pt, ok := c.inner.(proofTimer); ok {
		pt.ResetProofCollectionStats()
	}
}

// stats returns the accumulated CommitView call count and total duration since the last reset.
func (c *timingViewCommitter) stats() (calls int64, total time.Duration) {
	return c.calls.Load(), time.Duration(c.nanos.Load())
}

// proofStats returns the wrapped committer's accumulated proof-collection call count and total
// duration since the last reset. Returns (0, 0) when the wrapped committer does not collect proofs
// (the no-op committer) or does not expose the stats.
func (c *timingViewCommitter) proofStats() (calls int64, total time.Duration) {
	if pt, ok := c.inner.(proofTimer); ok {
		return pt.ProofCollectionStats()
	}
	return 0, 0
}

// proofTimer is implemented by committers that accumulate proof-collection timing (the payloadless
// committer). It lets the benchmark report per-block proof-collection cost — which runs concurrently
// with the state commit inside CommitView — separately from the overall CommitView duration.
type proofTimer interface {
	ProofCollectionStats() (calls int64, total time.Duration)
	ResetProofCollectionStats()
}
