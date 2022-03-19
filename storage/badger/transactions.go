package badger

import (
	"fmt"
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// Transactions ...
type Transactions struct {
	db         *badger.DB
	cache      *Cache
	indexCache *Cache
}

// NewTransactions ...
func NewTransactions(cacheMetrics module.CacheMetrics, db *badger.DB) *Transactions {
	store := func(key interface{}, val interface{}) func(*transaction.Tx) error {
		txID := key.(flow.Identifier)
		flowTx := val.(*flow.TransactionBody)
		return transaction.WithTx(operation.SkipDuplicates(operation.InsertTransaction(txID, flowTx)))
	}

	storeIndex := func(key interface{}, val interface{}) func(*transaction.Tx) error {
		blockIdIndexKey := key.(string)
		txId := val.(flow.Identifier)
		return transaction.WithTx(operation.SkipDuplicates(operation.InsertTransactionByIndex(blockIdIndexKey, txId)))
	}

	retrieve := func(key interface{}) func(tx *badger.Txn) (interface{}, error) {
		txID := key.(flow.Identifier)
		var flowTx flow.TransactionBody
		return func(tx *badger.Txn) (interface{}, error) {
			err := operation.RetrieveTransaction(txID, &flowTx)(tx)
			return &flowTx, err
		}
	}

	retrieveIndex := func(key interface{}) func(tx *badger.Txn) (interface{}, error) {
		blockId, txIndex, err := KeyToBlockIDIndex(key.(string))
		if err != nil {

		}
		var flowTx flow.TransactionBody
		return func(tx *badger.Txn) (interface{}, error) {
			err := operation.RetrieveTransactionIDByIndex(blockId, txIndex, &flowTx)(tx)
			return &flowTx, err
		}
	}

	t := &Transactions{
		db: db,
		cache: newCache(cacheMetrics, metrics.ResourceTransaction,
			withLimit(flow.DefaultTransactionExpiry+100),
			withStore(store),
			withRetrieve(retrieve)),
		indexCache: newCache(cacheMetrics, metrics.ResourceTransaction,
			withLimit(flow.DefaultTransactionExpiry+100),
			withStore(storeIndex),
			withRetrieve(retrieveIndex)),
	}

	return t
}

// Store ...
func (t *Transactions) Store(flowTx *flow.TransactionBody) error {
	return operation.RetryOnConflictTx(t.db, transaction.Update, t.storeTx(flowTx))
}

func (t *Transactions) StoreTxIDByBlockIDTxIndex(blockID flow.Identifier, txTndex uint32, txID flow.Identifier) error {
	return operation.RetryOnConflictTx(t.db, transaction.Update, t.storeTxIDByIndex(blockID, txTndex, txID))
}

// ByID ...
func (t *Transactions) ByID(txID flow.Identifier) (*flow.TransactionBody, error) {
	tx := t.db.NewTransaction(false)
	defer tx.Discard()
	return t.retrieveTx(txID)(tx)
}

// TransactionIDByBlockIDIndex ...
func (t *Transactions) TransactionIDByBlockIDIndex(blockID flow.Identifier, txIndex uint32) (*flow.Identifier, error) {
	tx := t.db.NewTransaction(false)
	defer tx.Discard()
	return t.retrieveTxId(blockID, txIndex)(tx)
}

func (t *Transactions) storeTx(flowTx *flow.TransactionBody) func(*transaction.Tx) error {
	return t.cache.PutTx(flowTx.ID(), flowTx)
}

func (t *Transactions) storeTxIDByIndex(
	blockID flow.Identifier,
	txIndex uint32,
	txID flow.Identifier,
) func(*transaction.Tx) error {
	key := KeyFromBlockIDIndex(blockID, txIndex)
	return t.indexCache.PutTx(key, txID)
}

func (t *Transactions) retrieveTx(txID flow.Identifier) func(*badger.Txn) (*flow.TransactionBody, error) {
	return func(tx *badger.Txn) (*flow.TransactionBody, error) {
		val, err := t.cache.Get(txID)(tx)
		if err != nil {
			return nil, err
		}
		return val.(*flow.TransactionBody), err
	}
}

func (t *Transactions) retrieveTxId(blockID flow.Identifier, txIndex uint32) func(*badger.Txn) (*flow.Identifier, error) {
	return func(tx *badger.Txn) (*flow.Identifier, error) {
		key := KeyFromBlockIDIndex(blockID, txIndex)
		val, err := t.indexCache.Get(key)(tx)
		if err != nil {
			return nil, err
		}
		transactionResult, ok := val.(flow.Identifier)
		if !ok {
			return nil, fmt.Errorf("could not convert transaction ID: %w", err)
		}
		return &transactionResult, nil
	}
}
