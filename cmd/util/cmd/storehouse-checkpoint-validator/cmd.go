package storehouse_checkpoint_validator

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/engine/execution/storehouse"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	pebblestorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/storage/store"
)

var (
	flagPebbleDir     string
	flagDataDir       string
	flagCheckpointDir string
	flagBlockHeight   uint64
	flagWorkerCount   int
)

var Cmd = &cobra.Command{
	Use:   "storehouse-checkpoint-validator",
	Short: "Validate registers in storehouse against checkpoint file",
	Long: `Validate registers in storehouse against checkpoint file.
This command validates that all registers in the checkpoint file match the registers stored in the pebble database.
The checkpoint directory must contain a root.checkpoint file with a single trie.`,
	RunE: runE,
}

func init() {
	Cmd.Flags().StringVar(&flagPebbleDir, "pebble-dir", "",
		"directory containing the Pebble database with register store")
	_ = Cmd.MarkFlagRequired("pebble-dir")

	Cmd.Flags().StringVar(&flagDataDir, "datadir", "/var/flow/data/protocol",
		"directory containing the protocol database")

	Cmd.Flags().StringVar(&flagCheckpointDir, "checkpoint-dir", "",
		"directory containing the checkpoint file (must have root.checkpoint)")
	_ = Cmd.MarkFlagRequired("checkpoint-dir")

	Cmd.Flags().Uint64Var(&flagBlockHeight, "block-height", 0,
		"block height to validate against")
	_ = Cmd.MarkFlagRequired("block-height")

	Cmd.Flags().IntVar(&flagWorkerCount, "worker-count", 4,
		"number of worker goroutines for validation")
}

func runE(*cobra.Command, []string) error {
	log.Info().
		Str("pebble-dir", flagPebbleDir).
		Str("datadir", flagDataDir).
		Str("checkpoint-dir", flagCheckpointDir).
		Uint64("block-height", flagBlockHeight).
		Int("worker-count", flagWorkerCount).
		Msg("starting storehouse checkpoint validation")

	// Open pebble DB for register store
	// Note: Register store uses a special comparer, so we use OpenRegisterPebbleDB
	pebbleDB, err := pebblestorage.OpenRegisterPebbleDB(log.Logger, flagPebbleDir)
	if err != nil {
		return fmt.Errorf("failed to open pebble database at %s: %w", flagPebbleDir, err)
	}
	defer func() {
		if closeErr := pebbleDB.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("failed to close pebble database")
		}
	}()

	// Initialize register store
	// Using PruningDisabled to ensure we can access all registers
	registerStore, err := pebblestorage.NewRegisters(pebbleDB, pebblestorage.PruningDisabled)
	if err != nil {
		return fmt.Errorf("failed to initialize register store: %w", err)
	}

	// Open protocol database from datadir
	protocolPebbleDB, err := pebblestorage.ShouldOpenDefaultPebbleDB(log.Logger, flagDataDir)
	if err != nil {
		return fmt.Errorf("failed to open protocol database at %s: %w", flagDataDir, err)
	}
	defer func() {
		if closeErr := protocolPebbleDB.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("failed to close protocol database")
		}
	}()

	protocolDB := pebbleimpl.ToDB(protocolPebbleDB)

	// Initialize storage components
	metricsCollector := &metrics.NoopCollector{}
	storages := store.InitAll(metricsCollector, protocolDB)

	// Validate checkpoint
	ctx := context.Background()
	err = storehouse.ValidateWithCheckpoint(
		log.Logger,
		ctx,
		registerStore,
		storages.Results,
		storages.Headers,
		flagCheckpointDir,
		flagBlockHeight,
		flagWorkerCount,
	)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	log.Info().Msg("validation completed successfully")
	return nil
}
