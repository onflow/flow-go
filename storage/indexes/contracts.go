package indexes

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
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

// DeploymentsByContractID returns a paginated list of all recorded deployments for the given
// contract, ordered from most recent to oldest.
//
// Expected error returns during normal operation:
//   - [storage.ErrInvalidQuery]: if limit is zero
func (idx *ContractDeploymentsIndex) DeploymentsByContractID(
	id string,
	limit uint32,
	cursor *access.ContractDeploymentCursor,
	filter storage.IndexFilter[*access.ContractDeployment],
) (access.ContractDeploymentPage, error) {
	if limit == 0 {
		return access.ContractDeploymentPage{}, fmt.Errorf("limit must be greater than zero: %w", storage.ErrInvalidQuery)
	}

	prefix := makeContractDeploymentContractPrefix(id)

	// When resuming from a cursor, start from the cursor's key so we can skip it.
	// Since cursor key bytes > prefix bytes (cursor key extends the prefix), we
	// need to use the incremented prefix as the end bound to satisfy startKey <= endKey.
	startKey := prefix
	var cursorKey []byte
	if cursor != nil {
		cursorKey = makeContractDeploymentKey(id, cursor.Height, cursor.TxIndex, cursor.EventIndex)
		startKey = cursorKey
	}
	endKey := incrementContractPrefix(prefix)

	fetchLimit := int(limit + 1)
	var collected []access.ContractDeployment

	err := operation.IterateKeys(idx.db.Reader(), startKey, endKey,
		func(keyCopy []byte, getValue func(any) error) (bail bool, err error) {
			// Skip the cursor entry itself (it was already returned on the previous page).
			if cursorKey != nil && bytes.Equal(keyCopy, cursorKey) {
				return false, nil
			}

			// Ensure the key belongs to this contract (has the expected prefix).
			if !bytes.HasPrefix(keyCopy, prefix) {
				return true, nil
			}

			var val storedContractDeployment
			if err := getValue(&val); err != nil {
				return true, fmt.Errorf("could not read value: %w", err)
			}
			d, err := reconstructDeployment(keyCopy, val)
			if err != nil {
				return true, fmt.Errorf("could not reconstruct deployment: %w", err)
			}

			if filter != nil && !filter(&d) {
				return false, nil
			}

			collected = append(collected, d)
			if len(collected) >= fetchLimit {
				return true, nil
			}
			return false, nil
		}, storage.DefaultIteratorOptions())

	if err != nil {
		return access.ContractDeploymentPage{}, fmt.Errorf("could not iterate contract deployments for %s: %w", id, err)
	}

	return buildDeploymentPageByPosition(collected, limit), nil
}

// All returns the latest deployment for each indexed contract, using cursor-based pagination.
// Results are ordered by contract identifier (ascending).
//
// Expected error returns during normal operation:
//   - [storage.ErrInvalidQuery]: if limit is zero
func (idx *ContractDeploymentsIndex) All(
	limit uint32,
	cursor *access.ContractDeploymentCursor,
	filter storage.IndexFilter[*access.ContractDeployment],
) (access.ContractDeploymentPage, error) {
	if limit == 0 {
		return access.ContractDeploymentPage{}, fmt.Errorf("limit must be greater than zero: %w", storage.ErrInvalidQuery)
	}
	return lookupAllContractDeployments(idx.db.Reader(), limit, cursor, filter)
}

// ByAddress returns the latest deployment for each contract deployed by the given address,
// using cursor-based pagination. Results are ordered by contract identifier (ascending).
//
// Expected error returns during normal operation:
//   - [storage.ErrInvalidQuery]: if limit is zero
func (idx *ContractDeploymentsIndex) ByAddress(
	account flow.Address,
	limit uint32,
	cursor *access.ContractDeploymentCursor,
	filter storage.IndexFilter[*access.ContractDeployment],
) (access.ContractDeploymentPage, error) {
	if limit == 0 {
		return access.ContractDeploymentPage{}, fmt.Errorf("limit must be greater than zero: %w", storage.ErrInvalidQuery)
	}
	return lookupContractDeploymentsByAddress(idx.db.Reader(), account, limit, cursor, filter)
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

// lookupAllContractDeployments iterates the primary key space and returns the latest deployment
// for each unique contract, with cursor-based pagination.
//
// No error returns are expected during normal operation.
func lookupAllContractDeployments(
	reader storage.Reader,
	limit uint32,
	cursor *access.ContractDeploymentCursor,
	filter storage.IndexFilter[*access.ContractDeployment],
) (access.ContractDeploymentPage, error) {
	// Scan the full contract deployment key space. The end key is one byte past the
	// codeContractDeployment prefix to bound iteration to contract deployment keys only.
	startKey := []byte{codeContractDeployment}
	endKey := []byte{codeContractDeployment + 1}

	if cursor != nil && cursor.ContractID != "" {
		// Resume from the cursor's contract. We start from the contract's own key prefix and
		// skip entries whose contractID equals the cursor's ContractID (it was already returned).
		startKey = makeContractDeploymentContractPrefix(cursor.ContractID)
	}

	fetchLimit := int(limit + 1)
	var collected []access.ContractDeployment
	var prevContractID string

	err := operation.IterateKeys(reader, startKey, endKey,
		func(keyCopy []byte, getValue func(any) error) (bail bool, err error) {
			contractID, _, _, _, err := decodeContractDeploymentKey(keyCopy)
			if err != nil {
				return true, fmt.Errorf("could not decode key: %w", err)
			}

			// Skip duplicate keys for the same contract (only the first — most recent — matters).
			if contractID == prevContractID {
				return false, nil
			}

			// When resuming from a cursor, skip the cursor's own contract.
			if cursor != nil && cursor.ContractID != "" && contractID == cursor.ContractID {
				prevContractID = contractID
				return false, nil
			}

			// This is the first (most recent) entry for a new contract.
			var val storedContractDeployment
			if err := getValue(&val); err != nil {
				return true, fmt.Errorf("could not read value: %w", err)
			}
			d, err := reconstructDeployment(keyCopy, val)
			if err != nil {
				return true, fmt.Errorf("could not reconstruct deployment: %w", err)
			}

			prevContractID = contractID

			if filter != nil && !filter(&d) {
				return false, nil
			}

			collected = append(collected, d)
			if len(collected) >= fetchLimit {
				return true, nil
			}
			return false, nil
		}, storage.DefaultIteratorOptions())

	if err != nil {
		return access.ContractDeploymentPage{}, fmt.Errorf("could not iterate contract deployments: %w", err)
	}

	return buildContractDeploymentPageByID(collected, limit), nil
}

// lookupContractDeploymentsByAddress iterates the primary key space scoped to the given address
// and returns the latest deployment for each unique contract at that address, with cursor-based
// pagination.
//
// No error returns are expected during normal operation.
func lookupContractDeploymentsByAddress(
	reader storage.Reader,
	address flow.Address,
	limit uint32,
	cursor *access.ContractDeploymentCursor,
	filter storage.IndexFilter[*access.ContractDeployment],
) (access.ContractDeploymentPage, error) {
	addrPrefix := makeContractDeploymentAddressPrefix(address)

	startKey := addrPrefix
	if cursor != nil && cursor.ContractID != "" {
		// Resume from the cursor's contract. We start from the contract's own key prefix and
		// skip entries whose contractID equals the cursor's ContractID.
		startKey = makeContractDeploymentContractPrefix(cursor.ContractID)
	}

	// End key is one byte past the address prefix to bound the iteration to this address.
	endKey := incrementContractPrefix(addrPrefix)

	fetchLimit := int(limit + 1)
	var collected []access.ContractDeployment
	var prevContractID string

	err := operation.IterateKeys(reader, startKey, endKey,
		func(keyCopy []byte, getValue func(any) error) (bail bool, err error) {
			// Ensure the key still belongs to this address prefix.
			if !bytes.HasPrefix(keyCopy, addrPrefix) {
				return true, nil
			}

			contractID, _, _, _, err := decodeContractDeploymentKey(keyCopy)
			if err != nil {
				return true, fmt.Errorf("could not decode key: %w", err)
			}

			// Skip duplicate keys for the same contract (only the first — most recent — matters).
			if contractID == prevContractID {
				return false, nil
			}

			// When resuming from a cursor, skip the cursor's own contract.
			if cursor != nil && cursor.ContractID != "" && contractID == cursor.ContractID {
				prevContractID = contractID
				return false, nil
			}

			// This is the first (most recent) entry for a new contract.
			var val storedContractDeployment
			if err := getValue(&val); err != nil {
				return true, fmt.Errorf("could not read value: %w", err)
			}
			d, err := reconstructDeployment(keyCopy, val)
			if err != nil {
				return true, fmt.Errorf("could not reconstruct deployment: %w", err)
			}

			prevContractID = contractID

			if filter != nil && !filter(&d) {
				return false, nil
			}

			collected = append(collected, d)
			if len(collected) >= fetchLimit {
				return true, nil
			}
			return false, nil
		}, storage.DefaultIteratorOptions())

	if err != nil {
		return access.ContractDeploymentPage{}, fmt.Errorf("could not iterate contract deployments for address %s: %w", address.Hex(), err)
	}

	return buildContractDeploymentPageByID(collected, limit), nil
}

// buildContractDeploymentPageByID assembles a page for All/ByAddress queries.
// The fetching logic always fetches one more entry than the limit to determine if there are more
// results beyond this page. If more than limit entries were collected, the next cursor is set to
// the last returned entry's ContractID.
func buildContractDeploymentPageByID(collected []access.ContractDeployment, limit uint32) access.ContractDeploymentPage {
	if uint32(len(collected)) > limit {
		last := collected[limit-1]
		return access.ContractDeploymentPage{
			Deployments: collected[:limit],
			NextCursor:  &access.ContractDeploymentCursor{ContractID: last.ContractID},
		}
	}
	return access.ContractDeploymentPage{Deployments: collected}
}

// buildDeploymentPageByPosition assembles a page for DeploymentsByContractID queries.
// If more than limit entries were collected, the next cursor is set to the last returned
// deployment's position (Height, TxIndex, EventIndex).
func buildDeploymentPageByPosition(collected []access.ContractDeployment, limit uint32) access.ContractDeploymentPage {
	if uint32(len(collected)) > limit {
		last := collected[limit-1]
		return access.ContractDeploymentPage{
			Deployments: collected[:limit],
			NextCursor: &access.ContractDeploymentCursor{
				Height:     last.BlockHeight,
				TxIndex:    last.TxIndex,
				EventIndex: last.EventIndex,
			},
		}
	}
	return access.ContractDeploymentPage{Deployments: collected}
}

// incrementContractPrefix returns a copy of the prefix with the last byte incremented by one.
// This is used to construct an exclusive upper bound for prefix-based iteration.
func incrementContractPrefix(prefix []byte) []byte {
	result := make([]byte, len(prefix))
	copy(result, prefix)
	result[len(result)-1]++
	return result
}
