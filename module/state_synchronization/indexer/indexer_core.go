package indexer

import (
	"errors"
	"fmt"
	"time"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-core-contracts/lib/go/templates"

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// IndexerCore indexes the execution state.
type IndexerCore struct {
	log                      zerolog.Logger
	chainID                  flow.ChainID
	fvmEnv                   templates.Environment
	metrics                  module.ExecutionStateIndexerMetrics
	collectionExecutedMetric module.CollectionExecutedMetric

	registers             storage.RegisterIndex
	headers               storage.Headers
	events                storage.Events
	collections           storage.Collections
	transactions          storage.Transactions
	results               storage.LightTransactionResults
	scheduledTransactions storage.ScheduledTransactions
	protocolDB            storage.DB

	derivedChainData *derived.DerivedChainData
	serviceAddress   flow.Address
	lockManager      lockctx.Manager
}

// New execution state indexer used to ingest block execution data and index it by height.
// The passed RegisterIndex storage must be populated to include the first and last height otherwise the indexer
// won't be initialized to ensure we have bootstrapped the storage first.
func New(
	log zerolog.Logger,
	metrics module.ExecutionStateIndexerMetrics,
	protocolDB storage.DB,
	registers storage.RegisterIndex,
	headers storage.Headers,
	events storage.Events,
	collections storage.Collections,
	transactions storage.Transactions,
	results storage.LightTransactionResults,
	scheduledTransactions storage.ScheduledTransactions,
	chainID flow.ChainID,
	derivedChainData *derived.DerivedChainData,
	collectionExecutedMetric module.CollectionExecutedMetric,
	lockManager lockctx.Manager,
) *IndexerCore {
	log = log.With().Str("component", "execution_indexer").Logger()
	metrics.InitializeLatestHeight(registers.LatestHeight())

	log.Info().
		Uint64("first_height", registers.FirstHeight()).
		Uint64("latest_height", registers.LatestHeight()).
		Msg("indexer initialized")

	fvmEnv := systemcontracts.SystemContractsForChain(chainID).AsTemplateEnv()

	return &IndexerCore{
		log:                   log,
		metrics:               metrics,
		chainID:               chainID,
		fvmEnv:                fvmEnv,
		protocolDB:            protocolDB,
		registers:             registers,
		headers:               headers,
		collections:           collections,
		transactions:          transactions,
		events:                events,
		results:               results,
		scheduledTransactions: scheduledTransactions,
		serviceAddress:        chainID.Chain().ServiceAddress(),
		derivedChainData:      derivedChainData,

		collectionExecutedMetric: collectionExecutedMetric,
		lockManager:              lockManager,
	}
}

// RegisterValue retrieves register values by the register IDs at the provided block height.
// Even if the register wasn't indexed at the provided height, returns the highest height the register was indexed at.
// If a register is not found it will return a nil value and not an error.
//
// Expected error returns during normal operation:
//   - [storage.ErrHeightNotIndexed]: if the given height was not indexed yet or lower than the first indexed height.
func (c *IndexerCore) RegisterValue(ID flow.RegisterID, height uint64) (flow.RegisterValue, error) {
	value, err := c.registers.Get(ID, height)
	if err != nil {
		// only return an error if the error doesn't match the not found error, since we have
		// to gracefully handle not found values and instead assign nil, that is because the script executor
		// expects that behaviour
		if errors.Is(err, storage.ErrNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return value, nil
}

// IndexBlockData indexes all execution block data by height.
// This method shouldn't be used concurrently.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound]: if the block for execution data was not found
func (c *IndexerCore) IndexBlockData(data *execution_data.BlockExecutionDataEntity) error {
	header, err := c.headers.ByBlockID(data.BlockID)
	if err != nil {
		return fmt.Errorf("could not get the block by ID %s: %w", data.BlockID, err)
	}

	lg := c.log.With().
		Hex("block_id", logging.ID(data.BlockID)).
		Uint64("height", header.Height).
		Logger()

	lg.Debug().Msgf("indexing new block")

	// the height we are indexing must be exactly one bigger or same as the latest height indexed from the storage
	latest := c.registers.LatestHeight()
	if header.Height != latest+1 && header.Height != latest {
		return fmt.Errorf("must index block data with the next height %d, but got %d", latest+1, header.Height)
	}

	// allow rerunning the indexer for same height since we are fetching height from register storage, but there are other storages
	// for indexing resources which might fail to update the values, so this enables rerunning and reindexing those resources
	if header.Height == latest {
		lg.Warn().Msg("reindexing block data")
		c.metrics.BlockReindexed()
	}

	start := time.Now()
	g := errgroup.Group{}

	var eventCount, resultCount, registerCount int
	g.Go(func() error {
		start := time.Now()

		events := make([]flow.Event, 0)
		results := make([]flow.LightTransactionResult, 0)
		for _, chunk := range data.ChunkExecutionDatas {
			events = append(events, chunk.Events...)
			results = append(results, chunk.TransactionResults...)
		}

		systemChunkIndex := len(data.ChunkExecutionDatas) - 1
		systemChunkEvents := data.ChunkExecutionDatas[systemChunkIndex].Events
		systemChunkResults := data.ChunkExecutionDatas[systemChunkIndex].TransactionResults

		scheduledTransactionData, err := collectScheduledTransactions(c.fvmEnv, c.chainID, systemChunkResults, systemChunkEvents)
		if err != nil {
			return fmt.Errorf("could not collect scheduled transaction data: %w", err)
		}

		lctx := c.lockManager.NewContext()
		defer lctx.Release()
		if err = lctx.AcquireLock(storage.LockIndexScheduledTransaction); err != nil {
			return fmt.Errorf("could not acquire lock for indexing scheduled transactions: %w", err)
		}

		err = c.protocolDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			err := c.events.BatchStore(data.BlockID, []flow.EventsList{events}, rw)
			if err != nil {
				return fmt.Errorf("could not index events at height %d: %w", header.Height, err)
			}

			err = c.results.BatchStore(data.BlockID, results, rw)
			if err != nil {
				return fmt.Errorf("could not index transaction results at height %d: %w", header.Height, err)
			}

			for txID, scheduledTxID := range scheduledTransactionData {
				err = c.scheduledTransactions.BatchIndex(lctx, data.BlockID, txID, scheduledTxID, rw)
				if err != nil {
					return fmt.Errorf("could not index scheduled transaction (%d) %s at height %d: %w", scheduledTxID, txID, header.Height, err)
				}
			}

			return nil
		})

		if err != nil {
			return fmt.Errorf("could not commit block data: %w", err)
		}

		eventCount = len(events)
		resultCount = len(results)

		lg.Debug().
			Int("event_count", eventCount).
			Int("result_count", resultCount).
			Int("scheduled_tx_count", len(scheduledTransactionData)).
			Dur("duration_ms", time.Since(start)).
			Msg("indexed protocol data")

		return nil
	})

	g.Go(func() error {
		start := time.Now()

		// index all collections except the system chunk
		// Note: the access ingestion engine also indexes collections, starting when the block is
		// finalized. This process can fall behind due to the node being offline, resource issues
		// or network congestion. This indexer ensures that collections are never farther behind
		// than the latest indexed block. Calling the collection handler with a collection that
		// has already been indexed is a noop.
		indexedCount := 0
		if len(data.ChunkExecutionDatas) > 0 {
			for _, chunk := range data.ChunkExecutionDatas[0 : len(data.ChunkExecutionDatas)-1] {
				err := c.indexCollection(chunk.Collection)
				if err != nil {
					return err
				}
				indexedCount++
			}
		}

		lg.Debug().
			Int("collection_count", indexedCount).
			Dur("duration_ms", time.Since(start)).
			Msg("indexed collections")

		return nil
	})

	g.Go(func() error {
		start := time.Now()

		// we are iterating all the registers and overwrite any existing register at the same path
		// this will make sure if we have multiple register changes only the last change will get persisted
		// if block has two chucks:
		// first chunk updates: { X: 1, Y: 2 }
		// second chunk updates: { X: 2 }
		// then we should persist only {X: 2: Y: 2}
		payloads := make(map[ledger.Path]*ledger.Payload)
		events := make([]flow.Event, 0)
		collections := make([]*flow.Collection, 0)
		for _, chunk := range data.ChunkExecutionDatas {
			events = append(events, chunk.Events...)
			collections = append(collections, chunk.Collection)
			update := chunk.TrieUpdate
			if update != nil {
				// this should never happen but we check anyway
				if len(update.Paths) != len(update.Payloads) {
					return fmt.Errorf("update paths length is %d and payloads length is %d and they don't match", len(update.Paths), len(update.Payloads))
				}

				for i, path := range update.Paths {
					payloads[path] = update.Payloads[i]
				}
			}
		}

		err = c.indexRegisters(payloads, header.Height)
		if err != nil {
			return fmt.Errorf("could not index register payloads at height %d: %w", header.Height, err)
		}

		err = c.updateProgramCache(header, events, collections)
		if err != nil {
			return fmt.Errorf("could not update program cache at height %d: %w", header.Height, err)
		}

		registerCount = len(payloads)

		lg.Debug().
			Int("register_count", registerCount).
			Dur("duration_ms", time.Since(start)).
			Msg("indexed registers")

		return nil
	})

	err = g.Wait()
	if err != nil {
		return fmt.Errorf("failed to index block data at height %d: %w", header.Height, err)
	}

	c.metrics.BlockIndexed(header.Height, time.Since(start), eventCount, registerCount, resultCount)
	lg.Debug().
		Dur("duration_ms", time.Since(start)).
		Msg("indexed block data")

	return nil
}

// collectScheduledTransactions processes the system chunk's events and transaction results,
// and returns a mapping from transaction ID to scheduled transaction ID for all scheduled transactions
// executed within the system chunk.
// The method also verifies that the transactions and events are consistent with the expected
// system collection, and returns an error if there are any inconsistencies.
//
// No error returns are expected during normal operations.
func collectScheduledTransactions(
	fvmEnv templates.Environment,
	chainID flow.ChainID,
	systemChunkResults []flow.LightTransactionResult,
	systemChunkEvents []flow.Event,
) (map[flow.Identifier]uint64, error) {
	if len(systemChunkResults) == 0 {
		return nil, fmt.Errorf("system chunk contained 0 transaction results")
	}

	scheduledTransactionIDs := make([]uint64, 0)
	pendingExecutionEvents := make([]flow.Event, 0)

	// extract the pending execution events and create a mapping from transaction ID to scheduled transaction ID
	for i, event := range systemChunkEvents {
		if blueprints.IsPendingExecutionEvent(fvmEnv, event) {
			id, _, err := blueprints.ParsePendingExecutionEvent(event)
			if err != nil {
				return nil, fmt.Errorf("could not get callback details from event %d: %w", i, err)
			}
			scheduledTransactionIDs = append(scheduledTransactionIDs, uint64(id))
			pendingExecutionEvents = append(pendingExecutionEvents, event)
		}
	}

	// there are 3 possible valid cases:
	// 1. N (1 or more) scheduled transaction were executed, there should be N + 2 results
	//    (N scheduled transactions, process callback tx, and the standard system tx)
	// 2. 0 scheduled transactions were executed, and scheduled transactions are enabled. there should be 2 results
	//    (process callback tx, and the standard system tx)
	// 3. 0 scheduled transactions were executed, and scheduled transactions are disabled. there should be 1 result
	//    (the standard system tx)
	// there is currently no way to determine if scheduled transactions are enabled or disabled, so
	// we simply check that there are either 1 or 2 results when there are 0 scheduled transactions.
	// eventually, we should check using the execution version from the dynamic protocol state.

	scheduledTransactionData := make(map[flow.Identifier]uint64)
	if len(scheduledTransactionIDs) == 0 {
		if len(systemChunkResults) > 2 {
			return nil, fmt.Errorf("system chunk contained %d results, and 0 scheduled transactions", len(systemChunkResults))
		}
		// this block either did not contain any scheduled transactions, or scheduled transactions were disabled
		return scheduledTransactionData, nil
	}

	// if there were scheduled transactions, there should be exactly 2 more results than there were
	// scheduled transactions.
	if len(scheduledTransactionIDs) != len(systemChunkResults)-2 {
		return nil, fmt.Errorf("system chunk contained %d results, but found %d scheduled callbacks", len(systemChunkResults), len(scheduledTransactionIDs))
	}

	// reconstruct the system collection, and verify that the results match the expected transaction
	systemCollection, err := blueprints.SystemCollection(chainID.Chain(), pendingExecutionEvents)
	if err != nil {
		return nil, fmt.Errorf("could not construct system collection: %w", err)
	}

	// sanity check that the following loop will behave as expected. since we already check the expected
	// number of results, this should never fail unless expectations about the number of tx changes
	// and there is a bug.
	if len(systemChunkResults) != len(systemCollection.Transactions) {
		return nil, fmt.Errorf("system chunk contained %d results, but expected %d", len(systemChunkResults), len(systemCollection.Transactions))
	}

	for i, tx := range systemCollection.Transactions {
		txID := tx.ID()
		if txID != systemChunkResults[i].TransactionID {
			return nil, fmt.Errorf("system chunk result at index %d does not match expected. got: %v, expected: %v", i, systemChunkResults[i].TransactionID, txID)
		}
		if i > 0 && i < len(systemChunkResults)-1 {
			scheduledTransactionData[txID] = scheduledTransactionIDs[i-1]
		}
	}

	return scheduledTransactionData, nil
}

func (c *IndexerCore) updateProgramCache(header *flow.Header, events []flow.Event, collections []*flow.Collection) error {
	if c.derivedChainData == nil {
		return nil
	}

	derivedBlockData := c.derivedChainData.GetOrCreateDerivedBlockData(
		header.ID(),
		header.ParentID,
	)

	// get a list of all contracts that were updated in this block
	updatedContracts, err := findContractUpdates(events)
	if err != nil {
		return fmt.Errorf("could not find contract updates for block %d: %w", header.Height, err)
	}

	// invalidate cache entries for all modified programs
	tx, err := derivedBlockData.NewDerivedTransactionData(0, 0)
	if err != nil {
		return fmt.Errorf("could not create derived transaction data for block %d: %w", header.Height, err)
	}

	tx.AddInvalidator(&accessInvalidator{
		programs: &programInvalidator{
			invalidated:   updatedContracts,
			invalidateAll: hasAuthorizedTransaction(collections, c.serviceAddress),
		},
		executionParameters: &executionParametersInvalidator{
			invalidateAll: hasAuthorizedTransaction(collections, c.serviceAddress),
		},
	})

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("could not commit derived transaction data for block %d: %w", header.Height, err)
	}

	return nil
}

func (c *IndexerCore) indexRegisters(registers map[ledger.Path]*ledger.Payload, height uint64) error {
	regEntries := make(flow.RegisterEntries, 0, len(registers))

	for _, payload := range registers {
		k, err := payload.Key()
		if err != nil {
			return err
		}

		id, err := convert.LedgerKeyToRegisterID(k)
		if err != nil {
			return err
		}

		regEntries = append(regEntries, flow.RegisterEntry{
			Key:   id,
			Value: payload.Value(),
		})
	}

	return c.registers.Store(regEntries, height)
}

func (c *IndexerCore) indexCollection(collection *flow.Collection) error {
	lctx := c.lockManager.NewContext()
	defer lctx.Release()
	err := lctx.AcquireLock(storage.LockInsertCollection)
	if err != nil {
		return fmt.Errorf("could not acquire lock for indexing collections: %w", err)
	}

	err = IndexCollection(lctx, collection, c.collections, c.log, c.collectionExecutedMetric)
	if err != nil {
		return fmt.Errorf("could not handle collection")
	}
	return nil
}

// IndexCollection handles the response of the collection request made earlier when a block was received.
//
// No error returns are expected during normal operations.
func IndexCollection(
	lctx lockctx.Proof,
	collection *flow.Collection,
	collections storage.Collections,
	logger zerolog.Logger,
	collectionExecutedMetric module.CollectionExecutedMetric,
) error {

	// FIX: we can't index guarantees here, as we might have more than one block
	// with the same collection as long as it is not finalized

	// store the collection, including constituent transactions, and index transactionID -> collectionID
	light, err := collections.StoreAndIndexByTransaction(lctx, collection)
	if err != nil {
		return err
	}

	collectionExecutedMetric.CollectionFinalized(light)
	collectionExecutedMetric.CollectionExecuted(light)
	return nil
}
