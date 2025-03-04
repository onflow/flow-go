package migration

import (
	"errors"
	"fmt"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/block_iterator/latest"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/badgerimpl"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
)

// MigrateLastSealedExecutedResultToPebble copy the execution data, such events, transaction results etc
// for the last executed and sealed block from badger to pebble
func MigrateLastSealedExecutedResultToPebble(badgerDB *badger.DB, pebbleDB *pebble.DB, ps protocol.State) error {
	bdb := badgerimpl.ToDB(badgerDB)
	pdb := pebbleimpl.ToDB(pebbleDB)

	// get last sealed and executed block in badger
	lastExecutedSealedHeightInBadger, err := latest.LatestSealedAndExecutedHeight(ps, bdb)
	if err != nil {
		return fmt.Errorf("failed to get last executed sealed block: %w", err)
	}

	// read all the data and save to pebble
	header, err := ps.AtHeight(lastExecutedSealedHeightInBadger).Head()
	if err != nil {
		return fmt.Errorf("failed to get block at height %d: %w", lastExecutedSealedHeightInBadger, err)
	}

	blockID := header.ID()

	// create badger storage modules
	badgerEvents, badgerServiceEvents, badgerTransactionResults, badgerMyReceipts, badgerCommits := createStores(bdb)

	// read data from badger
	events, err := badgerEvents.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("failed to get events for block %s: %w", blockID, err)
	}

	serviceEvents, err := badgerServiceEvents.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("failed to get service events for block %s: %w", blockID, err)
	}

	transactionResults, err := badgerTransactionResults.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("failed to get transaction results for block %s: %w", blockID, err)
	}

	receipt, err := badgerMyReceipts.MyReceipt(blockID)
	if err != nil {
		return fmt.Errorf("failed to get receipt for block %s: %w", blockID, err)
	}

	commmit, err := badgerCommits.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("failed to get commit for block %s: %w", blockID, err)
	}

	// create pebble storage modules
	pebbleEvents, pebbleServiceEvents, pebbleTransactionResults, pebbleReceipts, pebbleCommits := createStores(pdb)

	// store data to pebble in a batch update
	err = pdb.WithReaderBatchWriter(func(batch storage.ReaderBatchWriter) error {
		if err := pebbleEvents.BatchStore(blockID, []flow.EventsList{events}, batch); err != nil {
			return fmt.Errorf("failed to store events for block %s: %w", blockID, err)
		}

		if err := pebbleServiceEvents.BatchStore(blockID, serviceEvents, batch); err != nil {
			return fmt.Errorf("failed to store service events for block %s: %w", blockID, err)
		}

		if err := pebbleTransactionResults.BatchStore(blockID, transactionResults, batch); err != nil {
			return fmt.Errorf("failed to store transaction results for block %s: %w", blockID, err)
		}

		if err := pebbleReceipts.BatchStoreMyReceipt(receipt, batch); err != nil {
			return fmt.Errorf("failed to store receipt for block %s: %w", blockID, err)
		}

		if err := pebbleCommits.BatchStore(blockID, commmit, batch); err != nil {
			return fmt.Errorf("failed to store commit for block %s: %w", blockID, err)
		}

		var existingExecuted flow.Identifier
		err = operation.RetrieveExecutedBlock(batch.GlobalReader(), &existingExecuted)
		if err == nil {
			// there is an executed block in pebble, compare if it's newer than the badger one,
			// if newer, it means EN is storing new results in pebble, in this case, we don't
			// want to update the executed block with the badger one.

			header, err := ps.AtBlockID(existingExecuted).Head()
			if err != nil {
				return fmt.Errorf("failed to get block at height from badger %d: %w", lastExecutedSealedHeightInBadger, err)
			}

			if header.Height > lastExecutedSealedHeightInBadger {
				// existing executed in pebble is higher than badger, no need to update
				return nil
			}

			// otherwise continue to pebble in pebble
		}

		if !errors.Is(err, storage.ErrNotFound) {
			// exception
			return fmt.Errorf("failed to retrieve executed block from pebble: %w", err)
		}

		// no executed block in pebble or badger has newer executed block than pebble,
		// set this block as last executed block
		// if there is no executed block in pebble, set this block as last executed block
		if err := operation.UpdateExecutedBlock(batch.Writer(), blockID); err != nil {
			return fmt.Errorf("failed to update executed block in pebble: %w", err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to write data to pebble: %w", err)
	}

	return nil
}

func createStores(db storage.DB) (
	storage.Events,
	storage.ServiceEvents,
	storage.TransactionResults,
	storage.MyExecutionReceipts,
	storage.Commits,
) {

	noop := metrics.NewNoopCollector()
	events := store.NewEvents(noop, db)
	serviceEvents := store.NewServiceEvents(noop, db)
	transactionResults := store.NewTransactionResults(noop, db, 1)
	results := store.NewExecutionResults(noop, db)
	receipts := store.NewExecutionReceipts(noop, db, results, 1)
	myReceipts := store.NewMyExecutionReceipts(noop, db, receipts)
	commits := store.NewCommits(noop, db)
	return events, serviceEvents, transactionResults, myReceipts, commits
}
