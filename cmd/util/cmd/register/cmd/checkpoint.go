package cmd

import (
	"fmt"

	pStorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var flagPebbleCheckpointPath string

func init() {
	rootCmd.AddCommand(checkpointCmd)

	checkpointCmd.Flags().StringVar(&flagPebbleCheckpointPath, "checkpoint-dir", "", "the path of the checkpoint")
	checkpointCmd.MarkFlagRequired("checkpoint-dir")
}

var checkpointCmd = &cobra.Command{
	Use:   "checkpoint",
	Short: "create a pebble checkpoint",
	Run: func(cmd *cobra.Command, args []string) {
		err := runCheckpoint(flagRegisterDir, flagPebbleCheckpointPath)
		if err != nil {
			log.Error().Err(err).Msgf("fail to checkpoint")
		}
	},
}

func runCheckpoint(registerDir string, pebbleCheckpointDir string) error {
	log.Info().Msgf("importing checkpoint %v from dir %v", pebbleCheckpointDir, registerDir)

	registers, pdb, err := pStorage.NewBootstrappedRegistersWithPath(registerDir)
	if err != nil {
		return fmt.Errorf("could not create registers storage: %w", err)
	}

	defer func() {
		pdb.Close()
	}()

	err = registers.Checkpoint(pebbleCheckpointDir)
	if err != nil {
		return err
	}

	log.Info().Msgf("successfully created checkpoint at %v", pebbleCheckpointDir)
	return nil
}
