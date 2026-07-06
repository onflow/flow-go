package rollback_trie_to_height

import (
	"encoding/hex"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	prometheusWAL "github.com/onflow/wal/wal"

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
// Approach:
//
// The reconstruction never walks the WAL backwards. It instead resets, on top of the latest (base)
// trie the ledger already has, every register that could differ between the base state and the target
// state back to its historical value. This works because a trie's root hash is a pure function of its
// final register key/value content — independent of the order or path of the updates that produced it.
// So applying the historical value of every differing register on top of the base yields a trie whose
// root equals the target block's parent state commitment.
//
// Concretely, `runE` performs the following steps:
//
//  1. Load the base trie. The ledger's WAL and checkpoints are replayed into a (payloadless) forest;
//     the base root is the most recently touched state (or --base-state-commitment).
//
//  2. Resolve the target state commitment — the known root to reconstruct — from the protocol/results
//     database at --target-height (or from --target-state-commitment).
//
//  3. Resolve the WAL segment range to scan. The registers that differ between the target and base
//     states are exactly those written by the blocks after the target, whose update records begin at
//     the one whose base root equals the target commitment (the block executed right after the
//     target). That segment is auto-detected (see [resolveSegmentRange]); the scan runs from there to
//     the newest segment, avoiding a needless full-history pass.
//
//  4. Build the rollback update (see [BuildRollbackTrieUpdate]): collect the set of registers written
//     in that segment range from the WAL, read each one's value as of the target height from the
//     Storehouse register store (a not-found register reads back as empty, i.e. a deletion), and
//     assemble a [ledger.TrieUpdate] anchored on the base root. Rewriting a register that did not
//     actually change with its unchanged value is a no-op on the root, so an over-broad register set
//     is harmless.
//
//  5. Verify — apply the update to the base trie and require the resulting root to equal the target
//     state commitment. Only if it matches is the record trusted; this also guards against a missing
//     WAL segment or insufficient register-history retention.
//
//  6. Write the record via RecordUpdate (skipped under --dry-run). On the next ledger startup, replay
//     applies this record last, leaving the ledger's latest trie at the target historical state,
//     ready for isolated re-execution.
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
		"first WAL segment number to scan for written registers; -1 (default) auto-detects the segment "+
			"where the target state commitment first appears. Set explicitly only to override auto-detection.")

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

	// 4. Determine the WAL segment range to scan for written registers. The registers that differ
	// between the target and base states are exactly those written by the blocks after the target, so
	// the scan need only start at the segment where the target state commitment first appears as an
	// update's base root (the block executed immediately after the target). Scanning earlier segments
	// is a harmless superset but wastes a full-history pass.
	from, to, err := resolveSegmentRange(logger, flagExecutionStateDir, diskWal, flagFromSegment, targetCommit)
	if err != nil {
		return err
	}
	logger.Info().Int("from_segment", from).Int("to_segment", to).Msg("resolved WAL segment range to scan")

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

// resolveSegmentRange returns the [from, to] WAL segment range to scan for written registers. `to`
// is always the last segment on disk.
//
// `from` is fromFlag when it is non-negative (an explicit override). Otherwise `from` is auto-detected
// as the segment where targetCommit first appears as an update record's base root — i.e. the segment
// holding the block executed immediately after the target height. Starting there yields exactly the
// blocks whose writes differ between the target and base states; earlier segments would only add
// no-op rewrites.
//
// Expected error returns during normal operation:
//   - an error when targetCommit cannot be located in the WAL (the target is older than the retained
//     WAL/checkpoint window, or a covering segment is missing).
func resolveSegmentRange(
	logger zerolog.Logger,
	dir string,
	diskWal *wal.DiskWAL,
	fromFlag int,
	targetCommit ledger.RootHash,
) (int, int, error) {
	first, last, err := diskWal.Segments()
	if err != nil {
		return 0, 0, fmt.Errorf("cannot list WAL segments: %w", err)
	}
	if first < 0 {
		return 0, 0, fmt.Errorf("no WAL segments found in %s", dir)
	}

	if fromFlag >= 0 {
		if fromFlag < first {
			return 0, 0, fmt.Errorf("requested from-segment %d is before the first segment on disk %d", fromFlag, first)
		}
		if fromFlag > last {
			return 0, 0, fmt.Errorf("requested from-segment %d is after the last segment on disk %d", fromFlag, last)
		}
		return fromFlag, last, nil
	}

	from, err := findSegmentWithBaseRoot(logger, dir, first, last, targetCommit)
	if err != nil {
		return 0, 0, err
	}
	return from, last, nil
}

// findSegmentWithBaseRoot returns the WAL segment in [first, last] that contains an update record
// whose base root equals targetCommit — the block executed immediately after the target height.
//
// The search runs backwards from the newest segment because re-execution targets are typically
// recent, so the target commitment is usually near the end. On the Flow chain every block changes the
// state commitment (the system transaction always updates block-level registers), so a given
// commitment appears as a base root in at most one update record; the newest matching segment is
// therefore the only matching segment. The final root verification in the caller is the backstop
// against any mislocation.
//
// Expected error returns during normal operation:
//   - an error when no update record in [first, last] has targetCommit as its base root.
func findSegmentWithBaseRoot(
	logger zerolog.Logger,
	dir string,
	first, last int,
	targetCommit ledger.RootHash,
) (int, error) {
	for seg := last; seg >= first; seg-- {
		logger.Info().
			Int("segment", seg).
			Int("first_segment", first).
			Int("last_segment", last).
			Msg("scanning WAL segment for target state commitment as a base root")
		found, err := segmentContainsBaseRoot(logger, dir, seg, targetCommit)
		if err != nil {
			return 0, fmt.Errorf("cannot scan WAL segment %d: %w", seg, err)
		}
		if found {
			return seg, nil
		}
	}
	return 0, fmt.Errorf(
		"target state commitment %s not found as a base root in WAL segments [%d,%d]; "+
			"the target height may be older than the retained WAL/checkpoint window, or a covering segment is missing",
		targetCommit, first, last,
	)
}

// segmentContainsBaseRoot reports whether the single WAL segment `seg` contains an update record
// whose base root equals targetRoot.
//
// No error returns are expected during normal operation.
func segmentContainsBaseRoot(logger zerolog.Logger, dir string, seg int, targetRoot ledger.RootHash) (bool, error) {
	sr, err := prometheusWAL.NewSegmentsRangeReader(logger, prometheusWAL.SegmentRange{
		Dir:   dir,
		First: seg,
		Last:  seg,
	})
	if err != nil {
		return false, fmt.Errorf("cannot create WAL segment reader: %w", err)
	}
	defer sr.Close()

	reader := prometheusWAL.NewReader(sr)
	for reader.Next() {
		operation, _, update, err := wal.Decode(reader.Record())
		if err != nil {
			return false, fmt.Errorf("cannot decode WAL record: %w", err)
		}
		if operation == wal.WALUpdate && update.RootHash.Equals(targetRoot) {
			return true, nil
		}
	}
	return false, reader.Err()
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
