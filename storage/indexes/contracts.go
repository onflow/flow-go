package indexes

import (
	"encoding/binary"
	"fmt"
	"math"
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
	contractDeploymentKeyOverhead = 1 + 8 + 4 + 4

	// minContractIDLength is the minimum length of a contractID.
	// e.g. "A.1234567890abcdef.a" = 22 bytes
	minContractIDLength = 4 + flow.AddressLength*2 + 1
)

// storedContractDeployment holds the fields of a [access.ContractDeployment] that are not
// derivable from the key. Fields derivable from the key (ContractID, Address, Height, TxIndex,
// EventIndex) are not stored here.
type storedContractDeployment struct {
	TransactionID flow.Identifier
	Code          []byte
	CodeHash      []byte
}

// ContractDeploymentsIndex implements [storage.ContractDeploymentsIndex] using Pebble.
//
// Key format: [codeContractDeployment][contractID bytes][~height(8)][txIndex(4)][eventIndex(4)]
//
// The one's complement of height (~height) ensures that the most recent deployment (highest
// height) has the smallest byte value, so it appears first during ascending key iteration.
//
// Because contractIDs have the form "A.{address_hex}.{name}", a prefix scan on the address
// portion suffices for ByAddress — no secondary by-address key is needed.
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
	prefix := makeContractDeploymentContractPrefix(id)

	var result *access.ContractDeployment
	err := operation.IterateKeys(idx.db.Reader(), prefix, prefix,
		func(keyCopy []byte, getValue func(any) error) (bail bool, err error) {
			var val storedContractDeployment
			if err := getValue(&val); err != nil {
				return true, fmt.Errorf("could not read value: %w", err)
			}
			d, err := reconstructDeployment(keyCopy, val)
			if err != nil {
				return true, fmt.Errorf("could not reconstruct deployment: %w", err)
			}
			result = &d
			// The first key is the most recent (due to ~height ordering). Stop immediately.
			return true, nil
		}, storage.DefaultIteratorOptions())

	if err != nil {
		return access.ContractDeployment{}, fmt.Errorf("could not iterate contract deployments for %s: %w", id, err)
	}

	if result == nil {
		return access.ContractDeployment{}, storage.ErrNotFound
	}
	return *result, nil
}

// DeploymentsByContractID returns an iterator over all recorded deployments for the given
// contract, ordered from most recent to oldest (descending block height).
//
// cursor is a pointer to an [access.ContractDeploymentCursor]:
//   - nil means start from the most recent deployment
//   - non-nil means start at the cursor position (inclusive)
//
// No error returns are expected during normal operation.
func (idx *ContractDeploymentsIndex) DeploymentsByContractID(
	id string,
	cursor *access.ContractDeploymentCursor,
) (storage.ContractDeploymentIterator, error) {
	prefix := makeContractDeploymentContractPrefix(id)

	startKey := prefix
	if cursor != nil {
		startKey = makeContractDeploymentKey(id, cursor.Height, cursor.TxIndex, cursor.EventIndex)
	}
	// endKey is the maximum possible key for this contractID: the key suffix is all 0xFF bytes
	// (height=0 gives ~height=0xFFFFFFFFFFFFFFFF, and max tx/event indices are all 0xFF).
	// PrefixUpperBound(endKey) naturally overflows the suffix and increments the contractID's last
	// byte, giving a tight upper bound that excludes keys from any other contractID.
	endKey := makeContractDeploymentKey(id, 0, math.MaxUint32, math.MaxUint32)

	reader := idx.db.Reader()
	storageIter, err := reader.NewIter(startKey, endKey, storage.DefaultIteratorOptions())
	if err != nil {
		return nil, fmt.Errorf("could not create iterator for contract %s: %w", id, err)
	}

	return iterator.Build(storageIter, decodeContractDeploymentCursor, reconstructContractDeploymentFromCursor), nil
}

// All returns an iterator over the latest deployment for each indexed contract,
// ordered by contract identifier (ascending).
//
// cursor is a pointer to an [access.ContractDeploymentCursor]:
//   - nil means start from the first contract (by identifier)
//   - non-nil means start at the cursor's contract (inclusive)
//
// No error returns are expected during normal operation.
func (idx *ContractDeploymentsIndex) All(
	cursor *access.ContractDeploymentCursor,
) (storage.ContractDeploymentIterator, error) {
	startKey := []byte{codeContractDeployment}
	if cursor != nil && cursor.ContractID != "" {
		startKey = makeContractDeploymentContractPrefix(cursor.ContractID)
	}
	endKey := []byte{codeContractDeployment + 1}

	reader := idx.db.Reader()
	storageIter, err := reader.NewIter(startKey, endKey, storage.DefaultIteratorOptions())
	if err != nil {
		return nil, fmt.Errorf("could not create iterator for all contracts: %w", err)
	}

	return buildDeduplicatingContractIterator(storageIter), nil
}

// ByAddress returns an iterator over the latest deployment for each contract deployed by the
// given address, ordered by contract identifier (ascending).
//
// cursor is a pointer to an [access.ContractDeploymentCursor]:
//   - nil means start from the first contract at the address (by identifier)
//   - non-nil means start at the cursor's contract (inclusive)
//
// No error returns are expected during normal operation.
func (idx *ContractDeploymentsIndex) ByAddress(
	account flow.Address,
	cursor *access.ContractDeploymentCursor,
) (storage.ContractDeploymentIterator, error) {
	addrPrefix := makeContractDeploymentAddressPrefix(account)

	startKey := addrPrefix
	if cursor != nil && cursor.ContractID != "" {
		startKey = makeContractDeploymentContractPrefix(cursor.ContractID)
	}
	endKey := incrementContractPrefix(addrPrefix)

	reader := idx.db.Reader()
	storageIter, err := reader.NewIter(startKey, endKey, storage.DefaultIteratorOptions())
	if err != nil {
		return nil, fmt.Errorf("could not create iterator for address %s: %w", account.Hex(), err)
	}

	return buildDeduplicatingContractIterator(storageIter), nil
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
// The caller must hold the [storage.LockIndexContractDeployments] lock until the batch is committed.
//
// No error returns are expected during normal operation.
func storeAllContractDeployments(rw storage.ReaderBatchWriter, deployments []access.ContractDeployment) error {
	writer := rw.Writer()
	for _, d := range deployments {
		if len(d.ContractID) < minContractIDLength {
			return fmt.Errorf("contract ID %q is too short", d.ContractID)
		}

		key := makeContractDeploymentKey(d.ContractID, d.BlockHeight, d.TxIndex, d.EventIndex)

		exists, err := operation.KeyExists(rw.GlobalReader(), key)
		if err != nil {
			return fmt.Errorf("could not check key for deployment %s: %w", d.ContractID, err)
		}
		if exists {
			return fmt.Errorf("deployment %s at height %d already exists: %w", d.ContractID, d.BlockHeight, storage.ErrAlreadyExists)
		}

		val := storedContractDeployment{
			TransactionID: d.TransactionID,
			Code:          d.Code,
			CodeHash:      d.CodeHash,
		}
		if err := operation.UpsertByKey(writer, key, val); err != nil {
			return fmt.Errorf("could not store deployment %s: %w", d.ContractID, err)
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
	offset += 8

	binary.BigEndian.PutUint32(key[offset:], txIndex)
	offset += 4

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
	contractIDEnd := len(key) - 16
	contractID = string(key[1:contractIDEnd])

	offset := contractIDEnd
	height = ^binary.BigEndian.Uint64(key[offset:])
	offset += 8

	txIndex = binary.BigEndian.Uint32(key[offset:])
	offset += 4

	eventIndex = binary.BigEndian.Uint32(key[offset:])

	return contractID, height, txIndex, eventIndex, nil
}

// reconstructDeployment reconstructs a full [access.ContractDeployment] from a key and stored value.
//
// Any error indicates a malformed key or unexpected contractID format.
func reconstructDeployment(key []byte, val storedContractDeployment) (access.ContractDeployment, error) {
	contractID, height, txIndex, eventIndex, err := decodeContractDeploymentKey(key)
	if err != nil {
		return access.ContractDeployment{}, fmt.Errorf("could not decode key: %w", err)
	}

	addr, err := addressFromContractID(contractID)
	if err != nil {
		return access.ContractDeployment{}, fmt.Errorf("could not extract address from contract ID %s: %w", contractID, err)
	}

	return access.ContractDeployment{
		ContractID:    contractID,
		Address:       addr,
		BlockHeight:   height,
		TransactionID: val.TransactionID,
		TxIndex:       txIndex,
		EventIndex:    eventIndex,
		Code:          val.Code,
		CodeHash:      val.CodeHash,
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

// decodeContractDeploymentCursor decodes a primary key into a [access.ContractDeploymentCursor]
// containing the contractID, height, txIndex, and eventIndex.
//
// Any error indicates a malformed key.
func decodeContractDeploymentCursor(key []byte) (access.ContractDeploymentCursor, error) {
	contractID, height, txIndex, eventIndex, err := decodeContractDeploymentKey(key)
	if err != nil {
		return access.ContractDeploymentCursor{}, err
	}
	return access.ContractDeploymentCursor{
		ContractID: contractID,
		Height:     height,
		TxIndex:    txIndex,
		EventIndex: eventIndex,
	}, nil
}

// reconstructContractDeploymentFromCursor builds a full [access.ContractDeployment] from a
// decoded cursor and the raw msgpack-encoded value bytes.
//
// Any error indicates a malformed value or an unrecognized contractID format.
func reconstructContractDeploymentFromCursor(cur access.ContractDeploymentCursor, value []byte, dest *access.ContractDeployment) error {
	var stored storedContractDeployment
	if err := msgpack.Unmarshal(value, &stored); err != nil {
		return fmt.Errorf("could not unmarshal contract deployment: %w", err)
	}
	addr, err := addressFromContractID(cur.ContractID)
	if err != nil {
		return fmt.Errorf("could not extract address from contract ID %s: %w", cur.ContractID, err)
	}
	*dest = access.ContractDeployment{
		ContractID:    cur.ContractID,
		Address:       addr,
		BlockHeight:   cur.Height,
		TransactionID: stored.TransactionID,
		TxIndex:       cur.TxIndex,
		EventIndex:    cur.EventIndex,
		Code:          stored.Code,
		CodeHash:      stored.CodeHash,
	}
	return nil
}

// buildDeduplicatingContractIterator wraps a storage iterator and returns a
// [storage.ContractDeploymentIterator] that yields exactly one entry per unique contractID
// (the first — most recent — entry encountered). The underlying storageIter is closed when
// iteration is complete or stopped early.
func buildDeduplicatingContractIterator(storageIter storage.Iterator) storage.ContractDeploymentIterator {
	return func(yield func(storage.IteratorEntry[access.ContractDeployment, access.ContractDeploymentCursor]) bool) {
		defer storageIter.Close()
		var prevContractID string
		for storageIter.First(); storageIter.Valid(); storageIter.Next() {
			item := storageIter.IterItem()
			key := item.KeyCopy(nil)

			// Decode the contractID for deduplication. On error, yield the entry so that the
			// error is visible to the caller via entry.Value().
			contractID, _, _, _, decodeErr := decodeContractDeploymentKey(key)
			if decodeErr == nil {
				if contractID == prevContractID {
					continue
				}
				prevContractID = contractID
			}

			getValue := func(cur access.ContractDeploymentCursor, v *access.ContractDeployment) error {
				return item.Value(func(val []byte) error {
					return reconstructContractDeploymentFromCursor(cur, val, v)
				})
			}

			entry := iterator.NewEntry(key, decodeContractDeploymentCursor, getValue)
			if !yield(entry) {
				return
			}
		}
	}
}

// incrementContractPrefix returns a copy of the prefix with the last byte incremented by one.
// This is used to construct an exclusive upper bound for prefix-based iteration.
func incrementContractPrefix(prefix []byte) []byte {
	result := make([]byte, len(prefix))
	copy(result, prefix)
	result[len(result)-1]++
	return result
}
