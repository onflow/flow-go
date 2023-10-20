package cmd

import (
	"fmt"

	pStorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var flagPruneHeight uint64

func init() {
	rootCmd.AddCommand(registersCmd)

	registersCmd.Flags().Uint64VarP(&flagPruneHeight, "height", "", 0, "the height to be pruned")
}

var registersCmd = &cobra.Command{
	Use:   "prune",
	Short: "prune by height",
	Run: func(cmd *cobra.Command, args []string) {
		err := run(flagRegisterDir, flagPruneHeight)
		if err != nil {
			log.Error().Err(err).Msgf("fail to prune")
		}
	},
}

func run(dir string, pruneHeight uint64) error {
	pdb, err := pStorage.OpenRegisterPebbleDB(dir)
	if err != nil {
		return err
	}

	bootstrapped, err := pStorage.IsBootstrapped(pdb)
	if err != nil {
		return fmt.Errorf("could not check if registers db is bootstrapped: %w", err)
	}

	if !bootstrapped {
		return fmt.Errorf("registers db is not bootstrapped")
	}

	registers, err := pStorage.NewRegisters(pdb)
	if err != nil {
		return fmt.Errorf("could not create registers storage: %w", err)
	}

	log.Info().Msgf("pruning by height %v", pruneHeight)
	err = registers.PruneByHeight(pruneHeight)
	if err != nil {
		return err
	}

	log.Info().Msgf("successfully pruned by height %v", pruneHeight)
	return nil
}
