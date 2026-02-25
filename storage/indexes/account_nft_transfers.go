package indexes

import (
	"encoding/binary"
	"fmt"

	"github.com/jordanschalm/lockctx"
	"github.com/vmihailenco/msgpack/v4"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/indexes/iterator"
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
	*IndexState
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
	state, err := NewIndexState(
		db,
		storage.LockIndexNonFungibleTokenTransfers,
		keyAccountNFTTransferFirstHeightKey,
		keyAccountNFTTransferLatestHeightKey,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create index state: %w", err)
	}
	return &NonFungibleTokenTransfers{IndexState: state}, nil
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
	state, err := BootstrapIndexState(
		lctx,
		rw,
		db,
		storage.LockIndexNonFungibleTokenTransfers,
		keyAccountNFTTransferFirstHeightKey,
		keyAccountNFTTransferLatestHeightKey,
		initialStartHeight,
	)
	if err != nil {
		return nil, fmt.Errorf("could not bootstrap non-fungible token transfers: %w", err)
	}

	if err := storeAllNFTTransfers(rw, initialStartHeight, transfers); err != nil {
		return nil, fmt.Errorf("could not store non-fungible token transfers: %w", err)
	}

	return &NonFungibleTokenTransfers{IndexState: state}, nil
}

// ByAddress returns an iterator over non-fungible token transfers involving the given account,
// ordered in descending block height (newest first), with ascending transaction and event
// index within each block. Returns an exhausted iterator and no error if the account has
// no transfers.
//
// `cursor` is a pointer to an [access.TransferCursor]:
//   - nil means start from the latest indexed height
//   - non-nil means start at the cursor position (inclusive)
//
// Expected error returns during normal operations:
//   - [storage.ErrHeightNotIndexed] if the cursor height is outside of the indexed range
func (idx *NonFungibleTokenTransfers) ByAddress(
	account flow.Address,
	cursor *access.TransferCursor,
) (storage.IndexIterator[access.NonFungibleTokenTransfer, access.TransferCursor], error) {
	latestHeight := idx.latestHeight.Load()
	startKey := makeNFTTransferKeyPrefix(account, latestHeight)

	if cursor != nil {
		if err := validateCursorHeight(cursor.BlockHeight, idx.firstHeight, latestHeight); err != nil {
			return nil, err
		}
		startKey = makeNFTTransferKey(account, cursor.BlockHeight, cursor.TransactionIndex, cursor.EventIndex)
	}

	endKey := makeNFTTransferKeyPrefix(account, idx.firstHeight)

	iter, err := idx.db.Reader().NewIter(startKey, endKey, storage.DefaultIteratorOptions())
	if err != nil {
		return nil, fmt.Errorf("could not create iterator: %w", err)
	}

	return iterator.Build(iter, decodeNFTTransferKeyCursor, reconstructNFTTransfer), nil
}

// Store indexes all non-fungible token transfers for a block.
// Each transfer is indexed under both the source and recipient addresses.
// Must be called sequentially with consecutive heights (latestHeight + 1).
// The caller must hold the [storage.LockIndexNonFungibleTokenTransfers] lock until the batch is committed.
//
// Expected error returns during normal operations:
//   - [storage.ErrAlreadyExists] if the block height is already indexed
func (idx *NonFungibleTokenTransfers) Store(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockHeight uint64, transfers []access.NonFungibleTokenTransfer) error {
	if err := idx.PrepareStore(lctx, rw, blockHeight); err != nil {
		return fmt.Errorf("could not prepare store for block %d: %w", blockHeight, err)
	}

	return storeAllNFTTransfers(rw, blockHeight, transfers)
}

// decodeNFTTransferKeyCursor decodes a non-fungible token transfer key into a [access.TransferCursor].
//
// Any error indicates the key is not valid.
func decodeNFTTransferKeyCursor(key []byte) (access.TransferCursor, error) {
	_, height, txIndex, eventIndex, err := decodeNFTTransferKey(key)
	if err != nil {
		return access.TransferCursor{}, err
	}
	return access.TransferCursor{
		BlockHeight:      height,
		TransactionIndex: txIndex,
		EventIndex:       eventIndex,
	}, nil
}

// reconstructNFTTransfer decodes a stored value into an [access.NonFungibleTokenTransfer].
//
// Any error indicates the value is not valid.
func reconstructNFTTransfer(cursor access.TransferCursor, value []byte, dest *access.NonFungibleTokenTransfer) error {
	var stored storedNonFungibleTokenTransfer
	if err := msgpack.Unmarshal(value, &stored); err != nil {
		return fmt.Errorf("could not decode value: %w", err)
	}
	*dest = access.NonFungibleTokenTransfer{
		TransactionID:    stored.TransactionID,
		BlockHeight:      cursor.BlockHeight,
		TransactionIndex: cursor.TransactionIndex,
		EventIndices:     stored.EventIndices,
		SourceAddress:    stored.SourceAddress,
		RecipientAddress: stored.RecipientAddress,
		TokenType:        stored.TokenType,
		ID:               stored.ID,
	}
	return nil
}

// storeAllNFTTransfers writes all non-fungible token transfer entries for a block.
// Each transfer produces two entries: one keyed by source address and one keyed by recipient address.
// The caller must hold the [storage.LockIndexNonFungibleTokenTransfers] lock until the batch is committed.
//
// No error returns are expected during normal operation.
func storeAllNFTTransfers(rw storage.ReaderBatchWriter, blockHeight uint64, transfers []access.NonFungibleTokenTransfer) error {
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
