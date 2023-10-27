package cmd

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/storage/pebble"
	pStorage "github.com/onflow/flow-go/storage/pebble"
)

var flagCheckpointPath string
var flagCheckpointHeight uint64
var flagWorkerCount int

func init() {
	rootCmd.AddCommand(importCmd)

	importCmd.Flags().StringVar(&flagCheckpointPath, "checkpoint", "", "the path of the checkpoint")
	_ = importCmd.MarkFlagRequired("checkpoint")
	importCmd.Flags().Uint64Var(&flagCheckpointHeight, "height", 0, "the height of the checkpoint")
	_ = importCmd.MarkFlagRequired("height")
	importCmd.Flags().IntVar(&flagWorkerCount, "worker-count", 10, "the worker count for importing checkpoint")
}

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "import a checkpoint",
	Run: func(cmd *cobra.Command, args []string) {
		err := runImport(flagRegisterDir, flagCheckpointPath, flagCheckpointHeight, flagWorkerCount)
		if err != nil {
			log.Error().Err(err).Msgf("fail to import")
		}
	},
}

func runImport(dir string, checkpoint string, checkpointHeight uint64, workerCount int) error {
	log.Info().Msgf("importing checkpoint %v from dir %v", checkpoint, dir)
	if checkpointHeight == 0 {
		return fmt.Errorf("height should not be 0")
	}

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

	err = bootstrapper.IndexCheckpointFile(context.Background(), workerCount)
	if err != nil {
		return fmt.Errorf("could not index checkpoint file: %w", err)
	}

	log.Info().Msgf("successfully imported checkpoint %v", checkpoint)
	return nil
}
