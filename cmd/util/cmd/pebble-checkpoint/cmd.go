package cmd

import (
	"fmt"

	"github.com/onflow/flow-go/storage/pebble"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	flagPebbleDir string
	flagOutput    string
)

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
}

func runE(*cobra.Command, []string) error {
	log.Info().Msgf("creating checkpoint from Pebble database at %v to %v", flagPebbleDir, flagOutput)

	// Initialize Pebble DB
	db, err := pebble.MustOpenDefaultPebbleDB(log.Logger, flagPebbleDir)
	if err != nil {
		return fmt.Errorf("failed to initialize Pebble database %v: %w", flagPebbleDir, err)
	}

	// Create checkpoint
	err = db.Checkpoint(flagOutput)
	if err != nil {
		return fmt.Errorf("failed to create checkpoint %v: %w", flagOutput, err)
	}

	log.Info().Msgf("successfully created checkpoint at %v", flagOutput)
	return nil
}
