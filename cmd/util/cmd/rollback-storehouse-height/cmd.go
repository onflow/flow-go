package rollback_storehouse_height

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	pebbleds "github.com/ipfs/go-ds-pebble"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/engine/execution/storehouse"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	pebblestorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/storage/store"
)

var (
	flagRegisterDir      string
	flagDataDir          string
	flagExecutionDataDir string
	flagHeight           uint64
)

// Cmd rolls the storehouse register store back to a target height.
//
// It removes every register update stored above the target height and lowers the register
// store's latest indexed height to exactly the target height, using the block's execution
// data to identify which registers were updated at each rolled-back height.
//
// This is an offline operation: the execution node must be stopped so that no process holds
// the register DB open. It must be run BEFORE `rollback-executed-height`, because it resolves
// each height's execution data through the execution result, and `rollback-executed-height`
// removes those results.
var Cmd = &cobra.Command{
	Use:   "rollback-storehouse-height",
	Short: "Rollback the storehouse register store to a target height",
	Long: `Rollback the storehouse register store to a target height.

Removes every register update stored strictly above the target height, so that after it
completes the register store's last finalized-and-executed height is exactly the target
height. The registers updated at each rolled-back height are determined from the block's
execution data.

The node must be stopped (the register DB is opened exclusively). Run this BEFORE
rollback-executed-height, which removes the execution results this command depends on.`,
	RunE: runE,
}

func init() {
	Cmd.Flags().StringVar(&flagRegisterDir, "register-dir", "",
		"directory containing the Pebble register store")
	_ = Cmd.MarkFlagRequired("register-dir")

	Cmd.Flags().StringVar(&flagDataDir, "datadir", "/var/flow/data/protocol",
		"directory containing the protocol database")

	Cmd.Flags().StringVar(&flagExecutionDataDir, "execution-data-dir", "/var/flow/data/execution_data",
		"directory containing the execution data blobstore")

	Cmd.Flags().Uint64Var(&flagHeight, "height", 0,
		"the target height to roll the register store back to")
	_ = Cmd.MarkFlagRequired("height")
}

func runE(*cobra.Command, []string) error {
	log.Info().
		Str("register-dir", flagRegisterDir).
		Str("datadir", flagDataDir).
		Str("execution-data-dir", flagExecutionDataDir).
		Uint64("height", flagHeight).
		Msg("starting storehouse register store rollback")

	// Open the register pebble DB.
	// Note: the register store uses a custom comparer, so we must use OpenRegisterPebbleDB.
	// Opening it takes an exclusive directory lock, which fails if a node still holds it open.
	registerDB, err := pebblestorage.OpenRegisterPebbleDB(log.Logger, flagRegisterDir)
	if err != nil {
		return fmt.Errorf("failed to open register db at %s: %w", flagRegisterDir, err)
	}
	defer func() {
		if closeErr := registerDB.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("failed to close register db")
		}
	}()

	// Open the protocol database for headers and execution results.
	protocolPebbleDB, err := pebblestorage.ShouldOpenDefaultPebbleDB(log.Logger, flagDataDir)
	if err != nil {
		return fmt.Errorf("failed to open protocol db at %s: %w", flagDataDir, err)
	}
	defer func() {
		if closeErr := protocolPebbleDB.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("failed to close protocol db")
		}
	}()

	metricsCollector := &metrics.NoopCollector{}
	storages := store.InitAll(metricsCollector, pebbleimpl.ToDB(protocolPebbleDB))

	// Open the execution data store (blobstore).
	datastoreDir := filepath.Join(flagExecutionDataDir, "blobstore")
	if _, statErr := os.Stat(datastoreDir); statErr != nil {
		return fmt.Errorf("execution data blobstore not found at %s: %w", datastoreDir, statErr)
	}
	ds, err := pebbleds.NewDatastore(datastoreDir, nil)
	if err != nil {
		return fmt.Errorf("failed to open execution data datastore at %s: %w", datastoreDir, err)
	}
	defer func() {
		if closeErr := ds.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("failed to close execution data datastore")
		}
	}()
	executionDataStore := execution_data.NewExecutionDataStore(blobs.NewBlobstore(ds), execution_data.DefaultSerializer)

	// The provider recomputes, from each block's execution data, the same register set the
	// storehouse indexer wrote at that height — the exact keys to remove.
	provider := storehouse.NewExecutionDataRegisterUpdatesProvider(executionDataStore, storages.Results)

	ctx := context.Background()

	// updatedRegisters resolves the register IDs updated at a given (finalized) height.
	// It logs the height → blockID → result ID → execution data ID mapping, and returns an
	// error (aborting the rollback) if the execution result or data for the height is missing.
	updatedRegisters := func(height uint64) ([]flow.RegisterID, error) {
		blockID, err := storages.Headers.BlockIDByHeight(height)
		if err != nil {
			return nil, fmt.Errorf("cannot get finalized block ID at height %d: %w", height, err)
		}

		result, err := storages.Results.ByBlockID(blockID)
		if err != nil {
			return nil, fmt.Errorf("cannot get execution result for block %v at height %d: %w", blockID, height, err)
		}

		log.Info().
			Uint64("height", height).
			Str("block_id", blockID.String()).
			Str("result_id", result.ID().String()).
			Str("execution_data_id", result.ExecutionDataID.String()).
			Msg("resolving register updates for block")

		entries, found, err := provider.RegisterUpdatesByBlockID(ctx, blockID)
		if err != nil {
			return nil, fmt.Errorf("cannot get register updates for block %v at height %d: %w", blockID, height, err)
		}
		if !found {
			return nil, fmt.Errorf("no execution result found for block %v at height %d", blockID, height)
		}

		regIDs := make([]flow.RegisterID, len(entries))
		for i, entry := range entries {
			regIDs[i] = entry.Key
		}
		return regIDs, nil
	}

	err = pebblestorage.RollbackRegisterStoreToHeight(log.Logger, registerDB, flagHeight, updatedRegisters)
	if err != nil {
		return fmt.Errorf("failed to roll back register store to height %d: %w", flagHeight, err)
	}

	log.Info().Msgf("register store rolled back to height %d", flagHeight)
	return nil
}
