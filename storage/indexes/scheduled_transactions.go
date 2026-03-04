package indexes

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/jordanschalm/lockctx"
	"github.com/vmihailenco/msgpack/v4"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/indexes/iterator"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/utils/visited"
)

const (
	// scheduledTxPrimaryKeyLen is [code(1)][~id(8)] = 9 bytes
	scheduledTxPrimaryKeyLen = 1 + heightLen
	// scheduledTxByAddrKeyLen is [code(1)][address(8)][~id(8)] = 17 bytes
	scheduledTxByAddrKeyLen = 1 + flow.AddressLength + heightLen
)

// ScheduledTransactionsIndex implements [storage.ScheduledTransactionsIndex] using Pebble.
//
// Primary key format:    [codeScheduledTransaction][~id]                → ScheduledTransaction value
// By-address key format: [codeScheduledTransactionByAddress][addr][~id] → nil (key-only)
//
// One's complement of id (~id) gives descending iteration order in ascending byte space.
//
// All read methods are safe for concurrent access. Write methods must be called sequentially.
type ScheduledTransactionsIndex struct {
	*IndexState
}

var _ storage.ScheduledTransactionsIndex = (*ScheduledTransactionsIndex)(nil)

// NewScheduledTransactionsIndex creates a new index backed by db.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotBootstrapped]: if the index has not been initialized
func NewScheduledTransactionsIndex(db storage.DB) (*ScheduledTransactionsIndex, error) {
	state, err := NewIndexState(
		db,
		storage.LockIndexScheduledTransactionsIndex,
		keyScheduledTxFirstHeightKey,
		keyScheduledTxLatestHeightKey,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create index state: %w", err)
	}
	return &ScheduledTransactionsIndex{IndexState: state}, nil
}

// BootstrapScheduledTransactions initializes the index with the given start height and initial
// scheduled transactions, and returns a new [ScheduledTransactionsIndex].
// The caller must hold the [storage.LockIndexScheduledTransactionsIndex] lock until the batch
// is committed.
//
// Expected error returns during normal operation:
//   - [storage.ErrAlreadyExists]: if any bounds key already exists
func BootstrapScheduledTransactions(
	lctx lockctx.Proof,
	rw storage.ReaderBatchWriter,
	db storage.DB,
	initialStartHeight uint64,
	scheduledTxs []access.ScheduledTransaction,
) (*ScheduledTransactionsIndex, error) {
	state, err := BootstrapIndexState(
		lctx,
		rw,
		db,
		storage.LockIndexScheduledTransactionsIndex,
		keyScheduledTxFirstHeightKey,
		keyScheduledTxLatestHeightKey,
		initialStartHeight,
	)
	if err != nil {
		return nil, fmt.Errorf("could not bootstrap scheduled transactions: %w", err)
	}

	if err := storeAllScheduledTransactions(rw, scheduledTxs); err != nil {
		return nil, fmt.Errorf("could not store scheduled transactions: %w", err)
	}

	return &ScheduledTransactionsIndex{IndexState: state}, nil
}

// ByID returns the scheduled transaction with the given ID.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound]: if no scheduled transaction with the given ID exists
func (idx *ScheduledTransactionsIndex) ByID(id uint64) (access.ScheduledTransaction, error) {
	var tx access.ScheduledTransaction
	if err := operation.RetrieveByKey(idx.db.Reader(), makeScheduledTxPrimaryKey(id), &tx); err != nil {
		return access.ScheduledTransaction{}, fmt.Errorf("could not retrieve scheduled transaction %d: %w", id, err)
	}
	return tx, nil
}

// All returns an iterator over all scheduled transactions in descending ID order.
// Returns an exhausted iterator and no error if no transactions exist.
//
// `cursor` is a pointer to an [access.ScheduledTransactionCursor]:
//   - nil means start from the highest indexed ID
//   - non-nil means start at the cursor ID (inclusive)
//
// No error returns are expected during normal operation.
func (idx *ScheduledTransactionsIndex) All(
	cursor *access.ScheduledTransactionCursor,
) (storage.ScheduledTransactionIterator, error) {
	startKey, endKey := idx.rangeKeysAll(cursor)

	reader := idx.db.Reader()
	iter, err := reader.NewIter(startKey, endKey, storage.DefaultIteratorOptions())
	if err != nil {
		return nil, fmt.Errorf("could not create iterator: %w", err)
	}

	return iterator.Build(iter, decodeScheduledTxCursor, reconstructScheduledTx), nil
}

// ByAddress returns an iterator over scheduled transactions for the given account in
// descending ID order. Returns an exhausted iterator and no error if the account has no
// transactions.
//
// `cursor` is a pointer to an [access.ScheduledTransactionCursor]:
//   - nil means start from the highest indexed ID
//   - non-nil means start at the cursor ID (inclusive)
//
// No error returns are expected during normal operation.
func (idx *ScheduledTransactionsIndex) ByAddress(
	account flow.Address,
	cursor *access.ScheduledTransactionCursor,
) (storage.ScheduledTransactionIterator, error) {
	startKey, endKey := idx.rangeKeysAddress(account, cursor)

	reader := idx.db.Reader()
	iter, err := reader.NewIter(startKey, endKey, storage.DefaultIteratorOptions())
	if err != nil {
		return nil, fmt.Errorf("could not create iterator: %w", err)
	}

	// The by-address index is key-only (nil values). The getValue closure performs
	// a secondary lookup into the primary index using the decoded cursor's ID.
	getValue := func(cur access.ScheduledTransactionCursor, _ []byte) (*access.ScheduledTransaction, error) {
		var tx access.ScheduledTransaction
		if err := operation.RetrieveByKey(reader, makeScheduledTxPrimaryKey(cur.ID), &tx); err != nil {
			return nil, err
		}
		return &tx, nil
	}

	return iterator.Build(iter, decodeScheduledTxByAddrCursor, getValue), nil
}

// rangeKeysAll computes the start and end keys for iterating over all scheduled transactions based
// on the provided cursor.
func (idx *ScheduledTransactionsIndex) rangeKeysAll(cursor *access.ScheduledTransactionCursor) (startKey, endKey []byte) {
	if cursor == nil {
		// keys include the one's complement of the ID, so iteration is in descending order of ids.
		startKey = makeScheduledTxPrimaryKey(math.MaxUint64)
		endKey = makeScheduledTxPrimaryKey(0)
		return startKey, endKey
	}

	startKey = makeScheduledTxPrimaryKey(cursor.ID)
	endKey = makeScheduledTxPrimaryKey(0)
	return startKey, endKey
}

// rangeKeysAddress computes the start and end keys for iterating over scheduled transactions of an
// account, based on the provided cursor.
func (idx *ScheduledTransactionsIndex) rangeKeysAddress(address flow.Address, cursor *access.ScheduledTransactionCursor) (startKey, endKey []byte) {
	if cursor == nil {
		// keys include the one's complement of the ID, so iteration is in descending order of ids.
		startKey := makeScheduledTxByAddrKey(address, math.MaxUint64)
		endKey := makeScheduledTxByAddrKey(address, 0)
		return startKey, endKey
	}

	startKey = makeScheduledTxByAddrKey(address, cursor.ID)
	endKey = makeScheduledTxByAddrKey(address, 0)
	return startKey, endKey
}

// Store indexes new scheduled transactions from the block and advances the latest indexed height.
// Must be called with consecutive block heights.
// The caller must hold the [storage.LockIndexScheduledTransactionsIndex] lock until committed.
//
// Expected error returns during normal operation:
//   - [storage.ErrAlreadyExists]: if blockHeight is already indexed
func (idx *ScheduledTransactionsIndex) Store(
	lctx lockctx.Proof,
	rw storage.ReaderBatchWriter,
	blockHeight uint64,
	scheduledTxs []access.ScheduledTransaction,
) error {
	if err := idx.PrepareStore(lctx, rw, blockHeight); err != nil {
		return fmt.Errorf("could not prepare store for block %d: %w", blockHeight, err)
	}

	return storeAllScheduledTransactions(rw, scheduledTxs)
}

// storeAllScheduledTransactions writes all scheduled transaction entries to the batch.
// The caller must hold the [storage.LockIndexScheduledTransactionsIndex] lock until the batch
// is committed.
//
// Expected error returns during normal operation:
//   - [storage.ErrAlreadyExists]: if any scheduled transaction ID already exists
func storeAllScheduledTransactions(rw storage.ReaderBatchWriter, scheduledTxs []access.ScheduledTransaction) error {
	writer := rw.Writer()
	seen := visited.New[uint64]()
	for _, tx := range scheduledTxs {
		if seen.Visit(tx.ID) {
			return fmt.Errorf("scheduled transaction %d appears more than once in batch: %w", tx.ID, storage.ErrAlreadyExists)
		}

		primaryKey := makeScheduledTxPrimaryKey(tx.ID)

		exists, err := operation.KeyExists(rw.GlobalReader(), primaryKey)
		if err != nil {
			return fmt.Errorf("could not check key for tx %d: %w", tx.ID, err)
		}
		if exists {
			return fmt.Errorf("scheduled transaction %d already exists: %w", tx.ID, storage.ErrAlreadyExists)
		}

		if err := operation.UpsertByKey(writer, primaryKey, tx); err != nil {
			return fmt.Errorf("could not store tx %d: %w", tx.ID, err)
		}
		if err := operation.UpsertByKey(writer, makeScheduledTxByAddrKey(tx.TransactionHandlerOwner, tx.ID), nil); err != nil {
			return fmt.Errorf("could not store by-address key for tx %d: %w", tx.ID, err)
		}
	}
	return nil
}

// Executed updates the transaction's status to Executed and records the ID of the
// transaction that emitted the Executed event.
// The caller must hold the [storage.LockIndexScheduledTransactionsIndex] lock until committed.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound]: if no entry with the given ID exists
//   - [storage.ErrInvalidStatusTransition]: if the transaction is already Cancelled, Executed, or Failed
func (idx *ScheduledTransactionsIndex) Executed(
	lctx lockctx.Proof,
	rw storage.ReaderBatchWriter,
	scheduledTxID uint64,
	transactionID flow.Identifier,
) error {
	if !lctx.HoldsLock(storage.LockIndexScheduledTransactionsIndex) {
		return fmt.Errorf("missing required lock: %s", storage.LockIndexScheduledTransactionsIndex)
	}

	key := makeScheduledTxPrimaryKey(scheduledTxID)
	var tx access.ScheduledTransaction
	if err := operation.RetrieveByKey(rw.GlobalReader(), key, &tx); err != nil {
		return fmt.Errorf("could not retrieve scheduled transaction %d: %w", scheduledTxID, err)
	}
	if tx.Status != access.ScheduledTxStatusScheduled {
		return fmt.Errorf("tx %d already in terminal state %s: %w", scheduledTxID, tx.Status, storage.ErrInvalidStatusTransition)
	}

	tx.Status = access.ScheduledTxStatusExecuted
	tx.ExecutedTransactionID = transactionID

	if err := operation.UpsertByKey(rw.Writer(), key, tx); err != nil {
		return fmt.Errorf("could not update scheduled transaction %d: %w", scheduledTxID, err)
	}
	return nil
}

// Cancelled updates the transaction's status to Cancelled and records fee amounts
// and the ID of the transaction that emitted the Canceled event.
// The caller must hold the [storage.LockIndexScheduledTransactionsIndex] lock until committed.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound]: if no entry with the given ID exists
//   - [storage.ErrInvalidStatusTransition]: if the transaction is already Executed, Cancelled, or Failed
func (idx *ScheduledTransactionsIndex) Cancelled(
	lctx lockctx.Proof,
	rw storage.ReaderBatchWriter,
	scheduledTxID uint64,
	feesReturned uint64,
	feesDeducted uint64,
	transactionID flow.Identifier,
) error {
	if !lctx.HoldsLock(storage.LockIndexScheduledTransactionsIndex) {
		return fmt.Errorf("missing required lock: %s", storage.LockIndexScheduledTransactionsIndex)
	}

	key := makeScheduledTxPrimaryKey(scheduledTxID)
	var tx access.ScheduledTransaction
	if err := operation.RetrieveByKey(rw.GlobalReader(), key, &tx); err != nil {
		return fmt.Errorf("could not retrieve scheduled transaction %d: %w", scheduledTxID, err)
	}
	if tx.Status != access.ScheduledTxStatusScheduled {
		return fmt.Errorf("tx %d already in terminal state %s: %w", scheduledTxID, tx.Status, storage.ErrInvalidStatusTransition)
	}

	tx.Status = access.ScheduledTxStatusCancelled
	tx.FeesReturned = feesReturned
	tx.FeesDeducted = feesDeducted
	tx.CancelledTransactionID = transactionID

	if err := operation.UpsertByKey(rw.Writer(), key, tx); err != nil {
		return fmt.Errorf("could not update scheduled transaction %d: %w", scheduledTxID, err)
	}
	return nil
}

// Failed updates the transaction's status to Failed and records the ID of the executor
// transaction that attempted (and failed) to execute the scheduled transaction.
// The caller must hold the [storage.LockIndexScheduledTransactionsIndex] lock until committed.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound]: if no entry with the given ID exists
//   - [storage.ErrInvalidStatusTransition]: if the transaction is already Executed, Cancelled, or Failed
func (idx *ScheduledTransactionsIndex) Failed(
	lctx lockctx.Proof,
	rw storage.ReaderBatchWriter,
	scheduledTxID uint64,
	transactionID flow.Identifier,
) error {
	if !lctx.HoldsLock(storage.LockIndexScheduledTransactionsIndex) {
		return fmt.Errorf("missing required lock: %s", storage.LockIndexScheduledTransactionsIndex)
	}

	key := makeScheduledTxPrimaryKey(scheduledTxID)

	var tx access.ScheduledTransaction
	if err := operation.RetrieveByKey(rw.GlobalReader(), key, &tx); err != nil {
		return fmt.Errorf("could not retrieve scheduled transaction %d: %w", scheduledTxID, err)
	}
	if tx.Status != access.ScheduledTxStatusScheduled {
		return fmt.Errorf("tx %d already in terminal state %s: %w", scheduledTxID, tx.Status, storage.ErrInvalidStatusTransition)
	}

	tx.Status = access.ScheduledTxStatusFailed
	tx.ExecutedTransactionID = transactionID

	if err := operation.UpsertByKey(rw.Writer(), key, tx); err != nil {
		return fmt.Errorf("could not update scheduled transaction %d: %w", scheduledTxID, err)
	}
	return nil
}

// reconstructScheduledTx decodes a msgpack-encoded value into a [access.ScheduledTransaction].
//
// Any error indicates a malformed value.
func reconstructScheduledTx(_ access.ScheduledTransactionCursor, value []byte) (*access.ScheduledTransaction, error) {
	var tx access.ScheduledTransaction
	if err := msgpack.Unmarshal(value, &tx); err != nil {
		return nil, err
	}
	return &tx, nil
}

// makeScheduledTxPrimaryKey creates a primary key [code][~id].
// One's complement ensures higher IDs sort first during forward iteration.
func makeScheduledTxPrimaryKey(id uint64) []byte {
	key := make([]byte, scheduledTxPrimaryKeyLen)
	key[0] = codeScheduledTransaction
	binary.BigEndian.PutUint64(key[1:], ^id)
	return key
}

// makeScheduledTxByAddrKey creates a by-address key [code][address][~id].
func makeScheduledTxByAddrKey(addr flow.Address, id uint64) []byte {
	key := make([]byte, scheduledTxByAddrKeyLen)
	key[0] = codeScheduledTransactionByAddress
	copy(key[1:1+flow.AddressLength], addr[:])
	binary.BigEndian.PutUint64(key[1+flow.AddressLength:], ^id)
	return key
}

// decodeScheduledTxCursor decodes a primary key and returns the ID.
//
// Any error indicates a malformed key.
func decodeScheduledTxCursor(key []byte) (access.ScheduledTransactionCursor, error) {
	if len(key) != scheduledTxPrimaryKeyLen {
		return access.ScheduledTransactionCursor{}, fmt.Errorf("invalid primary key length: expected %d, got %d", scheduledTxPrimaryKeyLen, len(key))
	}
	if key[0] != codeScheduledTransaction {
		return access.ScheduledTransactionCursor{}, fmt.Errorf("invalid prefix: expected %d, got %d", codeScheduledTransaction, key[0])
	}
	return access.ScheduledTransactionCursor{
		ID: ^binary.BigEndian.Uint64(key[1:]),
	}, nil
}

// decodeScheduledTxByAddrCursor decodes a by-address key and returns the cursor ID.
//
// Any error indicates a malformed key.
func decodeScheduledTxByAddrCursor(key []byte) (access.ScheduledTransactionCursor, error) {
	if len(key) != scheduledTxByAddrKeyLen {
		return access.ScheduledTransactionCursor{}, fmt.Errorf(
			"invalid by-address key length: expected %d, got %d", scheduledTxByAddrKeyLen, len(key))
	}
	if key[0] != codeScheduledTransactionByAddress {
		return access.ScheduledTransactionCursor{}, fmt.Errorf(
			"invalid prefix: expected %d, got %d", codeScheduledTransactionByAddress, key[0])
	}

	// skip prefix and address
	offset := 1 + flow.AddressLength
	id := ^binary.BigEndian.Uint64(key[offset:])

	return access.ScheduledTransactionCursor{
		ID: id,
	}, nil
}
