package indexes

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"github.com/jordanschalm/lockctx"
	"go.uber.org/atomic"

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
	db           storage.DB
	firstHeight  uint64
	latestHeight *atomic.Uint64
}

// storedFungibleTokenTransfer is the internal value stored in the database for each FT transfer entry.
// Amount is stored as []byte via [big.Int.Bytes] for msgpack compatibility.
type storedFungibleTokenTransfer struct {
	TransactionID    flow.Identifier
	EventIndices     []uint32 // Index of the event within the transaction
	SourceAddress    flow.Address
	RecipientAddress flow.Address
	TokenType        string
	Amount           []byte // big.Int.Bytes() representation
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
	firstHeight, err := heightLookup(db.Reader(), keyAccountFTTransferFirstHeightKey)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, storage.ErrNotBootstrapped
		}
		return nil, fmt.Errorf("could not get first height: %w", err)
	}

	persistedLatestHeight, err := heightLookup(db.Reader(), keyAccountFTTransferLatestHeightKey)
	if err != nil {
		return nil, fmt.Errorf("could not get latest height: %w", err)
	}

	return &FungibleTokenTransfers{
		db:           db,
		firstHeight:  firstHeight,
		latestHeight: atomic.NewUint64(persistedLatestHeight),
	}, nil
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
	err := initializeFTTransfers(lctx, rw, initialStartHeight, transfers)
	if err != nil {
		return nil, fmt.Errorf("could not bootstrap fungible token transfers: %w", err)
	}

	return &FungibleTokenTransfers{
		db:           db,
		firstHeight:  initialStartHeight,
		latestHeight: atomic.NewUint64(initialStartHeight),
	}, nil
}

// FirstIndexedHeight returns the first (oldest) block height that has been indexed.
func (idx *FungibleTokenTransfers) FirstIndexedHeight() uint64 {
	return idx.firstHeight
}

// LatestIndexedHeight returns the latest block height that has been indexed.
func (idx *FungibleTokenTransfers) LatestIndexedHeight() uint64 {
	return idx.latestHeight.Load()
}

// TransfersByAddress retrieves fungible token transfers involving the given account using cursor-based
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
func (idx *FungibleTokenTransfers) TransfersByAddress(
	account flow.Address,
	limit uint32,
	cursor *access.TransferCursor,
	filter storage.IndexFilter[*access.FungibleTokenTransfer],
) (access.FungibleTokenTransfersPage, error) {
	if limit == 0 {
		return access.FungibleTokenTransfersPage{}, fmt.Errorf("limit must be greater than 0")
	}

	latestHeight := idx.latestHeight.Load()
	if cursor != nil {
		if cursor.BlockHeight > latestHeight {
			return access.FungibleTokenTransfersPage{}, fmt.Errorf(
				"cursor height %d is greater than latest indexed height %d: %w",
				cursor.BlockHeight, latestHeight, storage.ErrHeightNotIndexed)
		}
		if cursor.BlockHeight < idx.firstHeight {
			return access.FungibleTokenTransfersPage{}, fmt.Errorf(
				"cursor height %d is before first indexed height %d: %w",
				cursor.BlockHeight, idx.firstHeight, storage.ErrHeightNotIndexed)
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
// Repeated calls at the latest height are a no-op.
// Must be called sequentially with consecutive heights (latestHeight + 1).
// The caller must hold the [storage.LockIndexFungibleTokenTransfers] lock until the batch is committed.
//
// Expected error returns during normal operations:
//   - [storage.ErrAlreadyExists] if the block height is already indexed
func (idx *FungibleTokenTransfers) Store(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockHeight uint64, transfers []access.FungibleTokenTransfer) error {
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

	err := indexFTTransfers(lctx, rw, blockHeight, transfers)
	if err != nil {
		return fmt.Errorf("could not index fungible token transfers: %w", err)
	}

	storage.OnCommitSucceed(rw, func() {
		idx.latestHeight.Store(blockHeight)
	})

	return nil
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
	if limit == 0 {
		return access.FungibleTokenTransfersPage{}, nil
	}

	// Start from the latest height (prefix covers all tx/event indexes at that height).
	startKey := makeFTTransferKeyPrefix(address, highestHeight)

	// End bound: first indexed height (inclusive via prefix).
	endKey := makeFTTransferKeyPrefix(address, lowestHeight)

	// We fetch limit+1 to determine if there are more results beyond this page.
	fetchLimit := limit + 1

	var collected []access.FungibleTokenTransfer
	skipFirst := cursor != nil // skip the entry at the exact cursor position

	err := operation.IterateKeys(reader, startKey, endKey,
		func(keyCopy []byte, getValue func(any) error) (bail bool, err error) {
			_, height, txIndex, eventIndex, err := decodeFTTransferKey(keyCopy)
			if err != nil {
				return true, fmt.Errorf("could not decode key: %w", err)
			}

			// Skip entries at or before the cursor position (it was the last item of the previous page).
			if skipFirst {
				if height > cursor.BlockHeight {
					return false, nil
				}
				if height == cursor.BlockHeight && txIndex < cursor.TransactionIndex {
					return false, nil
				}
				if height == cursor.BlockHeight && txIndex == cursor.TransactionIndex && eventIndex <= cursor.EventIndex {
					return false, nil
				}
				// Past the cursor position. Stop skipping.
				skipFirst = false
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

			if uint32(len(collected)) >= fetchLimit {
				return true, nil // bail after collecting enough
			}

			return false, nil
		}, storage.DefaultIteratorOptions())

	if err != nil {
		return access.FungibleTokenTransfersPage{}, fmt.Errorf("could not iterate keys: %w", err)
	}

	page := access.FungibleTokenTransfersPage{}

	if uint32(len(collected)) > limit {
		// The extra entry tells us there are more results.
		page.Transfers = collected[:limit]
		last := collected[limit-1]
		page.NextCursor = &access.TransferCursor{
			BlockHeight:      last.BlockHeight,
			TransactionIndex: last.TransactionIndex,
			EventIndex:       ftEventIndex(last),
		}
	} else {
		page.Transfers = collected
	}

	return page, nil
}

// indexFTTransfers indexes all fungible token transfers for a block.
// Each transfer produces two entries: one keyed by source address and one keyed by recipient address.
// The caller must hold the [storage.LockIndexFungibleTokenTransfers] lock until the batch is committed.
//
// No error returns are expected during normal operation.
func indexFTTransfers(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockHeight uint64, transfers []access.FungibleTokenTransfer) error {
	if !lctx.HoldsLock(storage.LockIndexFungibleTokenTransfers) {
		return fmt.Errorf("missing required lock: %s", storage.LockIndexFungibleTokenTransfers)
	}

	latestHeight, err := heightLookup(rw.GlobalReader(), keyAccountFTTransferLatestHeightKey)
	if err != nil {
		return fmt.Errorf("could not get latest indexed height: %w", err)
	}
	if blockHeight != latestHeight+1 {
		return fmt.Errorf("must index consecutive heights: expected %d, got %d", latestHeight+1, blockHeight)
	}

	writer := rw.Writer()

	for _, entry := range transfers {
		if entry.BlockHeight != blockHeight {
			return fmt.Errorf("block height mismatch: expected %d, got %d", blockHeight, entry.BlockHeight)
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

	// Update latest height
	if err := operation.UpsertByKey(writer, keyAccountFTTransferLatestHeightKey, blockHeight); err != nil {
		return fmt.Errorf("could not update latest height: %w", err)
	}

	return nil
}

// initializeFTTransfers initializes the fungible token transfer index with data from the first block.
// The caller must hold the [storage.LockIndexFungibleTokenTransfers] lock until the batch is committed.
//
// Expected error returns during normal operations:
//   - [storage.ErrAlreadyExists] if the bounds keys already exist
func initializeFTTransfers(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockHeight uint64, transfers []access.FungibleTokenTransfer) error {
	if !lctx.HoldsLock(storage.LockIndexFungibleTokenTransfers) {
		return fmt.Errorf("missing required lock: %s", storage.LockIndexFungibleTokenTransfers)
	}

	exists, err := operation.KeyExists(rw.GlobalReader(), keyAccountFTTransferFirstHeightKey)
	if err != nil {
		return fmt.Errorf("could not check if first height key exists: %w", err)
	}
	if exists {
		return fmt.Errorf("first height key already exists: %w", storage.ErrAlreadyExists)
	}

	exists, err = operation.KeyExists(rw.GlobalReader(), keyAccountFTTransferLatestHeightKey)
	if err != nil {
		return fmt.Errorf("could not check if latest height key exists: %w", err)
	}
	if exists {
		return fmt.Errorf("latest height key already exists: %w", storage.ErrAlreadyExists)
	}

	writer := rw.Writer()

	for _, entry := range transfers {
		if entry.BlockHeight != blockHeight {
			return fmt.Errorf("block height mismatch: expected %d, got %d", blockHeight, entry.BlockHeight)
		}

		value := makeFTTransferValue(entry)
		eventIndex := ftEventIndex(entry)

		// Index under source address
		sourceKey := makeFTTransferKey(entry.SourceAddress, entry.BlockHeight, entry.TransactionIndex, eventIndex)
		if err := operation.UpsertByKey(writer, sourceKey, value); err != nil {
			return fmt.Errorf("could not set key for source %s, tx %s: %w", entry.SourceAddress, entry.TransactionID, err)
		}

		// Index under recipient address
		recipientKey := makeFTTransferKey(entry.RecipientAddress, entry.BlockHeight, entry.TransactionIndex, eventIndex)
		if err := operation.UpsertByKey(writer, recipientKey, value); err != nil {
			return fmt.Errorf("could not set key for recipient %s, tx %s: %w", entry.RecipientAddress, entry.TransactionID, err)
		}
	}

	if err := operation.UpsertByKey(writer, keyAccountFTTransferFirstHeightKey, blockHeight); err != nil {
		return fmt.Errorf("could not update first height: %w", err)
	}
	if err := operation.UpsertByKey(writer, keyAccountFTTransferLatestHeightKey, blockHeight); err != nil {
		return fmt.Errorf("could not update latest height: %w", err)
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

// heightLookup reads a height value from the database.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotFound] if the height is not found
func heightLookup(reader storage.Reader, key []byte) (uint64, error) {
	var height uint64
	if err := operation.RetrieveByKey(reader, key, &height); err != nil {
		return 0, err
	}
	return height, nil
}
