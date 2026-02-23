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

// NonFungibleTokenTransfers implements [storage.NonFungibleTokenTransfers] using Pebble.
// It provides an index mapping accounts to their non-fungible token transfers, ordered by block height
// in descending order (newest first).
//
// Each transfer is indexed under both the source and recipient addresses.
//
// Key format: [prefix][address][~block_height][tx_index][event_index]
// - prefix: 1 byte (codeNonFungibleTokenTransfers)
// - address: 8 bytes ([flow.Address])
// - ~block_height: 8 bytes (one's complement for descending sort)
// - tx_index: 4 bytes (uint32, big-endian)
// - event_index: 4 bytes (uint32, big-endian)
//
// Value format: [storedNonFungibleTokenTransfer]
//
// All read methods are safe for concurrent access. Write methods (Store)
// must be called sequentially with consecutive heights.
type NonFungibleTokenTransfers struct {
	db           storage.DB
	firstHeight  uint64
	latestHeight *atomic.Uint64
}

// storedNonFungibleTokenTransfer is the internal value stored in the database for each NFT transfer entry.
type storedNonFungibleTokenTransfer struct {
	TransactionID    flow.Identifier
	EventIndices     []uint32
	SourceAddress    flow.Address
	RecipientAddress flow.Address
	TokenType        string
	ID               uint64
}

const (
	// nftTransferKeyLen is the total length of a non-fungible token transfer index key
	// 1 (prefix) + 8 (address) + 8 (height) + 4 (txIndex) + 4 (eventIndex) = 25
	nftTransferKeyLen = 1 + flow.AddressLength + 8 + 4 + 4

	// nftTransferPrefixLen is the length of the prefix used for iteration (prefix + address)
	nftTransferPrefixLen = 1 + flow.AddressLength

	// nftTransferPrefixWithHeightLen includes the height for range queries
	nftTransferPrefixWithHeightLen = nftTransferPrefixLen + 8
)

var _ storage.NonFungibleTokenTransfers = (*NonFungibleTokenTransfers)(nil)

// NewNonFungibleTokenTransfers creates a new NonFungibleTokenTransfers backed by the given database.
//
// If the index has not been initialized, construction will fail with [storage.ErrNotBootstrapped].
// The caller should retry with [BootstrapNonFungibleTokenTransfers] passing the required initialization data.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotBootstrapped] if the index has not been initialized
func NewNonFungibleTokenTransfers(db storage.DB) (*NonFungibleTokenTransfers, error) {
	firstHeight, err := readHeight(db.Reader(), keyAccountNFTTransferFirstHeightKey)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, storage.ErrNotBootstrapped
		}
		return nil, fmt.Errorf("could not get first height: %w", err)
	}

	persistedLatestHeight, err := readHeight(db.Reader(), keyAccountNFTTransferLatestHeightKey)
	if err != nil {
		return nil, fmt.Errorf("could not get latest height: %w", err)
	}

	return &NonFungibleTokenTransfers{
		db:           db,
		firstHeight:  firstHeight,
		latestHeight: atomic.NewUint64(persistedLatestHeight),
	}, nil
}

// BootstrapNonFungibleTokenTransfers initializes the non-fungible token transfer index with data from the
// first block, and returns a new [NonFungibleTokenTransfers] instance.
// The caller must hold the [storage.LockIndexNonFungibleTokenTransfers] lock until the batch is committed.
//
// Expected error returns during normal operations:
//   - [storage.ErrAlreadyExists] if data is found while initializing
func BootstrapNonFungibleTokenTransfers(
	lctx lockctx.Proof,
	rw storage.ReaderBatchWriter,
	db storage.DB,
	initialStartHeight uint64,
	transfers []access.NonFungibleTokenTransfer,
) (*NonFungibleTokenTransfers, error) {
	err := initializeNFTTransfers(lctx, rw, initialStartHeight, transfers)
	if err != nil {
		return nil, fmt.Errorf("could not bootstrap non-fungible token transfers: %w", err)
	}

	return &NonFungibleTokenTransfers{
		db:           db,
		firstHeight:  initialStartHeight,
		latestHeight: atomic.NewUint64(initialStartHeight),
	}, nil
}

// FirstIndexedHeight returns the first (oldest) block height that has been indexed.
func (idx *NonFungibleTokenTransfers) FirstIndexedHeight() uint64 {
	return idx.firstHeight
}

// LatestIndexedHeight returns the latest block height that has been indexed.
func (idx *NonFungibleTokenTransfers) LatestIndexedHeight() uint64 {
	return idx.latestHeight.Load()
}

// ByAddress retrieves non-fungible token transfers involving the given account,
// using cursor-based pagination. Results are returned in descending order (newest first).
//
// If `cursor` is nil, the query starts from the latest indexed height.
// If `cursor` is provided, the query resumes from the cursor position.
//
// Expected error returns during normal operations:
//   - [storage.ErrHeightNotIndexed] if the cursor height is outside of the indexed range
func (idx *NonFungibleTokenTransfers) ByAddress(
	account flow.Address,
	limit uint32,
	cursor *access.TransferCursor,
	filter storage.IndexFilter[*access.NonFungibleTokenTransfer],
) (access.NonFungibleTokenTransfersPage, error) {
	if err := validateLimit(limit); err != nil {
		return access.NonFungibleTokenTransfersPage{}, errors.Join(storage.ErrInvalidQuery, err)
	}

	latestHeight := idx.latestHeight.Load()
	if cursor != nil {
		if err := validateCursorHeight(cursor.BlockHeight, idx.firstHeight, latestHeight); err != nil {
			return access.NonFungibleTokenTransfersPage{}, err
		}
		latestHeight = cursor.BlockHeight
	}

	page, err := lookupNFTTransfers(idx.db.Reader(), account, idx.firstHeight, latestHeight, limit, cursor, filter)
	if err != nil {
		return access.NonFungibleTokenTransfersPage{}, fmt.Errorf("could not lookup non-fungible token transfers: %w", err)
	}

	return page, nil
}

// Store indexes all non-fungible token transfers for a block.
// Each transfer is indexed under both the source and recipient addresses.
// Must be called sequentially with consecutive heights (latestHeight + 1).
// The caller must hold the [storage.LockIndexNonFungibleTokenTransfers] lock until the batch is committed.
//
// Expected error returns during normal operations:
//   - [storage.ErrAlreadyExists] if the block height is already indexed
func (idx *NonFungibleTokenTransfers) Store(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockHeight uint64, transfers []access.NonFungibleTokenTransfer) error {
	latestHeight := idx.latestHeight.Load()
	if err := validateStoreHeight(blockHeight, latestHeight); err != nil {
		return err
	}

	err := indexNFTTransfers(lctx, rw, blockHeight, transfers)
	if err != nil {
		return fmt.Errorf("could not index non-fungible token transfers: %w", err)
	}

	storage.OnCommitSucceed(rw, func() {
		idx.latestHeight.Store(blockHeight)
	})

	return nil
}

// lookupNFTTransfers retrieves non-fungible token transfers for a given address within the specified
// block height range (inclusive), using cursor-based pagination. Results are returned in descending
// order (newest first). Returns an empty page if no transfers are found.
//
// No error returns are expected during normal operation.
func lookupNFTTransfers(
	reader storage.Reader,
	address flow.Address,
	lowestHeight uint64,
	highestHeight uint64,
	limit uint32,
	cursor *access.TransferCursor,
	filter storage.IndexFilter[*access.NonFungibleTokenTransfer],
) (access.NonFungibleTokenTransfersPage, error) {
	startKey := makeNFTTransferKeyPrefix(address, highestHeight)
	endKey := makeNFTTransferKeyPrefix(address, lowestHeight)

	fetchLimit := limit + 1

	var collected []access.NonFungibleTokenTransfer
	skipFirst := cursor != nil

	err := operation.IterateKeys(reader, startKey, endKey,
		func(keyCopy []byte, getValue func(any) error) (bail bool, err error) {
			_, height, txIndex, eventIndex, err := decodeNFTTransferKey(keyCopy)
			if err != nil {
				return true, fmt.Errorf("could not decode key: %w", err)
			}

			// Skip entries at or before the cursor position
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
				skipFirst = false
			}

			var stored storedNonFungibleTokenTransfer
			if err := getValue(&stored); err != nil {
				return true, fmt.Errorf("could not unmarshal value: %w", err)
			}

			transfer := access.NonFungibleTokenTransfer{
				TransactionID:    stored.TransactionID,
				BlockHeight:      height,
				TransactionIndex: txIndex,
				EventIndices:     stored.EventIndices,
				SourceAddress:    stored.SourceAddress,
				RecipientAddress: stored.RecipientAddress,
				TokenType:        stored.TokenType,
				ID:               stored.ID,
			}

			if filter != nil && !filter(&transfer) {
				return false, nil
			}

			collected = append(collected, transfer)

			if uint32(len(collected)) >= fetchLimit {
				return true, nil
			}

			return false, nil
		}, storage.DefaultIteratorOptions())

	if err != nil {
		return access.NonFungibleTokenTransfersPage{}, fmt.Errorf("could not iterate keys: %w", err)
	}

	page := access.NonFungibleTokenTransfersPage{}

	if uint32(len(collected)) > limit {
		page.Transfers = collected[:limit]
		last := collected[limit-1]
		page.NextCursor = &access.TransferCursor{
			BlockHeight:      last.BlockHeight,
			TransactionIndex: last.TransactionIndex,
			EventIndex:       nftTransferEventIndex(last),
		}
	} else {
		page.Transfers = collected
	}

	return page, nil
}

// indexNFTTransfers indexes all non-fungible token transfers for a block.
// Each transfer produces two entries: one keyed by source address and one keyed by recipient address.
// The caller must hold the [storage.LockIndexNonFungibleTokenTransfers] lock until the batch is committed.
//
// No error returns are expected during normal operation.
func indexNFTTransfers(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockHeight uint64, transfers []access.NonFungibleTokenTransfer) error {
	if !lctx.HoldsLock(storage.LockIndexNonFungibleTokenTransfers) {
		return fmt.Errorf("missing required lock: %s", storage.LockIndexNonFungibleTokenTransfers)
	}

	latestHeight, err := readHeight(rw.GlobalReader(), keyAccountNFTTransferLatestHeightKey)
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

		value := makeNFTTransferValue(entry)

		eventIndex := nftTransferEventIndex(entry)

		// Index under source address
		sourceKey := makeNFTTransferKey(entry.SourceAddress, entry.BlockHeight, entry.TransactionIndex, eventIndex)
		if err := operation.UpsertByKey(writer, sourceKey, value); err != nil {
			return fmt.Errorf("could not set key for source %s, tx %s: %w", entry.SourceAddress, entry.TransactionID, err)
		}

		// Index under recipient address
		recipientKey := makeNFTTransferKey(entry.RecipientAddress, entry.BlockHeight, entry.TransactionIndex, eventIndex)
		if err := operation.UpsertByKey(writer, recipientKey, value); err != nil {
			return fmt.Errorf("could not set key for recipient %s, tx %s: %w", entry.RecipientAddress, entry.TransactionID, err)
		}
	}

	// Update latest height
	if err := operation.UpsertByKey(writer, keyAccountNFTTransferLatestHeightKey, blockHeight); err != nil {
		return fmt.Errorf("could not update latest height: %w", err)
	}

	return nil
}

// initializeNFTTransfers initializes the non-fungible token transfer index with data from the first block.
// The caller must hold the [storage.LockIndexNonFungibleTokenTransfers] lock until the batch is committed.
//
// Expected error returns during normal operations:
//   - [storage.ErrAlreadyExists] if the bounds keys already exist
func initializeNFTTransfers(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockHeight uint64, transfers []access.NonFungibleTokenTransfer) error {
	if !lctx.HoldsLock(storage.LockIndexNonFungibleTokenTransfers) {
		return fmt.Errorf("missing required lock: %s", storage.LockIndexNonFungibleTokenTransfers)
	}

	exists, err := operation.KeyExists(rw.GlobalReader(), keyAccountNFTTransferFirstHeightKey)
	if err != nil {
		return fmt.Errorf("could not check if first height key exists: %w", err)
	}
	if exists {
		return fmt.Errorf("first height key already exists: %w", storage.ErrAlreadyExists)
	}

	exists, err = operation.KeyExists(rw.GlobalReader(), keyAccountNFTTransferLatestHeightKey)
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

		value := makeNFTTransferValue(entry)

		eventIndex := nftTransferEventIndex(entry)

		// Index under source address
		sourceKey := makeNFTTransferKey(entry.SourceAddress, entry.BlockHeight, entry.TransactionIndex, eventIndex)
		if err := operation.UpsertByKey(writer, sourceKey, value); err != nil {
			return fmt.Errorf("could not set key for source %s, tx %s: %w", entry.SourceAddress, entry.TransactionID, err)
		}

		// Index under recipient address
		recipientKey := makeNFTTransferKey(entry.RecipientAddress, entry.BlockHeight, entry.TransactionIndex, eventIndex)
		if err := operation.UpsertByKey(writer, recipientKey, value); err != nil {
			return fmt.Errorf("could not set key for recipient %s, tx %s: %w", entry.RecipientAddress, entry.TransactionID, err)
		}
	}

	if err := operation.UpsertByKey(writer, keyAccountNFTTransferFirstHeightKey, blockHeight); err != nil {
		return fmt.Errorf("could not update first height: %w", err)
	}
	if err := operation.UpsertByKey(writer, keyAccountNFTTransferLatestHeightKey, blockHeight); err != nil {
		return fmt.Errorf("could not update latest height: %w", err)
	}

	return nil
}

func nftTransferEventIndex(entry access.NonFungibleTokenTransfer) uint32 {
	// use the last event index. this is either the deposit event or the last withdrawal event
	// if the vault was destroyed.
	return entry.EventIndices[len(entry.EventIndices)-1]
}

// makeNFTTransferValue builds the stored value for a non-fungible token transfer index entry.
func makeNFTTransferValue(entry access.NonFungibleTokenTransfer) storedNonFungibleTokenTransfer {
	return storedNonFungibleTokenTransfer{
		TransactionID:    entry.TransactionID,
		EventIndices:     entry.EventIndices,
		SourceAddress:    entry.SourceAddress,
		RecipientAddress: entry.RecipientAddress,
		TokenType:        entry.TokenType,
		ID:               entry.ID,
	}
}

// makeNFTTransferKey creates a full key for a non-fungible token transfer index entry.
// Key format: [prefix][address][~block_height][tx_index][event_index]
func makeNFTTransferKey(address flow.Address, height uint64, txIndex uint32, eventIndex uint32) []byte {
	key := make([]byte, nftTransferKeyLen)

	key[0] = codeAccountNonFungibleTokenTransfers
	copy(key[1:1+flow.AddressLength], address[:])

	binary.BigEndian.PutUint64(key[1+flow.AddressLength:], ^height)
	binary.BigEndian.PutUint32(key[1+flow.AddressLength+8:], txIndex)
	binary.BigEndian.PutUint32(key[1+flow.AddressLength+8+4:], eventIndex)

	return key
}

// makeNFTTransferKeyPrefix creates a prefix key for iteration, up to and including the height.
// Key format: [prefix][address][~block_height]
func makeNFTTransferKeyPrefix(address flow.Address, height uint64) []byte {
	prefix := make([]byte, nftTransferPrefixWithHeightLen)

	prefix[0] = codeAccountNonFungibleTokenTransfers
	copy(prefix[1:1+flow.AddressLength], address[:])

	binary.BigEndian.PutUint64(prefix[1+flow.AddressLength:], ^height)

	return prefix
}

// decodeNFTTransferKey decodes a non-fungible token transfer key into its components.
//
// Any error indicates the key is not valid.
func decodeNFTTransferKey(key []byte) (flow.Address, uint64, uint32, uint32, error) {
	if len(key) != nftTransferKeyLen {
		return flow.Address{}, 0, 0, 0, fmt.Errorf("invalid key length: expected %d, got %d",
			nftTransferKeyLen, len(key))
	}

	if key[0] != codeAccountNonFungibleTokenTransfers {
		return flow.Address{}, 0, 0, 0, fmt.Errorf("invalid prefix: expected %d, got %d",
			codeAccountNonFungibleTokenTransfers, key[0])
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
