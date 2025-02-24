package cmd

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	pebblestorage "github.com/onflow/flow-go/storage/pebble"
)

var flagTargetDir string

func init() {
	rootCmd.AddCommand(pebbleCheckpointCmd)

	pebbleCheckpointCmd.Flags().StringVar(&flagTargetDir, "target-dir", "", "the target directory to checkpoint")
}

var pebbleCheckpointCmd = &cobra.Command{
	Use:   "pebble-checkpoint",
	Short: "create a pebble checkpoint of the target directory",
	Run: func(cmd *cobra.Command, args []string) {
		err := CreatePebbleCheckpoint(flagTargetDir, flagPebbleDir)
		if err != nil {
			log.Error().Err(err).Msg("could not create pebble checkpoint")
		}
	},
}

func CreatePebbleCheckpoint(targetDir string, pebbleDir string) error {
	log.Info().Msgf("creating pebble checkpoint of %s to %s", pebbleDir, targetDir)

	db, err := pebblestorage.MustOpenDefaultPebbleDB(pebbleDir)
	if err != nil {
		return fmt.Errorf("could not open pebble db: %w", err)
	}

	err = db.Checkpoint(targetDir)
	if err != nil {
		return fmt.Errorf("could not create pebble checkpoint: %w", err)
	}

	log.Info().Msgf("creating pebble checkpoint of %s to %s", pebbleDir, targetDir)
	return nil
}
