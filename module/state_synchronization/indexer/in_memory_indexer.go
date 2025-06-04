package indexer

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage/store/inmemory/unsynchronized"
	"github.com/onflow/flow-go/utils/logging"
)

// InMemoryIndexer handles indexing of block execution data in memory.
// It stores data in unsynchronized in-memory caches that are designed
// to be populated once before being read.
type InMemoryIndexer struct {
	log             zerolog.Logger
	registers       *unsynchronized.Registers
	events          *unsynchronized.Events
	collections     *unsynchronized.Collections
	transactions    *unsynchronized.Transactions
	results         *unsynchronized.LightTransactionResults
	txResultErrMsgs *unsynchronized.TransactionResultErrorMessages
	executionResult *flow.ExecutionResult
	header          *flow.Header
}

// NewInMemoryIndexer creates a new indexer that uses in-memory storage implementations.
// This is designed for processing unsealed blocks in the optimistic syncing pipeline.
// The caches are created externally and passed to the indexer, as they will also be used
// by the persister to save data permanently when a block is sealed.
func NewInMemoryIndexer(
	log zerolog.Logger,
	registers *unsynchronized.Registers,
	events *unsynchronized.Events,
	collections *unsynchronized.Collections,
	transactions *unsynchronized.Transactions,
	results *unsynchronized.LightTransactionResults,
	txResultErrMsgs *unsynchronized.TransactionResultErrorMessages,
	executionResult *flow.ExecutionResult,
	header *flow.Header,
) *InMemoryIndexer {
	indexer := &InMemoryIndexer{
		log:             log.With().Str("component", "in_memory_indexer").Logger(),
		registers:       registers,
		events:          events,
		collections:     collections,
		transactions:    transactions,
		results:         results,
		txResultErrMsgs: txResultErrMsgs,
		executionResult: executionResult,
		header:          header,
	}

	indexer.log.Info().
		Uint64("latest_height", header.Height).
		Msg("indexer initialized")

	return indexer
}

// IndexTxResultErrorMessagesData index transaction result error messages
func (i *InMemoryIndexer) IndexTxResultErrorMessagesData(txResultErrMsgs []flow.TransactionResultErrorMessage) error {
	if err := i.txResultErrMsgs.Store(i.executionResult.BlockID, txResultErrMsgs); err != nil {
		return fmt.Errorf("could not index transaction result error messages: %w", err)
	}
	return nil
}

// IndexBlockData indexes all execution block data.
// Expected errors:
// - convert.UnexpectedLedgerKeyFormat if the key is not in the expected format
// - storage.ErrHeightNotIndexed if the given height does not match the storage's block height.
func (i *InMemoryIndexer) IndexBlockData(data *execution_data.BlockExecutionDataEntity) error {
	log := i.log.With().Hex("block_id", logging.ID(data.BlockID)).Logger()
	log.Debug().Msg("indexing block data")

	if i.executionResult.BlockID != data.BlockID {
		return fmt.Errorf("invalid block execution data. expected block_id=%s, actual block_id=%s", i.executionResult.BlockID, data.BlockID)
	}

	start := time.Now()

	events := make([]flow.Event, 0)
	results := make([]flow.LightTransactionResult, 0)
	registers := make(map[ledger.Path]*ledger.Payload)
	indexedCollections := 0

	// Process all chunk data in a single pass
	for idx, chunk := range data.ChunkExecutionDatas {
		// Collect events
		events = append(events, chunk.Events...)

		// Collect transaction results
		results = append(results, chunk.TransactionResults...)

		// Process collections (except system chunk)
		if idx < len(data.ChunkExecutionDatas)-1 {
			if err := i.indexCollection(chunk.Collection); err != nil {
				return fmt.Errorf("could not handle collection: %w", err)
			}
			indexedCollections++
		}

		// Process register updates
		if chunk.TrieUpdate != nil {
			// Verify trie update integrity
			if len(chunk.TrieUpdate.Paths) != len(chunk.TrieUpdate.Payloads) {
				return fmt.Errorf("update paths length is %d and registers length is %d and they don't match",
					len(chunk.TrieUpdate.Paths), len(chunk.TrieUpdate.Payloads))
			}

			// Collect registers (last one for a path wins)
			for i, path := range chunk.TrieUpdate.Paths {
				registers[path] = chunk.TrieUpdate.Payloads[i]
			}
		}
	}

	if err := i.events.Store(data.BlockID, []flow.EventsList{events}); err != nil {
		return fmt.Errorf("could not index events: %w", err)
	}

	if err := i.results.Store(data.BlockID, results); err != nil {
		return fmt.Errorf("could not index transaction results: %w", err)
	}

	if err := i.indexRegisters(registers, i.header.Height); err != nil {
		return fmt.Errorf("could not index registers: %w", err)
	}

	duration := time.Since(start)

	log.Debug().
		Dur("duration_ms", duration).
		Int("event_count", len(events)).
		Int("register_count", len(registers)).
		Int("result_count", len(results)).
		Int("collection_count", indexedCollections).
		Msg("indexed block data")

	return nil
}

// indexRegisters processes register payloads and stores them in the register database.
// Expected errors:
// - convert.UnexpectedLedgerKeyFormat if the key is not in the expected format
// - storage.ErrHeightNotIndexed if the given height does not match the storage's block height.
func (i *InMemoryIndexer) indexRegisters(registers map[ledger.Path]*ledger.Payload, height uint64) error {
	regEntries := make(flow.RegisterEntries, 0, len(registers))

	for _, register := range registers {
		k, err := register.Key()
		if err != nil {
			return fmt.Errorf("failed to get ledger key: %w", err)
		}

		id, err := convert.LedgerKeyToRegisterID(k)
		if err != nil {
			return fmt.Errorf("failed to convert ledger key to register id: %w", err)
		}

		regEntries = append(regEntries, flow.RegisterEntry{
			Key:   id,
			Value: register.Value(),
		})
	}

	return i.registers.Store(regEntries, height)
}

// indexCollection processes a collection and its associated transactions.
// No errors are expected during normal operation.
func (i *InMemoryIndexer) indexCollection(collection *flow.Collection) error {
	// Store the full collection
	if err := i.collections.Store(collection); err != nil {
		return fmt.Errorf("failed to store collection: %w", err)
	}

	// Store the light collection
	light := collection.Light()
	if err := i.collections.StoreLightAndIndexByTransaction(&light); err != nil {
		return fmt.Errorf("failed to store light collection and transaction index: %w", err)
	}

	// Store each of the transaction bodies
	for _, tx := range collection.Transactions {
		if err := i.transactions.Store(tx); err != nil {
			return fmt.Errorf("could not store transaction (%s): %w", tx.ID().String(), err)
		}
	}

	return nil
}
