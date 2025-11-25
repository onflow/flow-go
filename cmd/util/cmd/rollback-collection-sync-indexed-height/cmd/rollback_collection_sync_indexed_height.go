package cmd

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/store"
)

var (
	flagHeight uint64
)

var Cmd = &cobra.Command{
	Use:   "rollback-collection-sync-indexed-height",
	Short: "Rollback the collection sync indexed height",
	RunE:  runE,
}

func init() {
	// collections from height will be removed
	Cmd.Flags().Uint64Var(&flagHeight, "height", 0,
		"the start height to rollback collection sync indexed data from")
	_ = Cmd.MarkFlagRequired("height")

	common.InitDataDirFlag(Cmd, &flagDatadir)
}

func runE(*cobra.Command, []string) error {
	lockManager := storage.MakeSingletonLockManager()

	log.Info().
		Str("datadir", flagDatadir).
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

		headers := store.NewHeaders(metrics, db)
		collections := store.NewCollections(db, storages.Transactions)
		blocks := store.NewBlocks(db, headers, storages.Payloads)

		// Get the current progress heights and validate rollback height
		rootBlockHeight := state.Params().FinalizedRoot().Height
		lastFullBlockHeightProgress, err := store.NewConsumerProgress(db, module.ConsumeProgressLastFullBlockHeight).Initialize(rootBlockHeight)
		if err != nil {
			return fmt.Errorf("could not initialize last full block height progress: %w", err)
		}

		fetchAndIndexedProgress, err := store.NewConsumerProgress(db, module.ConsumeProgressAccessFetchAndIndexedCollectionsBlockHeight).Initialize(rootBlockHeight)
		if err != nil {
			return fmt.Errorf("could not initialize fetch and indexed collections block height progress: %w", err)
		}

		lastFullBlockHeight, err := lastFullBlockHeightProgress.ProcessedIndex()
		if err != nil {
			return fmt.Errorf("could not get last full block height: %w", err)
		}

		fetchAndIndexedHeight, err := fetchAndIndexedProgress.ProcessedIndex()
		if err != nil {
			return fmt.Errorf("could not get fetch and indexed collections block height: %w", err)
		}

		// Find the highest height of the two progress trackers
		maxProgressHeight := max(lastFullBlockHeight, fetchAndIndexedHeight)

		// Validate that rollback height is not beyond the highest progress height
		if flagHeight > maxProgressHeight {
			return fmt.Errorf("cannot rollback to height %v: it is beyond the highest progress height %v (lastFullBlockHeight: %v, fetchAndIndexedHeight: %v)",
				flagHeight, maxProgressHeight, lastFullBlockHeight, fetchAndIndexedHeight)
		}

		protocolDBBatch := db.NewBatch()
		defer protocolDBBatch.Close()

		// remove collections and transactions starting from rollback height + 1
		// Note: rollback height is the highest height that is NOT removed
		removeFromHeight := flagHeight + 1
		err = removeCollectionsFromHeight(
			protocolDBBatch,
			state,
			blocks,
			collections,
			storages.Transactions,
			removeFromHeight)

		if err != nil {
			return fmt.Errorf("could not remove collections from height %v: %w", removeFromHeight, err)
		}

		// Reset both progress heights to the rollback height
		// Note: rollback height is the highest height that is NOT removed (we start removing from rollback height + 1)
		err = lastFullBlockHeightProgress.BatchSetProcessedIndex(flagHeight, protocolDBBatch)
		if err != nil {
			return fmt.Errorf("could not set last full block height to %v: %w", flagHeight, err)
		}

		err = fetchAndIndexedProgress.BatchSetProcessedIndex(flagHeight, protocolDBBatch)
		if err != nil {
			return fmt.Errorf("could not set fetch and indexed collections block height to %v: %w", flagHeight, err)
		}

		err = protocolDBBatch.Commit()
		if err != nil {
			return fmt.Errorf("could not flush write batch at %v: %w", flagHeight, err)
		}

		log.Info().
			Uint64("rollback_height", flagHeight).
			Uint64("previous_last_full_block_height", lastFullBlockHeight).
			Uint64("previous_fetch_and_indexed_height", fetchAndIndexedHeight).
			Msg("collection sync indexed height rolled back and progress heights reset")

		return nil
	})
}

// removeCollectionsFromHeight removes all collections and transactions indexed by BatchStoreAndIndexByTransaction
// from the specified block height onward to the latest finalized height.
func removeCollectionsFromHeight(
	protocolDBBatch storage.Batch,
	protoState protocol.State,
	blocks storage.Blocks,
	collections storage.Collections,
	transactions *store.Transactions,
	fromHeight uint64,
) error {
	log.Info().Msgf("removing collections for blocks from height: %v", fromHeight)

	root := protoState.Params().FinalizedRoot()

	if fromHeight <= root.Height {
		return fmt.Errorf("can only remove collections for blocks above root block. fromHeight: %v, rootHeight: %v", fromHeight, root.Height)
	}

	final, err := protoState.Final().Head()
	if err != nil {
		return fmt.Errorf("could not get finalized height: %w", err)
	}

	if fromHeight > final.Height {
		return fmt.Errorf("could not remove collections for unfinalized height: %v, finalized height: %v", fromHeight, final.Height)
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

		err = removeCollectionsForBlock(protocolDBBatch, blocks, collections, transactions, blockID)
		if err != nil {
			return fmt.Errorf("could not remove collections for finalized block: %v, %w", blockID, err)
		}

		finalRemoved++
		log.Info().Msgf("collections at height %v have been removed. progress (%v/%v)", height, finalRemoved, total)
	}

	// removing for pending blocks
	pendings, err := protoState.Final().Descendants()
	if err != nil {
		return fmt.Errorf("could not get pending blocks: %w", err)
	}

	pendingRemoved := 0
	total = len(pendings)

	for _, pending := range pendings {
		err = removeCollectionsForBlock(protocolDBBatch, blocks, collections, transactions, pending)
		if err != nil {
			return fmt.Errorf("could not remove collections for pending block %v: %w", pending, err)
		}

		pendingRemoved++
		log.Info().Msgf("collections for pending block %v have been removed. progress (%v/%v)", pending, pendingRemoved, total)
	}

	log.Info().Msgf("removed collections from height %v. removed for %v finalized blocks, and %v pending blocks",
		fromHeight, finalRemoved, pendingRemoved)

	return nil
}

// removeCollectionsForBlock removes all collections and transactions for a given block.
// This removes data inserted by BatchStoreAndIndexByTransaction:
// - The collection itself
// - The transaction-to-collection index
// - The transactions
// All data to be removed will be removed in a batch write.
func removeCollectionsForBlock(
	protocolDBBatch storage.Batch,
	blocks storage.Blocks,
	collections storage.Collections,
	transactions *store.Transactions,
	blockID flow.Identifier,
) error {
	// Get the block to find its collection guarantees
	block, err := blocks.ByID(blockID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			log.Info().Msgf("block not found for block %v", blockID)
			return nil
		}
		return fmt.Errorf("could not find block %v: %w", blockID, err)
	}

	// Get collection guarantees from the block payload
	for _, guarantee := range block.Payload.Guarantees {
		collectionID := guarantee.CollectionID

		// Get the light collection to find its transactions
		lightCollection, err := collections.LightByID(collectionID)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				log.Info().Msgf("collection not found for collection %v in block %v", collectionID, blockID)
				continue
			}
			return fmt.Errorf("could not retrieve collection %v: %w", collectionID, err)
		}

		// Remove transaction indices (transaction-to-collection index)
		for _, txID := range lightCollection.Transactions {
			err = operation.RemoveCollectionTransactionIndices(protocolDBBatch.Writer(), txID)
			if err != nil {
				return fmt.Errorf("could not remove collection transaction indices for tx %v: %w", txID, err)
			}

			// Remove the transaction
			err = transactions.RemoveBatch(protocolDBBatch, txID)
			if err != nil {
				return fmt.Errorf("could not remove transaction %v: %w", txID, err)
			}
		}

		// Remove the collection itself
		err = operation.RemoveCollection(protocolDBBatch.Writer(), collectionID)
		if err != nil {
			return fmt.Errorf("could not remove collection %v: %w", collectionID, err)
		}
	}

	return nil
}
