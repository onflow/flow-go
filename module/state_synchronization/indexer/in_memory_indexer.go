package indexer

import (
	"fmt"
	"time"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage"
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
	results         *unsynchronized.LightTransactionResults
	txResultErrMsgs *unsynchronized.TransactionResultErrorMessages
	executionResult *flow.ExecutionResult
	header          *flow.Header
	lockManager     storage.LockManager
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
	results *unsynchronized.LightTransactionResults,
	txResultErrMsgs *unsynchronized.TransactionResultErrorMessages,
	executionResult *flow.ExecutionResult,
	header *flow.Header,
	lockManager storage.LockManager,
) *InMemoryIndexer {
	indexer := &InMemoryIndexer{
		log:             log.With().Str("component", "in_memory_indexer").Logger(),
		registers:       registers,
		events:          events,
		collections:     collections,
		results:         results,
		txResultErrMsgs: txResultErrMsgs,
		executionResult: executionResult,
		header:          header,
		lockManager:     lockManager,
	}

	indexer.log.Info().
		Uint64("latest_height", header.Height).
		Msg("indexer initialized")

	return indexer
}

// IndexTxResultErrorMessagesData index transaction result error messages
// No errors are expected during normal operation.
func (i *InMemoryIndexer) IndexTxResultErrorMessagesData(txResultErrMsgs []flow.TransactionResultErrorMessage) error {
	if err := i.txResultErrMsgs.Store(i.executionResult.BlockID, txResultErrMsgs); err != nil {
		return fmt.Errorf("could not index transaction result error messages: %w", err)
	}
	return nil
}

// IndexBlockData indexes all execution block data.
// No errors are expected during normal operation.
func (i *InMemoryIndexer) IndexBlockData(data *execution_data.BlockExecutionDataEntity) error {
	log := i.log.With().Hex("block_id", logging.ID(data.BlockID)).Logger()
	log.Debug().Msg("indexing block data")

	if i.executionResult.BlockID != data.BlockID {
		return fmt.Errorf("invalid block execution data. expected block_id=%s, actual block_id=%s", i.executionResult.BlockID, data.BlockID)
	}

	start := time.Now()

	events := make([]flow.Event, 0)
	results := make([]flow.LightTransactionResult, 0)
	regEntries := make(flow.RegisterEntries, 0)
	indexedCollections := 0

	lctx := i.lockManager.NewContext()
	err := lctx.AcquireLock(storage.LockInsertCollection)
	if err != nil {
		return fmt.Errorf("could not acquire lock for collection insert: %w", err)
	}
	defer lctx.Release()

	for idx, chunk := range data.ChunkExecutionDatas {
		// Collect events
		events = append(events, chunk.Events...)

		// Collect transaction results
		results = append(results, chunk.TransactionResults...)

		// Process collections (except system chunk)
		if idx < len(data.ChunkExecutionDatas)-1 {
			if err := i.indexCollection(lctx, chunk.Collection); err != nil {
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
			mapping := make(map[ledger.Path]*ledger.Payload)
			for i, path := range chunk.TrieUpdate.Paths {
				mapping[path] = chunk.TrieUpdate.Payloads[i]
			}

			// Convert final payloads to register entries
			for _, payload := range mapping {
				entry, err := i.toRegisterEntry(payload)
				if err != nil {
					return fmt.Errorf("could not convert payload to register entry: %w", err)
				}
				regEntries = append(regEntries, *entry)
			}
		}
	}

	if err := i.events.Store(data.BlockID, []flow.EventsList{events}); err != nil {
		return fmt.Errorf("could not index events: %w", err)
	}

	if err := i.results.Store(data.BlockID, results); err != nil {
		return fmt.Errorf("could not index transaction results: %w", err)
	}

	if err := i.registers.Store(regEntries, i.header.Height); err != nil {
		return fmt.Errorf("could not index registers: %w", err)
	}

	log.Debug().
		Dur("duration_ms", time.Since(start)).
		Int("event_count", len(events)).
		Int("register_count", len(regEntries)).
		Int("result_count", len(results)).
		Int("collection_count", indexedCollections).
		Msg("indexed block data")

	return nil
}

// toRegisterEntry converts a ledger payload to a register entry.
//
// No error returns are expected during normal operation.
func (i *InMemoryIndexer) toRegisterEntry(payload *ledger.Payload) (*flow.RegisterEntry, error) {
	k, err := payload.Key()
	if err != nil {
		return nil, fmt.Errorf("failed to get ledger key: %w", err)
	}

	id, err := convert.LedgerKeyToRegisterID(k)
	if err != nil {
		return nil, fmt.Errorf("failed to convert ledger key to register id: %w", err)
	}

	return &flow.RegisterEntry{
		Key:   id,
		Value: payload.Value(),
	}, nil
}

// indexCollection stores a collection and its associated transactions.
//
// No error returns are expected during normal operation.
func (i *InMemoryIndexer) indexCollection(lctx lockctx.Proof, collection *flow.Collection) error {
	_, err := i.collections.StoreAndIndexByTransaction(lctx, collection)
	return err
}
