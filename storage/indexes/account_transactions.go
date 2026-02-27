package indexes

import (
	"encoding/binary"
	"fmt"
	"slices"

	"github.com/jordanschalm/lockctx"
	"github.com/vmihailenco/msgpack/v4"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/indexes/iterator"
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
// Value format: storedAccountTransaction
// - tx_id: 32 bytes (flow.Identifier)
// - roles: variable length ([]access.TransactionRole)
//
// All read methods are safe for concurrent access. Write methods (Store)
// must be called sequentially with consecutive heights.
type AccountTransactions struct {
	*IndexState
}

type storedAccountTransaction struct {
	TransactionID flow.Identifier
	Roles         []access.TransactionRole
}

const (
	// accountTxKeyLen is the total length of an account transaction index key
	// 1 (prefix) + 8 (address) + 8 (height) + 4 (txIndex) = 21
	accountTxKeyLen = 1 + flow.AddressLength + heightLen + txIndexLen

	// accountTxPrefixLen is the length of the prefix used for iteration (prefix + address)
	accountTxPrefixLen = 1 + flow.AddressLength

	// accountTxPrefixWithHeightLen includes the height for range queries
	accountTxPrefixWithHeightLen = accountTxPrefixLen + heightLen
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
	state, err := NewIndexState(
		db,
		storage.LockIndexAccountTransactions,
		keyAccountTransactionFirstHeightKey,
		keyAccountTransactionLatestHeightKey,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create index state: %w", err)
	}
	return &AccountTransactions{IndexState: state}, nil
}

// BootstrapAccountTransactions initializes the account transactions index with data from the first block,
// and returns a new [AccountTransactions] instance.
// The caller must hold the [storage.LockIndexAccountTransactions] lock until the batch is committed.
//
// Expected error returns during normal operations:
//   - [storage.ErrAlreadyExists] if any data is found while initializing
func BootstrapAccountTransactions(
	lctx lockctx.Proof,
	rw storage.ReaderBatchWriter,
	db storage.DB,
	initialStartHeight uint64,
	txData []access.AccountTransaction,
) (*AccountTransactions, error) {
	state, err := BootstrapIndexState(
		lctx,
		rw,
		db,
		storage.LockIndexAccountTransactions,
		keyAccountTransactionFirstHeightKey,
		keyAccountTransactionLatestHeightKey,
		initialStartHeight)
	if err != nil {
		return nil, fmt.Errorf("could not bootstrap account transactions: %w", err)
	}

	if err := storeAllAccountTransactions(rw, initialStartHeight, txData); err != nil {
		return nil, fmt.Errorf("could not store account transactions: %w", err)
	}

	return &AccountTransactions{IndexState: state}, nil
}

// ByAddress returns an iterator over transactions for the given account, ordered in descending
// block height (newest first), with ascending transaction index within each block.
// Returns an exhausted iterator and no error if the account has no transactions.
//
// `cursor` is a pointer to an [access.AccountTransactionCursor]:
//   - nil means start from the latest indexed height
//   - non-nil means start at the cursor position (inclusive)
//
// Expected error returns during normal operations:
//   - [storage.ErrHeightNotIndexed] if the cursor height extends beyond indexed heights
func (idx *AccountTransactions) ByAddress(
	account flow.Address,
	cursor *access.AccountTransactionCursor,
) (storage.IndexIterator[access.AccountTransaction, access.AccountTransactionCursor], error) {
	startKey, endKey, err := idx.rangeKeys(account, cursor)
	if err != nil {
		return nil, fmt.Errorf("could not determine range keys: %w", err)
	}

	iter, err := idx.db.Reader().NewIter(startKey, endKey, storage.DefaultIteratorOptions())
	if err != nil {
		return nil, fmt.Errorf("could not create iterator: %w", err)
	}

	return iterator.Build(iter, decodeAccountTxKey, reconstructAccountTransaction), nil
}

// rangeKeys computes the start and end keys for iterating over transactions of an account, based on
// the provided cursor.
//
// Any error indicates the cursor is invalid
func (idx *AccountTransactions) rangeKeys(account flow.Address, cursor *access.AccountTransactionCursor) (startKey, endKey []byte, err error) {
	latestHeight := idx.latestHeight.Load()
	if cursor == nil {
		// keys include the one's complement of the height, so iteration is in descending order of height.
		startKey = makeAccountTxKeyPrefix(account, latestHeight)
		endKey = makeAccountTxKeyPrefix(account, idx.firstHeight)
		return startKey, endKey, nil
	}

	if err := validateCursorHeight(cursor.BlockHeight, idx.firstHeight, latestHeight); err != nil {
		return nil, nil, err
	}

	// since the cursor may point to a transaction within idx.firstHeight, we need to use the last
	// possible key for the prefix.
	startKey = makeAccountTxKey(account, cursor.BlockHeight, cursor.TransactionIndex)
	endKey = makeAccountTxKeyPrefix(account, idx.firstHeight)
	endKey = storage.PrefixInclusiveEnd(endKey, startKey)

	return startKey, endKey, nil
}

// Store indexes all account-transaction associations for a block.
// Must be called sequentially with consecutive heights (latestHeight + 1).
// The caller must hold the [storage.LockIndexAccountTransactions] lock until the batch is committed.
//
// Expected error returns during normal operations:
//   - [storage.ErrAlreadyExists] if the block height is already indexed
func (idx *AccountTransactions) Store(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockHeight uint64, txData []access.AccountTransaction) error {
	if err := idx.PrepareStore(lctx, rw, blockHeight); err != nil {
		return fmt.Errorf("could not prepare store for block %d: %w", blockHeight, err)
	}

	return storeAllAccountTransactions(rw, blockHeight, txData)
}

// storeAllAccountTransactions stores all account transactions for a given block height.
// The caller must hold the [storage.LockIndexAccountTransactions] lock until the batch is committed.
//
// No error returns are expected during normal operation.
func storeAllAccountTransactions(rw storage.ReaderBatchWriter, blockHeight uint64, txData []access.AccountTransaction) error {
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
			// since the block height was already checked to be exactly the next expected height, there
			// should not be any data in the db for this height. if there is, the db is in an inconsistent
			// state.
			return fmt.Errorf("account transaction %s at height %d already indexed", entry.Address, entry.BlockHeight)
		}

		value := makeAccountTxValue(entry)
		if err := operation.UpsertByKey(writer, key, value); err != nil {
			return fmt.Errorf("could not set key for account %s, tx %s: %w", entry.Address, entry.TransactionID, err)
		}
	}

	return nil
}

func reconstructAccountTransaction(key access.AccountTransactionCursor, value []byte) (*access.AccountTransaction, error) {
	var stored storedAccountTransaction
	if err := msgpack.Unmarshal(value, &stored); err != nil {
		return nil, fmt.Errorf("could not decode value: %w", err)
	}
	return &access.AccountTransaction{
		Address:          key.Address,
		BlockHeight:      key.BlockHeight,
		TransactionID:    stored.TransactionID,
		TransactionIndex: key.TransactionIndex,
		Roles:            stored.Roles,
	}, nil
}

// makeAccountTxValue builds the value for an account transaction index entry.
func makeAccountTxValue(entry access.AccountTransaction) storedAccountTransaction {
	// enforce that stored roles are sorted in ascending order
	slices.Sort(entry.Roles)

	// deduplicate roles
	entry.Roles = slices.Compact(entry.Roles)

	return storedAccountTransaction{
		TransactionID: entry.TransactionID,
		Roles:         entry.Roles,
	}
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
func decodeAccountTxKey(key []byte) (access.AccountTransactionCursor, error) {
	if len(key) != accountTxKeyLen {
		return access.AccountTransactionCursor{}, fmt.Errorf("invalid key length: expected %d, got %d",
			accountTxKeyLen, len(key))
	}

	if key[0] != codeAccountTransactions {
		return access.AccountTransactionCursor{}, fmt.Errorf("invalid prefix: expected %d, got %d",
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

	return access.AccountTransactionCursor{
		Address:          address,
		BlockHeight:      height,
		TransactionIndex: txIndex,
	}, nil
}
