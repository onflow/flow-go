package rollback_trie_to_height

import (
	"encoding/hex"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/payloadless"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	pebblestorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/storage/store"
)

var (
	flagExecutionStateDir string
	flagRegisterDir       string
	flagDataDir           string
	flagTargetHeight      uint64
	flagTargetCommit      string
	flagBaseCommit        string
	flagFromSegment       int
	flagDryRun            bool
)

// Cmd produces a single WAL update record that, when replayed by the ledger service on startup,
// rolls the ledger's latest trie back to the historical state at a target block height.
//
// The reconstruction resets, on top of the latest (base) trie, every register that has been written
// since the target height back to its historical value — read from the Storehouse register store as
// of the target height. Because a trie's root hash is a pure function of its register content, the
// resulting trie has the target block's state commitment. The command verifies this equality before
// writing (or, in dry-run mode, without writing) the WAL record.
var Cmd = &cobra.Command{
	Use:   "rollback-trie-to-height",
	Short: "Produce a WAL update that rolls the ledger's trie back to a historical state",
	Long: `Produce a single WAL update record that rolls the ledger's latest trie back to the
historical state at a target block height.

The set of registers that differ between the latest state and the target state is collected from the
WAL segments; each such register's value as of the target height is read from the Storehouse register
store; and a WAL update record anchored on the latest state commitment is written with those historical
values. On the next ledger startup, replaying this record leaves the ledger's latest trie at the target
historical state, ready for isolated re-execution.

The reconstructed root is verified against the target block's state commitment (derived from the
protocol/results database, or supplied via --target-state-commitment) before the record is trusted.`,
	RunE: runE,
}

func init() {
	Cmd.Flags().StringVar(&flagExecutionStateDir, "execution-state-dir", "",
		"directory containing the ledger WAL segments and checkpoints")
	_ = Cmd.MarkFlagRequired("execution-state-dir")

	Cmd.Flags().StringVar(&flagRegisterDir, "register-dir", "",
		"directory containing the Pebble Storehouse register store")
	_ = Cmd.MarkFlagRequired("register-dir")

	Cmd.Flags().StringVar(&flagDataDir, "datadir", "/var/flow/data/protocol",
		"directory containing the protocol/results database (used to derive state commitments by height)")

	Cmd.Flags().Uint64Var(&flagTargetHeight, "target-height", 0,
		"the block height whose historical trie state should be reconstructed")
	_ = Cmd.MarkFlagRequired("target-height")

	Cmd.Flags().StringVar(&flagTargetCommit, "target-state-commitment", "",
		"hex-encoded target state commitment to verify against; if empty it is derived from the protocol/results database at --target-height")

	Cmd.Flags().StringVar(&flagBaseCommit, "base-state-commitment", "",
		"hex-encoded base (anchor) state commitment; if empty the ledger's most recently touched state is used")

	Cmd.Flags().IntVar(&flagFromSegment, "from-segment", -1,
		"first WAL segment number to scan for written registers; -1 scans from the first segment on disk. "+
			"Bounding the scan is safe only when the target height is at or above the height of that segment.")

	Cmd.Flags().BoolVar(&flagDryRun, "dry-run", false,
		"verify the reconstruction without writing the WAL update record")
}

func runE(*cobra.Command, []string) error {
	logger := log.Logger

	logger.Info().
		Str("execution-state-dir", flagExecutionStateDir).
		Str("register-dir", flagRegisterDir).
		Str("datadir", flagDataDir).
		Uint64("target-height", flagTargetHeight).
		Bool("dry-run", flagDryRun).
		Msg("starting trie rollback")

	// 1. Open the Storehouse register store (read-only usage).
	registers, closeRegisters, err := openRegisterStore(flagRegisterDir)
	if err != nil {
		return err
	}
	defer closeRegisters()

	logger.Info().
		Uint64("first_height", registers.FirstHeight()).
		Uint64("latest_height", registers.LatestHeight()).
		Msg("opened register store")

	if flagTargetHeight < registers.FirstHeight() || flagTargetHeight > registers.LatestHeight() {
		return fmt.Errorf("target height %d is outside the register store's indexed range [%d, %d]",
			flagTargetHeight, registers.FirstHeight(), registers.LatestHeight())
	}

	// 2. Load the ledger's payloadless forest from the WAL + checkpoints and determine the base trie.
	diskWal, err := wal.NewDiskWAL(
		logger, nil, metrics.NewNoopCollector(), flagExecutionStateDir,
		complete.DefaultCacheSize, pathfinder.PathByteSize, wal.SegmentSize,
	)
	if err != nil {
		return fmt.Errorf("cannot open disk WAL at %s: %w", flagExecutionStateDir, err)
	}
	defer func() {
		<-diskWal.Done()
	}()

	forest, err := payloadless.NewForest(complete.DefaultCacheSize, metrics.NewNoopCollector(), nil)
	if err != nil {
		return fmt.Errorf("cannot create payloadless forest: %w", err)
	}

	logger.Info().Msg("replaying WAL onto payloadless forest to load the base trie")
	if err := diskWal.ReplayOnPayloadlessForest(forest); err != nil {
		return fmt.Errorf("cannot replay WAL onto payloadless forest: %w", err)
	}

	baseRoot, err := resolveBaseRoot(forest)
	if err != nil {
		return err
	}
	logger.Info().Str("base_root", baseRoot.String()).Msg("resolved base trie")

	// 3. Determine the target state commitment used to verify the reconstruction.
	targetCommit, err := resolveTargetCommit(logger, flagDataDir)
	if err != nil {
		return err
	}
	logger.Info().Str("target_commit", targetCommit.String()).Msg("resolved target state commitment")

	// 4. Determine the WAL segment range to scan for written registers.
	from, to, err := resolveSegmentRange(diskWal, flagFromSegment)
	if err != nil {
		return err
	}

	// 5. Build the rollback trie update.
	trieUpdate, err := BuildRollbackTrieUpdate(logger, flagExecutionStateDir, from, to, baseRoot, registers, flagTargetHeight)
	if err != nil {
		return fmt.Errorf("cannot build rollback trie update: %w", err)
	}

	// 6. Verify the reconstructed root equals the target state commitment (side-effect free).
	newTrie, err := forest.NewTrie(trieUpdate)
	if err != nil {
		return fmt.Errorf("cannot apply rollback trie update to base trie: %w", err)
	}
	newRoot := newTrie.RootHash()
	if !newRoot.Equals(targetCommit) {
		return fmt.Errorf("reconstruction verification FAILED: reconstructed root %s != target state commitment %s "+
			"(a WAL segment covering the range may be missing, or the register store does not retain history down to the target height)",
			newRoot, targetCommit)
	}
	logger.Info().
		Str("reconstructed_root", newRoot.String()).
		Msg("verification succeeded: rollback update reproduces the target state commitment")

	// 7. Write the WAL update record (unless dry-run).
	if flagDryRun {
		logger.Info().Msg("dry-run: not writing the WAL update record")
		return nil
	}

	segmentNum, skipped, err := diskWal.RecordUpdate(trieUpdate)
	if err != nil {
		return fmt.Errorf("cannot write rollback WAL update record: %w", err)
	}
	if skipped {
		return fmt.Errorf("WAL recording is paused; the rollback update was not written")
	}
	logger.Info().
		Int("segment", segmentNum).
		Msg("wrote rollback WAL update record; on next ledger startup the latest trie will be at the target historical state")

	return nil
}

// openRegisterStore opens the Pebble-backed Storehouse register store at dir and returns it together
// with a close function. Pruning is disabled so that the full retained register history is queryable.
//
// No error returns are expected during normal operation.
func openRegisterStore(dir string) (*pebblestorage.Registers, func(), error) {
	db, err := pebblestorage.OpenRegisterPebbleDB(log.Logger, dir)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot open register Pebble DB at %s: %w", dir, err)
	}

	registers, err := pebblestorage.NewRegisters(db, pebblestorage.PruningDisabled)
	if err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("cannot initialize register store: %w", err)
	}

	closeFn := func() {
		if err := db.Close(); err != nil {
			log.Error().Err(err).Msg("cannot close register Pebble DB")
		}
	}
	return registers, closeFn, nil
}

// resolveBaseRoot returns the base (anchor) root hash: either the value of --base-state-commitment,
// or the payloadless forest's most recently touched trie root when the flag is empty.
//
// No error returns are expected during normal operation.
func resolveBaseRoot(forest *payloadless.Forest) (ledger.RootHash, error) {
	if flagBaseCommit != "" {
		return parseRootHash(flagBaseCommit)
	}
	baseRoot, err := forest.MostRecentTouchedRootHash()
	if err != nil {
		return ledger.RootHash{}, fmt.Errorf("cannot determine most recently touched state from WAL: %w", err)
	}
	return baseRoot, nil
}

// resolveTargetCommit returns the target state commitment: either the value of
// --target-state-commitment, or the final state commitment recorded at --target-height in the
// protocol/results database when the flag is empty.
//
// No error returns are expected during normal operation.
func resolveTargetCommit(logger zerolog.Logger, dataDir string) (ledger.RootHash, error) {
	if flagTargetCommit != "" {
		return parseRootHash(flagTargetCommit)
	}

	protocolDB, err := pebblestorage.ShouldOpenDefaultPebbleDB(logger, dataDir)
	if err != nil {
		return ledger.RootHash{}, fmt.Errorf("cannot open protocol DB at %s: %w", dataDir, err)
	}
	defer func() {
		if err := protocolDB.Close(); err != nil {
			logger.Error().Err(err).Msg("cannot close protocol DB")
		}
	}()

	storages := store.InitAll(metrics.NewNoopCollector(), pebbleimpl.ToDB(protocolDB))
	return rootHashByHeight(storages.Headers, storages.Results, flagTargetHeight)
}

// rootHashByHeight returns the final state commitment of the finalized block at the given height,
// looked up via the protocol headers and execution results stores.
//
// No error returns are expected during normal operation.
func rootHashByHeight(headers storage.Headers, results storage.ExecutionResults, height uint64) (ledger.RootHash, error) {
	blockID, err := headers.BlockIDByHeight(height)
	if err != nil {
		return ledger.RootHash{}, fmt.Errorf("could not get block ID at height %d: %w", height, err)
	}
	result, err := results.ByBlockID(blockID)
	if err != nil {
		return ledger.RootHash{}, fmt.Errorf("could not get execution result for block %s: %w", blockID, err)
	}
	commit, err := result.FinalStateCommitment()
	if err != nil {
		return ledger.RootHash{}, fmt.Errorf("could not get final state commitment for block %s: %w", blockID, err)
	}
	return ledger.RootHash(commit), nil
}

// resolveSegmentRange returns the [from, to] WAL segment range to scan. `to` is always the last
// segment on disk. `from` is fromFlag when non-negative, otherwise the first segment on disk.
//
// No error returns are expected during normal operation.
func resolveSegmentRange(diskWal *wal.DiskWAL, fromFlag int) (int, int, error) {
	first, last, err := diskWal.Segments()
	if err != nil {
		return 0, 0, fmt.Errorf("cannot list WAL segments: %w", err)
	}
	if first < 0 {
		return 0, 0, fmt.Errorf("no WAL segments found in %s", flagExecutionStateDir)
	}
	from := first
	if fromFlag >= 0 {
		if fromFlag < first {
			return 0, 0, fmt.Errorf("requested from-segment %d is before the first segment on disk %d", fromFlag, first)
		}
		if fromFlag > last {
			return 0, 0, fmt.Errorf("requested from-segment %d is after the last segment on disk %d", fromFlag, last)
		}
		from = fromFlag
	}
	return from, last, nil
}

// parseRootHash decodes a hex-encoded state commitment / root hash.
//
// No error returns are expected during normal operation.
func parseRootHash(hexStr string) (ledger.RootHash, error) {
	b, err := hex.DecodeString(hexStr)
	if err != nil {
		return ledger.RootHash{}, fmt.Errorf("cannot hex-decode root hash %q: %w", hexStr, err)
	}
	rh, err := ledger.ToRootHash(b)
	if err != nil {
		return ledger.RootHash{}, fmt.Errorf("invalid root hash %q: %w", hexStr, err)
	}
	return rh, nil
}
