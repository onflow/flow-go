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
	cache      *Cache[TwoIdentifier, flow.TransactionResult]       // Key: blockID + txID
	indexCache *Cache[IdentifierAndUint32, flow.TransactionResult] // Key: blockID + txIndex
	blockCache *Cache[flow.Identifier, []flow.TransactionResult]   // Key: blockID
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

func NewTransactionResults(collector module.CacheMetrics, db storage.DB, transactionResultsCacheSize uint) *TransactionResults {
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

	return &TransactionResults{
		db: db,
		cache: newCache(collector, metrics.ResourceTransactionResults,
			withLimit[TwoIdentifier, flow.TransactionResult](transactionResultsCacheSize),
			withStore(noopStore[TwoIdentifier, flow.TransactionResult]),
			withRetrieve(retrieve),
		),
		indexCache: newCache(collector, metrics.ResourceTransactionResultIndices,
			withLimit[IdentifierAndUint32, flow.TransactionResult](transactionResultsCacheSize),
			withStore(noopStore[IdentifierAndUint32, flow.TransactionResult]),
			withRetrieve(retrieveIndex),
		),
		blockCache: newCache(collector, metrics.ResourceTransactionResultIndices,
			withLimit[flow.Identifier, []flow.TransactionResult](transactionResultsCacheSize),
			withStore(noopStore[flow.Identifier, []flow.TransactionResult]),
			withRetrieve(retrieveForBlock),
		),
	}
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

// BatchRemoveByBlockID batch removes transaction results by block ID
func (tr *TransactionResults) BatchRemoveByBlockID(blockID flow.Identifier, batch storage.ReaderBatchWriter) error {
	// TODO: Remove records from tr.cache by prefix (blockID) and optimize removal.
	//
	// Currently, tr.cache can be out of sync with underlying database
	// when transaction results are removed by this functions.
	//
	// Even though cache.RemoveFunc() is maybe fast enough for
	// a 1000-item cache, using 10K-item cache size and pruning
	// multiple blocks in one batch can slow down commit phase
	// unless we optimize for that use case.
	//
	// To unblock PR onflow/flow-go#7324 which has several fixes,
	// remove-by-prefix optimization will be tracked in separate issue/PR.
	//
	// Code fix (below) and test are commented out (not deleted) because
	// we can use the code as a quick stop-gap fix if needed, and
	// we can reuse the test even if a new (faster) approach is implemented.
	/*
		storage.OnCommitSucceed(batch, func() {
			keyPrefix := KeyFromBlockID(blockID)

			tr.cache.RemoveFunc(func(key string) bool {
				return strings.HasPrefix(key, keyPrefix)
			})
		})
	*/

	return operation.BatchRemoveTransactionResultsByBlockID(blockID, batch)
}
