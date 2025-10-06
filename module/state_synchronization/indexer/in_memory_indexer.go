package indexer

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

// <component_spec>
// InMemoryIndexer indexes block execution data for a single ExecutionResult into a mempool. It is
// designed to be used as part of the optimistic syncing processing pipeline, to index data for
// unsealed execution results which is eventually persisted when the execution result is sealed.
//
// The data contained within the BlockExecutionData is verified by verifications nodes as part of the
// approval process. Once the execution result is sealed, the Access node can accept it as valid
// without further verification. However, with optimistic syncing, the Access node may index data
// for execution results that are not sealed. Since this data is not certified by the protocol, it
// must not be persisted to disk. It may be used by the Access node to serve Access API requests,
// with the understanding that it may be later determined to be invalid.
//
// The provided BlockExecutionData is received over the network and its hash is compared to the value
// included in an ExecutionResult within a certified block. This guarantees it is the same data that
// was produced by an execution node whose stake is at risk if the data is incorrect. It is not
// practical for an Access node to verify all data, but the indexer may perform opportunistic checks
// to ensure the data is generally consistent.
//
// Transaction error messages are received directly from execution nodes with no protocol guarantees.
// The node must validate that there is a one-to-one mapping between failed transactions and
// transaction error messages.
// Since the error messages are requested directly from the execution nodes, it's possible that they
// are delayed. To avoid blocking the indexing process if ENs are unresponsive, the processing pipeline
// may skip the call to `ValidateTxErrors()` if the error messages are not ready. In this case, the
// the error messages may be validated and backfilled later.
// </component_spec>
//
// Safe for concurrent use.
type InMemoryIndexer struct {
	log             zerolog.Logger
	block           *flow.Block
	executionResult *flow.ExecutionResult
}

// IndexerData is the collection of data ingested by the indexer.
type IndexerData struct {
	Events       []flow.Event
	Collections  []*flow.Collection
	Transactions []*flow.TransactionBody
	Results      []flow.LightTransactionResult
	Registers    []flow.RegisterEntry
}

// NewInMemoryIndexer returns a new indexer that indexes block execution data and error messages for
// a single ExecutionResult.
func NewInMemoryIndexer(
	log zerolog.Logger,
	block *flow.Block,
	executionResult *flow.ExecutionResult,
) (*InMemoryIndexer, error) {
	if block.ID() != executionResult.BlockID {
		return nil, fmt.Errorf("block ID and execution result block ID must match")
	}

	return &InMemoryIndexer{
		log: log.With().
			Str("component", "in_memory_indexer").
			Str("execution_result_id", executionResult.ExecutionDataID.String()).
			Str("block_id", executionResult.BlockID.String()).
			Logger(),
		block:           block,
		executionResult: executionResult,
	}, nil
}

// IndexBlockData indexes all execution block data.
//
// The method is idempotent and does not modify the state of the indexer.
//
// All error returns are benign and side-effect free for the node. They indicate that the BlockExecutionData
// is inconsistent with the execution result and its block, which points to invalid data produced by
// an external node.
func (i *InMemoryIndexer) IndexBlockData(data *execution_data.BlockExecutionData) (*IndexerData, error) {
	if data.BlockID != i.executionResult.BlockID {
		return nil, fmt.Errorf("unexpected block execution data: expected block_id=%s, actual block_id=%s", i.executionResult.BlockID, data.BlockID)
	}

	// sanity check: the execution data should contain data for all collections in the block, plus
	// the system chunk
	if len(data.ChunkExecutionDatas) != len(i.block.Payload.Guarantees)+1 {
		return nil, fmt.Errorf("block execution data chunk (%d) count does not match block guarantee (%d) plus system chunk",
			len(data.ChunkExecutionDatas), len(i.block.Payload.Guarantees))
	}

	start := time.Now()
	i.log.Debug().Msg("indexing block data")

	events := make([]flow.Event, 0)
	results := make([]flow.LightTransactionResult, 0)
	collections := make([]*flow.Collection, 0)
	transactions := make([]*flow.TransactionBody, 0)
	registerUpdates := make(map[ledger.Path]*ledger.Payload)

	for idx, chunk := range data.ChunkExecutionDatas {
		events = append(events, chunk.Events...)
		results = append(results, chunk.TransactionResults...)

		// do not index tx and collections from the system chunk since they can be generated on demand
		if idx < len(data.ChunkExecutionDatas)-1 {
			collections = append(collections, chunk.Collection)
			transactions = append(transactions, chunk.Collection.Transactions...)
		}

		// sanity check: there must be a one-to-one mapping between transactions and results
		if len(chunk.Collection.Transactions) != len(chunk.TransactionResults) {
			return nil, fmt.Errorf("number of transactions (%d) does not match number of results (%d)",
				len(chunk.Collection.Transactions), len(chunk.TransactionResults))
		}

		// collect register updates
		if chunk.TrieUpdate != nil {
			// sanity check: there must be a one-to-one mapping between paths and payloads
			if len(chunk.TrieUpdate.Paths) != len(chunk.TrieUpdate.Payloads) {
				return nil, fmt.Errorf("number of ledger paths (%d) does not match number of ledger payloads (%d)",
					len(chunk.TrieUpdate.Paths), len(chunk.TrieUpdate.Payloads))
			}

			// collect registers (last update for a path within the block is persisted)
			for i, path := range chunk.TrieUpdate.Paths {
				registerUpdates[path] = chunk.TrieUpdate.Payloads[i]
			}
		}
	}

	// convert final payloads to register entries
	registerEntries := make([]flow.RegisterEntry, 0, len(registerUpdates))
	for path, payload := range registerUpdates {
		key, value, err := convert.PayloadToRegister(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to convert payload to register entry (path: %s): %w", path.String(), err)
		}

		registerEntries = append(registerEntries, flow.RegisterEntry{
			Key:   key,
			Value: value,
		})
	}

	i.log.Debug().
		Dur("duration_ms", time.Since(start)).
		Int("event_count", len(events)).
		Int("register_count", len(registerEntries)).
		Int("result_count", len(results)).
		Int("collection_count", len(collections)).
		Msg("indexed block data")

	return &IndexerData{
		Events:       events,
		Collections:  collections,
		Transactions: transactions,
		Results:      results,
		Registers:    registerEntries,
	}, nil
}

// ValidateTxErrors validates that the transaction results and error messages are consistent, and
// returns an error if they are not.
//
// All error returns are benign and side-effect free for the node. They indicate that the transaction
// results and error messages are inconsistent, which points to invalid data produced by an external
// node.
func ValidateTxErrors(results []flow.LightTransactionResult, txResultErrMsgs []flow.TransactionResultErrorMessage) error {
	txWithErrors := make(map[flow.Identifier]bool)
	for _, txResult := range txResultErrMsgs {
		txWithErrors[txResult.TransactionID] = true
	}

	failedCount := 0
	for _, txResult := range results {
		if !txResult.Failed {
			continue
		}
		failedCount++

		if !txWithErrors[txResult.TransactionID] {
			return fmt.Errorf("transaction %s failed but no error message was provided", txResult.TransactionID)
		}
	}

	// make sure there are not extra error messages
	if failedCount != len(txWithErrors) {
		return fmt.Errorf("number of failed transactions (%d) does not match number of transaction error messages (%d)", failedCount, len(txWithErrors))
	}

	return nil
}
