package reexecute_block

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	pebblestorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/storage/store"
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
node's databases are untouched. This is the compute-only mode from doc/re-execute-block.md, intended
for benchmarking execution cost/time and for deterministic replay; it needs no reconstructed trie.

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

	// Build the compute-only computation manager (no committer, in-memory exec-data provider).
	manager, err := NewComputeOnlyManager(
		logger, chainID, storages.Headers, state,
		flagTransactionFeesDisabled, flagScheduledTransactionsEnabled,
	)
	if err != nil {
		return fmt.Errorf("could not create compute-only manager: %w", err)
	}

	ctx := context.Background()
	mismatches := 0
	for height := flagFromHeight; height <= toHeight; height++ {
		mismatch, err := reExecuteHeight(ctx, logger, manager, stores, registers, height, flagVerify)
		if err != nil {
			return fmt.Errorf("could not re-execute height %d: %w", height, err)
		}
		if mismatch {
			mismatches++
			if flagStopOnMismatch {
				return fmt.Errorf("events mismatch at height %d", height)
			}
		}
	}

	if flagVerify && mismatches > 0 {
		return fmt.Errorf("re-execution finished with %d block(s) whose events did not match", mismatches)
	}

	logger.Info().Msg("re-execution finished")
	return nil
}

// reExecuteHeight re-executes the block at the given height and, when verify is set, compares its
// events against the stored events. It returns whether a verification mismatch occurred.
//
// No error returns are expected during normal operation.
func reExecuteHeight(
	ctx context.Context,
	logger zerolog.Logger,
	manager *computation.Manager,
	stores *reexecStores,
	registers RegisterGetter,
	height uint64,
	verify bool,
) (mismatch bool, err error) {
	block, err := stores.blocks.ByHeight(height)
	if err != nil {
		return false, fmt.Errorf("could not get block at height %d: %w", height, err)
	}
	blockID := block.ID()
	parentID := block.ParentID

	// The block executes against its parent's final state. Registers as of the parent height feed the
	// open-world snapshot, and the parent state commitment is the trie the block starts from.
	if height == 0 {
		return false, fmt.Errorf("cannot re-execute the root block (height 0)")
	}
	parentHeight := height - 1

	parentCommit, err := stores.commits.ByBlockID(parentID)
	if err != nil {
		return false, fmt.Errorf("could not get parent state commitment for block %s: %w", parentID, err)
	}

	// The parent block's execution result ID links the new result's PreviousResultID. It is not
	// required for execution; fall back to the zero ID if the parent result is not stored.
	var parentResultID flow.Identifier
	if parentResult, err := stores.results.ByBlockID(parentID); err == nil {
		parentResultID = parentResult.ID()
	} else if !errors.Is(err, storage.ErrNotFound) {
		return false, fmt.Errorf("could not get parent execution result for block %s: %w", parentID, err)
	}

	executableBlock, err := AssembleExecutableBlock(block, stores.collections, parentCommit)
	if err != nil {
		return false, fmt.Errorf("could not assemble executable block %s: %w", blockID, err)
	}

	registerSnapshot := StorehouseSnapshotAtHeight(registers, parentHeight)

	start := time.Now()
	result, err := manager.ComputeBlock(ctx, parentResultID, executableBlock, registerSnapshot)
	if err != nil {
		return false, fmt.Errorf("could not compute block %s: %w", blockID, err)
	}
	elapsed := time.Since(start)

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

	logger.Info().
		Uint64("height", height).
		Hex("block_id", blockID[:]).
		Int("collections", len(executableBlock.CompleteCollections)).
		Int("transactions", len(txResults)).
		Int("failed_transactions", failedTxs).
		Uint64("total_computation_used", totalComputation).
		Int("events", len(events)).
		Dur("duration", elapsed).
		Msg("re-executed block")

	if !verify {
		return false, nil
	}
	return verifyEvents(logger, stores.events, blockID, height, result)
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
