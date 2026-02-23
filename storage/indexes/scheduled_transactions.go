package indexes

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/jordanschalm/lockctx"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

const (
	// scheduledTxPrimaryKeyLen is [code(1)][~id(8)] = 9 bytes
	scheduledTxPrimaryKeyLen = 1 + 8
	// scheduledTxByAddrKeyLen is [code(1)][address(8)][~id(8)] = 17 bytes
	scheduledTxByAddrKeyLen = 1 + flow.AddressLength + 8
)

// ScheduledTransactionsIndex implements [storage.ScheduledTransactionsIndex] using Pebble.
//
// Primary key format:    [codeScheduledTransaction][~id]                 → ScheduledTransaction value
// By-address key format: [codeScheduledTransactionByAddress][addr][~id] → nil (key-only)
//
// One's complement of id (~id) gives descending iteration order in ascending byte space.
//
// All read methods are safe for concurrent access. Write methods must be called sequentially.
type ScheduledTransactionsIndex struct {
	db           storage.DB
	firstHeight  uint64
	latestHeight *atomic.Uint64
}

var _ storage.ScheduledTransactionsIndex = (*ScheduledTransactionsIndex)(nil)

// NewScheduledTransactionsIndex creates a new index backed by db.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotBootstrapped]: if the index has not been initialized
func NewScheduledTransactionsIndex(db storage.DB) (*ScheduledTransactionsIndex, error) {
	firstHeight, err := readHeight(db.Reader(), keyScheduledTxFirstHeightKey)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, storage.ErrNotBootstrapped
		}
		return nil, fmt.Errorf("could not get first height: %w", err)
	}

	latestHeight, err := readHeight(db.Reader(), keyScheduledTxLatestHeightKey)
	if err != nil {
		return nil, fmt.Errorf("could not get latest height: %w", err)
	}

	return &ScheduledTransactionsIndex{
		db:           db,
		firstHeight:  firstHeight,
		latestHeight: atomic.NewUint64(latestHeight),
	}, nil
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
	if err := initializeScheduledTransactions(lctx, rw, initialStartHeight, scheduledTxs); err != nil {
		return nil, fmt.Errorf("could not bootstrap scheduled transactions: %w", err)
	}

	return &ScheduledTransactionsIndex{
		db:           db,
		firstHeight:  initialStartHeight,
		latestHeight: atomic.NewUint64(initialStartHeight),
	}, nil
}

// initializeScheduledTransactions writes the initial bounds keys and scheduled transactions to the
// database. The caller must hold the [storage.LockIndexScheduledTransactionsIndex] lock until the
// batch is committed.
//
// Expected error returns during normal operation:
//   - [storage.ErrAlreadyExists]: if any bounds key already exists
func initializeScheduledTransactions(
	lctx lockctx.Proof,
	rw storage.ReaderBatchWriter,
	initialStartHeight uint64,
	scheduledTxs []access.ScheduledTransaction,
) error {
	if !lctx.HoldsLock(storage.LockIndexScheduledTransactionsIndex) {
		return fmt.Errorf("missing required lock: %s", storage.LockIndexScheduledTransactionsIndex)
	}

	for _, key := range [][]byte{keyScheduledTxFirstHeightKey, keyScheduledTxLatestHeightKey} {
		exists, err := operation.KeyExists(rw.GlobalReader(), key)
		if err != nil {
			return fmt.Errorf("could not check bounds key: %w", err)
		}
		if exists {
			return fmt.Errorf("bounds key already exists: %w", storage.ErrAlreadyExists)
		}
	}

	writer := rw.Writer()
	if err := operation.UpsertByKey(writer, keyScheduledTxFirstHeightKey, initialStartHeight); err != nil {
		return fmt.Errorf("could not set first height: %w", err)
	}
	if err := operation.UpsertByKey(writer, keyScheduledTxLatestHeightKey, initialStartHeight); err != nil {
		return fmt.Errorf("could not set latest height: %w", err)
	}

	for _, tx := range scheduledTxs {
		key := makeScheduledTxPrimaryKey(tx.ID)
		exists, err := operation.KeyExists(rw.GlobalReader(), key)
		if err != nil {
			return fmt.Errorf("could not check key for tx %d: %w", tx.ID, err)
		}
		if exists {
			return fmt.Errorf("tx %d already exists during bootstrap: %w", tx.ID, storage.ErrAlreadyExists)
		}
		if err := operation.UpsertByKey(writer, key, tx); err != nil {
			return fmt.Errorf("could not store tx %d: %w", tx.ID, err)
		}
		if err := operation.UpsertByKey(writer, makeScheduledTxByAddrKey(tx.TransactionHandlerOwner, tx.ID), nil); err != nil {
			return fmt.Errorf("could not store by-address key for tx %d: %w", tx.ID, err)
		}
	}

	return nil
}

// FirstIndexedHeight returns the first (oldest) block height that has been indexed.
func (idx *ScheduledTransactionsIndex) FirstIndexedHeight() uint64 {
	return idx.firstHeight
}

// LatestIndexedHeight returns the latest block height that has been indexed.
func (idx *ScheduledTransactionsIndex) LatestIndexedHeight() uint64 {
	return idx.latestHeight.Load()
}

// ByID returns the scheduled transaction with the given ID.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound]: if no entry with the given ID exists
func (idx *ScheduledTransactionsIndex) ByID(id uint64) (access.ScheduledTransaction, error) {
	var tx access.ScheduledTransaction
	if err := operation.RetrieveByKey(idx.db.Reader(), makeScheduledTxPrimaryKey(id), &tx); err != nil {
		return access.ScheduledTransaction{}, fmt.Errorf("could not retrieve scheduled transaction %d: %w", id, err)
	}
	return tx, nil
}

// All retrieves all scheduled transactions using cursor-based pagination in descending ID order.
//
// Expected error returns during normal operation:
//   - [storage.ErrInvalidQuery]: if limit is invalid
func (idx *ScheduledTransactionsIndex) All(
	limit uint32,
	cursor *access.ScheduledTransactionCursor,
	filter storage.IndexFilter[*access.ScheduledTransaction],
) (access.ScheduledTransactionsPage, error) {
	if err := validateLimit(limit); err != nil {
		return access.ScheduledTransactionsPage{}, errors.Join(storage.ErrInvalidQuery, err)
	}
	return lookupAllScheduledTxs(idx.db.Reader(), limit, cursor, filter)
}

// ByAddress retrieves scheduled transactions for the given address in descending ID order.
//
// Expected error returns during normal operation:
//   - [storage.ErrInvalidQuery]: if limit is invalid
func (idx *ScheduledTransactionsIndex) ByAddress(
	account flow.Address,
	limit uint32,
	cursor *access.ScheduledTransactionCursor,
	filter storage.IndexFilter[*access.ScheduledTransaction],
) (access.ScheduledTransactionsPage, error) {
	if err := validateLimit(limit); err != nil {
		return access.ScheduledTransactionsPage{}, errors.Join(storage.ErrInvalidQuery, err)
	}
	return lookupScheduledTxsByAddress(idx.db.Reader(), account, limit, cursor, filter)
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
	if !lctx.HoldsLock(storage.LockIndexScheduledTransactionsIndex) {
		return fmt.Errorf("missing required lock: %s", storage.LockIndexScheduledTransactionsIndex)
	}

	if err := validateStoreHeight(blockHeight, idx.latestHeight.Load()); err != nil {
		return fmt.Errorf("invalid store height: %w", err)
	}

	writer := rw.Writer()
	for _, tx := range scheduledTxs {
		primaryKey := makeScheduledTxPrimaryKey(tx.ID)

		var existing access.ScheduledTransaction
		err := operation.RetrieveByKey(rw.GlobalReader(), primaryKey, &existing)
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("could not check key for tx %d: %w", tx.ID, err)
		}
		if err == nil {
			return fmt.Errorf("scheduled transaction %d already exists: %w", tx.ID, storage.ErrAlreadyExists)
		}

		if err := operation.UpsertByKey(writer, primaryKey, tx); err != nil {
			return fmt.Errorf("could not store tx %d: %w", tx.ID, err)
		}
		if err := operation.UpsertByKey(writer, makeScheduledTxByAddrKey(tx.TransactionHandlerOwner, tx.ID), nil); err != nil {
			return fmt.Errorf("could not store by-address key for tx %d: %w", tx.ID, err)
		}
	}

	if err := operation.UpsertByKey(writer, keyScheduledTxLatestHeightKey, blockHeight); err != nil {
		return fmt.Errorf("could not update latest height: %w", err)
	}

	storage.OnCommitSucceed(rw, func() {
		idx.latestHeight.Store(blockHeight)
	})

	return nil
}

// Executed updates the transaction's status to Executed and sets the final execution effort
// and the ID of the transaction that emitted the Executed event.
// The caller must hold the [storage.LockIndexScheduledTransactionsIndex] lock until committed.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound]: if no entry with the given ID exists
//   - [storage.ErrInvalidStatusTransition]: if the transaction is already Cancelled, Executed, or Failed
func (idx *ScheduledTransactionsIndex) Executed(
	lctx lockctx.Proof,
	rw storage.ReaderBatchWriter,
	scheduledTxID uint64,
	executionEffort uint64,
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
	tx.ExecutionEffort = executionEffort
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
	tx.FailedTransactionID = transactionID

	if err := operation.UpsertByKey(rw.Writer(), key, tx); err != nil {
		return fmt.Errorf("could not update scheduled transaction %d: %w", scheduledTxID, err)
	}
	return nil
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

// decodeScheduledTxPrimaryKey decodes a primary key and returns the ID.
//
// Any error indicates a malformed key.
func decodeScheduledTxPrimaryKey(key []byte) (uint64, error) {
	if len(key) != scheduledTxPrimaryKeyLen {
		return 0, fmt.Errorf("invalid primary key length: expected %d, got %d", scheduledTxPrimaryKeyLen, len(key))
	}
	if key[0] != codeScheduledTransaction {
		return 0, fmt.Errorf("invalid prefix: expected %d, got %d", codeScheduledTransaction, key[0])
	}
	return ^binary.BigEndian.Uint64(key[1:]), nil
}

// decodeScheduledTxByAddrKey decodes a by-address key into address and ID.
//
// Any error indicates a malformed key.
func decodeScheduledTxByAddrKey(key []byte) (flow.Address, uint64, error) {
	if len(key) != scheduledTxByAddrKeyLen {
		return flow.Address{}, 0, fmt.Errorf(
			"invalid by-address key length: expected %d, got %d", scheduledTxByAddrKeyLen, len(key))
	}
	if key[0] != codeScheduledTransactionByAddress {
		return flow.Address{}, 0, fmt.Errorf(
			"invalid prefix: expected %d, got %d", codeScheduledTransactionByAddress, key[0])
	}

	offset := 1

	addr := flow.BytesToAddress(key[offset : offset+flow.AddressLength])
	offset += flow.AddressLength

	id := ^binary.BigEndian.Uint64(key[offset:])
	return addr, id, nil
}

// lookupAllScheduledTxs iterates primary keys in descending ID order with cursor pagination.
//
// No error returns are expected during normal operation.
func lookupAllScheduledTxs(
	reader storage.Reader,
	limit uint32,
	cursor *access.ScheduledTransactionCursor,
	filter storage.IndexFilter[*access.ScheduledTransaction],
) (access.ScheduledTransactionsPage, error) {
	// startKey covers the highest possible ID (byte value smallest due to one's complement).
	// endKey covers ID=0 (byte value largest due to one's complement).
	// Forward iteration goes from small bytes to large bytes: descending ID order.
	startKey := makeScheduledTxPrimaryKey(math.MaxUint64)
	endKey := makeScheduledTxPrimaryKey(0)

	skipFirst := false
	if cursor != nil {
		// Resume after the cursor's ID. The cursor ID was the last entry returned,
		// so we start from it and skip the first matching entry.
		startKey = makeScheduledTxPrimaryKey(cursor.ID)
		skipFirst = true
	}

	// We fetch limit+1 to determine if there are more results beyond this page.
	fetchLimit := int(limit + 1)

	var collected []access.ScheduledTransaction
	err := operation.IterateKeys(reader, startKey, endKey,
		func(keyCopy []byte, getValue func(any) error) (bail bool, err error) {
			id, err := decodeScheduledTxPrimaryKey(keyCopy)
			if err != nil {
				return true, fmt.Errorf("could not decode key: %w", err)
			}

			// Skip the exact cursor entry (it was the last item of the previous page).
			if skipFirst && id == cursor.ID {
				return false, nil
			}
			skipFirst = false

			var tx access.ScheduledTransaction
			if err := getValue(&tx); err != nil {
				return true, fmt.Errorf("could not read value: %w", err)
			}

			if filter != nil && !filter(&tx) {
				return false, nil
			}

			collected = append(collected, tx)
			if len(collected) >= fetchLimit {
				return true, nil // bail after collecting enough
			}

			return false, nil
		}, storage.DefaultIteratorOptions())

	if err != nil {
		return access.ScheduledTransactionsPage{}, fmt.Errorf("could not iterate keys: %w", err)
	}

	return buildScheduledTxPage(collected, limit), nil
}

// lookupScheduledTxsByAddress iterates by-address keys and fetches full entries from primary index.
//
// No error returns are expected during normal operation.
func lookupScheduledTxsByAddress(
	reader storage.Reader,
	address flow.Address,
	limit uint32,
	cursor *access.ScheduledTransactionCursor,
	filter storage.IndexFilter[*access.ScheduledTransaction],
) (access.ScheduledTransactionsPage, error) {
	startKey := makeScheduledTxByAddrKey(address, math.MaxUint64)
	endKey := makeScheduledTxByAddrKey(address, 0)

	skipFirst := false
	if cursor != nil {
		startKey = makeScheduledTxByAddrKey(address, cursor.ID)
		skipFirst = true
	}

	// We fetch limit+1 to determine if there are more results beyond this page.
	fetchLimit := int(limit + 1)

	// placeholder sentinel to indicate iteration is complete.
	// IterateKeysByPrefixRange only stops when an error is returned or the range is fully iterated.
	errDone := errors.New("done")

	var collected []access.ScheduledTransaction

	// the by address index is a key-only index, so we do not need to inspect the actual value. Instead,
	// we lookup the entry within the primary index.
	err := operation.IterateKeysByPrefixRange(reader, startKey, endKey,
		func(keyCopy []byte) error {
			_, id, err := decodeScheduledTxByAddrKey(keyCopy)
			if err != nil {
				return fmt.Errorf("could not decode by-address key: %w", err)
			}

			// Skip the exact cursor entry.
			if skipFirst && id == cursor.ID {
				return nil
			}
			skipFirst = false

			var tx access.ScheduledTransaction
			if err := operation.RetrieveByKey(reader, makeScheduledTxPrimaryKey(id), &tx); err != nil {
				return fmt.Errorf("could not retrieve tx %d from primary: %w", id, err)
			}

			if filter != nil && !filter(&tx) {
				return nil
			}

			collected = append(collected, tx)

			if len(collected) >= fetchLimit {
				return errDone
			}

			return nil
		})

	if err != nil && !errors.Is(err, errDone) {
		return access.ScheduledTransactionsPage{}, fmt.Errorf("could not iterate by-address keys: %w", err)
	}

	return buildScheduledTxPage(collected, limit), nil
}

// buildScheduledTxPage assembles a page from collected results.
// The fetching logic always fetches one more entry than the limit to determine if there are more
// results beyond this page. If more than limit entries were collected, the next cursor is set to
// the last returned entry's ID.
func buildScheduledTxPage(collected []access.ScheduledTransaction, limit uint32) access.ScheduledTransactionsPage {
	if uint32(len(collected)) > limit {
		last := collected[limit-1]
		return access.ScheduledTransactionsPage{
			Transactions: collected[:limit],
			NextCursor:   &access.ScheduledTransactionCursor{ID: last.ID},
		}
	}
	return access.ScheduledTransactionsPage{Transactions: collected}
}
