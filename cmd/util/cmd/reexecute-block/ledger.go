package reexecute_block

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution/computation/committer"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/ledger/complete/wal/fixtures"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
)

// openPayloadlessLedger opens a payloadless ledger for proof-generation mode and returns the ledger
// together with a close function that must be called to release resources.
//
// The ledger seeds its in-memory forest from the latest V7 (payloadless) checkpoint plus newer WAL
// segments — the same recovery [complete.NewPayloadlessLedger] performs at node startup. A
// [fixtures.NoopPayloadlessCompactor] is attached so that when the committer's Set publishes a trie
// update, the update is acknowledged and applied to the in-memory forest only: no WAL records and no
// checkpoints are written. For a multi-block range this is what lets block N+1 start from the trie
// block N produced.
//
// Directory usage:
//   - triedir is the read source: the V7 checkpoint and WAL segments to replay live here.
//   - walDir, when non-empty and different from triedir, is where the DiskWAL is opened, so the empty
//     trailing segment created on open, the exclusive lock, and any WAL writes land there instead of
//     in triedir. triedir's checkpoint and segments are symlinked into walDir so replay still finds
//     them (read-only). This keeps the node's real trie directory pristine; the operator can delete
//     walDir after the run. When walDir is empty the WAL is opened directly over triedir (in which
//     case that one empty segment and the lock are created in triedir, like other ledger-read utils).
//
// Opening a [wal.DiskWAL] takes an exclusive lock on its directory, so the node must be stopped.
//
// The V7-checkpoint pre-check runs before the WAL is opened so a trie dir that is not
// payloadless-ready fails fast without creating the empty segment or taking the lock.
//
// Expected error returns during normal operation:
//   - error if triedir is empty.
//   - error if no V7 (payloadless) checkpoint exists in triedir.
//   - error if walDir is set but not empty (it must be a fresh, disposable directory).
func openPayloadlessLedger(
	logger zerolog.Logger,
	triedir string,
	walDir string,
) (*complete.PayloadlessLedger, func(), error) {
	if triedir == "" {
		return nil, nil, fmt.Errorf("payloadless committer requires a non-empty --triedir")
	}

	if err := requireV7Checkpoint(logger, triedir); err != nil {
		return nil, nil, err
	}

	// Decide where the WAL is opened. When a separate walDir is requested, stage it with symlinks to
	// triedir's checkpoint and segment files so replay reads from there while new writes stay isolated.
	walOpenDir := triedir
	if walDir != "" && walDir != triedir {
		if err := stageWALDir(logger, triedir, walDir); err != nil {
			return nil, nil, fmt.Errorf("could not stage wal dir %s: %w", walDir, err)
		}
		walOpenDir = walDir
	}

	diskWAL, err := wal.NewDiskWAL(
		logger.With().Str("subcomponent", "wal").Logger(),
		nil,
		metrics.NewNoopCollector(),
		walOpenDir,
		complete.DefaultCacheSize,
		pathfinder.PathByteSize,
		wal.SegmentSize,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("could not open disk WAL at %s: %w", walOpenDir, err)
	}

	// NewPayloadlessLedger replays the latest V7 checkpoint plus newer segments into the forest.
	// Record is paused during that replay so no WAL writes happen while seeding.
	ledger, err := complete.NewPayloadlessLedger(
		diskWAL,
		complete.DefaultCacheSize,
		metrics.NewNoopCollector(),
		logger.With().Str("subcomponent", "ledger").Logger(),
		complete.DefaultPathFinderVersion,
	)
	if err != nil {
		<-diskWAL.Done()
		return nil, nil, fmt.Errorf("could not create payloadless ledger from %s: %w", triedir, err)
	}

	// The noop compactor drains the ledger's trie-update channel and acknowledges each update without
	// writing to the WAL, so Set does not block and nothing durable is produced.
	compactor := fixtures.NewNoopPayloadlessCompactor(ledger)
	<-compactor.Ready()

	closeFn := func() {
		// Ordering mirrors the production/full-ledger shutdown: the ledger closes its trie-update
		// channel first so the compactor can drain, then the WAL is closed and its lock released.
		<-ledger.Done()
		<-compactor.Done()
		<-diskWAL.Done()
	}

	return ledger, closeFn, nil
}

// newPayloadlessCommitter builds the production payloadless view committer over the given ledger.
// The committer's logger is forced to debug level so its per-collection proof-generation timing (the
// ground-truth measurement for the ProveAndReconstruct benchmark) is emitted regardless of the tool's
// global log level, without turning on debug logging everywhere.
func newPayloadlessCommitter(
	logger zerolog.Logger,
	ledger *complete.PayloadlessLedger,
) *committer.PayloadlessLedgerViewCommitter {
	return committer.NewPayloadlessLedgerViewCommitter(
		ledger,
		trace.NewNoopTracer(),
		logger.Level(zerolog.DebugLevel),
		complete.DefaultPathFinderVersion,
	)
}

// requireV7Checkpoint verifies that triedir contains a V7 (payloadless) checkpoint — either a numbered
// one written by the compactor or a V7 root checkpoint converted during bootstrap — before the ledger
// is opened. This mirrors the validation in factory.newLocalPayloadlessLedger, including the hint to
// run the checkpoint-convert-v7 util when only V6 checkpoints are present.
//
// Expected error returns during normal operation:
//   - error if no V7 checkpoint of either kind exists in triedir.
func requireV7Checkpoint(logger zerolog.Logger, triedir string) error {
	v7Numbers, latestV7, err := wal.ListV7Checkpoints(triedir)
	if err != nil {
		return fmt.Errorf("could not list V7 checkpoints in %s: %w", triedir, err)
	}
	if latestV7 >= 0 {
		logger.Info().
			Str("triedir", triedir).
			Int("latest_v7", latestV7).
			Int("v7_count", len(v7Numbers)).
			Msg("payloadless ledger: V7 checkpoint discovered; the ledger will seed from it")
		return nil
	}

	hasV7Root, err := wal.HasRootCheckpointV7(triedir)
	if err != nil {
		return fmt.Errorf("could not check for V7 root checkpoint in %s: %w", triedir, err)
	}
	if hasV7Root {
		logger.Info().
			Str("triedir", triedir).
			Msg("payloadless ledger: V7 root checkpoint discovered; the ledger will seed from it")
		return nil
	}

	// No V7 checkpoint. Point the operator at the convert util when V6 checkpoints exist. A failure to
	// list V6 checkpoints here is non-fatal: we still want the operator to see the primary "no V7"
	// error.
	v6Numbers, latestV6, v6ListErr := wal.ListV6Checkpoints(triedir)
	if v6ListErr != nil {
		logger.Warn().Err(v6ListErr).
			Str("triedir", triedir).
			Msg("payloadless ledger: could not also list V6 checkpoints while reporting missing V7")
	}
	if latestV6 >= 0 {
		return fmt.Errorf(
			"no V7 (payloadless) checkpoint found in %s but %d V6 checkpoint(s) exist (latest: %d); "+
				"run the `checkpoint-convert-v7` util to produce a V7 checkpoint first",
			triedir, len(v6Numbers), latestV6,
		)
	}
	return fmt.Errorf(
		"no V7 (payloadless) checkpoint found in %s; a V7 checkpoint is required for proof-generation mode",
		triedir,
	)
}

// stageWALDir prepares walDir as an isolated WAL directory that replays from sourceDir. It symlinks
// every entry of sourceDir (except the WAL `.lock` file) into walDir, so [complete.NewPayloadlessLedger]
// finds the checkpoint and WAL segments to replay there while the DiskWAL opened over walDir writes its
// new (empty) trailing segment, lock, and any subsequent WAL records into walDir only — leaving
// sourceDir untouched. The operator can delete walDir after the run to discard those files.
//
// The symlinked segments keep their original names, so the WAL's segment sequence stays contiguous and
// the newly created trailing segment follows on from the last existing one. Existing segments are only
// ever read (the attached noop compactor writes nothing), so the symlink targets in sourceDir are not
// modified.
//
// walDir must be empty or not yet exist, so a stale directory from a prior run cannot silently mix in.
//
// Expected error returns during normal operation:
//   - error if walDir already exists and is non-empty.
func stageWALDir(logger zerolog.Logger, sourceDir, walDir string) error {
	entries, err := os.ReadDir(walDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("could not read wal dir %s: %w", walDir, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("wal dir %s is not empty; use a fresh directory or remove it first", walDir)
	}
	if err := os.MkdirAll(walDir, 0755); err != nil {
		return fmt.Errorf("could not create wal dir %s: %w", walDir, err)
	}

	srcEntries, err := os.ReadDir(sourceDir)
	if err != nil {
		return fmt.Errorf("could not read source dir %s: %w", sourceDir, err)
	}

	linked := 0
	for _, entry := range srcEntries {
		// Skip the WAL lock file: the DiskWAL opened over walDir creates and manages its own lock there.
		if entry.Name() == ".lock" {
			continue
		}
		target, err := filepath.Abs(filepath.Join(sourceDir, entry.Name()))
		if err != nil {
			return fmt.Errorf("could not resolve absolute path for %s: %w", entry.Name(), err)
		}
		if err := os.Symlink(target, filepath.Join(walDir, entry.Name())); err != nil {
			return fmt.Errorf("could not symlink %s into wal dir: %w", entry.Name(), err)
		}
		linked++
	}

	logger.Info().
		Str("source_dir", sourceDir).
		Str("wal_dir", walDir).
		Int("linked_entries", linked).
		Msg("staged wal dir: replaying from symlinked checkpoints/segments; new WAL files land here " +
			"and can be deleted after the run")
	return nil
}
