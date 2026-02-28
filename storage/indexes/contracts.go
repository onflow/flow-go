package indexes

import (
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/jordanschalm/lockctx"
	"github.com/vmihailenco/msgpack/v4"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/indexes/iterator"
	"github.com/onflow/flow-go/storage/operation"
)

const (
	// contractDeploymentKeyOverhead is the number of bytes in a contract deployment key that are not
	// part of the contractID. The format is:
	//
	//	[code(1)][contractID bytes][~height(8)][txIndex(4)][eventIndex(4)]
	//
	// so the overhead is code(1) + ~height(8) + txIndex(4) + eventIndex(4) = 17 bytes.
	contractDeploymentKeyOverhead = 1 + heightLen + txIndexLen + eventIndexLen

	// minContractIDLength is the minimum length of a contractID.
	// e.g. "[code][A.1234567890abcdef.a]" = 22 bytes
	// Address is a hex-encoded string
	minContractIDLength = 4 + flow.AddressLength*2 + 1
)

// storedContractDeployment holds the fields of a [access.ContractDeployment] that are not
// derivable from the primary key. Fields derivable from the key (ContractID, Address, Height,
// TxIndex, EventIndex) are not stored here.
type storedContractDeployment struct {
	TransactionID flow.Identifier
	Code          []byte
	CodeHash      []byte
}

// ContractDeploymentsIndex implements [storage.ContractDeploymentsIndex] using Pebble.
//
// Primary index key format:
//
//	[codeContractDeployment][contractID bytes][~height(8)][txIndex(4)][eventIndex(4)]
//
// The one's complement of height (~height) ensures that the most recent deployment (highest
// height) has the smallest byte value, so it appears first during ascending key iteration.
//
// [All] and [ByAddress] use [BuildPrefixIterator] over the primary index with the prefix
// [codeContractDeployment][contractID bytes], which yields exactly one entry (the most recent
// deployment) per contract without a separate secondary index.
//
// All read methods are safe for concurrent access. Write methods must be called sequentially.
type ContractDeploymentsIndex struct {
	*IndexState
}

var _ storage.ContractDeploymentsIndex = (*ContractDeploymentsIndex)(nil)

// NewContractDeploymentsIndex creates a new index backed by db.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotBootstrapped]: if the index has not been initialized
func NewContractDeploymentsIndex(db storage.DB) (*ContractDeploymentsIndex, error) {
	state, err := NewIndexState(
		db,
		storage.LockIndexContractDeployments,
		keyContractDeploymentFirstHeightKey,
		keyContractDeploymentLatestHeightKey,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create index state: %w", err)
	}
	return &ContractDeploymentsIndex{IndexState: state}, nil
}

// BootstrapContractDeployments initializes the index with the given start height and initial
// contract deployments, and returns a new [ContractDeploymentsIndex].
// The caller must hold the [storage.LockIndexContractDeployments] lock until the batch is committed.
//
// Expected error returns during normal operation:
//   - [storage.ErrAlreadyExists]: if any bounds key already exists
func BootstrapContractDeployments(
	lctx lockctx.Proof,
	rw storage.ReaderBatchWriter,
	db storage.DB,
	initialStartHeight uint64,
	deployments []access.ContractDeployment,
) (*ContractDeploymentsIndex, error) {
	state, err := BootstrapIndexState(
		lctx,
		rw,
		db,
		storage.LockIndexContractDeployments,
		keyContractDeploymentFirstHeightKey,
		keyContractDeploymentLatestHeightKey,
		initialStartHeight,
	)
	if err != nil {
		return nil, fmt.Errorf("could not bootstrap contract deployments: %w", err)
	}

	if err := storeAllContractDeployments(rw, deployments); err != nil {
		return nil, fmt.Errorf("could not store initial contract deployments: %w", err)
	}

	return &ContractDeploymentsIndex{IndexState: state}, nil
}

// ByContractID returns the most recent deployment for the given contract identifier.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound]: if no deployment for the given contract ID exists
func (idx *ContractDeploymentsIndex) ByContractID(id string) (access.ContractDeployment, error) {
	// pass a nil cursor to indicate search should start from the latest deployment
	iter, err := idx.DeploymentsByContractID(id, nil)
	if err != nil {
		return access.ContractDeployment{}, fmt.Errorf("could not get deployments by contract ID: %w", err)
	}

	// iterate over deployments for the contract, and return the first one (most recent)
	for item, err := range iter {
		if err != nil {
			return access.ContractDeployment{}, fmt.Errorf("could not iterate contract deployments for %s: %w", id, err)
		}
		return item.Value()
	}

	// no deployments were found for the contract ID
	return access.ContractDeployment{}, storage.ErrNotFound
}

// DeploymentsByContractID returns an iterator over all recorded deployments for the given
// contract, ordered from most recent to oldest (descending block height).
//
// cursor is a pointer to an [access.ContractDeploymentsCursor]:
//   - nil means start from the most recent deployment
//   - non-nil means start at the cursor position (inclusive)
//
// Returns an exhausted iterator (zero items) and no error if no deployments exist for the given contract ID.
//
// No error returns are expected during normal operation.
func (idx *ContractDeploymentsIndex) DeploymentsByContractID(
	id string,
	cursor *access.ContractDeploymentsCursor,
) (storage.ContractDeploymentIterator, error) {
	startKey, endKey, err := idx.rangeKeysByID(id, cursor)
	if err != nil {
		return nil, fmt.Errorf("could not determine range keys: %w", err)
	}

	reader := idx.db.Reader()
	storageIter, err := reader.NewIter(startKey, endKey, storage.DefaultIteratorOptions())
	if err != nil {
		return nil, fmt.Errorf("could not create iterator for contract %s: %w", id, err)
	}

	return iterator.Build(storageIter, decodeDeploymentCursor, reconstructContractDeployment), nil
}

// All returns an iterator over the latest deployment for each indexed contract,
// ordered by contract identifier (ascending).
//
// cursor is a pointer to an [access.ContractDeploymentsCursor]:
//   - nil means start from the first contract (by identifier)
//   - non-nil resumes from cursor.ContractID (inclusive); other cursor fields are ignored
//
// Returns an exhausted iterator (zero items) and no error if no contracts exist.
//
// No error returns are expected during normal operation.
func (idx *ContractDeploymentsIndex) All(
	cursor *access.ContractDeploymentsCursor,
) (storage.ContractDeploymentIterator, error) {
	startKey, endKey, err := idx.rangeKeysAll(cursor)
	if err != nil {
		return nil, fmt.Errorf("could not determine range keys: %w", err)
	}

	reader := idx.db.Reader()
	storageIter, err := reader.NewIter(startKey, endKey, storage.DefaultIteratorOptions())
	if err != nil {
		return nil, fmt.Errorf("could not create iterator for all contracts: %w", err)
	}

	// this prefix iterator will return the first entry for each prefix returned by contractDeploymentKeyPrefix
	return iterator.BuildPrefixIterator(
		storageIter,
		decodeDeploymentCursor,
		reconstructContractDeployment,
		contractDeploymentKeyPrefix,
	), nil
}

// ByAddress returns an iterator over the latest deployment for each contract deployed by the
// given address, ordered by contract identifier (ascending).
//
// cursor is a pointer to an [access.ContractDeploymentsCursor]:
//   - nil means start from the first contract at the address (by identifier)
//   - non-nil resumes from cursor.ContractID (inclusive); other cursor fields are ignored
//
// Returns an exhausted iterator (zero items) and no error if no deployments exist for the given address.
//
// No error returns are expected during normal operation.
func (idx *ContractDeploymentsIndex) ByAddress(
	account flow.Address,
	cursor *access.ContractDeploymentsCursor,
) (storage.ContractDeploymentIterator, error) {
	startKey, endKey, err := idx.rangeKeysByAddress(account, cursor)
	if err != nil {
		return nil, fmt.Errorf("could not determine range keys: %w", err)
	}

	reader := idx.db.Reader()
	storageIter, err := reader.NewIter(startKey, endKey, storage.DefaultIteratorOptions())
	if err != nil {
		return nil, fmt.Errorf("could not create iterator for address %s: %w", account.Hex(), err)
	}

	// this prefix iterator will return the first entry for each prefix returned by contractDeploymentKeyPrefix
	return iterator.BuildPrefixIterator(
		storageIter,
		decodeDeploymentCursor,
		reconstructContractDeployment,
		contractDeploymentKeyPrefix,
	), nil
}

// rangeKeysByID computes the start and end keys for iterating over contracts by contract id, based on
// the provided cursor.
//
// Any error indicates the cursor is invalid
func (idx *ContractDeploymentsIndex) rangeKeysByID(contractID string, cursor *access.ContractDeploymentsCursor) (startKey, endKey []byte, err error) {
	prefix := makeContractDeploymentContractPrefix(contractID)

	latestHeight := idx.latestHeight.Load()
	if cursor == nil {
		// by default, iterate over all deployments for the exact contractID
		return prefix, prefix, nil
	}

	if err := validateCursorHeight(cursor.BlockHeight, idx.firstHeight, latestHeight); err != nil {
		return nil, nil, err
	}

	startKey = makeContractDeploymentKey(contractID, cursor.BlockHeight, cursor.TransactionIndex, cursor.EventIndex)
	endKey = storage.PrefixInclusiveEnd(prefix, startKey)

	return startKey, endKey, nil
}

// rangeKeysAll computes the start and end keys for iterating over all contracts, based on the provided cursor.
//
// Any error indicates the cursor is invalid
func (idx *ContractDeploymentsIndex) rangeKeysAll(cursor *access.ContractDeploymentsCursor) (startKey, endKey []byte, err error) {
	prefix := []byte{codeContractDeployment}

	if cursor == nil || cursor.ContractID == "" {
		// by default, iterate over all contracts
		return prefix, prefix, nil
	}

	startKey = makeContractDeploymentContractPrefix(cursor.ContractID)
	endKey = storage.PrefixInclusiveEnd(prefix, startKey)

	return startKey, endKey, nil
}

// rangeKeysByAddress computes the start and end keys for iterating over contracts by address, based on
// the provided cursor.
//
// Any error indicates the cursor is invalid
func (idx *ContractDeploymentsIndex) rangeKeysByAddress(account flow.Address, cursor *access.ContractDeploymentsCursor) (startKey, endKey []byte, err error) {
	prefix := makeContractDeploymentAddressPrefix(account)

	if cursor == nil || cursor.ContractID == "" {
		// by default, iterate over all contracts for the address
		return prefix, prefix, nil
	}

	startKey = makeContractDeploymentContractPrefix(cursor.ContractID)
	endKey = storage.PrefixInclusiveEnd(prefix, startKey)

	return startKey, endKey, nil
}

// Store indexes all contract deployments from the given block and advances the latest indexed
// height to blockHeight. Must be called with consecutive block heights.
// The caller must hold the [storage.LockIndexContractDeployments] lock until the batch is committed.
//
// Expected error returns during normal operation:
//   - [storage.ErrAlreadyExists]: if blockHeight has already been indexed
func (idx *ContractDeploymentsIndex) Store(
	lctx lockctx.Proof,
	rw storage.ReaderBatchWriter,
	blockHeight uint64,
	deployments []access.ContractDeployment,
) error {
	if err := idx.PrepareStore(lctx, rw, blockHeight); err != nil {
		return fmt.Errorf("could not prepare store for block %d: %w", blockHeight, err)
	}

	return storeAllContractDeployments(rw, deployments)
}

// storeAllContractDeployments writes all contract deployment entries to the batch.
// For each deployment it writes a primary index entry at
// [codeContractDeployment][contractID][~height][txIndex][eventIndex].
//
// The caller must hold the [storage.LockIndexContractDeployments] lock until the batch is committed.
//
// No error returns are expected during normal operation.
func storeAllContractDeployments(rw storage.ReaderBatchWriter, deployments []access.ContractDeployment) error {
	writer := rw.Writer()
	for _, d := range deployments {
		if len(d.ContractID) < minContractIDLength {
			return fmt.Errorf("contract ID %q is invalid", d.ContractID)
		}

		primaryKey := makeContractDeploymentKey(d.ContractID, d.BlockHeight, d.TransactionIndex, d.EventIndex)
		exists, err := operation.KeyExists(rw.GlobalReader(), primaryKey)
		if err != nil {
			return fmt.Errorf("could not check key for deployment %s: %w", d.ContractID, err)
		}
		if exists {
			return fmt.Errorf("deployment %s at height %d already exists: %w", d.ContractID, d.BlockHeight, storage.ErrAlreadyExists)
		}
		primaryVal := storedContractDeployment{
			TransactionID: d.TransactionID,
			Code:          d.Code,
			CodeHash:      d.CodeHash,
		}
		if err := operation.UpsertByKey(writer, primaryKey, primaryVal); err != nil {
			return fmt.Errorf("could not store primary deployment entry for %s: %w", d.ContractID, err)
		}
	}
	return nil
}

// makeContractDeploymentKey creates a primary key for the given contractID, height, txIndex,
// and eventIndex.
//
// Key format: [codeContractDeployment][contractID bytes][~height(8)][txIndex(4)][eventIndex(4)]
func makeContractDeploymentKey(contractID string, height uint64, txIndex, eventIndex uint32) []byte {
	contractIDBytes := []byte(contractID)
	key := make([]byte, contractDeploymentKeyOverhead+len(contractIDBytes))
	offset := 0

	key[offset] = codeContractDeployment
	offset++

	copy(key[offset:], contractIDBytes)
	offset += len(contractIDBytes)

	binary.BigEndian.PutUint64(key[offset:], ^height) // one's complement for descending height order
	offset += heightLen

	binary.BigEndian.PutUint32(key[offset:], txIndex)
	offset += txIndexLen

	binary.BigEndian.PutUint32(key[offset:], eventIndex)

	return key
}

// makeContractDeploymentContractPrefix returns the prefix used to iterate over all deployments
// of a specific contract:
//
//	[codeContractDeployment][contractID bytes]
func makeContractDeploymentContractPrefix(contractID string) []byte {
	contractIDBytes := []byte(contractID)
	prefix := make([]byte, 1+len(contractIDBytes))
	prefix[0] = codeContractDeployment
	copy(prefix[1:], contractIDBytes)
	return prefix
}

// makeContractDeploymentAddressPrefix returns the prefix used to iterate over all contracts
// deployed by a specific address:
//
//	[codeContractDeployment]["A.{addr.Hex()}." bytes]
//
// Because contractIDs have the form "A.{address_hex}.{name}", this prefix matches all contracts
// owned by the given address.
func makeContractDeploymentAddressPrefix(addr flow.Address) []byte {
	addrPart := fmt.Sprintf("A.%s.", addr.Hex())
	prefix := make([]byte, 1+len(addrPart))
	prefix[0] = codeContractDeployment
	copy(prefix[1:], addrPart)
	return prefix
}

// contractDeploymentKeyPrefix returns the contract-ID portion of a primary key:
//
//	[codeContractDeployment][contractID bytes]
//
// It strips the fixed 16-byte suffix ([~height(8)][txIndex(4)][eventIndex(4)]) from the key.
// Used as the keyPrefix argument to [iterator.BuildPrefixIterator] so that all deployments of
// the same contract are grouped together and only the first (most recent) is yielded.
func contractDeploymentKeyPrefix(key []byte) []byte {
	return key[:len(key)-heightLen-txIndexLen-eventIndexLen]
}

// decodeDeploymentCursor decodes a primary key into an [access.ContractDeploymentsCursor].
//
// Any error indicates a malformed key.
func decodeDeploymentCursor(key []byte) (access.ContractDeploymentsCursor, error) {
	contractID, height, txIndex, eventIndex, err := decodeContractDeploymentKey(key)
	if err != nil {
		return access.ContractDeploymentsCursor{}, err
	}
	return access.ContractDeploymentsCursor{
		ContractID:       contractID,
		BlockHeight:      height,
		TransactionIndex: txIndex,
		EventIndex:       eventIndex,
	}, nil
}

// decodeContractDeploymentKey decodes a primary key into its components.
//
// Any error indicates a malformed key.
func decodeContractDeploymentKey(key []byte) (contractID string, height uint64, txIndex uint32, eventIndex uint32, err error) {
	if len(key) < contractDeploymentKeyOverhead {
		return "", 0, 0, 0, fmt.Errorf("key too short: %d bytes", len(key))
	}
	if key[0] != codeContractDeployment {
		return "", 0, 0, 0, fmt.Errorf("invalid prefix: expected %d, got %d", codeContractDeployment, key[0])
	}

	// The fixed-size suffix is ~height(8) + txIndex(4) + eventIndex(4) = 16 bytes.
	// Everything between the code byte and the suffix is the contractID.
	contractIDEnd := len(key) - heightLen - txIndexLen - eventIndexLen
	contractID = string(key[1:contractIDEnd])

	offset := contractIDEnd
	height = ^binary.BigEndian.Uint64(key[offset:])
	offset += heightLen

	txIndex = binary.BigEndian.Uint32(key[offset:])
	offset += txIndexLen

	eventIndex = binary.BigEndian.Uint32(key[offset:])

	return contractID, height, txIndex, eventIndex, nil
}

// reconstructContractDeployment builds a full [access.ContractDeployment] from a decoded
// [access.ContractDeploymentsCursor] and the primary index value bytes.
//
// Any error indicates a malformed value.
func reconstructContractDeployment(cursor access.ContractDeploymentsCursor, val []byte) (*access.ContractDeployment, error) {
	var stored storedContractDeployment
	if err := msgpack.Unmarshal(val, &stored); err != nil {
		return nil, fmt.Errorf("could not unmarshal contract deployment: %w", err)
	}
	addr, err := addressFromContractID(cursor.ContractID)
	if err != nil {
		return nil, fmt.Errorf("could not extract address from contract ID %s: %w", cursor.ContractID, err)
	}
	return &access.ContractDeployment{
		ContractID:       cursor.ContractID,
		Address:          addr,
		BlockHeight:      cursor.BlockHeight,
		TransactionID:    stored.TransactionID,
		TransactionIndex: cursor.TransactionIndex,
		EventIndex:       cursor.EventIndex,
		Code:             stored.Code,
		CodeHash:         stored.CodeHash,
	}, nil
}

// addressFromContractID extracts the [flow.Address] from a contract identifier of the form
// "A.{address_hex}.{name}".
//
// Any error indicates the contractID does not follow the expected format.
func addressFromContractID(contractID string) (flow.Address, error) {
	if !strings.HasPrefix(contractID, "A.") {
		return flow.Address{}, fmt.Errorf("unexpected contract ID format: %q", contractID)
	}
	addrHex, _, ok := strings.Cut(contractID[2:], ".") // strip "A." then split on "."
	if !ok {
		return flow.Address{}, fmt.Errorf("unexpected contract ID format (no second dot): %q", contractID)
	}
	addr, err := flow.StringToAddress(addrHex)
	if err != nil {
		return flow.Address{}, fmt.Errorf("could not parse address %q from contract ID %q: %w", addrHex, contractID, err)
	}
	return addr, nil
}
