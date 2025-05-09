package indexer

import (
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store/inmemory/unsynchronized"
	"github.com/onflow/flow-go/utils/logging"
)

// InMemoryIndexerConfig holds configuration parameters for the in-memory indexer
type InMemoryIndexerConfig struct {
	Log          zerolog.Logger
	Metrics      module.ExecutionStateIndexerMetrics
	Registers    *unsynchronized.Registers
	Events       *unsynchronized.Events
	Collections  *unsynchronized.Collections
	Transactions *unsynchronized.Transactions
	Results      *unsynchronized.LightTransactionResults
}

// InMemoryIndexer handles indexing of block execution data in memory.
// It stores data in unsynchronized in-memory caches that are designed
// to be populated once before being read.
type InMemoryIndexer struct {
	log          zerolog.Logger
	metrics      module.ExecutionStateIndexerMetrics
	registers    *unsynchronized.Registers
	events       *unsynchronized.Events
	collections  *unsynchronized.Collections
	transactions *unsynchronized.Transactions
	results      *unsynchronized.LightTransactionResults
}

// NewInMemoryIndexer creates a new indexer that uses in-memory storage implementations.
// This is designed for processing unsealed blocks in the optimistic syncing pipeline.
// The caches are created externally and passed to the indexer, as they will also be used
// by the persister to save data permanently when a block is sealed.
func NewInMemoryIndexer(config InMemoryIndexerConfig) *InMemoryIndexer {
	indexer := &InMemoryIndexer{
		log:          config.Log.With().Str("component", "in_memory_indexer").Logger(),
		metrics:      config.Metrics,
		registers:    config.Registers,
		events:       config.Events,
		collections:  config.Collections,
		transactions: config.Transactions,
		results:      config.Results,
	}

	indexer.metrics.InitializeLatestHeight(indexer.registers.LatestHeight())

	indexer.log.Info().
		Uint64("first_height", indexer.registers.FirstHeight()).
		Uint64("latest_height", indexer.registers.LatestHeight()).
		Msg("indexer initialized")

	return indexer
}

// RegisterValue retrieves register values by the register IDs at the provided block height.
// If a register is not found it will return a nil value and not an error.
// Expected errors:
// - storage.ErrHeightNotIndexed if the given height was not indexed yet or lower than the first indexed height.
func (i *InMemoryIndexer) RegisterValue(ID flow.RegisterID, height uint64) (flow.RegisterValue, error) {
	value, err := i.registers.Get(ID, height)
	if err != nil {
		// only return an error if the error doesn't match the not found error, since we have
		// to gracefully handle not found values and instead assign nil, that is because the script executor
		// expects that behavior
		if errors.Is(err, storage.ErrNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return value, nil
}

// IndexBlockData indexes all execution block data.
// Unlike the original implementation, this doesn't use batches as the in-memory
// caches don't support batched writes.
func (i *InMemoryIndexer) IndexBlockData(data *execution_data.BlockExecutionDataEntity) error {
	log := i.log.With().Hex("block_id", logging.ID(data.BlockID)).Logger()
	log.Debug().Msgf("indexing block data")

	start := time.Now()

	// Initialize collection containers
	events := make([]flow.Event, 0)
	results := make([]flow.LightTransactionResult, 0)
	payloads := make(map[ledger.Path]*ledger.Payload)
	indexedCollections := 0

	// Process all chunk data in a single pass
	for idx, chunk := range data.ChunkExecutionDatas {
		// Collect events
		events = append(events, chunk.Events...)

		// Collect transaction results
		results = append(results, chunk.TransactionResults...)

		// Process collections (except system chunk)
		if idx < len(data.ChunkExecutionDatas)-1 {
			if err := i.handleCollection(chunk.Collection); err != nil {
				return fmt.Errorf("could not handle collection: %w", err)
			}
			indexedCollections++
		}

		// Process register updates
		if chunk.TrieUpdate != nil {
			// Verify trie update integrity
			if len(chunk.TrieUpdate.Paths) != len(chunk.TrieUpdate.Payloads) {
				return fmt.Errorf("update paths length is %d and payloads length is %d and they don't match",
					len(chunk.TrieUpdate.Paths), len(chunk.TrieUpdate.Payloads))
			}

			// Collect payloads (last one for a path wins)
			for i, path := range chunk.TrieUpdate.Paths {
				payloads[path] = chunk.TrieUpdate.Payloads[i]
			}
		}
	}

	// Store events
	if err := i.events.Store(data.BlockID, []flow.EventsList{events}); err != nil {
		return fmt.Errorf("could not index events: %w", err)
	}

	// Store transaction results
	if err := i.results.Store(data.BlockID, results); err != nil {
		return fmt.Errorf("could not index transaction results: %w", err)
	}

	// Store registers
	if err := i.indexRegisters(payloads, i.registers.LatestHeight()); err != nil {
		return fmt.Errorf("could not index register payloads: %w", err)
	}

	duration := time.Since(start)

	i.metrics.BlockIndexed(
		i.registers.LatestHeight(),
		duration,
		len(events),
		len(payloads),
		len(results),
	)

	log.Debug().
		Dur("duration_ms", duration).
		Int("event_count", len(events)).
		Int("register_count", len(payloads)).
		Int("result_count", len(results)).
		Int("collection_count", indexedCollections).
		Msg("indexed block data")

	return nil
}

// indexRegisters processes register payloads and stores them in the register database.
func (i *InMemoryIndexer) indexRegisters(registers map[ledger.Path]*ledger.Payload, height uint64) error {
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

	return i.registers.Store(regEntries, height)
}

// handleCollection processes a collection and its associated transactions.
func (i *InMemoryIndexer) handleCollection(collection *flow.Collection) error {
	if collection == nil {
		return nil
	}

	// Store the full collection
	if err := i.collections.Store(collection); err != nil {
		return err
	}

	// Store the light collection
	light := collection.Light()
	if err := i.collections.StoreLightAndIndexByTransaction(&light); err != nil {
		return err
	}

	// Store each of the transaction bodies
	for _, tx := range collection.Transactions {
		err := i.transactions.Store(tx)
		if err != nil {
			return fmt.Errorf("could not store transaction (%s): %w", tx.ID().String(), err)
		}
	}

	return nil
}
