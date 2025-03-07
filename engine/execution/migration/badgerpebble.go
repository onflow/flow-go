package migration

import (
	"errors"
	"fmt"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution/state/bootstrap"
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

var (
	mainnet26SporkID flow.Identifier
	testnet52SporkID flow.Identifier
)

func init() {
	var err error

	mainnet26SporkID, err = flow.HexStringToIdentifier("45894bde3f45dbfd89cab12be84e9172385da5079d40bf63979ca8a6a7ede741")
	if err != nil {
		panic(fmt.Sprintf("failed to parse Mainnet26SporkID: %v", err))
	}

	testnet52SporkID, err = flow.HexStringToIdentifier("5b88b81cfce2619305213489c2137f98e6efa0ec333dab2c31042b743388a3ce")
	if err != nil {
		panic(fmt.Sprintf("failed to parse Testnet52SporkID: %v", err))
	}
}

// MigrateLastSealedExecutedResultToPebble run the migration to the pebble database, so that
// it has necessary data to be able execute the next block.
// the migration includes the following operations:
//  1. bootstrap the pebble database
//  2. copy execution data of the last sealed and executed block from badger to pebble.
//     the execution data includes the execution result and statecommitment, which is the minimum data needed from the database
//     to be able to continue executing the next block
func MigrateLastSealedExecutedResultToPebble(logger zerolog.Logger, badgerDB *badger.DB, pebbleDB *pebble.DB, state protocol.State, rootSeal *flow.Seal) error {
	// only run the migration for mainnet26 and testnet52
	sporkID := state.Params().SporkID()
	if sporkID != mainnet26SporkID && sporkID != testnet52SporkID {
		logger.Warn().Msgf("spork ID %v is not Mainnet26SporkID %v or Testnet52SporkID %v, skip migration",
			sporkID, mainnet26SporkID, testnet52SporkID)
		return nil
	}

	bdb := badgerimpl.ToDB(badgerDB)
	pdb := pebbleimpl.ToDB(pebbleDB)
	lg := logger.With().Str("module", "badger-pebble-migration").Logger()

	// bootstrap pebble database
	bootstrapper := bootstrap.NewBootstrapper(logger)
	commit, bootstrapped, err := bootstrapper.IsBootstrapped(pdb)
	if err != nil {
		return fmt.Errorf("could not query database to know whether database has been bootstrapped: %w", err)
	}

	if !bootstrapped {
		err = bootstrapper.BootstrapExecutionDatabase(pdb, rootSeal)
		if err != nil {
			return fmt.Errorf("could not bootstrap pebble execution database: %w", err)
		}
	}

	// get last sealed and executed block in badger
	lastExecutedSealedHeightInBadger, err := latest.LatestSealedAndExecutedHeight(state, bdb)
	if err != nil {
		return fmt.Errorf("failed to get last executed sealed block: %w", err)
	}

	// read all the data and save to pebble
	header, err := state.AtHeight(lastExecutedSealedHeightInBadger).Head()
	if err != nil {
		return fmt.Errorf("failed to get block at height %d: %w", lastExecutedSealedHeightInBadger, err)
	}

	blockID := header.ID()

	lg.Info().Msgf(
		"migrating last executed and sealed block %v (%v) from badger to pebble",
		header.Height, blockID)

	// create badger storage modules
	badgerResults, badgerCommits := createStores(bdb)
	// read data from badger
	result, commit, err := readResultsForBlock(blockID, badgerResults, badgerCommits)

	if err != nil {
		return fmt.Errorf("failed to read data from badger: %w", err)
	}

	// create pebble storage modules
	pebbleResults, pebbleCommits := createStores(pdb)

	// store data to pebble in a batch update
	err = pdb.WithReaderBatchWriter(func(batch storage.ReaderBatchWriter) error {
		var existingExecuted flow.Identifier
		err = operation.RetrieveExecutedBlock(batch.GlobalReader(), &existingExecuted)
		if err == nil {
			// there is an executed block in pebble, compare if it's newer than the badger one,
			// if newer, it means EN is storing new results in pebble, in this case, we don't
			// want to update the executed block with the badger one.

			header, err := state.AtBlockID(existingExecuted).Head()
			if err != nil {
				return fmt.Errorf("failed to get block at height %d from badger: %w", lastExecutedSealedHeightInBadger, err)
			}

			if header.Height > lastExecutedSealedHeightInBadger {
				// existing executed in pebble is higher than badger, no need to store anything
				lg.Info().Msgf("existing executed block %v in pebble is newer than %v in badger, skip update",
					header.Height, lastExecutedSealedHeightInBadger)
				return nil
			}

			// otherwise continue to update last executed block in pebble
			lg.Info().Msgf("existing executed block %v in pebble is older than %v in badger, update executed block",
				header.Height, lastExecutedSealedHeightInBadger,
			)
		} else if !errors.Is(err, storage.ErrNotFound) {
			// exception
			return fmt.Errorf("failed to retrieve executed block from pebble: %w", err)
		}

		if err := pebbleResults.BatchStore(result, batch); err != nil {
			return fmt.Errorf("failed to store receipt for block %s: %w", blockID, err)
		}

		if err := pebbleResults.BatchIndex(blockID, result.ID(), batch); err != nil {
			return fmt.Errorf("failed to index result for block %s: %w", blockID, err)
		}

		if err := pebbleCommits.BatchStore(blockID, commit, batch); err != nil {
			return fmt.Errorf("failed to store commit for block %s: %w", blockID, err)
		}

		// two cases here:
		// 1. no executed block in pebble
		// 		in this case: set this block as last executed block
		// 2. badger has newer executed block than pebble
		//		in this case: set this block as last executed block
		if err := operation.UpdateExecutedBlock(batch.Writer(), blockID); err != nil {
			return fmt.Errorf("failed to update executed block in pebble: %w", err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to write data to pebble: %w", err)
	}

	lg.Info().Msgf("migrated last executed and sealed block %v (%v) from badger to pebble",
		header.Height, blockID)

	return nil
}

func readResultsForBlock(
	blockID flow.Identifier,
	resultsStore storage.ExecutionResults,
	commitsStore storage.Commits,
) (
	*flow.ExecutionResult, flow.StateCommitment, error) {
	result, err := resultsStore.ByBlockID(blockID)
	if err != nil {
		return nil, flow.DummyStateCommitment, fmt.Errorf("failed to get receipt for block %s: %w", blockID, err)
	}

	commit, err := commitsStore.ByBlockID(blockID)
	if err != nil {
		return nil, flow.DummyStateCommitment, fmt.Errorf("failed to get commit for block %s: %w", blockID, err)
	}
	return result, commit, nil
}

func createStores(db storage.DB) (storage.ExecutionResults, storage.Commits) {
	noop := metrics.NewNoopCollector()
	results := store.NewExecutionResults(noop, db)
	commits := store.NewCommits(noop, db)
	return results, commits
}
