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
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	pebblestorage "github.com/onflow/flow-go/storage/pebble"
)

var (
	flagDatadir           string
	flagExecutionStateDir string
	flagRegisterDir       string
)

// importWorkerCount is the number of concurrent workers used to index the checkpoint
// registers into the register store.
const importWorkerCount = 16

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
  4. Imports the checkpoint registers into the register store with 16 workers, setting the
     register store's first and latest heights to the resolved block height.`,
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
}

// runE implements the import-compact-execution-state-to-storehouse command.
func runE(*cobra.Command, []string) error {
	log.Info().
		Str("datadir", flagDatadir).
		Str("execution-state-dir", flagExecutionStateDir).
		Str("register-dir", flagRegisterDir).
		Msg("starting import-compact-execution-state-to-storehouse")

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

		blockID, height, commit, err = lastFinalizedAndExecutedBlock(state, db, storages.Headers, storages.Commits)
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
	err = esbootstrap.ImportRegistersFromCheckpoint(log.Logger, checkpointFile, height, rootHash, pebbleDB, importWorkerCount)
	if err != nil {
		return fmt.Errorf("cannot import registers from checkpoint %s: %w", checkpointFile, err)
	}

	log.Info().
		Str("checkpoint", checkpointName).
		Uint64("height", height).
		Int("worker-count", importWorkerCount).
		Msg("register store bootstrapped from compacted execution state")

	return nil
}

// lastFinalizedAndExecutedBlock returns the block ID, height and state commitment of the
// highest finalized and executed block. It replicates the ledger-backed
// [state.ExecutionState.GetHighestFinalizedExecuted] logic: the executed block pointer is
// capped by the finalized head, and the state commitment of the resulting block must be
// present in the commits store.
//
// No error returns are expected during normal operation.
func lastFinalizedAndExecutedBlock(
	state protocol.State,
	db storage.DB,
	headers storage.Headers,
	commits storage.Commits,
) (flow.Identifier, uint64, flow.StateCommitment, error) {
	finalized, err := state.Final().Head()
	if err != nil {
		return flow.ZeroID, 0, flow.DummyStateCommitment, fmt.Errorf("cannot get finalized head: %w", err)
	}

	var executedBlockID flow.Identifier
	err = operation.RetrieveExecutedBlock(db.Reader(), &executedBlockID)
	if err != nil {
		return flow.ZeroID, 0, flow.DummyStateCommitment, fmt.Errorf("cannot retrieve executed block: %w", err)
	}

	executedHeader, err := headers.ByBlockID(executedBlockID)
	if err != nil {
		return flow.ZeroID, 0, flow.DummyStateCommitment, fmt.Errorf("cannot retrieve executed header %v: %w", executedBlockID, err)
	}

	// the highest finalized and executed height is the min of the two
	highest := min(finalized.Height, executedHeader.Height)

	blockID, err := headers.BlockIDByHeight(highest)
	if err != nil {
		return flow.ZeroID, 0, flow.DummyStateCommitment, fmt.Errorf("cannot get block ID by height %d: %w", highest, err)
	}

	commit, err := commits.ByBlockID(blockID)
	if err != nil {
		return flow.ZeroID, 0, flow.DummyStateCommitment, fmt.Errorf("cannot get state commitment for block %v (height %d): %w", blockID, highest, err)
	}

	return blockID, highest, commit, nil
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
		return fmt.Errorf("register store directory %s is not empty (%d entries); register store must not be bootstrapped",
			dir, len(entries))
	}

	return nil
}
