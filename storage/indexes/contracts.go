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
	// part of the contract name. The format is:
	//
	//	[code(1)][address bytes(8)][contract name][~height(8)][txIndex(4)][eventIndex(4)]
	//
	// so the overhead is code(1) + address(8) + ~height(8) + txIndex(4) + eventIndex(4) = 25 bytes.
	contractDeploymentKeyOverhead = 1 + flow.AddressLength + heightLen + txIndexLen + eventIndexLen

	// minValidKeyLen is the minimum length of a valid contract deployment key.
	// This is contractDeploymentKeyOverhead, plus a 1 character contract name.
	minValidKeyLen = contractDeploymentKeyOverhead + 1
)

// storedContractDeployment holds the fields of a [access.ContractDeployment] that are not
// derivable from the primary key. Fields derivable from the key (ContractName, Address, Height,
// TxIndex, EventIndex) are not stored here.
type storedContractDeployment struct {
	TransactionID flow.Identifier
	Code          []byte
	CodeHash      []byte
	IsPlaceholder bool
	IsDeleted     bool
}

// ContractDeploymentsIndex implements [storage.ContractDeploymentsIndex].
//
// Primary index key format:
//
//	[codeContractDeployment][address bytes(8)][contract name][~height(8)][txIndex(4)][eventIndex(4)]
//
// The one's complement of height (~height) ensures that the most recent deployment (highest
// height) has the smallest byte value, so it appears first during ascending key iteration.
//
// [All] and [ByAddress] use [BuildPrefixIterator] over the primary index with the prefix
// [codeContractDeployment][address bytes(8)][contract name], which yields exactly one entry
// (the most recent deployment) per contract without a separate secondary index.
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

// ByContract returns the most recent deployment for the given contract.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound]: if no deployment for the given contract exists
func (idx *ContractDeploymentsIndex) ByContract(account flow.Address, name string) (access.ContractDeployment, error) {
	// pass a nil cursor to indicate search should start from the latest deployment
	iter, err := idx.DeploymentsByContract(account, name, nil)
	if err != nil {
		return access.ContractDeployment{}, fmt.Errorf("could not get deployments for %s.%s: %w", account.Hex(), name, err)
	}

	// iterate over deployments for the contract, and return the first one (most recent)
	for item, err := range iter {
		if err != nil {
			return access.ContractDeployment{}, fmt.Errorf("could not iterate contract deployments for %s.%s: %w", account.Hex(), name, err)
		}
		return item.Value()
	}

	// no deployments were found
	return access.ContractDeployment{}, storage.ErrNotFound
}

// DeploymentsByContract returns an iterator over all recorded deployments for the given
// contract, ordered from most recent to oldest (descending block height).
//
// cursor is a pointer to an [access.ContractDeploymentsCursor]:
//   - nil means start from the most recent deployment
//   - non-nil means start at the cursor position (inclusive)
//
// Returns an exhausted iterator (zero items) and no error if no deployments exist for the given contract.
//
// No error returns are expected during normal operation.
func (idx *ContractDeploymentsIndex) DeploymentsByContract(
	account flow.Address,
	name string,
	cursor *access.ContractDeploymentsCursor,
) (storage.ContractDeploymentIterator, error) {
	startKey, endKey, err := idx.rangeKeysByContract(account, name, cursor)
	if err != nil {
		return nil, fmt.Errorf("could not determine range keys: %w", err)
	}

	reader := idx.db.Reader()
	storageIter, err := reader.NewIter(startKey, endKey, storage.DefaultIteratorOptions())
	if err != nil {
		return nil, fmt.Errorf("could not create iterator for contract %s.%s: %w", account.Hex(), name, err)
	}

	return iterator.Build(storageIter, decodeDeploymentCursor, reconstructContractDeployment), nil
}

// All returns an iterator over the latest deployment for each indexed contract,
// ordered by contract identifier (ascending).
//
// cursor is a pointer to an [access.ContractDeploymentsCursor]:
//   - nil means start from the first contract (by identifier)
//   - non-nil resumes from (cursor.Address, cursor.ContractName) (inclusive); other cursor fields are ignored
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
//   - non-nil resumes from (cursor.Address, cursor.ContractName) (inclusive); other cursor fields are ignored
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

// rangeKeysByContract computes the start and end keys for iterating over deployments of a specific
// contract, based on the provided cursor.
//
// Any error indicates the cursor is invalid.
func (idx *ContractDeploymentsIndex) rangeKeysByContract(account flow.Address, name string, cursor *access.ContractDeploymentsCursor) (startKey, endKey []byte, err error) {
	prefix := makeContractDeploymentContractPrefix(account, name)

	latestHeight := idx.latestHeight.Load()
	if cursor == nil {
		// by default, iterate over all deployments for the contract
		return prefix, prefix, nil
	}

	if err := validateCursorHeight(cursor.BlockHeight, idx.firstHeight, latestHeight); err != nil {
		return nil, nil, err
	}

	startKey = makeContractDeploymentKey(account, name, cursor.BlockHeight, cursor.TransactionIndex, cursor.EventIndex)
	endKey = storage.PrefixInclusiveEnd(prefix, startKey)

	return startKey, endKey, nil
}

// rangeKeysAll computes the start and end keys for iterating over all contracts, based on the provided cursor.
//
// Any error indicates the cursor is invalid.
func (idx *ContractDeploymentsIndex) rangeKeysAll(cursor *access.ContractDeploymentsCursor) (startKey, endKey []byte, err error) {
	prefix := []byte{codeContractDeployment}

	if cursor == nil || cursor.ContractName == "" {
		// by default, iterate over all contracts
		return prefix, prefix, nil
	}

	startKey = makeContractDeploymentContractPrefix(cursor.Address, cursor.ContractName)
	endKey = storage.PrefixInclusiveEnd(prefix, startKey)

	return startKey, endKey, nil
}

// rangeKeysByAddress computes the start and end keys for iterating over contracts by address, based on
// the provided cursor.
//
// Any error indicates the cursor is invalid.
func (idx *ContractDeploymentsIndex) rangeKeysByAddress(account flow.Address, cursor *access.ContractDeploymentsCursor) (startKey, endKey []byte, err error) {
	prefix := makeContractDeploymentAddressPrefix(account)

	if cursor == nil || cursor.ContractName == "" {
		// by default, iterate over all contracts for the address
		return prefix, prefix, nil
	}

	startKey = makeContractDeploymentContractPrefix(cursor.Address, cursor.ContractName)
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
// [codeContractDeployment][address bytes][contract name][~height][txIndex][eventIndex].
//
// The caller must hold the [storage.LockIndexContractDeployments] lock until the batch is committed.
//
// No error returns are expected during normal operation.
func storeAllContractDeployments(rw storage.ReaderBatchWriter, deployments []access.ContractDeployment) error {
	writer := rw.Writer()
	for _, d := range deployments {
		if d.ContractName == "" || strings.Contains(d.ContractName, ".") {
			return fmt.Errorf("deployment for %s has invalid contract name: %q", d.Address.Hex(), d.ContractName)
		}

		primaryKey := makeContractDeploymentKey(d.Address, d.ContractName, d.BlockHeight, d.TransactionIndex, d.EventIndex)
		exists, err := operation.KeyExists(rw.GlobalReader(), primaryKey)
		if err != nil {
			return fmt.Errorf("could not check key for deployment %s.%s: %w", d.Address.Hex(), d.ContractName, err)
		}
		if exists {
			return fmt.Errorf("deployment A.%s.%s at height %d already exists: %w", d.Address.Hex(), d.ContractName, d.BlockHeight, storage.ErrAlreadyExists)
		}
		primaryVal := storedContractDeployment{
			TransactionID: d.TransactionID,
			Code:          d.Code,
			CodeHash:      d.CodeHash,
			IsPlaceholder: d.IsPlaceholder,
			IsDeleted:     d.IsDeleted,
		}
		if err := operation.UpsertByKey(writer, primaryKey, primaryVal); err != nil {
			return fmt.Errorf("could not store primary deployment entry for A.%s.%s: %w", d.Address.Hex(), d.ContractName, err)
		}
	}
	return nil
}

// makeContractDeploymentKey creates a primary key for the given address, contract name, height,
// txIndex, and eventIndex.
//
// Key format: [codeContractDeployment][address bytes(8)][contract name][~height(8)][txIndex(4)][eventIndex(4)]
func makeContractDeploymentKey(addr flow.Address, name string, height uint64, txIndex, eventIndex uint32) []byte {
	nameBytes := []byte(name)
	key := make([]byte, contractDeploymentKeyOverhead+len(nameBytes))
	offset := 0

	key[offset] = codeContractDeployment
	offset++

	copy(key[offset:], addr[:])
	offset += flow.AddressLength

	copy(key[offset:], nameBytes)
	offset += len(nameBytes)

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
//	[codeContractDeployment][address bytes(8)][contract name bytes]
func makeContractDeploymentContractPrefix(addr flow.Address, name string) []byte {
	nameBytes := []byte(name)
	prefix := make([]byte, 1+flow.AddressLength+len(nameBytes))
	prefix[0] = codeContractDeployment
	copy(prefix[1:], addr[:])
	copy(prefix[1+flow.AddressLength:], nameBytes)
	return prefix
}

// makeContractDeploymentAddressPrefix returns the prefix used to iterate over all contracts
// deployed by a specific address:
//
//	[codeContractDeployment][address bytes(8)]
//
// This prefix matches all contracts owned by the given address, since the address occupies a
// fixed 8-byte slot at the start of every primary key.
func makeContractDeploymentAddressPrefix(addr flow.Address) []byte {
	prefix := make([]byte, 1+flow.AddressLength)
	prefix[0] = codeContractDeployment
	copy(prefix[1:], addr[:])
	return prefix
}

// contractDeploymentKeyPrefix returns the contract-specific portion of a primary key:
//
//	[codeContractDeployment][address bytes(8)][contract name bytes]
//
// It strips the fixed 16-byte suffix ([~height(8)][txIndex(4)][eventIndex(4)]) from the key.
// Used as the keyPrefix argument to [iterator.BuildPrefixIterator] so that all deployments of
// the same contract are grouped together and only the first (most recent) is yielded.
func contractDeploymentKeyPrefix(key []byte) ([]byte, error) {
	if len(key) < minValidKeyLen {
		return nil, fmt.Errorf("key too short: expected at least %d bytes, got %d", minValidKeyLen, len(key))
	}
	if key[0] != codeContractDeployment {
		return nil, fmt.Errorf("invalid key prefix: expected %d, got %d", codeContractDeployment, key[0])
	}
	return key[:len(key)-heightLen-txIndexLen-eventIndexLen], nil
}

// decodeDeploymentCursor decodes a primary key into an [access.ContractDeploymentsCursor].
//
// Any error indicates a malformed key.
func decodeDeploymentCursor(key []byte) (access.ContractDeploymentsCursor, error) {
	if len(key) < minValidKeyLen {
		return access.ContractDeploymentsCursor{}, fmt.Errorf("key too short: %d bytes", len(key))
	}
	if key[0] != codeContractDeployment {
		return access.ContractDeploymentsCursor{}, fmt.Errorf("invalid prefix: expected %d, got %d", codeContractDeployment, key[0])
	}
	offset := 1

	var addr flow.Address
	copy(addr[:], key[offset:offset+flow.AddressLength])
	offset += flow.AddressLength

	// The fixed-size suffix is ~height(8) + txIndex(4) + eventIndex(4) = 16 bytes.
	// Everything between the address and the suffix is the contract name.
	nameEnd := len(key) - heightLen - txIndexLen - eventIndexLen
	contractName := string(key[offset:nameEnd])
	offset = nameEnd

	height := ^binary.BigEndian.Uint64(key[offset:])
	offset += heightLen

	txIndex := binary.BigEndian.Uint32(key[offset:])
	offset += txIndexLen

	eventIndex := binary.BigEndian.Uint32(key[offset:])

	return access.ContractDeploymentsCursor{
		Address:          addr,
		ContractName:     contractName,
		BlockHeight:      height,
		TransactionIndex: txIndex,
		EventIndex:       eventIndex,
	}, nil
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
	return &access.ContractDeployment{
		ContractName:     cursor.ContractName,
		Address:          cursor.Address,
		BlockHeight:      cursor.BlockHeight,
		TransactionID:    stored.TransactionID,
		TransactionIndex: cursor.TransactionIndex,
		EventIndex:       cursor.EventIndex,
		Code:             stored.Code,
		CodeHash:         stored.CodeHash,
		IsPlaceholder:    stored.IsPlaceholder,
		IsDeleted:        stored.IsDeleted,
	}, nil
}
