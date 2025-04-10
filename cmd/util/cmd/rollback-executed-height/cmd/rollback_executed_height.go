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
	"github.com/onflow/flow-go/storage/operation/badgerimpl"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	storagepebble "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/storage/store"
)

var (
	flagHeight           uint64
	flagDataDir          string
	flagChunkDataPackDir string
)

var Cmd = &cobra.Command{
	Use:   "rollback-executed-height",
	Short: "Rollback the executed height",
	Run:   run,
}

func init() {

	// execution results from height + 1 will be removed
	Cmd.Flags().Uint64Var(&flagHeight, "height", 0,
		"the height of the block to update the highest executed height")
	_ = Cmd.MarkFlagRequired("height")

	Cmd.Flags().StringVar(&flagDataDir, "datadir", "",
		"directory that stores the protocol state")
	_ = Cmd.MarkFlagRequired("datadir")

	Cmd.Flags().StringVar(&flagChunkDataPackDir, "chunk_data_pack_dir", "/var/flow/data/chunk_data_pack",
		"directory that stores the protocol state")
	_ = Cmd.MarkFlagRequired("chunk_data_pack_dir")
}

func run(*cobra.Command, []string) {
	log.Info().
		Str("datadir", flagDataDir).
		Str("chunk_data_pack_dir", flagChunkDataPackDir).
		Uint64("height", flagHeight).
		Msg("flags")

	if flagHeight == 0 {
		// this would be a mistake that the height flag is used but no height value
		// was specified, so the default value 0 is used.
		log.Fatal().Msg("height must be above 0")
	}

	bdb := common.InitStorage(flagDataDir)
	storages := common.InitStorages(bdb)
	state, err := common.InitProtocolState(bdb, storages)
	if err != nil {
		log.Fatal().Err(err).Msg("could not init protocol states")
	}

	db := badgerimpl.ToDB(bdb)
	metrics := &metrics.NoopCollector{}

	transactionResults := store.NewTransactionResults(metrics, db, badger.DefaultCacheSize)
	commits := store.NewCommits(metrics, db)
	results := store.NewExecutionResults(metrics, db)
	receipts := store.NewExecutionReceipts(metrics, db, results, badger.DefaultCacheSize)
	myReceipts := store.NewMyExecutionReceipts(metrics, db, receipts)
	headers := badger.NewHeaders(metrics, bdb)
	events := store.NewEvents(metrics, db)
	serviceEvents := store.NewServiceEvents(metrics, db)
	transactions := badger.NewTransactions(metrics, bdb)
	collections := badger.NewCollections(bdb, transactions)
	// require the chunk data pack data must exist before returning the storage module
	chunkDataPacksPebbleDB, err := storagepebble.MustOpenDefaultPebbleDB(
		log.Logger.With().Str("pebbledb", "cdp").Logger(), flagChunkDataPackDir)
	if err != nil {
		log.Fatal().Err(err).Msgf("could not open chunk data pack DB at %v", flagChunkDataPackDir)
	}
	chunkDataPacksDB := pebbleimpl.ToDB(chunkDataPacksPebbleDB)
	chunkDataPacks := store.NewChunkDataPacks(metrics, chunkDataPacksDB, collections, 1000)
	chunkBatch := chunkDataPacksDB.NewBatch()
	defer chunkBatch.Close()

	writeBatch := db.NewBatch()
	defer writeBatch.Close()

	err = removeExecutionResultsFromHeight(
		writeBatch,
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
		log.Fatal().Err(err).Msgf("could not remove result from height %v", flagHeight)
	}

	// remove chunk data packs first, because otherwise the index to find chunk data pack will be removed.
	err = chunkBatch.Commit()
	if err != nil {
		log.Fatal().Err(err).Msgf("could not commit chunk batch at %v", flagHeight)
	}

	err = writeBatch.Commit()
	if err != nil {
		log.Fatal().Err(err).Msgf("could not flush write batch at %v", flagHeight)
	}

	header, err := state.AtHeight(flagHeight).Head()
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get block header at height %v", flagHeight)
	}

	err = headers.RollbackExecutedBlock(header)
	if err != nil {
		log.Fatal().Err(err).Msgf("could not roll back executed block at height %v", flagHeight)
	}

	log.Info().Msgf("executed height rolled back to %v", flagHeight)

}

// use badger instances directly instead of stroage interfaces so that the interface don't
// need to include the Remove methods
func removeExecutionResultsFromHeight(
	writeBatch storage.Batch,
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

		err = removeForBlockID(writeBatch, chunkBatch, commits, transactionResults, results, chunkDataPacks, myReceipts, events, serviceEvents, blockID)
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
		err = removeForBlockID(writeBatch, chunkBatch, commits, transactionResults, results, chunkDataPacks, myReceipts, events, serviceEvents, pending)

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
	writeBatch storage.Batch,
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

	for _, chunk := range result.Chunks {
		chunkID := chunk.ID()
		// remove chunk data pack
		err := chunks.BatchRemove(chunkID, chunkBatch)
		if err != nil {
			return fmt.Errorf("could not remove chunk id %v for block id %v: %w", chunkID, blockID, err)
		}

	}

	// remove commits
	err = commits.BatchRemoveByBlockID(blockID, writeBatch)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("could not remove by block ID %v: %w", blockID, err)
		}

		log.Warn().Msgf("statecommitment not found for block %v", blockID)
	}

	// remove transaction results
	err = transactionResults.BatchRemoveByBlockID(blockID, writeBatch)
	if err != nil {
		return fmt.Errorf("could not remove transaction results by BlockID %v: %w", blockID, err)
	}

	// remove own execution results index
	err = myReceipts.BatchRemoveIndexByBlockID(blockID, writeBatch)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("could not remove own receipt by BlockID %v: %w", blockID, err)
		}

		log.Warn().Msgf("own receipt not found for block %v", blockID)
	}

	// remove events
	err = events.BatchRemoveByBlockID(blockID, writeBatch)
	if err != nil {
		return fmt.Errorf("could not remove events by BlockID %v: %w", blockID, err)
	}

	// remove service events
	err = serviceEvents.BatchRemoveByBlockID(blockID, writeBatch)
	if err != nil {
		return fmt.Errorf("could not remove service events by blockID %v: %w", blockID, err)
	}

	// remove execution result index
	err = results.BatchRemoveIndexByBlockID(blockID, writeBatch)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("could not remove result by BlockID %v: %w", blockID, err)
		}

		log.Warn().Msgf("result not found for block %v", blockID)
	}

	return nil
}
