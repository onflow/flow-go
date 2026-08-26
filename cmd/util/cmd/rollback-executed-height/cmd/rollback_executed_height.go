package cmd

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	storagepebble "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/storage/store"
)

var (
	flagHeight           uint64
	flagChunkDataPackDir string
)

var Cmd = &cobra.Command{
	Use:   "rollback-executed-height",
	Short: "Rollback the executed height",
	RunE:  runE,
}

func init() {

	// execution results from height + 1 will be removed
	Cmd.Flags().Uint64Var(&flagHeight, "height", 0,
		"the height of the block to update the highest executed height")
	_ = Cmd.MarkFlagRequired("height")

	common.InitDataDirFlag(Cmd, &flagDatadir)

	Cmd.Flags().StringVar(&flagChunkDataPackDir, "chunk_data_pack_dir", "/var/flow/data/chunk_data_pack",
		"directory that stores the protocol state")
	_ = Cmd.MarkFlagRequired("chunk_data_pack_dir")
}

func runE(*cobra.Command, []string) error {
	lockManager := storage.MakeSingletonLockManager()

	log.Info().
		Str("datadir", flagDatadir).
		Str("chunk_data_pack_dir", flagChunkDataPackDir).
		Uint64("height", flagHeight).
		Msg("flags")

	if flagHeight == 0 {
		// this would be a mistake that the height flag is used but no height value
		// was specified, so the default value 0 is used.
		return fmt.Errorf("height must be above 0: %v", flagHeight)
	}

	return common.WithStorage(flagDatadir, func(db storage.DB) error {
		storages := common.InitStorages(db)
		state, err := common.OpenProtocolState(lockManager, db, storages)
		if err != nil {
			return fmt.Errorf("could not open protocol states: %w", err)
		}

		metrics := &metrics.NoopCollector{}

		transactionResults, err := store.NewTransactionResults(metrics, db, badger.DefaultCacheSize)
		if err != nil {
			return err
		}
		commits := store.NewCommits(metrics, db)
		results := store.NewExecutionResults(metrics, db)
		receipts := store.NewExecutionReceipts(metrics, db, results, badger.DefaultCacheSize)
		myReceipts := store.NewMyExecutionReceipts(metrics, db, receipts)
		headers := store.NewHeaders(metrics, db)
		events := store.NewEvents(metrics, db)
		serviceEvents := store.NewServiceEvents(metrics, db)
		transactions := store.NewTransactions(metrics, db)
		collections := store.NewCollections(db, transactions)
		// require the chunk data pack data must exist before returning the storage module
		chunkDataPacksPebbleDB, err := storagepebble.ShouldOpenDefaultPebbleDB(
			log.Logger.With().Str("pebbledb", "cdp").Logger(), flagChunkDataPackDir)
		if err != nil {
			return fmt.Errorf("could not open chunk data pack DB at %v: %w", flagChunkDataPackDir, err)
		}
		chunkDataPacksDB := pebbleimpl.ToDB(chunkDataPacksPebbleDB)
		storedChunkDataPacks := store.NewStoredChunkDataPacks(metrics, chunkDataPacksDB, 1000)
		chunkDataPacks := store.NewChunkDataPacks(metrics, db, storedChunkDataPacks, collections, 1000)
		protocolDBBatch := db.NewBatch()
		defer protocolDBBatch.Close()

		// collect chunk IDs to be removed
		chunkIDs, err := common.RemoveExecutionResultsFromHeight(
			protocolDBBatch,
			state,
			transactionResults,
			commits,
			chunkDataPacks,
			results,
			myReceipts,
			events,
			serviceEvents,
			flagHeight+1)

		if err != nil {
			return fmt.Errorf("could not remove result from height %v: %w", flagHeight, err)
		}

		// remove chunk data packs first, because otherwise the index to find chunk data pack will be removed.
		if len(chunkIDs) > 0 {
			_, err := chunkDataPacks.BatchRemove(chunkIDs, protocolDBBatch)
			if err != nil {
				return fmt.Errorf("could not remove chunk data packs at %v: %w", flagHeight, err)
			}
		}

		err = protocolDBBatch.Commit()
		if err != nil {
			return fmt.Errorf("could not flush write batch at %v: %w", flagHeight, err)
		}

		header, err := state.AtHeight(flagHeight).Head()
		if err != nil {
			return fmt.Errorf("could not get block header at height %v: %w", flagHeight, err)
		}

		err = headers.RollbackExecutedBlock(header)
		if err != nil {
			return fmt.Errorf("could not roll back executed block at height %v: %w", flagHeight, err)
		}

		log.Info().Msgf("executed height rolled back to %v", flagHeight)

		return nil
	})
}
