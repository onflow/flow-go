// Package compact_execution_state provides the compact-execution-state utility command.
// It produces a "compacted sealed state": a single-trie V6 checkpoint whose root hash
// matches the last sealed and executed block's state commitment, paired with a WAL that
// is trimmed to end at exactly that root hash.  This compact form lets an execution node
// bootstrap its register store (Storehouse) from a much smaller on-disk footprint than
// the full WAL history.
package compact_execution_state

import (
	"errors"
	"fmt"
	"math"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	utilledger "github.com/onflow/flow-go/cmd/util/ledger/util"
	exestate "github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	flowWAL "github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/ledger/factory"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	storagepebble "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/storage/store"
	utilsio "github.com/onflow/flow-go/utils/io"
)

var (
	flagDatadir           string
	flagExecutionStateDir string
	flagChunkDataPackDir  string
	flagBackupDir         string
)

var Cmd = &cobra.Command{
	Use:   "compact-execution-state",
	Short: "Compact the execution state to a single-trie checkpoint at the last sealed executed block",
	Long: `compact-execution-state produces a "compacted sealed state" by:
  1. Resolving the last sealed and executed block and its state commitment C.
  2. Locating the WAL segment that contains the last occurrence of root hash C.
  3. Trimming the WAL to end at that root hash (backed-up segments are moved to --backup-dir).
     The WAL must be trimmed before checkpoint extraction because the ledger forest only
     retains a bounded number of recent tries; without trimming, C could be evicted while
     the full WAL is replayed.
  4. Extracting a single-trie V6 checkpoint from the trimmed WAL.
  5. Moving any pre-existing checkpoint at or beyond the trimmed WAL segment to --backup-dir.
  6. Moving the checkpoint into the execution-state directory, named after the WAL segment.
  7. Rolling back the highest-executed-block pointer to the sealed height.
	8. validating the compacted execution state: the highest finalized-and-executed block
		 reported by the execution state must match the sealed anchor, its commit must exist
		 in the protocol database, and its root hash must be present in the compacted ledger.`,
	RunE: runE,
}

// init registers the compact-execution-state command flags.
func init() {
	common.InitDataDirFlag(Cmd, &flagDatadir)
	_ = Cmd.MarkFlagRequired("datadir")

	Cmd.Flags().StringVar(&flagExecutionStateDir, "execution-state-dir", "/var/flow/data/execution",
		"directory containing the execution state WAL and checkpoints")
	_ = Cmd.MarkFlagRequired("execution-state-dir")

	Cmd.Flags().StringVar(&flagChunkDataPackDir, "chunk-data-pack-dir", "/var/flow/data/chunk_data_pack",
		"directory containing the chunk data pack database")
	_ = Cmd.MarkFlagRequired("chunk-data-pack-dir")

	Cmd.Flags().StringVar(&flagBackupDir, "backup-dir", "",
		"directory to receive trimmed WAL segments; must be empty or non-existent")
	_ = Cmd.MarkFlagRequired("backup-dir")
}

// runE implements the compact-execution-state command.
func runE(*cobra.Command, []string) error {
	lockManager := storage.MakeSingletonLockManager()

	log.Info().
		Str("datadir", flagDatadir).
		Str("execution-state-dir", flagExecutionStateDir).
		Str("chunk-data-pack-dir", flagChunkDataPackDir).
		Str("backup-dir", flagBackupDir).
		Msg("starting compact-execution-state")

	if flagBackupDir == flagExecutionStateDir {
		return fmt.Errorf("--backup-dir must differ from --execution-state-dir")
	}

	if err := utilsio.EnsureEmptyOrCreate(flagBackupDir); err != nil {
		return fmt.Errorf("--backup-dir validation failed: %w", err)
	}

	// ── Step 1: resolve anchor (last sealed and executed block ) ──────────────
	// The WAL is trimmed before extracting the checkpoint because ReadTrie replays
	// the WAL into an in-memory forest with bounded capacity. If the full, untrimmed
	// WAL were replayed, the target state commitment could be evicted before the trie
	// could be read.

	var sealedHeader *flow.Header
	var stateCommitment flow.StateCommitment

	err := common.WithStorage(flagDatadir, func(db storage.DB) error {
		storages := common.InitStorages(db)
		state, err := common.OpenProtocolState(lockManager, db, storages)
		if err != nil {
			return fmt.Errorf("cannot open protocol state: %w", err)
		}

		sealedHead, err := state.Sealed().Head()
		if err != nil {
			return fmt.Errorf("cannot get sealed head: %w", err)
		}

		root := state.Params().SealedRoot()

		// Binary-search for the highest sealed height that has been executed.
		// Executed blocks form a contiguous range [root, X]; we find X.
		lo, hi := root.Height, sealedHead.Height
		for lo <= hi {
			mid := lo + (hi-lo)/2
			header, err := state.AtHeight(mid).Head()
			if err != nil {
				return fmt.Errorf("cannot get header at height %d: %w", mid, err)
			}

			commit, err := storages.Commits.ByBlockID(header.ID())
			if err == nil {
				log.Debug().Uint64("height", mid).Uint64("lo", lo).Uint64("hi", hi).Msg("executed: searching higher")
				sealedHeader = header
				stateCommitment = commit
				lo = mid + 1
			} else if errors.Is(err, storage.ErrNotFound) {
				log.Debug().Uint64("height", mid).Uint64("lo", lo).Uint64("hi", hi).Msg("not executed: searching lower")
				hi = mid - 1
			} else {
				return fmt.Errorf("cannot check execution state at height %d: %w", mid, err)
			}
		}

		if sealedHeader == nil {
			return fmt.Errorf("no executed sealed block found between root height %d and sealed height %d",
				root.Height, sealedHead.Height)
		}

		return nil
	})
	if err != nil {
		return err
	}

	rootHash := ledger.RootHash(stateCommitment)

	log.Info().
		Uint64("sealed-height", sealedHeader.Height).
		Hex("state-commitment", stateCommitment[:]).
		Msg("resolved last sealed and executed block")

	// ── Step 2: locate WAL segment ────────────────────────────────────────────
	segment, offset, err := common.SearchRootHashBackward(
		log.Logger, rootHash, flagExecutionStateDir, common.DefaultWALFrom, common.DefaultWALTo)
	if err != nil {
		return fmt.Errorf("cannot locate root hash in WAL: %w", err)
	}

	log.Info().
		Int("segment", segment).
		Int64("offset", offset).
		Msg("found root hash in WAL")

	// ── Step 3: trim WAL ──────────────────────────────────────────────────────
	trimTmpDir, err := os.MkdirTemp(flagBackupDir, "compact-trim-*")
	if err != nil {
		return fmt.Errorf("cannot create trim temp dir: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(trimTmpDir); err != nil {
			log.Error().Err(err).Str("dir", trimTmpDir).Msg("cannot remove trim temp dir")
		}
	}()

	newSegmentFile, err := common.TrimWALSegmentToHash(log.Logger, flagExecutionStateDir, segment, offset, rootHash, trimTmpDir)
	if err != nil {
		return fmt.Errorf("cannot trim WAL segment %d: %w", segment, err)
	}

	if err = common.BackupAndReplaceWALSegment(segment, flagExecutionStateDir, flagBackupDir, newSegmentFile); err != nil {
		return fmt.Errorf("cannot backup and replace WAL segment %d: %w", segment, err)
	}

	log.Info().Int("segment", segment).Msg("WAL trimmed")

	// ── Step 4: extract single-trie checkpoint ────────────────────────────────
	// Keep the checkpoint temp directory on the same filesystem as the execution-state
	// directory so MoveCheckpointFiles can rename the V6 checkpoint parts atomically.
	checkpointTmpDir, err := os.MkdirTemp(flagExecutionStateDir, "compact-checkpoint-*")
	if err != nil {
		return fmt.Errorf("cannot create checkpoint temp dir: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(checkpointTmpDir); err != nil {
			log.Error().Err(err).Str("dir", checkpointTmpDir).Msg("cannot remove checkpoint temp dir")
		}
	}()

	const tmpCheckpointName = "checkpoint.tmp"

	t, err := utilledger.ReadTrie(flagExecutionStateDir, stateCommitment)
	if err != nil {
		return fmt.Errorf("cannot read trie from WAL at state commitment %x: %w", stateCommitment, err)
	}

	if err = flowWAL.StoreCheckpointV6Concurrently([]*trie.MTrie{t}, checkpointTmpDir, tmpCheckpointName, log.Logger); err != nil {
		return fmt.Errorf("cannot write checkpoint: %w", err)
	}

	log.Info().Msg("checkpoint extracted")

	// ── Step 5: move stale checkpoints to backup ──────────────────────────────
	// The trimmed WAL ends at segment `segment`; any pre-existing checkpoint at or
	// beyond this segment references state newer than the target block and is
	// inconsistent with the trimmed WAL, so it is moved to the backup directory.
	err = common.BackupCheckpointsFrom(log.Logger, flagExecutionStateDir, flagBackupDir, segment)
	if err != nil {
		return fmt.Errorf("cannot backup checkpoints newer than or equal to trimmed segment: %w", err)
	}

	// ── Step 6: name checkpoint after WAL segment ─────────────────────────────
	destName := flowWAL.NumberToFilename(segment)
	if err = common.MoveCheckpointFiles(checkpointTmpDir, tmpCheckpointName, flagExecutionStateDir, destName); err != nil {
		return fmt.Errorf("cannot move checkpoint to execution state dir: %w", err)
	}

	log.Info().Str("name", destName).Msg("checkpoint placed in execution state dir")

	// ── Step 7: roll back executed height and validate ─────────────────────
	err = common.WithStorage(flagDatadir, func(db storage.DB) error {
		storages := common.InitStorages(db)
		state, err := common.OpenProtocolState(lockManager, db, storages)
		if err != nil {
			return fmt.Errorf("cannot open protocol state: %w", err)
		}

		m := &metrics.NoopCollector{}

		transactionResults, err := store.NewTransactionResults(m, db, badger.DefaultCacheSize)
		if err != nil {
			return fmt.Errorf("cannot open transaction results store: %w", err)
		}

		myReceipts := store.NewMyExecutionReceipts(m, db, storages.Receipts)
		events := store.NewEvents(m, db)
		serviceEvents := store.NewServiceEvents(m, db)

		cdpPebbleDB, err := storagepebble.ShouldOpenDefaultPebbleDB(
			log.Logger.With().Str("pebbledb", "cdp").Logger(), flagChunkDataPackDir)
		if err != nil {
			return fmt.Errorf("cannot open chunk data pack DB: %w", err)
		}
		defer func() {
			if cerr := cdpPebbleDB.Close(); cerr != nil {
				log.Error().Err(cerr).Msg("cannot close chunk data pack DB")
			}
		}()
		cdpDB := pebbleimpl.ToDB(cdpPebbleDB)
		storedCDP := store.NewStoredChunkDataPacks(m, cdpDB, 1000)
		chunkDataPacks := store.NewChunkDataPacks(m, db, storedCDP, storages.Collections, 1000)

		var executedBlockID flow.Identifier
		err = operation.RetrieveExecutedBlock(db.Reader(), &executedBlockID)
		if err != nil {
			return fmt.Errorf("cannot retrieve executed block: %w", err)
		}
		executedHeader, err := storages.Headers.ByBlockID(executedBlockID)
		if err != nil {
			return fmt.Errorf("cannot retrieve executed header: %w", err)
		}

		if executedHeader.Height <= sealedHeader.Height {
			log.Info().
				Uint64("executed-height", executedHeader.Height).
				Uint64("sealed-height", sealedHeader.Height).
				Msg("executed height is already at or below sealed height; skipping execution-result rollback")
		} else {
			batch := db.NewBatch()
			defer batch.Close()

			chunkIDs, err := common.RemoveExecutionResultsFromHeight(
				batch, state, transactionResults, storages.Commits, storages.Results,
				myReceipts, events, serviceEvents, sealedHeader.Height+1)
			if err != nil {
				return fmt.Errorf("cannot remove execution results: %w", err)
			}

			if len(chunkIDs) > 0 {
				if _, err = chunkDataPacks.BatchRemove(chunkIDs, batch); err != nil {
					return fmt.Errorf("cannot remove chunk data packs: %w", err)
				}
			}

			if err = batch.Commit(); err != nil {
				return fmt.Errorf("cannot commit batch: %w", err)
			}

			if err = storages.Headers.RollbackExecutedBlock(sealedHeader); err != nil {
				return fmt.Errorf("cannot rollback executed block: %w", err)
			}

			log.Info().Uint64("height", sealedHeader.Height).Msg("executed height rolled back")
		}

		// ── validate compacted execution state ──────────────────────────────────
		// Replicating the execution node's startup wiring (LoadExecutionState and
		// LoadExecutionStateLedger in cmd/execution_builder.go), so that the block
		// reported by GetHighestFinalizedExecuted is exactly the block the node will
		// resume from after restarting on the compacted state.
		ledgerStorage, err := openValidationLedger(flagExecutionStateDir)
		if err != nil {
			return err
		}
		defer func() {
			<-ledgerStorage.Done()
		}()

		execState := exestate.NewExecutionState(
			ledgerStorage,
			storages.Commits,
			storages.Blocks,
			storages.Headers,
			chunkDataPacks,
			storages.Results,
			myReceipts,
			events,
			serviceEvents,
			transactionResults,
			db,
			func() (uint64, error) {
				final, err := state.Final().Head()
				if err != nil {
					return 0, err
				}
				return final.Height, nil
			},
			trace.NewNoopTracer(),
			nil,   // register store (storehouse) is not used by compaction
			false, // storehouse disabled
			lockManager,
		)

		if err := validateCompactedState(execState, ledgerStorage, storages.Headers, storages.Commits, sealedHeader, stateCommitment); err != nil {
			return err
		}

		log.Info().
			Uint64("height", sealedHeader.Height).
			Hex("state-commitment", stateCommitment[:]).
			Msg("validated compacted execution state")

		return nil
	})
	if err != nil {
		return err
	}

	log.Info().
		Uint64("sealed-height", sealedHeader.Height).
		Str("checkpoint", destName).
		Msg("compact-execution-state complete")

	return nil
}

// openValidationLedger opens a local ledger over the trimmed WAL and extracted
// checkpoint in execStateDir, using the same factory the execution node uses at
// startup (ledger/factory.NewLedger in LoadExecutionStateLedger).
//
// CheckpointDistance is set to the maximum value to prevent the compactor from
// creating new checkpoints during this read-only validation, mirroring other
// read-only execution-state utilities. The caller must wait for the returned ledger
// to become Ready before use and call Done afterwards to release the WAL file lock.
//
// No error returns are expected during normal operation.
func openValidationLedger(execStateDir string) (ledger.Ledger, error) {
	const (
		checkpointDistance = math.MaxInt
		checkpointsToKeep  = 1
	)

	ledgerStorage, err := factory.NewLedger(factory.Config{
		Triedir:            execStateDir,
		MTrieCacheSize:     ledger.DefaultMTrieCacheSize,
		CheckpointDistance: checkpointDistance,
		CheckpointsToKeep:  checkpointsToKeep,
		WALMetrics:         &metrics.NoopCollector{},
		LedgerMetrics:      &metrics.NoopCollector{},
		Logger:             log.Logger,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot open local ledger over %s: %w", execStateDir, err)
	}

	// wait for the WAL replay (checkpoint loading) to complete
	<-ledgerStorage.Ready()

	return ledgerStorage, nil
}

// validateCompactedState verifies that the compacted execution state is consistent
// with the protocol state and the ledger built over the trimmed WAL and extracted
// checkpoint:
//   - the highest finalized-and-executed block reported by execState (the block the
//     execution node resumes from after restart) matches the resolved anchor block;
//   - the anchor block's state commitment exists in the commits store;
//   - the ledger contains the corresponding root hash.
//
// No error returns are expected during normal operation.
func validateCompactedState(
	execState exestate.ExecutionState,
	ledgerStorage ledger.Ledger,
	headers storage.Headers,
	commits storage.Commits,
	sealedHeader *flow.Header,
	stateCommitment flow.StateCommitment,
) error {
	highest, err := execState.GetHighestFinalizedExecuted()
	if err != nil {
		return fmt.Errorf("cannot get highest finalized and executed block: %w", err)
	}

	if highest != sealedHeader.Height {
		return fmt.Errorf("highest finalized and executed height %d does not match the resolved sealed height %d",
			highest, sealedHeader.Height)
	}

	blockID, err := headers.BlockIDByHeight(highest)
	if err != nil {
		return fmt.Errorf("cannot get block ID at height %d: %w", highest, err)
	}

	commit, err := commits.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("cannot get state commitment of the highest executed block %v: %w", blockID, err)
	}

	if commit != stateCommitment {
		return fmt.Errorf("state commitment %x of the highest executed block does not match the resolved state commitment %x",
			commit, stateCommitment)
	}

	if !ledgerStorage.HasState(ledger.State(commit)) {
		return fmt.Errorf("root hash %x not found in the compacted ledger", ledger.RootHash(commit))
	}

	return nil
}
