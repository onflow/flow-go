package cmd

import (
	"fmt"

	pStorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var flagPruneHeight uint64

func init() {
	rootCmd.AddCommand(pruneCmd)

	pruneCmd.Flags().Uint64VarP(&flagPruneHeight, "height", "", 0, "the height to be pruned")
	pruneCmd.MarkFlagRequired("height")
}

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "prune by height",
	Run: func(cmd *cobra.Command, args []string) {
		err := runPrune(flagRegisterDir, flagPruneHeight)
		if err != nil {
			log.Error().Err(err).Msgf("fail to prune")
		}
	},
}

func runPrune(dir string, pruneHeight uint64) error {
	log.Info().Msgf("pruning by height %v from dir %v", pruneHeight, dir)

	registers, pdb, err := pStorage.NewBootstrappedRegistersWithPath(dir)
	if err != nil {
		return fmt.Errorf("could not create registers storage: %w", err)
	}

	defer func() {
		pdb.Close()
	}()

	err = registers.PruneByHeight(pruneHeight)
	if err != nil {
		return err
	}

	log.Info().Msgf("successfully pruned by height %v", pruneHeight)
	return nil
}
