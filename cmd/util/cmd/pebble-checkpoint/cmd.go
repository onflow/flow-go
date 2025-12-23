package cmd

import (
	"fmt"

	"github.com/cockroachdb/pebble/v2"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	flowpebble "github.com/onflow/flow-go/storage/pebble"
)

var (
	flagPebbleDir string
	flagOutput    string
	flagDBType    string
)

// Note: Although checkpoint is fast to create, it is not free. When creating a checkpoint, the
// underlying pebble sstables are hard-linked to the checkpoint directory, which means the compaction
// process will not be able to delete the sstables until the checkpoint is deleted. This can lead to
// increased disk usage if checkpoints are created frequently without being cleaned up.
var Cmd = &cobra.Command{
	Use:   "pebble-checkpoint",
	Short: "Create a checkpoint from a Pebble database",
	RunE:  runE,
}

func init() {
	Cmd.Flags().StringVar(&flagPebbleDir, "pebbledir", "",
		"directory containing the Pebble database")
	_ = Cmd.MarkFlagRequired("pebbledir")

	Cmd.Flags().StringVar(&flagOutput, "output", "",
		"output directory for the checkpoint")
	_ = Cmd.MarkFlagRequired("output")

	Cmd.Flags().StringVar(&flagDBType, "db-type", "register",
		"type of pebble database: 'register' (uses MVCCComparer) or 'protocol' (uses default comparer)")
}

func runE(*cobra.Command, []string) error {
	log.Info().Msgf("creating checkpoint from Pebble database at %v to %v", flagPebbleDir, flagOutput)

	var db *pebble.DB
	var err error

	switch flagDBType {
	case "register":
		db, err = flowpebble.OpenRegisterPebbleDB(log.Logger, flagPebbleDir)
	case "protocol":
		db, err = flowpebble.ShouldOpenDefaultPebbleDB(log.Logger, flagPebbleDir)
	default:
		return fmt.Errorf("unknown db-type %q, must be 'register' or 'protocol'", flagDBType)
	}

	if err != nil {
		return fmt.Errorf("failed to initialize Pebble database %v: %w", flagPebbleDir, err)
	}
	defer db.Close()

	// Create checkpoint
	err = db.Checkpoint(flagOutput)
	if err != nil {
		return fmt.Errorf("failed to create checkpoint %v: %w", flagOutput, err)
	}

	log.Info().Msgf("successfully created checkpoint at %v", flagOutput)
	return nil
}
