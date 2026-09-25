// Package import_compact_execution_state_to_storehouse provides the
// import-compact-execution-state-to-storehouse utility command.
//
// It bootstraps an empty storehouse register store (Pebble) from a compacted execution
// state: the single-trie V6 checkpoint produced by compact-execution-state, whose root
// hash equals the state commitment of the last finalized and executed block. This lets an
// execution node switch to payloadless (storehouse) execution without re-indexing every
// historical block: the register store starts at the compacted block instead of the spork
// root, so the node only needs to index forward from there.
package import_compact_execution_state_to_storehouse

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	esbootstrap "github.com/onflow/flow-go/engine/execution/state/bootstrap"
	"github.com/onflow/flow-go/ledger"
	flowWAL "github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	pebblestorage "github.com/onflow/flow-go/storage/pebble"
)

var (
	flagDatadir                     string
	flagExecutionStateDir           string
	flagRegisterDir                 string
	flagImportCheckpointWorkerCount int
	flagBootstrapMode               string
	flagSSTableIngestWorkerCount    int
)

// Bootstrap modes of the import-compact-execution-state-to-storehouse command.
const (
	// bootstrapModeBatched writes the checkpoint's registers to the register store with
	// batched writes.
	bootstrapModeBatched = "batched"
	// bootstrapModeSSTableIngest writes the checkpoint's registers to sstables and ingests
	// them into the register store.
	bootstrapModeSSTableIngest = "sstable-ingest"
)

var Cmd = &cobra.Command{
	Use:   "import-compact-execution-state-to-storehouse",
	Short: "Bootstrap an empty storehouse register store from a compacted execution state",
	Long: `import-compact-execution-state-to-storehouse bootstraps an empty storehouse register
store from a compacted execution state created by compact-execution-state.

The command:
  1. Resolves the last finalized and executed block and its state commitment C.
  2. Verifies the last checkpoint of the execution state is a compacted single-trie
     checkpoint whose root hash equals C.
  3. Verifies the register store directory does not exist or is empty, i.e. the register
     store has not been bootstrapped yet.
  4. Imports the checkpoint registers into the register store with --bootstrap-mode,
     setting the register store's first and latest heights to the resolved block height.

The batched bootstrap mode ('batched', the default) writes the registers with batched
writes, using --import-checkpoint-worker-count workers. The sstable bootstrap mode
('sstable-ingest') writes them to sstables and ingests those into pebble instead, which
avoids rewriting the register store through the memtables and compactions. It needs enough
free space in --register-dir for a temporary copy of the register data, and enough memory
per --sstable-ingest-worker-count worker to sort the largest of its buckets (about 1/256 of
the register data).`,
	RunE: runE,
}

// init registers the import-compact-execution-state-to-storehouse command flags.
func init() {
	common.InitDataDirFlag(Cmd, &flagDatadir)
	_ = Cmd.MarkFlagRequired("datadir")

	Cmd.Flags().StringVar(&flagExecutionStateDir, "execution-state-dir", "/var/flow/data/execution",
		"directory containing the execution state WAL and checkpoints")
	_ = Cmd.MarkFlagRequired("execution-state-dir")

	Cmd.Flags().StringVar(&flagRegisterDir, "register-dir", "/var/flow/data/register",
		"directory of the storehouse register store (Pebble) to bootstrap")
	_ = Cmd.MarkFlagRequired("register-dir")

	Cmd.Flags().IntVar(&flagImportCheckpointWorkerCount, "import-checkpoint-worker-count", 10,
		"number of workers to import checkpoint file during bootstrap ('batched' bootstrap mode)")

	Cmd.Flags().IntVar(&flagSSTableIngestWorkerCount, "sstable-ingest-worker-count", 4,
		"number of workers converting, sorting and writing the register data in the 'sstable-ingest' bootstrap "+
			"mode (the checkpoint's part files are read with twice as many workers); peak memory is roughly this "+
			"many times the larger of the target sstable size (128MB) and the largest bucket (about 1/256 of the "+
			"register data)")

	Cmd.Flags().StringVar(&flagBootstrapMode, "bootstrap-mode", bootstrapModeBatched,
		fmt.Sprintf("how to write the checkpoint registers to the register store: %q writes them with batched writes, "+
			"%q writes them to sstables and ingests those into pebble (needs free space in --register-dir and enough "+
			"memory to sort about 1/256 of the register data)", bootstrapModeBatched, bootstrapModeSSTableIngest))
}

// runE implements the import-compact-execution-state-to-storehouse command.
func runE(*cobra.Command, []string) error {
	log.Info().
		Str("datadir", flagDatadir).
		Str("execution-state-dir", flagExecutionStateDir).
		Str("register-dir", flagRegisterDir).
		Str("bootstrap-mode", flagBootstrapMode).
		Msg("starting import-compact-execution-state-to-storehouse")

	switch flagBootstrapMode {
	case bootstrapModeBatched, bootstrapModeSSTableIngest:
	default:
		return fmt.Errorf("invalid bootstrap mode %q, supported modes are %q and %q",
			flagBootstrapMode, bootstrapModeBatched, bootstrapModeSSTableIngest)
	}

	// The register store must not have been bootstrapped yet: importing a checkpoint into a
	// store whose heights are already populated would corrupt its height tracker.
	if err := ensureDirEmpty(flagRegisterDir); err != nil {
		return err
	}

	// ── Step 1: resolve the last finalized and executed block ─────────────────
	lockManager := storage.MakeSingletonLockManager()

	var (
		blockID flow.Identifier
		height  uint64
		commit  flow.StateCommitment
	)

	err := common.WithStorage(flagDatadir, func(db storage.DB) error {
		storages := common.InitStorages(db)
		state, err := common.OpenProtocolState(lockManager, db, storages)
		if err != nil {
			return fmt.Errorf("cannot open protocol state: %w", err)
		}

		blockID, height, commit, err = common.GetLastFinalizedAndExecutedBlock(state, db, storages.Headers, storages.Commits)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	log.Info().
		Hex("block-id", blockID[:]).
		Uint64("height", height).
		Hex("state-commitment", commit[:]).
		Msg("resolved last finalized and executed block")

	// ── Step 2: verify the last checkpoint is the matching compacted checkpoint ─
	_, last, err := flowWAL.ListCheckpoints(flagExecutionStateDir)
	if err != nil {
		return fmt.Errorf("cannot list checkpoints in %s: %w", flagExecutionStateDir, err)
	}
	if last < 0 {
		return fmt.Errorf("no checkpoint found in %s; run compact-execution-state first", flagExecutionStateDir)
	}

	checkpointName := flowWAL.NumberToFilename(last)
	rootHash := ledger.RootHash(commit)

	err = flowWAL.CheckpointHasSingleRootHash(log.Logger, flagExecutionStateDir, checkpointName, rootHash)
	if err != nil {
		return fmt.Errorf("last checkpoint %s is not a compacted single-trie checkpoint of the last finalized and executed block: %w",
			checkpointName, err)
	}

	log.Info().
		Str("checkpoint", checkpointName).
		Hex("root-hash", rootHash[:]).
		Msg("verified compacted single-trie checkpoint")

	// ── Step 3: bootstrap the register store from the checkpoint ──────────────
	pebbleDB, err := pebblestorage.OpenRegisterPebbleDB(log.Logger, flagRegisterDir)
	if err != nil {
		return fmt.Errorf("cannot open register store db at %s: %w", flagRegisterDir, err)
	}
	defer func() {
		if closeErr := pebbleDB.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("cannot close register store db")
		}
	}()

	bootstrapped, err := pebblestorage.IsBootstrapped(pebbleDB)
	if err != nil {
		return fmt.Errorf("cannot check whether register store is bootstrapped: %w", err)
	}
	if bootstrapped {
		return fmt.Errorf("register store at %s is already bootstrapped", flagRegisterDir)
	}

	checkpointFile := filepath.Join(flagExecutionStateDir, checkpointName)
	switch flagBootstrapMode {
	case bootstrapModeBatched:
		err = esbootstrap.ImportRegistersFromCheckpoint(log.Logger, checkpointFile, height, rootHash, pebbleDB, flagImportCheckpointWorkerCount)
	case bootstrapModeSSTableIngest:
		err = esbootstrap.ImportRegistersFromCheckpointSSTables(log.Logger, checkpointFile, height, rootHash, flagRegisterDir, flagSSTableIngestWorkerCount, pebbleDB)
	}
	if err != nil {
		return fmt.Errorf("cannot import registers from checkpoint %s: %w", checkpointFile, err)
	}

	log.Info().
		Str("checkpoint", checkpointName).
		Uint64("height", height).
		Str("bootstrap-mode", flagBootstrapMode).
		Msg("register store bootstrapped from compacted execution state")

	return nil
}

// ensureDirEmpty returns an error if dir exists and contains any entry. A non-existent or
// empty directory means the register store has not been bootstrapped yet.
//
// The returned error is not benign: importing a checkpoint requires the register store to
// not have been bootstrapped.
func ensureDirEmpty(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("cannot read register store directory %s: %w", dir, err)
	}

	if len(entries) > 0 {
		// The directory is non-empty in one of two situations, and the caller cannot
		// recover by simply re-running this command in either of them:
		//  - the register store was already bootstrapped, so importing a checkpoint would
		//    corrupt its height tracker;
		//  - a previous import failed partway. Both bootstrap modes persist the register
		//    store heights only after all registers have been written, so a partial import
		//    leaves registers on disk with unset heights.
		// Deleting the directory is required before retrying.
		return fmt.Errorf("register store directory %s is not empty (%d entries): "+
			"it either contains a bootstrapped register store or a partially imported store "+
			"left behind by a failed import; delete the directory before retrying",
			dir, len(entries))
	}

	return nil
}
