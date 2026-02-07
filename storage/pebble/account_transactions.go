package pebble

import (
	"encoding/binary"
	"fmt"

	"github.com/cockroachdb/pebble/v2"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// AccountTransactions implements storage.AccountTransactions using Pebble.
// It provides an index mapping accounts to their transactions, ordered by block height
// in descending order (newest first).
//
// Key format: [prefix][address][~block_height][tx_id][tx_index]
// - prefix: 1 byte (codeAccountTxIndex)
// - address: 8 bytes (flow.Address)
// - ~block_height: 8 bytes (one's complement for descending sort)
// - tx_id: 32 bytes (flow.Identifier)
// - tx_index: 4 bytes (uint32, big-endian)
//
// Value format: 1 byte boolean (IsAuthorizer)
//
// All read methods are safe for concurrent access. Write methods (Store)
// must be called sequentially with consecutive heights.
type AccountTransactions struct {
	db           *pebble.DB
	firstHeight  uint64
	latestHeight *atomic.Uint64
}

const (
	// accountTxKeyLen is the total length of an account transaction index key
	// 1 (prefix) + 8 (address) + 8 (height) + 32 (txID) + 4 (txIndex) = 53
	accountTxKeyLen = 1 + flow.AddressLength + 8 + flow.IdentifierLen + 4

	// accountTxPrefixLen is the length of the prefix used for iteration (prefix + address)
	accountTxPrefixLen = 1 + flow.AddressLength

	// accountTxPrefixWithHeightLen includes the height for range queries
	accountTxPrefixWithHeightLen = accountTxPrefixLen + 8
)

var (
	// accountTxLatestHeightKey stores the latest indexed height for account transactions
	accountTxLatestHeightKey = []byte{codeAccountTxLatestHeight}

	// accountTxFirstHeightKey stores the first indexed height for account transactions
	accountTxFirstHeightKey = []byte{codeAccountTxFirstHeight}
)

var _ storage.AccountTransactions = (*AccountTransactions)(nil)

// NewAccountTransactions creates a new AccountTransactions backed by the given Pebble database.
// The database must have been bootstrapped with first and latest height values.
//
// Expected errors during normal operations:
//   - storage.ErrNotBootstrapped if the database has not been initialized with height values
func NewAccountTransactions(db *pebble.DB) (*AccountTransactions, error) {
	firstHeight, err := accountTxFirstStoredHeight(db)
	if err != nil {
		return nil, fmt.Errorf("could not get first height: %w", err)
	}

	latestHeight, err := accountTxLatestStoredHeight(db)
	if err != nil {
		return nil, fmt.Errorf("could not get latest height: %w", err)
	}

	return &AccountTransactions{
		db:           db,
		firstHeight:  firstHeight,
		latestHeight: atomic.NewUint64(latestHeight),
	}, nil
}

// InitAccountTransactions initializes a new AccountTransactions database with the given
// starting height. This should only be called once when setting up a new database.
//
// No errors are expected during normal operation.
func InitAccountTransactions(db *pebble.DB, startHeight uint64) error {
	batch := db.NewBatch()
	defer batch.Close()

	heightBytes := encodedUint64(startHeight)

	if err := batch.Set(accountTxFirstHeightKey, heightBytes, nil); err != nil {
		return fmt.Errorf("could not set first height: %w", err)
	}

	if err := batch.Set(accountTxLatestHeightKey, heightBytes, nil); err != nil {
		return fmt.Errorf("could not set latest height: %w", err)
	}

	if err := batch.Commit(pebble.Sync); err != nil {
		return fmt.Errorf("could not commit init batch: %w", err)
	}

	return nil
}

// TransactionsByAddress retrieves transaction references for an account within the specified
// block height range (inclusive). Results are returned in descending order (newest first).
//
// Expected errors during normal operations:
//   - storage.ErrHeightNotIndexed if the requested range extends beyond indexed heights
func (idx *AccountTransactions) TransactionsByAddress(
	account flow.Address,
	startHeight uint64,
	endHeight uint64,
) ([]access.AccountTransaction, error) {
	latestHeight := idx.latestHeight.Load()
	if endHeight > latestHeight {
		endHeight = latestHeight
	}

	if startHeight < idx.firstHeight {
		return nil, fmt.Errorf("start height %d is before first indexed height %d: %w",
			startHeight, idx.firstHeight, storage.ErrHeightNotIndexed)
	}

	if startHeight > endHeight {
		return nil, fmt.Errorf("start height %d is greater than end height %d", startHeight, endHeight)
	}

	// Create iterator bounds
	// Lower bound: prefix + address + ~endHeight (one's complement makes this the lower bound for descending)
	// Upper bound: prefix + address + ~startHeight + 1 (exclusive)
	lowerBound := makeAccountTxKeyPrefix(account, endHeight)
	upperBound := makeAccountTxKeyPrefix(account, startHeight)
	// Increment upper bound to make it exclusive
	upperBound = incrementBytes(upperBound)

	iter, err := idx.db.NewIter(&pebble.IterOptions{
		LowerBound: lowerBound,
		UpperBound: upperBound,
	})
	if err != nil {
		return nil, fmt.Errorf("could not create iterator: %w", err)
	}
	defer iter.Close()

	var results []access.AccountTransaction

	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		value, err := iter.ValueAndErr()
		if err != nil {
			return nil, fmt.Errorf("could not read value: %w", err)
		}

		entry, err := decodeAccountTxKey(key, value)
		if err != nil {
			return nil, fmt.Errorf("could not decode key: %w", err)
		}

		results = append(results, entry)
	}

	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("iterator error: %w", err)
	}

	return results, nil
}

// LatestIndexedHeight returns the latest block height that has been indexed.
//
// Expected errors during normal operations:
//   - storage.ErrNotBootstrapped if the index has not been initialized
func (idx *AccountTransactions) LatestIndexedHeight() (uint64, error) {
	return idx.latestHeight.Load(), nil
}

// FirstIndexedHeight returns the first (oldest) block height that has been indexed.
//
// Expected errors during normal operations:
//   - storage.ErrNotBootstrapped if the index has not been initialized
func (idx *AccountTransactions) FirstIndexedHeight() (uint64, error) {
	return idx.firstHeight, nil
}

// Store indexes all account-transaction associations for a block.
// Must be called sequentially with consecutive heights (latestHeight + 1).
//
// CAUTION: Not safe for concurrent use.
//
// No errors are expected during normal operation.
func (idx *AccountTransactions) Store(blockHeight uint64, txData []access.AccountTransaction) error {
	latestHeight := idx.latestHeight.Load()

	// Allow re-indexing the same height (idempotent)
	if blockHeight == latestHeight {
		return nil
	}

	expectedHeight := latestHeight + 1
	if blockHeight != expectedHeight && blockHeight != latestHeight {
		return fmt.Errorf("must index consecutive heights: expected %d, got %d", expectedHeight, blockHeight)
	}

	batch := idx.db.NewBatch()
	defer batch.Close()

	for _, entry := range txData {
		if entry.BlockHeight != blockHeight {
			return fmt.Errorf("block height mismatch: expected %d, got %d", blockHeight, entry.BlockHeight)
		}

		key := makeAccountTxKey(entry.Address, entry.BlockHeight, entry.TransactionID, entry.TransactionIndex)
		value := encodeAccountTxValue(entry.IsAuthorizer)

		if err := batch.Set(key, value, nil); err != nil {
			return fmt.Errorf("could not set key for account %s, tx %s: %w", entry.Address, entry.TransactionID, err)
		}
	}

	// Update latest height
	if err := batch.Set(accountTxLatestHeightKey, encodedUint64(blockHeight), nil); err != nil {
		return fmt.Errorf("could not update latest height: %w", err)
	}

	if err := batch.Commit(pebble.Sync); err != nil {
		return fmt.Errorf("could not commit batch: %w", err)
	}

	idx.latestHeight.Store(blockHeight)

	return nil
}

// makeAccountTxKey creates a full key for an account transaction index entry.
// Key format: [prefix][address][~block_height][tx_id][tx_index]
func makeAccountTxKey(address flow.Address, height uint64, txID flow.Identifier, txIndex uint32) []byte {
	key := make([]byte, accountTxKeyLen)

	key[0] = codeAccountTxIndex
	copy(key[1:1+flow.AddressLength], address[:])

	// One's complement of height for descending order
	onesComplement := ^height
	binary.BigEndian.PutUint64(key[1+flow.AddressLength:], onesComplement)

	copy(key[1+flow.AddressLength+8:], txID[:])
	binary.BigEndian.PutUint32(key[1+flow.AddressLength+8+flow.IdentifierLen:], txIndex)

	return key
}

// makeAccountTxKeyPrefix creates a prefix key for iteration, up to and including the height.
// This is used to set iterator bounds for height range queries.
func makeAccountTxKeyPrefix(address flow.Address, height uint64) []byte {
	prefix := make([]byte, accountTxPrefixWithHeightLen)

	prefix[0] = codeAccountTxIndex
	copy(prefix[1:1+flow.AddressLength], address[:])

	// One's complement of height for descending order
	onesComplement := ^height
	binary.BigEndian.PutUint64(prefix[1+flow.AddressLength:], onesComplement)

	return prefix
}

// decodeAccountTxKey decodes a key and value into an AccountTransaction.
func decodeAccountTxKey(key []byte, value []byte) (access.AccountTransaction, error) {
	if len(key) != accountTxKeyLen {
		return access.AccountTransaction{}, fmt.Errorf("invalid key length: expected %d, got %d",
			accountTxKeyLen, len(key))
	}

	if key[0] != codeAccountTxIndex {
		return access.AccountTransaction{}, fmt.Errorf("invalid prefix: expected %d, got %d",
			codeAccountTxIndex, key[0])
	}

	// Skip prefix
	offset := 1

	address := flow.BytesToAddress(key[offset : offset+flow.AddressLength])
	offset += flow.AddressLength

	// Decode height (one's complement)
	onesComplement := binary.BigEndian.Uint64(key[offset:])
	height := ^onesComplement
	offset += 8

	// Decode transaction ID
	txID, err := flow.ByteSliceToId(key[offset : offset+flow.IdentifierLen])
	if err != nil {
		return access.AccountTransaction{}, fmt.Errorf("could not decode transaction ID (key %x): %w", key, err)
	}
	offset += flow.IdentifierLen

	// Decode transaction index
	txIndex := binary.BigEndian.Uint32(key[offset:])

	// Decode value
	isAuthorizer := decodeAccountTxValue(value)

	return access.AccountTransaction{
		Address:          address,
		BlockHeight:      height,
		TransactionID:    txID,
		TransactionIndex: txIndex,
		IsAuthorizer:     isAuthorizer,
	}, nil
}

// encodeAccountTxValue encodes the IsAuthorizer boolean as a single byte.
func encodeAccountTxValue(isAuthorizer bool) []byte {
	if isAuthorizer {
		return []byte{1}
	}
	return []byte{0}
}

// decodeAccountTxValue decodes the IsAuthorizer boolean from a single byte.
func decodeAccountTxValue(value []byte) bool {
	if len(value) == 0 {
		return false
	}
	return value[0] != 0
}

// incrementBytes returns a copy of the byte slice with the last byte incremented.
// This is used to create exclusive upper bounds for iteration.
// If the last byte would overflow, it carries to the previous byte, etc.
func incrementBytes(b []byte) []byte {
	result := make([]byte, len(b))
	copy(result, b)

	for i := len(result) - 1; i >= 0; i-- {
		result[i]++
		if result[i] != 0 {
			break
		}
	}

	return result
}

// accountTxFirstStoredHeight reads the first indexed height from the database.
func accountTxFirstStoredHeight(db *pebble.DB) (uint64, error) {
	return accountTxHeightLookup(db, accountTxFirstHeightKey)
}

// accountTxLatestStoredHeight reads the latest indexed height from the database.
func accountTxLatestStoredHeight(db *pebble.DB) (uint64, error) {
	return accountTxHeightLookup(db, accountTxLatestHeightKey)
}

// accountTxHeightLookup reads a height value from the database.
func accountTxHeightLookup(db *pebble.DB, key []byte) (uint64, error) {
	res, closer, err := db.Get(key)
	if err != nil {
		return 0, convertNotFoundError(err)
	}
	defer closer.Close()

	if len(res) != 8 {
		return 0, fmt.Errorf("invalid height value length: expected 8, got %d", len(res))
	}

	return binary.BigEndian.Uint64(res), nil
}
