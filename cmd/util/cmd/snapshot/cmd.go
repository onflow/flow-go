package snapshot

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"

	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	badgerstate "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/storage"
)

var (
	flagDatadir string
	flagHeight  uint64
)

// This command can be used to retrieve a snapshot of the protocol state at any finalized height.
// The resulting snapshot file can then be used to bootstrap another node's protocol state.
// This can be useful for recovering a node which is very far behind, or has a corrupted database
// that cannot be recovered otherwise.
//
// The recommended usage is to use a height which is just before the most recent epoch transition.
// This way the node will have a root block below the current epoch, and will sync all blocks
// from the current epoch.

var Cmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Retrieves a protocol state snapshot from the database, which can be used to instantiate another node",
	RunE:  runE,
}

func init() {

	common.InitDataDirFlag(Cmd, &flagDatadir)
	_ = Cmd.MarkFlagRequired("datadir")

	Cmd.Flags().Uint64Var(&flagHeight, "height", 0, "the height of the snapshot to retrieve")
	_ = Cmd.MarkFlagRequired("height")
}

func runE(*cobra.Command, []string) error {
	lockManager := storage.MakeSingletonLockManager()

	return common.WithStorage(flagDatadir, func(db storage.DB) error {
		chainID, err := badgerstate.GetChainIDFromLatestFinalizedHeader(db)
		if err != nil {
			return err
		}
		storages := common.InitStorages(db, chainID)
		state, err := common.OpenProtocolState(lockManager, db, storages)
		if err != nil {
			return fmt.Errorf("could not open protocol state: %w", err)
		}

		log := log.With().Uint64("block_height", flagHeight).Logger()

		snap := state.AtHeight(flagHeight)
		encoded, err := convert.SnapshotToBytes(snap)
		if err != nil {
			return fmt.Errorf("failed to encode snapshot: %w", err)
		}

		dir := filepath.Join(".", "root-protocol-state-snapshot.json")

		log.Info().Msgf("going to write snapshot to %s", dir)
		err = os.WriteFile(dir, encoded, 0600)
		if err != nil {
			return fmt.Errorf("failed to write snapshot: %w", err)
		}

		log.Info().Msgf("successfully wrote snapshot to %s", dir)
		return nil
	})
}
