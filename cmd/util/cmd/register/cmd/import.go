package cmd

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/storage/pebble"
	pStorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var flagCheckpointPath string
var flagCheckpointHeight uint64

func init() {
	rootCmd.AddCommand(importCmd)

	importCmd.Flags().StringVar(&flagCheckpointPath, "checkpoint", "", "the path of the checkpoint")
	importCmd.Flags().Uint64Var(&flagCheckpointHeight, "height", 0, "the height of the checkpoint")
}

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "import a checkpoint",
	Run: func(cmd *cobra.Command, args []string) {
		err := runImport(flagRegisterDir, flagCheckpointPath, flagCheckpointHeight)
		if err != nil {
			log.Error().Err(err).Msgf("fail to import")
		}
	},
}

func runImport(dir string, checkpoint string, checkpointHeight uint64) error {
	log.Info().Msgf("importing checkpoint %v from dir %v", checkpoint, dir)

	pdb, err := pStorage.OpenRegisterPebbleDB(dir)
	if err != nil {
		return err
	}

	defer func() {
		pdb.Close()
	}()

	bootstrapper, err := pebble.NewRegisterBootstrap(pdb, checkpoint, checkpointHeight, log.Logger)
	if err != nil {
		return err
	}

	err = bootstrapper.IndexCheckpointFile(context.Background())
	if err != nil {
		return fmt.Errorf("could not index checkpoint file: %w", err)
	}

	log.Info().Msgf("successfully imported checkpoint %v", checkpoint)
	return nil
}
