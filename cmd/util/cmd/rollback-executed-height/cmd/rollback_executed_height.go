package cmd

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/state/protocol"
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
		chunkDataPacks := store.NewChunkDataPacks(metrics, chunkDataPacksDB, storedChunkDataPacks, collections, 1000)
		chunkBatch := chunkDataPacksDB.NewBatch()
		defer chunkBatch.Close()

		protocolDBBatch := db.NewBatch()
		defer protocolDBBatch.Close()

		err = removeExecutionResultsFromHeight(
			protocolDBBatch,
			chunkBatch,
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
		err = chunkBatch.Commit()
		if err != nil {
			return fmt.Errorf("could not commit chunk batch at %v: %w", flagHeight, err)
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

// use badger instances directly instead of stroage interfaces so that the interface don't
// need to include the Remove methods
func removeExecutionResultsFromHeight(
	protocolDBBatch storage.Batch,
	chunkBatch storage.Batch,
	protoState protocol.State,
	transactionResults storage.TransactionResults,
	commits storage.Commits,
	chunkDataPacks storage.ChunkDataPacks,
	results storage.ExecutionResults,
	myReceipts storage.MyExecutionReceipts,
	events storage.Events,
	serviceEvents storage.ServiceEvents,
	fromHeight uint64) error {
	log.Info().Msgf("removing results for blocks from height: %v", fromHeight)

	root := protoState.Params().FinalizedRoot()

	if fromHeight <= root.Height {
		return fmt.Errorf("can only remove results for block above root block. fromHeight: %v, rootHeight: %v", fromHeight, root.Height)
	}

	final, err := protoState.Final().Head()
	if err != nil {
		return fmt.Errorf("could get not finalized height: %w", err)
	}

	if fromHeight > final.Height {
		return fmt.Errorf("could not remove results for unfinalized height: %v, finalized height: %v", fromHeight, final.Height)
	}

	finalRemoved := 0
	total := int(final.Height-fromHeight) + 1

	// removing for finalized blocks
	for height := fromHeight; height <= final.Height; height++ {
		head, err := protoState.AtHeight(height).Head()
		if err != nil {
			return fmt.Errorf("could not get header at height: %w", err)
		}

		blockID := head.ID()

		err = removeForBlockID(protocolDBBatch, chunkBatch, commits, transactionResults, results, chunkDataPacks, myReceipts, events, serviceEvents, blockID)
		if err != nil {
			return fmt.Errorf("could not remove result for finalized block: %v, %w", blockID, err)
		}

		finalRemoved++
		log.Info().Msgf("result at height %v has been removed. progress (%v/%v)", height, finalRemoved, total)
	}

	// removing for pending blocks
	pendings, err := protoState.Final().Descendants()
	if err != nil {
		return fmt.Errorf("could not get pending block: %w", err)
	}

	pendingRemoved := 0
	total = len(pendings)

	for _, pending := range pendings {
		err = removeForBlockID(protocolDBBatch, chunkBatch, commits, transactionResults, results, chunkDataPacks, myReceipts, events, serviceEvents, pending)

		if err != nil {
			return fmt.Errorf("could not remove result for pending block %v: %w", pending, err)
		}

		pendingRemoved++
		log.Info().Msgf("result for pending block %v has been removed. progress (%v/%v) ", pending, pendingRemoved, total)
	}

	log.Info().Msgf("removed height from %v. removed for %v finalized blocks, and %v pending blocks",
		fromHeight, finalRemoved, pendingRemoved)

	return nil
}

// removeForBlockID remove block execution related data for a given block.
// All data to be removed will be removed in a batch write.
// It bubbles up any error encountered
func removeForBlockID(
	protocolDBBatch storage.Batch,
	chunkBatch storage.Batch,
	commits storage.Commits,
	transactionResults storage.TransactionResults,
	results storage.ExecutionResults,
	chunks storage.ChunkDataPacks,
	myReceipts storage.MyExecutionReceipts,
	events storage.Events,
	serviceEvents storage.ServiceEvents,
	blockID flow.Identifier,
) error {
	result, err := results.ByBlockID(blockID)
	if errors.Is(err, storage.ErrNotFound) {
		log.Info().Msgf("result not found for block %v", blockID)
		return nil
	}

	if err != nil {
		return fmt.Errorf("could not find result for block %v: %w", blockID, err)
	}

	chunkIDs := make([]flow.Identifier, 0, len(result.Chunks))
	for _, chunk := range result.Chunks {
		chunkID := chunk.ID()
		chunkIDs = append(chunkIDs, chunkID)
	}

	if len(chunkIDs) > 0 {
		err := chunks.BatchRemove(chunkIDs, protocolDBBatch, chunkBatch)
		if err != nil {
			return fmt.Errorf("could not remove chunk data packs for block id %v: %w", blockID, err)
		}
	}

	// remove commits
	err = commits.BatchRemoveByBlockID(blockID, protocolDBBatch)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("could not remove by block ID %v: %w", blockID, err)
		}

		log.Warn().Msgf("statecommitment not found for block %v", blockID)
	}

	// remove transaction results
	err = transactionResults.BatchRemoveByBlockID(blockID, protocolDBBatch)
	if err != nil {
		return fmt.Errorf("could not remove transaction results by BlockID %v: %w", blockID, err)
	}

	// remove own execution results index
	err = myReceipts.BatchRemoveIndexByBlockID(blockID, protocolDBBatch)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("could not remove own receipt by BlockID %v: %w", blockID, err)
		}

		log.Warn().Msgf("own receipt not found for block %v", blockID)
	}

	// remove events
	err = events.BatchRemoveByBlockID(blockID, protocolDBBatch)
	if err != nil {
		return fmt.Errorf("could not remove events by BlockID %v: %w", blockID, err)
	}

	// remove service events
	err = serviceEvents.BatchRemoveByBlockID(blockID, protocolDBBatch)
	if err != nil {
		return fmt.Errorf("could not remove service events by blockID %v: %w", blockID, err)
	}

	// remove execution result index
	err = results.BatchRemoveIndexByBlockID(blockID, protocolDBBatch)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("could not remove result by BlockID %v: %w", blockID, err)
		}

		log.Warn().Msgf("result not found for block %v", blockID)
	}

	return nil
}
