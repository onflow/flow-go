package indexes

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

// FungibleTokenTransfers implements [storage.FungibleTokenTransfers] using Pebble.
// It provides an index mapping accounts to their fungible token transfers, ordered by block height
// in descending order (newest first).
//
// Each transfer is indexed under both the source and recipient addresses.
//
// Key format: [prefix][address][~block_height][tx_index][event_index]
// - prefix: 1 byte (codeFungibleTokenTransfers)
// - address: 8 bytes ([flow.Address])
// - ~block_height: 8 bytes (one's complement for descending sort)
// - tx_index: 4 bytes (uint32, big-endian)
// - event_index: 4 bytes (uint32, big-endian)
//
// Value format: [storedFungibleTokenTransfer]
//
// All read methods are safe for concurrent access. Write methods (Store)
// must be called sequentially with consecutive heights.
type FungibleTokenTransfers struct {
	*IndexState
}

// storedFungibleTokenTransfer is the internal value stored in the database for each FT transfer entry.
// Amount is stored as []byte via [big.Int.Bytes] for msgpack compatibility.
type storedFungibleTokenTransfer struct {
	TransactionID    flow.Identifier
	EventIndices     []uint32 // Index of the event within the transaction
	SourceAddress    flow.Address
	RecipientAddress flow.Address
	TokenType        string

	// Amount is the token amount transferred.
	// Stored as []byte (big.Int) instead of uint64 to allow storing both UFix64 and EVM UInt256 values.
	Amount []byte
}

const (
	// ftTransferKeyLen is the total length of a fungible token transfer index key
	// 1 (prefix) + 8 (address) + 8 (height) + 4 (txIndex) + 4 (eventIndex) = 25
	ftTransferKeyLen = 1 + flow.AddressLength + 8 + 4 + 4

	// ftTransferPrefixLen is the length of the prefix used for iteration (prefix + address)
	ftTransferPrefixLen = 1 + flow.AddressLength

	// ftTransferPrefixWithHeightLen includes the height for range queries
	ftTransferPrefixWithHeightLen = ftTransferPrefixLen + 8
)

var _ storage.FungibleTokenTransfers = (*FungibleTokenTransfers)(nil)

// NewFungibleTokenTransfers creates a new FungibleTokenTransfers backed by the given database.
//
// If the index has not been initialized, construction will fail with [storage.ErrNotBootstrapped].
// The caller should retry with [BootstrapFungibleTokenTransfers] passing the required initialization data.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotBootstrapped] if the index has not been initialized
func NewFungibleTokenTransfers(db storage.DB) (*FungibleTokenTransfers, error) {
	state, err := NewIndexState(
		db,
		storage.LockIndexFungibleTokenTransfers,
		keyAccountFTTransferFirstHeightKey,
		keyAccountFTTransferLatestHeightKey,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create index state: %w", err)
	}
	return &FungibleTokenTransfers{IndexState: state}, nil
}

// BootstrapFungibleTokenTransfers initializes the fungible token transfer index with data from the first block,
// and returns a new [FungibleTokenTransfers] instance.
// The caller must hold the [storage.LockIndexFungibleTokenTransfers] lock until the batch is committed.
//
// Expected error returns during normal operations:
//   - [storage.ErrAlreadyExists] if data is found while initializing
func BootstrapFungibleTokenTransfers(
	lctx lockctx.Proof,
	rw storage.ReaderBatchWriter,
	db storage.DB,
	initialStartHeight uint64,
	transfers []access.FungibleTokenTransfer,
) (*FungibleTokenTransfers, error) {
	state, err := BootstrapIndexState(
		lctx,
		rw,
		db,
		storage.LockIndexFungibleTokenTransfers,
		keyAccountFTTransferFirstHeightKey,
		keyAccountFTTransferLatestHeightKey,
		initialStartHeight,
	)
	if err != nil {
		return nil, fmt.Errorf("could not bootstrap fungible token transfers: %w", err)
	}

	if err := storeAllFTTransfers(rw, initialStartHeight, transfers); err != nil {
		return nil, fmt.Errorf("could not store fungible token transfers: %w", err)
	}

	return &FungibleTokenTransfers{IndexState: state}, nil
}

// ByAddress retrieves fungible token transfers involving the given account using cursor-based
// pagination. Results are returned in descending order (newest first).
//
// `limit` specifies the maximum number of results to return per page.
//
// `cursor` is a pointer to an [access.TransferCursor]:
//   - nil means start from the latest indexed height (first page)
//   - non-nil means resume after the cursor position (subsequent pages)
//
// `filter` is an optional filter to apply to the results. If nil, all transfers will be returned.
// The filter is applied before calculating the limit. For pagination to work correctly, the same
// filter must be applied to all pages.
//
// Expected error returns during normal operations:
//   - [storage.ErrHeightNotIndexed] if the cursor height extends beyond indexed heights
func (idx *FungibleTokenTransfers) ByAddress(
	account flow.Address,
	limit uint32,
	cursor *access.TransferCursor,
	filter storage.IndexFilter[*access.FungibleTokenTransfer],
) (access.FungibleTokenTransfersPage, error) {
	if err := validateLimit(limit); err != nil {
		return access.FungibleTokenTransfersPage{}, errors.Join(storage.ErrInvalidQuery, err)
	}

	latestHeight := idx.latestHeight.Load()
	if cursor != nil {
		if err := validateCursorHeight(cursor.BlockHeight, idx.firstHeight, latestHeight); err != nil {
			return access.FungibleTokenTransfersPage{}, err
		}
		latestHeight = cursor.BlockHeight
	}

	page, err := lookupFTTransfers(idx.db.Reader(), account, idx.firstHeight, latestHeight, limit, cursor, filter)
	if err != nil {
		return access.FungibleTokenTransfersPage{}, fmt.Errorf("could not lookup fungible token transfers: %w", err)
	}

	return page, nil
}

// Store indexes all fungible token transfers for a block.
// Each transfer is indexed under both the source and recipient addresses.
// Must be called sequentially with consecutive heights (latestHeight + 1).
// The caller must hold the [storage.LockIndexFungibleTokenTransfers] lock until the batch is committed.
//
// Expected error returns during normal operations:
//   - [storage.ErrAlreadyExists] if the block height is already indexed
func (idx *FungibleTokenTransfers) Store(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockHeight uint64, transfers []access.FungibleTokenTransfer) error {
	if err := idx.PrepareStore(lctx, rw, blockHeight); err != nil {
		return fmt.Errorf("could not prepare store for block %d: %w", blockHeight, err)
	}

	return storeAllFTTransfers(rw, blockHeight, transfers)
}

// lookupFTTransfers retrieves fungible token transfers for a given address using cursor-based
// pagination. Results are returned in descending order (newest first).
//
// If `cursor` is nil, iteration starts from highestHeight. If non-nil, iteration starts after the
// cursor position (the entry at the exact cursor position is skipped).
//
// The function collects up to `limit` entries, then peeks one more to determine whether a
// NextCursor should be set in the returned page.
//
// No error returns are expected during normal operation.
func lookupFTTransfers(
	reader storage.Reader,
	address flow.Address,
	lowestHeight uint64,
	highestHeight uint64,
	limit uint32,
	cursor *access.TransferCursor,
	filter storage.IndexFilter[*access.FungibleTokenTransfer],
) (access.FungibleTokenTransfersPage, error) {
	// Start from the latest height (prefix covers all tx/event indexes at that height).
	startKey := makeFTTransferKeyPrefix(address, highestHeight)

	// End bound: first indexed height (inclusive via prefix).
	endKey := makeFTTransferKeyPrefix(address, lowestHeight)

	// We fetch limit+1 to determine if there are more results beyond this page.
	// use uint64 to avoid overflows if limit is math.MaxUint32
	fetchLimit := uint64(limit) + 1

	var collected []access.FungibleTokenTransfer

	// TODO: construct the key, and use SeekGE to skip to the cursor position.
	err := operation.IterateKeys(reader, startKey, endKey,
		func(keyCopy []byte, getValue func(any) error) (bail bool, err error) {
			_, height, txIndex, eventIndex, err := decodeFTTransferKey(keyCopy)
			if err != nil {
				return true, fmt.Errorf("could not decode key: %w", err)
			}

			// the cursor is the next entry to return. skip all entries before it.
			if cursor != nil {
				// heights are descending (stored as one's complement); transaction and event indexes are ascending.
				if height > cursor.BlockHeight {
					return false, nil
				}
				if height == cursor.BlockHeight && txIndex < cursor.TransactionIndex {
					return false, nil
				}
				if height == cursor.BlockHeight && txIndex == cursor.TransactionIndex && eventIndex < cursor.EventIndex {
					return false, nil
				}
			}

			var stored storedFungibleTokenTransfer
			if err := getValue(&stored); err != nil {
				return true, fmt.Errorf("could not unmarshal value: %w", err)
			}

			transfer := access.FungibleTokenTransfer{
				TransactionID:    stored.TransactionID,
				BlockHeight:      height,
				TransactionIndex: txIndex,
				EventIndices:     stored.EventIndices,
				SourceAddress:    stored.SourceAddress,
				RecipientAddress: stored.RecipientAddress,
				TokenType:        stored.TokenType,
				Amount:           new(big.Int).SetBytes(stored.Amount),
			}

			if filter != nil && !filter(&transfer) {
				return false, nil
			}

			collected = append(collected, transfer)

			if uint64(len(collected)) >= fetchLimit {
				return true, nil // bail after collecting enough
			}

			return false, nil
		}, storage.DefaultIteratorOptions())

	if err != nil {
		return access.FungibleTokenTransfersPage{}, fmt.Errorf("could not iterate keys: %w", err)
	}

	if uint32(len(collected)) <= limit {
		return access.FungibleTokenTransfersPage{
			Transfers: collected,
		}, nil
	}

	// we fetched one extra entry to check if there are more results. use it as the next cursor.
	nextEntry := collected[limit]
	return access.FungibleTokenTransfersPage{
		Transfers: collected[:limit],
		NextCursor: &access.TransferCursor{
			BlockHeight:      nextEntry.BlockHeight,
			TransactionIndex: nextEntry.TransactionIndex,
			EventIndex:       ftEventIndex(nextEntry),
		},
	}, nil
}

// storeAllFTTransfers writes all fungible token transfer entries for a block.
// Each transfer produces two entries: one keyed by source address and one keyed by recipient address.
// The caller must hold the [storage.LockIndexFungibleTokenTransfers] lock until the batch is committed.
//
// No error returns are expected during normal operation.
func storeAllFTTransfers(rw storage.ReaderBatchWriter, blockHeight uint64, transfers []access.FungibleTokenTransfer) error {
	writer := rw.Writer()

	for _, entry := range transfers {
		if err := validateFTTransfer(blockHeight, entry); err != nil {
			return fmt.Errorf("invalid fungible token transfer: %w", err)
		}

		value := makeFTTransferValue(entry)
		eventIndex := ftEventIndex(entry)

		// Index under source address
		sourceKey := makeFTTransferKey(entry.SourceAddress, entry.BlockHeight, entry.TransactionIndex, eventIndex)
		if err := operation.UpsertByKey(writer, sourceKey, value); err != nil {
			return fmt.Errorf("could not set key for source %s, tx %s: %w", entry.SourceAddress, entry.TransactionID, err)
		}

		// Index under recipient address (if different from source, this creates a second entry;
		// if same, this is an idempotent overwrite with the same value)
		recipientKey := makeFTTransferKey(entry.RecipientAddress, entry.BlockHeight, entry.TransactionIndex, eventIndex)
		if err := operation.UpsertByKey(writer, recipientKey, value); err != nil {
			return fmt.Errorf("could not set key for recipient %s, tx %s: %w", entry.RecipientAddress, entry.TransactionID, err)
		}
	}

	return nil
}

func ftEventIndex(entry access.FungibleTokenTransfer) uint32 {
	// use the last event index. this is either the deposit event or the last withdrawal event
	// if the vault was destroyed.
	return entry.EventIndices[len(entry.EventIndices)-1]
}

// makeFTTransferValue builds the stored value for a fungible token transfer index entry.
func makeFTTransferValue(entry access.FungibleTokenTransfer) storedFungibleTokenTransfer {
	var amountBytes []byte
	if entry.Amount != nil {
		amountBytes = entry.Amount.Bytes()
	}
	return storedFungibleTokenTransfer{
		TransactionID:    entry.TransactionID,
		EventIndices:     entry.EventIndices,
		SourceAddress:    entry.SourceAddress,
		RecipientAddress: entry.RecipientAddress,
		TokenType:        entry.TokenType,
		Amount:           amountBytes,
	}
}

// makeFTTransferKey creates a full key for a fungible token transfer index entry.
// Key format: [prefix][address][~block_height][tx_index][event_index]
func makeFTTransferKey(address flow.Address, height uint64, txIndex uint32, eventIndex uint32) []byte {
	key := make([]byte, ftTransferKeyLen)

	key[0] = codeAccountFungibleTokenTransfers
	copy(key[1:1+flow.AddressLength], address[:])

	// One's complement of height for descending order
	binary.BigEndian.PutUint64(key[1+flow.AddressLength:], ^height)
	binary.BigEndian.PutUint32(key[1+flow.AddressLength+8:], txIndex)
	binary.BigEndian.PutUint32(key[1+flow.AddressLength+8+4:], eventIndex)

	return key
}

// makeFTTransferKeyPrefix creates a prefix key for iteration, up to and including the height.
// Key format: [prefix][address][~block_height]
func makeFTTransferKeyPrefix(address flow.Address, height uint64) []byte {
	prefix := make([]byte, ftTransferPrefixWithHeightLen)

	prefix[0] = codeAccountFungibleTokenTransfers
	copy(prefix[1:1+flow.AddressLength], address[:])

	binary.BigEndian.PutUint64(prefix[1+flow.AddressLength:], ^height)

	return prefix
}

// decodeFTTransferKey decodes a fungible token transfer key into its components.
//
// Any error indicates the key is not valid.
func decodeFTTransferKey(key []byte) (flow.Address, uint64, uint32, uint32, error) {
	if len(key) != ftTransferKeyLen {
		return flow.Address{}, 0, 0, 0, fmt.Errorf("invalid key length: expected %d, got %d",
			ftTransferKeyLen, len(key))
	}

	if key[0] != codeAccountFungibleTokenTransfers {
		return flow.Address{}, 0, 0, 0, fmt.Errorf("invalid prefix: expected %d, got %d",
			codeAccountFungibleTokenTransfers, key[0])
	}

	offset := 1

	address := flow.BytesToAddress(key[offset : offset+flow.AddressLength])
	offset += flow.AddressLength

	height := ^binary.BigEndian.Uint64(key[offset:])
	offset += 8

	txIndex := binary.BigEndian.Uint32(key[offset:])
	offset += 4

	eventIndex := binary.BigEndian.Uint32(key[offset:])

	return address, height, txIndex, eventIndex, nil
}

// validateFTTransfer validates the fungible token transfer is valid.
//
// Any error indicates the transfer is invalid.
func validateFTTransfer(blockHeight uint64, transfer access.FungibleTokenTransfer) error {
	if transfer.BlockHeight != blockHeight {
		return fmt.Errorf("block height mismatch: expected %d, got %d", blockHeight, transfer.BlockHeight)
	}
	if len(transfer.EventIndices) == 0 {
		return fmt.Errorf("transfer must have at least one event index (tx=%s)", transfer.TransactionID)
	}
	if transfer.Amount == nil {
		return fmt.Errorf("transfer amount is nil (tx=%s)", transfer.TransactionID)
	}
	return nil
}
