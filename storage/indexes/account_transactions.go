package indexes

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

// AccountTransactions implements storage.AccountTransactions using Pebble.
// It provides an index mapping accounts to their transactions, ordered by block height
// in descending order (newest first).
//
// Key format: [prefix][address][~block_height][tx_index]
// - prefix: 1 byte (codeAccountTransactions)
// - address: 8 bytes (flow.Address)
// - ~block_height: 8 bytes (one's complement for descending sort)
// - tx_index: 4 bytes (uint32, big-endian)
//
// Value format: [tx_id][is_authorizer]
// - tx_id: 32 bytes (flow.Identifier)
// - is_authorizer: 1 byte boolean
//
// All read methods are safe for concurrent access. Write methods (Store)
// must be called sequentially with consecutive heights.
type AccountTransactions struct {
	db           storage.DB
	lockManager  storage.LockManager
	firstHeight  uint64
	latestHeight *atomic.Uint64
}

type storedAccountTransaction struct {
	TransactionID flow.Identifier
	IsAuthorizer  bool
}

const (
	// accountTxKeyLen is the total length of an account transaction index key
	// 1 (prefix) + 8 (address) + 8 (height) + 4 (txIndex) = 21
	accountTxKeyLen = 1 + flow.AddressLength + 8 + 4

	// accountTxPrefixLen is the length of the prefix used for iteration (prefix + address)
	accountTxPrefixLen = 1 + flow.AddressLength

	// accountTxPrefixWithHeightLen includes the height for range queries
	accountTxPrefixWithHeightLen = accountTxPrefixLen + 8
)

var (
	// accountTxLatestHeightKey stores the latest indexed height for account transactions
	accountTxLatestHeightKey = []byte{codeIndexProcessedHeightUpperBound, codeAccountTransactions}

	// accountTxFirstHeightKey stores the first indexed height for account transactions
	accountTxFirstHeightKey = []byte{codeIndexProcessedHeightLowerBound, codeAccountTransactions}
)

var _ storage.AccountTransactions = (*AccountTransactions)(nil)

// NewAccountTransactions creates a new AccountTransactions backed by the given database.
//
// If the index has not been initialized, constuction will fail with [storage.ErrNotBootstrapped].
// The caller should retry with `BootstrapAccountTransactions` passing the required initialization data.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotBootstrapped] if the index has not been initialized
func NewAccountTransactions(db storage.DB) (*AccountTransactions, error) {
	firstHeight, err := accountTxFirstStoredHeight(db.Reader())
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, storage.ErrNotBootstrapped
		}
		return nil, fmt.Errorf("could not get first height: %w", err)
	}

	persistedLatestHeight, err := accountTxLatestStoredHeight(db.Reader())
	if err != nil {
		// if `firstHeight` is set, then `latestHeight` must be set as well, otherwise the database
		// is in a corrupted state.
		return nil, fmt.Errorf("could not get latest height: %w", err)
	}

	return &AccountTransactions{
		db:           db,
		firstHeight:  firstHeight,
		latestHeight: atomic.NewUint64(persistedLatestHeight),
	}, nil
}

func BootstrapAccountTransactions(lctx lockctx.Proof, rw storage.ReaderBatchWriter, db storage.DB, initialStartHeight uint64, txData []access.AccountTransaction) (*AccountTransactions, error) {
	err := initialize(lctx, rw, initialStartHeight, txData)
	if err != nil {
		return nil, fmt.Errorf("could not bootstrap account transactions: %w", err)
	}

	return &AccountTransactions{
		db:           db,
		firstHeight:  initialStartHeight,
		latestHeight: atomic.NewUint64(initialStartHeight),
	}, nil
}

// FirstIndexedHeight returns the first (oldest) block height that has been indexed.
//
// No error returns are expected during normal operation.
func (idx *AccountTransactions) FirstIndexedHeight() (uint64, error) {
	return idx.firstHeight, nil
}

// LatestIndexedHeight returns the latest block height that has been indexed.
//
// No error returns are expected during normal operation.
func (idx *AccountTransactions) LatestIndexedHeight() (uint64, error) {
	return idx.latestHeight.Load(), nil
}

// UninitializedFirstHeight returns the height the index will accept as the first height, and a boolean
// indicating if the index is initialized.
// If the index is not initialized, the first call to `Store` must include data for this height.
func (idx *AccountTransactions) UninitializedFirstHeight() (uint64, bool) {
	return idx.firstHeight, true
}

// TransactionsByAddress retrieves transaction references for an account within the specified
// block height range (inclusive). Results are returned in descending order (newest first).
//
// Expected error returns during normal operations:
//   - [storage.ErrHeightNotIndexed] if the requested range extends beyond indexed heights
func (idx *AccountTransactions) TransactionsByAddress(
	account flow.Address,
	startHeight uint64,
	endHeight uint64,
) ([]access.AccountTransaction, error) {
	latestHeight := idx.latestHeight.Load()
	if startHeight > latestHeight {
		return nil, fmt.Errorf("start height %d is greater than latest indexed height %d: %w",
			startHeight, latestHeight, storage.ErrHeightNotIndexed)
	}

	if startHeight < idx.firstHeight {
		return nil, fmt.Errorf("start height %d is before first indexed height %d: %w",
			startHeight, idx.firstHeight, storage.ErrHeightNotIndexed)
	}

	if endHeight > latestHeight {
		endHeight = latestHeight
	}

	if startHeight > endHeight {
		return nil, fmt.Errorf("start height %d is greater than end height %d", startHeight, endHeight)
	}

	results, err := lookupAccountTransactions(idx.db.Reader(), account, startHeight, endHeight)
	if err != nil {
		return nil, fmt.Errorf("could not lookup account transactions: %w", err)
	}

	return results, nil
}

// Store indexes all account-transaction associations for a block.
// Must be called sequentially with consecutive heights (latestHeight + 1).
// Calling with the last height is a no-op.
// The caller must hold the [storage.LockIndexAccountTransactions] lock until the batch is committed.
//
// Expected error returns during normal operations:
//   - [storage.ErrAlreadyExists] if the block height is already indexed
func (idx *AccountTransactions) Store(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockHeight uint64, txData []access.AccountTransaction) error {
	latestHeight := idx.latestHeight.Load()

	if blockHeight < latestHeight {
		return storage.ErrAlreadyExists
	}

	// Reindexing the latest height is a no-op
	if blockHeight == latestHeight {
		return nil
	}

	expectedHeight := latestHeight + 1
	if blockHeight != expectedHeight {
		return fmt.Errorf("must index consecutive heights: expected %d, got %d", expectedHeight, blockHeight)
	}

	err := indexAccountTransactions(lctx, rw, blockHeight, txData)
	if err != nil {
		return fmt.Errorf("could not index account transactions: %w", err)
	}

	storage.OnCommitSucceed(rw, func() {
		idx.latestHeight.Store(blockHeight)
	})

	return nil
}

// lookupAccountTransactions retrieves all account transactions for a given address within the specified
// block height range (inclusive). Results are returned in descending order (newest first).
// Returns an empty slice and no error if no transactions are found.
//
// No error returns are expected during normal operation.
func lookupAccountTransactions(reader storage.Reader, address flow.Address, startHeight uint64, endHeight uint64) ([]access.AccountTransaction, error) {
	// TODO(peter): I will be revisiting this logic when implementing the API integration. instead
	// of using a start/end height range, we'll use a pagination cursor and limit.

	// Create iterator bounds (inclusive)
	// Lower bound: prefix + address + ~endHeight (one's complement makes this the lower bound for descending)
	// Upper bound: prefix + address + ~startHeight
	lowerBound := makeAccountTxKeyPrefix(address, endHeight)
	upperBound := makeAccountTxKeyPrefix(address, startHeight)

	var results []access.AccountTransaction
	err := operation.IterateKeys(reader, lowerBound, upperBound,
		func(keyCopy []byte, getValue func(any) error) (bail bool, err error) {
			var stored storedAccountTransaction
			if err := getValue(&stored); err != nil {
				return true, fmt.Errorf("could not unmarshal value: %w", err)
			}

			address, height, txIndex, err := decodeAccountTxKey(keyCopy)
			if err != nil {
				return true, fmt.Errorf("could not decode key: %w", err)
			}

			results = append(results, access.AccountTransaction{
				Address:          address,
				BlockHeight:      height,
				TransactionID:    stored.TransactionID,
				TransactionIndex: txIndex,
				IsAuthorizer:     stored.IsAuthorizer,
			})

			return false, nil
		}, storage.DefaultIteratorOptions())

	if err != nil {
		return nil, fmt.Errorf("could not iterate keys: %w", err)
	}

	return results, nil
}

// indexAccountTransactions indexes all account-transaction associations for a block.
// The caller must hold the [storage.LockIndexAccountTransactions] lock until the batch is committed.
//
// Expected error returns during normal operations:
//   - [storage.ErrAlreadyExists] if the block height is already indexed
func indexAccountTransactions(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockHeight uint64, txData []access.AccountTransaction) error {
	if !lctx.HoldsLock(storage.LockIndexAccountTransactions) {
		return fmt.Errorf("missing required lock: %s", storage.LockIndexAccountTransactions)
	}

	latestHeight, err := accountTxLatestStoredHeight(rw.GlobalReader())
	if err != nil {
		return fmt.Errorf("could not get latest indexed height: %w", err)
	}
	if blockHeight != latestHeight+1 {
		return fmt.Errorf("must index consecutive heights: expected %d, got %d", latestHeight+1, blockHeight)
	}

	writer := rw.Writer()

	for _, entry := range txData {
		if entry.BlockHeight != blockHeight {
			return fmt.Errorf("block height mismatch: expected %d, got %d", blockHeight, entry.BlockHeight)
		}

		key := makeAccountTxKey(entry.Address, entry.BlockHeight, entry.TransactionIndex)

		exists, err := operation.KeyExists(rw.GlobalReader(), key)
		if err != nil {
			return fmt.Errorf("could not check if key exists: %w", err)
		}
		if exists {
			return fmt.Errorf("account transaction %s at height %d already indexed: %w", entry.Address, entry.BlockHeight, storage.ErrAlreadyExists)
		}

		stored := storedAccountTransaction{
			TransactionID: entry.TransactionID,
			IsAuthorizer:  entry.IsAuthorizer,
		}
		if err := operation.UpsertByKey(writer, key, stored); err != nil {
			return fmt.Errorf("could not set key for account %s, tx %s: %w", entry.Address, entry.TransactionID, err)
		}
	}

	// Update latest height
	if err := operation.UpsertByKey(writer, accountTxLatestHeightKey, blockHeight); err != nil {
		return fmt.Errorf("could not update latest height: %w", err)
	}

	return nil
}

// initialize initializes the account transactions index with data from the first block.
// The caller must hold the [storage.LockIndexAccountTransactions] lock until the batch is committed.
//
// Expected error returns during normal operations:
//   - [storage.ErrAlreadyExists] if any data is found for while initializing
func initialize(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockHeight uint64, txData []access.AccountTransaction) error {
	if !lctx.HoldsLock(storage.LockIndexAccountTransactions) {
		return fmt.Errorf("missing required lock: %s", storage.LockIndexAccountTransactions)
	}

	// double check the first/latest heights are not already stored
	exists, err := operation.KeyExists(rw.GlobalReader(), accountTxFirstHeightKey)
	if err != nil {
		return fmt.Errorf("could not check if first height key exists: %w", err)
	}
	if exists {
		return fmt.Errorf("first height key already exists: %w", storage.ErrAlreadyExists)
	}

	exists, err = operation.KeyExists(rw.GlobalReader(), accountTxLatestHeightKey)
	if err != nil {
		return fmt.Errorf("could not check if latest height key exists: %w", err)
	}
	if exists {
		return fmt.Errorf("latest height key already exists: %w", storage.ErrAlreadyExists)
	}

	writer := rw.Writer()

	for _, entry := range txData {
		if entry.BlockHeight != blockHeight {
			return fmt.Errorf("block height mismatch: expected %d, got %d", blockHeight, entry.BlockHeight)
		}

		key := makeAccountTxKey(entry.Address, entry.BlockHeight, entry.TransactionIndex)

		exists, err := operation.KeyExists(rw.GlobalReader(), key)
		if err != nil {
			return fmt.Errorf("could not check if key exists: %w", err)
		}
		if exists {
			return fmt.Errorf("account transaction %s at height %d already indexed: %w", entry.Address, entry.BlockHeight, storage.ErrAlreadyExists)
		}

		stored := storedAccountTransaction{
			TransactionID: entry.TransactionID,
			IsAuthorizer:  entry.IsAuthorizer,
		}
		if err := operation.UpsertByKey(writer, key, stored); err != nil {
			return fmt.Errorf("could not set key for account %s, tx %s: %w", entry.Address, entry.TransactionID, err)
		}
	}

	if err := operation.UpsertByKey(writer, accountTxFirstHeightKey, blockHeight); err != nil {
		return fmt.Errorf("could not update first height: %w", err)
	}
	if err := operation.UpsertByKey(writer, accountTxLatestHeightKey, blockHeight); err != nil {
		return fmt.Errorf("could not update latest height: %w", err)
	}

	return nil
}

// makeAccountTxKey creates a full key for an account transaction index entry.
// Key format: [prefix][address][~block_height][tx_index]
func makeAccountTxKey(address flow.Address, height uint64, txIndex uint32) []byte {
	key := make([]byte, accountTxKeyLen)

	key[0] = codeAccountTransactions
	copy(key[1:1+flow.AddressLength], address[:])

	// One's complement of height for descending order
	onesComplement := ^height
	binary.BigEndian.PutUint64(key[1+flow.AddressLength:], onesComplement)

	binary.BigEndian.PutUint32(key[1+flow.AddressLength+8:], txIndex)

	return key
}

// makeAccountTxKeyPrefix creates a prefix key for iteration, up to and including the height.
// This is used to set iterator bounds for height range queries.
// Key format: [prefix][address][~block_height]
func makeAccountTxKeyPrefix(address flow.Address, height uint64) []byte {
	prefix := make([]byte, accountTxPrefixWithHeightLen)

	prefix[0] = codeAccountTransactions
	copy(prefix[1:1+flow.AddressLength], address[:])

	// One's complement of height for descending order
	onesComplement := ^height
	binary.BigEndian.PutUint64(prefix[1+flow.AddressLength:], onesComplement)

	return prefix
}

// decodeAccountTxKey decodes a key and value into an AccountTransaction.
//
// Any error indicates the key is not valid.
func decodeAccountTxKey(key []byte) (flow.Address, uint64, uint32, error) {
	if len(key) != accountTxKeyLen {
		return flow.Address{}, 0, 0, fmt.Errorf("invalid key length: expected %d, got %d",
			accountTxKeyLen, len(key))
	}

	if key[0] != codeAccountTransactions {
		return flow.Address{}, 0, 0, fmt.Errorf("invalid prefix: expected %d, got %d",
			codeAccountTransactions, key[0])
	}

	// Skip prefix
	offset := 1

	address := flow.BytesToAddress(key[offset : offset+flow.AddressLength])
	offset += flow.AddressLength

	// Decode height (one's complement)
	onesComplement := binary.BigEndian.Uint64(key[offset:])
	height := ^onesComplement
	offset += 8

	// Decode transaction index
	txIndex := binary.BigEndian.Uint32(key[offset:])

	return address, height, txIndex, nil
}

// accountTxFirstStoredHeight reads the first indexed height from the database.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotFound] if the height is not found
func accountTxFirstStoredHeight(reader storage.Reader) (uint64, error) {
	return accountTxHeightLookup(reader, accountTxFirstHeightKey)
}

// accountTxLatestStoredHeight reads the latest indexed height from the database.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotFound] if the height is not found
func accountTxLatestStoredHeight(reader storage.Reader) (uint64, error) {
	return accountTxHeightLookup(reader, accountTxLatestHeightKey)
}

// accountTxHeightLookup reads a height value from the database.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotFound] if the height is not found
func accountTxHeightLookup(reader storage.Reader, key []byte) (uint64, error) {
	var height uint64
	if err := operation.RetrieveByKey(reader, key, &height); err != nil {
		return 0, err
	}
	return height, nil
}
