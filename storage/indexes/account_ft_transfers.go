package indexes

import (
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/jordanschalm/lockctx"
	"github.com/vmihailenco/msgpack/v4"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/indexes/iterator"
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

// ByAddress returns an iterator over fungible token transfers involving the given account,
// ordered in descending block height (newest first), with ascending transaction and event
// index within each block. Returns an exhausted iterator and no error if the account has
// no transfers.
//
// `cursor` is a pointer to an [access.TransferCursor]:
//   - nil means start from the latest indexed height
//   - non-nil means start at the cursor position (inclusive)
//
// Expected error returns during normal operations:
//   - [storage.ErrHeightNotIndexed] if the cursor height extends beyond indexed heights
func (idx *FungibleTokenTransfers) ByAddress(
	account flow.Address,
	cursor *access.TransferCursor,
) (storage.IndexIterator[access.FungibleTokenTransfer, access.TransferCursor], error) {
	latestHeight := idx.latestHeight.Load()
	startKey := makeFTTransferKeyPrefix(account, latestHeight)

	if cursor != nil {
		if err := validateCursorHeight(cursor.BlockHeight, idx.firstHeight, latestHeight); err != nil {
			return nil, err
		}
		startKey = makeFTTransferKey(account, cursor.BlockHeight, cursor.TransactionIndex, cursor.EventIndex)
	}

	endKey := makeFTTransferKeyPrefix(account, idx.firstHeight)

	iter, err := idx.db.Reader().NewIter(startKey, endKey, storage.DefaultIteratorOptions())
	if err != nil {
		return nil, fmt.Errorf("could not create iterator: %w", err)
	}

	return iterator.Build(iter, decodeFTTransferKeyCursor, reconstructFTTransfer), nil
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

// decodeFTTransferKeyCursor decodes a fungible token transfer key into a [access.TransferCursor].
//
// Any error indicates the key is not valid.
func decodeFTTransferKeyCursor(key []byte) (access.TransferCursor, error) {
	_, height, txIndex, eventIndex, err := decodeFTTransferKey(key)
	if err != nil {
		return access.TransferCursor{}, err
	}
	return access.TransferCursor{
		BlockHeight:      height,
		TransactionIndex: txIndex,
		EventIndex:       eventIndex,
	}, nil
}

// reconstructFTTransfer decodes a stored value into an [access.FungibleTokenTransfer].
//
// Any error indicates the value is not valid.
func reconstructFTTransfer(cursor access.TransferCursor, value []byte, dest *access.FungibleTokenTransfer) error {
	var stored storedFungibleTokenTransfer
	if err := msgpack.Unmarshal(value, &stored); err != nil {
		return fmt.Errorf("could not decode value: %w", err)
	}
	*dest = access.FungibleTokenTransfer{
		TransactionID:    stored.TransactionID,
		BlockHeight:      cursor.BlockHeight,
		TransactionIndex: cursor.TransactionIndex,
		EventIndices:     stored.EventIndices,
		SourceAddress:    stored.SourceAddress,
		RecipientAddress: stored.RecipientAddress,
		TokenType:        stored.TokenType,
		Amount:           new(big.Int).SetBytes(stored.Amount),
	}
	return nil
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
	return nil
}
