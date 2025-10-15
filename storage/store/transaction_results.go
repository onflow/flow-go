package store

import (
	"encoding/binary"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

var _ storage.TransactionResults = (*TransactionResults)(nil)

type TwoIdentifier [flow.IdentifierLen * 2]byte
type IdentifierAndUint32 [flow.IdentifierLen + 4]byte

type TransactionResults struct {
	db         storage.DB
	cache      *GroupCache[flow.Identifier, TwoIdentifier, flow.TransactionResult]       // Key: blockID + txID
	indexCache *GroupCache[flow.Identifier, IdentifierAndUint32, flow.TransactionResult] // Key: blockID + txIndex
	blockCache *Cache[flow.Identifier, []flow.TransactionResult]                         // Key: blockID
}

func KeyFromBlockIDTransactionID(blockID flow.Identifier, txID flow.Identifier) TwoIdentifier {
	var key TwoIdentifier
	n := copy(key[:], blockID[:])
	copy(key[n:], txID[:])
	return key
}

func KeyFromBlockIDIndex(blockID flow.Identifier, txIndex uint32) IdentifierAndUint32 {
	var key IdentifierAndUint32
	n := copy(key[:], blockID[:])
	binary.BigEndian.PutUint32(key[n:], txIndex)
	return key
}

func KeyToBlockIDTransactionID(key TwoIdentifier) (flow.Identifier, flow.Identifier) {
	blockID := flow.Identifier(key[:flow.IdentifierLen])
	transactionID := flow.Identifier(key[flow.IdentifierLen:])
	return blockID, transactionID
}

func KeyToBlockIDIndex(key IdentifierAndUint32) (flow.Identifier, uint32) {
	blockID := flow.Identifier(key[:flow.IdentifierLen])
	txIndex := binary.BigEndian.Uint32(key[flow.IdentifierLen:])
	return blockID, txIndex
}

func FirstIDFromTwoIdentifier(key TwoIdentifier) flow.Identifier {
	return flow.Identifier(key[:flow.IdentifierLen])
}

func IDFromIdentifierAndUint32(key IdentifierAndUint32) flow.Identifier {
	return flow.Identifier(key[:flow.IdentifierLen])
}

func NewTransactionResults(collector module.CacheMetrics, db storage.DB, transactionResultsCacheSize uint) (*TransactionResults, error) {
	retrieve := func(r storage.Reader, key TwoIdentifier) (flow.TransactionResult, error) {
		blockID, txID := KeyToBlockIDTransactionID(key)

		var txResult flow.TransactionResult
		err := operation.RetrieveTransactionResult(r, blockID, txID, &txResult)
		if err != nil {
			return flow.TransactionResult{}, err
		}
		return txResult, nil
	}

	retrieveIndex := func(r storage.Reader, key IdentifierAndUint32) (flow.TransactionResult, error) {
		blockID, txIndex := KeyToBlockIDIndex(key)

		var txResult flow.TransactionResult
		err := operation.RetrieveTransactionResultByIndex(r, blockID, txIndex, &txResult)
		if err != nil {
			return flow.TransactionResult{}, err
		}
		return txResult, nil
	}

	retrieveForBlock := func(r storage.Reader, blockID flow.Identifier) ([]flow.TransactionResult, error) {
		var txResults []flow.TransactionResult
		err := operation.LookupTransactionResultsByBlockIDUsingIndex(r, blockID, &txResults)
		if err != nil {
			return nil, err
		}
		return txResults, nil
	}

	cache, err := newGroupCache(
		collector,
		metrics.ResourceTransactionResults,
		FirstIDFromTwoIdentifier,
		withLimit[TwoIdentifier, flow.TransactionResult](transactionResultsCacheSize),
		withStore(noopStore[TwoIdentifier, flow.TransactionResult]),
		withRetrieve(retrieve),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction results group cache: %w", err)
	}

	indexCache, err := newGroupCache(
		collector,
		metrics.ResourceTransactionResultIndices,
		IDFromIdentifierAndUint32,
		withLimit[IdentifierAndUint32, flow.TransactionResult](transactionResultsCacheSize),
		withStore(noopStore[IdentifierAndUint32, flow.TransactionResult]),
		withRetrieve(retrieveIndex),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction index group cache: %w", err)
	}

	return &TransactionResults{
		db:         db,
		cache:      cache,
		indexCache: indexCache,
		blockCache: newCache(
			collector,
			metrics.ResourceTransactionResultIndices,
			withLimit[flow.Identifier, []flow.TransactionResult](transactionResultsCacheSize),
			withStore(noopStore[flow.Identifier, []flow.TransactionResult]),
			withRetrieve(retrieveForBlock),
		),
	}, nil
}

// BatchStore will store the transaction results for the given block ID in a batch
func (tr *TransactionResults) BatchStore(blockID flow.Identifier, transactionResults []flow.TransactionResult, batch storage.ReaderBatchWriter) error {
	w := batch.Writer()

	for i, result := range transactionResults {
		err := operation.InsertTransactionResult(w, blockID, &result)
		if err != nil {
			return fmt.Errorf("cannot batch insert tx result: %w", err)
		}

		err = operation.IndexTransactionResult(w, blockID, uint32(i), &result)
		if err != nil {
			return fmt.Errorf("cannot batch index tx result: %w", err)
		}
	}

	storage.OnCommitSucceed(batch, func() {
		for i, result := range transactionResults {
			key := KeyFromBlockIDTransactionID(blockID, result.TransactionID)
			// cache for each transaction, so that it's faster to retrieve
			tr.cache.Insert(key, result)

			index := uint32(i)

			keyIndex := KeyFromBlockIDIndex(blockID, index)
			tr.indexCache.Insert(keyIndex, result)
		}

		tr.blockCache.Insert(blockID, transactionResults)
	})
	return nil
}

// ByBlockIDTransactionID returns the runtime transaction result for the given block ID and transaction ID
func (tr *TransactionResults) ByBlockIDTransactionID(blockID flow.Identifier, txID flow.Identifier) (*flow.TransactionResult, error) {
	key := KeyFromBlockIDTransactionID(blockID, txID)
	transactionResult, err := tr.cache.Get(tr.db.Reader(), key)
	if err != nil {
		return nil, err
	}
	return &transactionResult, nil
}

// ByBlockIDTransactionIndex returns the runtime transaction result for the given block ID and transaction index
func (tr *TransactionResults) ByBlockIDTransactionIndex(blockID flow.Identifier, txIndex uint32) (*flow.TransactionResult, error) {
	key := KeyFromBlockIDIndex(blockID, txIndex)
	transactionResult, err := tr.indexCache.Get(tr.db.Reader(), key)
	if err != nil {
		return nil, err
	}
	return &transactionResult, nil
}

// ByBlockID gets all transaction results for a block, ordered by transaction index
func (tr *TransactionResults) ByBlockID(blockID flow.Identifier) ([]flow.TransactionResult, error) {
	transactionResults, err := tr.blockCache.Get(tr.db.Reader(), blockID)
	if err != nil {
		return nil, err
	}
	return transactionResults, nil
}

// RemoveByBlockID removes transaction results by block ID
func (tr *TransactionResults) RemoveByBlockID(blockID flow.Identifier) error {
	return tr.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return tr.BatchRemoveByBlockID(blockID, rw)
	})
}

// BatchRemoveByBlockID batch removes transaction results by block ID.
func (tr *TransactionResults) BatchRemoveByBlockID(blockID flow.Identifier, batch storage.ReaderBatchWriter) error {
	const batchDataKey = "TransactionResults.BatchRemoveByBlockID"

	// BatchRemoveByBlockID() receives ReaderBatchWriter and block ID to
	// remove the given block from the database and memory cache.
	// BatchRemoveByBlockID() can be called repeatedly with the same
	// ReaderBatchWriter and different block IDs to remove multiple blocks.
	//
	// To avoid locking TransactionResults cache for every removed block ID,
	// this function:
	// - saves and aggregates the received blockID in the ReaderBatchWriter's scope data,
	// - in the OnCommitSucceed callback, retrieves all saved block IDs and
	//   removes all cached blocks by locking the cache just once
	// After cache removal, the scoped block IDs in ReaderBatchWriter are removed.

	storage.OnCommitSucceed(batch, func() {
		batchData, _ := batch.ScopedValue(batchDataKey)

		if batchData != nil {
			batch.SetScopedValue(batchDataKey, nil)

			blockIDs := batchData.(map[flow.Identifier]struct{})

			if len(blockIDs) > 0 {
				blockIDsInSlice := make([]flow.Identifier, 0, len(blockIDs))
				for id := range blockIDs {
					blockIDsInSlice = append(blockIDsInSlice, id)
				}

				tr.cache.RemoveGroups(blockIDsInSlice)
			}
		}
	})

	saveBlockIDInBatchData(batch, batchDataKey, blockID)

	return operation.RemoveTransactionResultsByBlockID(blockID, batch)
}

func saveBlockIDInBatchData(batch storage.ReaderBatchWriter, batchDataKey string, blockID flow.Identifier) {
	var blockIDs map[flow.Identifier]struct{}

	batchValue, _ := batch.ScopedValue(batchDataKey)
	if batchValue == nil {
		blockIDs = make(map[flow.Identifier]struct{})
	} else {
		blockIDs = batchValue.(map[flow.Identifier]struct{})
	}

	blockIDs[blockID] = struct{}{}

	batch.SetScopedValue(batchDataKey, blockIDs)
}
