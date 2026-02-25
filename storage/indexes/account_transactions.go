package indexes

import (
	"encoding/binary"
	"errors"
	"fmt"
	"slices"

	"github.com/jordanschalm/lockctx"

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
	accountTxKeyLen = 1 + flow.AddressLength + 8 + 4

	// accountTxPrefixLen is the length of the prefix used for iteration (prefix + address)
	accountTxPrefixLen = 1 + flow.AddressLength

	// accountTxPrefixWithHeightLen includes the height for range queries
	accountTxPrefixWithHeightLen = accountTxPrefixLen + 8
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

// ByAddress retrieves transaction references for an account using cursor-based pagination.
// Results are returned in descending order (newest first).
// Returns an empty page and no error if the account
//
// `limit` specifies the maximum number of results to return per page.
//
// `cursor` is a pointer to an [access.AccountTransactionCursor]:
//   - nil means start from the latest indexed height (first page)
//   - non-nil means resume after the cursor position (subsequent pages)
//
// `filter` is an optional filter to apply to the results. If nil, all transactions will be returned.
// The filter is applied before calculating the limit. For pagination, to work correctly, the same
// filter must be applied to all pages.
//
// Expected error returns during normal operations:
//   - [storage.ErrHeightNotIndexed] if the cursor height extends beyond indexed heights
func (idx *AccountTransactions) ByAddress(
	account flow.Address,
	limit uint32,
	cursor *access.AccountTransactionCursor,
	filter storage.IndexFilter[*access.AccountTransaction],
) (access.AccountTransactionsPage, error) {
	if err := validateLimit(limit); err != nil {
		return access.AccountTransactionsPage{}, errors.Join(storage.ErrInvalidQuery, err)
	}

	latestHeight := idx.latestHeight.Load()
	if cursor != nil {
		if err := validateCursorHeight(cursor.BlockHeight, idx.firstHeight, latestHeight); err != nil {
			return access.AccountTransactionsPage{}, err
		}
		latestHeight = cursor.BlockHeight
	}

	page, err := lookupAccountTransactions(idx.db.Reader(), account, idx.firstHeight, latestHeight, limit, cursor, filter)
	if err != nil {
		return access.AccountTransactionsPage{}, fmt.Errorf("could not lookup account transactions: %w", err)
	}

	return page, nil
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

// lookupAccountTransactions retrieves account transactions for a given address using cursor-based
// pagination. Results are returned in descending order (newest first).
//
// If `cursor` is nil, iteration starts from latestHeight. If non-nil, iteration starts after the
// cursor position (the entry at the exact cursor position is skipped).
//
// The function collects up to `limit` entries, then peeks one more to determine whether a
// NextCursor should be set in the returned page.
// `limit` must be greater than 0.
//
// No error returns are expected during normal operation.
func lookupAccountTransactions(
	reader storage.Reader,
	address flow.Address,
	lowestHeight uint64,
	highestHeight uint64,
	limit uint32,
	cursor *access.AccountTransactionCursor,
	filter storage.IndexFilter[*access.AccountTransaction],
) (access.AccountTransactionsPage, error) {
	// Start from the latest height (prefix covers all tx indexes at that height).
	startKey := makeAccountTxKeyPrefix(address, highestHeight)

	// End bound: first indexed height (inclusive via prefix).
	endKey := makeAccountTxKeyPrefix(address, lowestHeight)

	// We fetch limit+1 to determine if there are more results beyond this page.
	// use uint64 to avoid overflows if limit is math.MaxUint32
	fetchLimit := uint64(limit) + 1

	var collected []access.AccountTransaction

	// TODO: construct the key, and use SeekGE to skip to the cursor position.
	err := operation.IterateKeys(reader, startKey, endKey,
		func(keyCopy []byte, getValue func(any) error) (bail bool, err error) {
			addr, height, txIndex, err := decodeAccountTxKey(keyCopy)
			if err != nil {
				return true, fmt.Errorf("could not decode key: %w", err)
			}

			// the cursor is the next entry to return. skip all entries before it.
			if cursor != nil {
				// heights are descending (stored as one's complement), and transaction indexes are ascending.
				if height > cursor.BlockHeight {
					return false, nil
				}
				if height == cursor.BlockHeight && txIndex < cursor.TransactionIndex {
					return false, nil
				}
			}

			var stored storedAccountTransaction
			if err := getValue(&stored); err != nil {
				return true, fmt.Errorf("could not unmarshal value: %w", err)
			}

			tx := access.AccountTransaction{
				Address:          addr,
				BlockHeight:      height,
				TransactionID:    stored.TransactionID,
				TransactionIndex: txIndex,
				Roles:            stored.Roles,
			}

			if filter != nil && !filter(&tx) {
				return false, nil
			}

			collected = append(collected, tx)

			if uint64(len(collected)) >= fetchLimit {
				return true, nil // bail after collecting enough
			}

			return false, nil
		}, storage.DefaultIteratorOptions())

	if err != nil {
		return access.AccountTransactionsPage{}, fmt.Errorf("could not iterate keys: %w", err)
	}

	if uint32(len(collected)) <= limit {
		return access.AccountTransactionsPage{
			Transactions: collected,
		}, nil
	}

	// we fetched one extra entry to check if there are more results. use it as the next cursor.
	nextEntry := collected[limit]
	return access.AccountTransactionsPage{
		Transactions: collected[:limit],
		NextCursor: &access.AccountTransactionCursor{
			BlockHeight:      nextEntry.BlockHeight,
			TransactionIndex: nextEntry.TransactionIndex,
		},
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
