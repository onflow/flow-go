package indexer

import (
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/logging"
)

// IndexerCore indexes the execution state.
type IndexerCore struct {
	log     zerolog.Logger
	metrics module.ExecutionStateIndexerMetrics

	registers    storage.RegisterIndex
	headers      storage.Headers
	events       storage.Events
	collections  storage.Collections
	transactions storage.Transactions
	results      storage.LightTransactionResults
	batcher      bstorage.BatchBuilder

	collectionExecutedMetric module.CollectionExecutedMetric

	chain            flow.Chain
	derivedChainData *derived.DerivedChainData
}

// New execution state indexer used to ingest block execution data and index it by height.
// The passed RegisterIndex storage must be populated to include the first and last height otherwise the indexer
// won't be initialized to ensure we have bootstrapped the storage first.
func New(
	log zerolog.Logger,
	metrics module.ExecutionStateIndexerMetrics,
	batcher bstorage.BatchBuilder,
	registers storage.RegisterIndex,
	headers storage.Headers,
	events storage.Events,
	collections storage.Collections,
	transactions storage.Transactions,
	results storage.LightTransactionResults,
	collectionExecutedMetric module.CollectionExecutedMetric,
) (*IndexerCore, error) {
	log = log.With().Str("component", "execution_indexer").Logger()
	metrics.InitializeLatestHeight(registers.LatestHeight())

	log.Info().
		Uint64("first_height", registers.FirstHeight()).
		Uint64("latest_height", registers.LatestHeight()).
		Msg("indexer initialized")

	return &IndexerCore{
		log:                      log,
		metrics:                  metrics,
		batcher:                  batcher,
		registers:                registers,
		headers:                  headers,
		events:                   events,
		collections:              collections,
		transactions:             transactions,
		results:                  results,
		collectionExecutedMetric: collectionExecutedMetric,
	}, nil
}

// RegisterValue retrieves register values by the register IDs at the provided block height.
// Even if the register wasn't indexed at the provided height, returns the highest height the register was indexed at.
// If a register is not found it will return a nil value and not an error.
// Expected errors:
// - storage.ErrHeightNotIndexed if the given height was not indexed yet or lower than the first indexed height.
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
// Expected errors:
// - storage.ErrNotFound if the block for execution data was not found
func (c *IndexerCore) IndexBlockData(data *execution_data.BlockExecutionDataEntity) error {
	block, err := c.headers.ByBlockID(data.BlockID)
	if err != nil {
		return fmt.Errorf("could not get the block by ID %s: %w", data.BlockID, err)
	}

	lg := c.log.With().
		Hex("block_id", logging.ID(data.BlockID)).
		Uint64("height", block.Height).
		Logger()

	lg.Debug().Msgf("indexing new block")

	// the height we are indexing must be exactly one bigger or same as the latest height indexed from the storage
	latest := c.registers.LatestHeight()
	if block.Height != latest+1 && block.Height != latest {
		return fmt.Errorf("must index block data with the next height %d, but got %d", latest+1, block.Height)
	}

	// allow rerunning the indexer for same height since we are fetching height from register storage, but there are other storages
	// for indexing resources which might fail to update the values, so this enables rerunning and reindexing those resources
	if block.Height == latest {
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

		batch := bstorage.NewBatch(c.batcher)

		err := c.events.BatchStore(data.BlockID, []flow.EventsList{events}, batch)
		if err != nil {
			return fmt.Errorf("could not index events at height %d: %w", block.Height, err)
		}

		err = c.results.BatchStore(data.BlockID, results, batch)
		if err != nil {
			return fmt.Errorf("could not index transaction results at height %d: %w", block.Height, err)
		}

		batch.Flush()
		if err != nil {
			return fmt.Errorf("batch flush error: %w", err)
		}

		eventCount = len(events)
		resultCount = len(results)

		lg.Debug().
			Int("event_count", eventCount).
			Int("result_count", resultCount).
			Dur("duration_ms", time.Since(start)).
			Msg("indexed badger data")

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
				err := HandleCollection(chunk.Collection, c.collections, c.transactions, c.log, c.collectionExecutedMetric)
				if err != nil {
					return fmt.Errorf("could not handle collection")
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

		err = c.indexRegisters(payloads, block.Height)
		if err != nil {
			return fmt.Errorf("could not index register payloads at height %d: %w", block.Height, err)
		}

		err = c.updateProgramCache(block.Height, events, collections)
		if err != nil {
			return fmt.Errorf("could not update program cache at height %d: %w", block.Height, err)
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
		return fmt.Errorf("failed to index block data at height %d: %w", block.Height, err)
	}

	c.metrics.BlockIndexed(block.Height, time.Since(start), eventCount, registerCount, resultCount)
	lg.Debug().
		Dur("duration_ms", time.Since(start)).
		Msg("indexed block data")

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

// HandleCollection handles the response of the a collection request made earlier when a block was received.
// No errors expected during normal operations.
func HandleCollection(
	collection *flow.Collection,
	collections storage.Collections,
	transactions storage.Transactions,
	logger zerolog.Logger,
	collectionExecutedMetric module.CollectionExecutedMetric,
) error {

	light := collection.Light()

	collectionExecutedMetric.CollectionFinalized(light)
	collectionExecutedMetric.CollectionExecuted(light)

	// FIX: we can't index guarantees here, as we might have more than one block
	// with the same collection as long as it is not finalized

	// store the light collection (collection minus the transaction body - those are stored separately)
	// and add transaction ids as index
	err := collections.StoreLightAndIndexByTransaction(&light)
	if err != nil {
		// ignore collection if already seen
		if errors.Is(err, storage.ErrAlreadyExists) {
			logger.Debug().
				Hex("collection_id", logging.Entity(light)).
				Msg("collection is already seen")
			return nil
		}
		return err
	}

	// now store each of the transaction body
	for _, tx := range collection.Transactions {
		err := transactions.Store(tx)
		if err != nil {
			return fmt.Errorf("could not store transaction (%x): %w", tx.ID(), err)
		}
	}

	return nil
}

func (c *IndexerCore) updateProgramCache(height uint64, events []flow.Event, collections []*flow.Collection) error {
	header, err := c.headers.ByHeight(height)
	if err != nil {
		return fmt.Errorf("could not get the header by height %d: %w", height, err)
	}

	derivedBlockData := c.derivedChainData.GetOrCreateDerivedBlockData(
		header.ID(),
		header.ParentID,
	)

	// configure the derived transaction data to allow scripts to cache programs
	derivedBlockData.AllowWritesInReadonlyTransaction()

	// get a list of all contracts that were updated in this block
	invalidatedPrograms, err := findContractUpdates(events)
	if err != nil {
		return fmt.Errorf("could not find contract updates for block %d: %w", height, err)
	}

	// invalidate cache entries for all modified programs
	tx, err := derivedBlockData.NewDerivedTransactionData(0, 0)
	if err != nil {
		return fmt.Errorf("could not create derived transaction data for block %d: %w", height, err)
	}

	tx.AddInvalidator(&accessInvalidator{
		programs: &programInvalidator{
			invalidated: invalidatedPrograms,
		},
		meterParamOverrides: &meterParamOverridesInvalidator{
			invalidateAll: hasAuthorizedTransaction(collections, c.chain.ServiceAddress()),
		},
	})

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("could not commit derived transaction data for block %d: %w", height, err)
	}

	return nil
}
