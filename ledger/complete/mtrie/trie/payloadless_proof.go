package trie

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/model/flow"
)

// ErrPayloadHashMismatch is returned when the actual payload value from storehouse
// does not match the hash stored in the payloadless proof.
var ErrPayloadHashMismatch = errors.New("payload hash mismatch: storehouse value inconsistent with trie")

// RegisterValueReader is a function type that reads register values.
// It returns:
//   - (value, nil) if the register is found
//   - (nil, nil) if the register is not found (treated as empty/deleted)
//   - (nil, error) for any other errors
type RegisterValueReader func(registerID flow.RegisterID) (flow.RegisterValue, error)

// PayloadlessProofs generates proofs for the given register IDs using a payloadless ledger,
// fetching actual payload values from the provided valueReader.
//
// This function is used when the ledger operates in payloadless mode (trie stores payload
// hashes instead of actual values) but we need to generate complete proofs with actual
// payload values for verification.
//
// The function:
//  1. Converts register IDs to ledger keys and creates a query
//  2. Calls ledger.Prove() to get encoded proof bytes (with hash values)
//  3. Decodes, verifies consistency, replaces with actual values, and re-encodes
//
// Expected errors during normal operation:
//   - ErrPayloadHashMismatch if storehouse value doesn't match the hash in the proof
func PayloadlessProofs(
	l ledger.Ledger,
	state ledger.State,
	registerIDs []flow.RegisterID,
	valueReader RegisterValueReader,
) ([]byte, error) {
	// Convert register IDs to ledger keys
	keys := make([]ledger.Key, 0, len(registerIDs))
	for _, id := range registerIDs {
		keys = append(keys, convert.RegisterIDToLedgerKey(id))
	}

	// Create query
	query, err := ledger.NewQuery(state, keys)
	if err != nil {
		return nil, fmt.Errorf("failed to create ledger query: %w", err)
	}

	// Get encoded proof from ledger (contains hash values for payloadless trie)
	encodedProof, err := l.Prove(query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate proof from ledger: %w", err)
	}

	// Reconstruct proof with actual values from storehouse
	return ReconstructPayloadlessProof(encodedProof, valueReader)
}

// ReconstructPayloadlessProof takes encoded proof bytes from a payloadless trie,
// fetches actual payload values from the storehouse via valueReader, verifies
// consistency, and returns re-encoded proof bytes with actual values.
//
// This implements the naive decode → verify → modify → encode approach for
// proof reconstruction when the ledger service runs in a separate process.
//
// The function:
//  1. Decodes the encoded proof bytes
//  2. For each inclusion proof:
//     - Extracts the register ID from the payload's key
//     - Fetches the actual value from valueReader
//     - Verifies consistency: HashLeaf(path, actualValue) == storedHash
//     - Replaces the hash payload with actual value payload
//  3. Re-encodes the proof with actual values
//
// Expected errors during normal operation:
//   - ErrPayloadHashMismatch if storehouse value doesn't match the hash in the proof
func ReconstructPayloadlessProof(
	encodedProof []byte,
	valueReader RegisterValueReader,
) ([]byte, error) {
	// Step 1: Decode the proof
	batchProof, err := ledger.DecodeTrieBatchProof(encodedProof)
	if err != nil {
		return nil, fmt.Errorf("failed to decode payloadless proof: %w", err)
	}

	// Step 2: For each inclusion proof, verify and replace payload
	for _, proof := range batchProof.Proofs {
		if !proof.Inclusion {
			// Non-inclusion proofs don't need value replacement
			continue
		}

		// Empty payloads represent non-existent registers (used for non-inclusion proofs
		// that are represented as inclusion proofs with empty values). Skip these.
		if proof.Payload.IsEmpty() {
			continue
		}

		// Extract the key from the payload
		key, err := proof.Payload.Key()
		if err != nil {
			return nil, fmt.Errorf("failed to get key from proof payload: %w", err)
		}

		// Convert key to register ID
		registerID, err := convert.LedgerKeyToRegisterID(key)
		if err != nil {
			return nil, fmt.Errorf("failed to convert key to register ID: %w", err)
		}

		// Get the stored value from the proof.
		// In payloadless mode, this should be a 32-byte hash (HashLeaf(path, actualValue)).
		// However, if the proof comes from a non-payloadless trie (e.g., during bootstrap
		// with mixed trie modes), the value is the actual register value, not a hash.
		// In that case, we keep the proof as-is since it already contains the actual value.
		storedValue := proof.Payload.Value()
		if len(storedValue) != hash.HashLen {
			// Not a hash - this proof comes from a non-payloadless trie.
			// The proof already contains the actual value, so skip reconstruction.
			continue
		}
		storedHash := storedValue

		// Fetch the actual value from the value reader
		// TODO: making value retrieval concurrent, and benchmark if faster
		actualValue, err := valueReader(registerID)
		if err != nil {
			return nil, fmt.Errorf("failed to read register value for %s: %w", registerID, err)
		}

		// Verify consistency: HashLeaf(path, actualValue) == storedHash
		// If the hash matches, this is a payloadless proof and we replace the hash with actual value.
		// If the hash doesn't match, the stored value is an actual 32-byte value from a non-payloadless
		// trie (e.g., mixed mode during bootstrap). In this case, skip reconstruction.
		expectedHash := hash.HashLeaf(hash.Hash(proof.Path), actualValue)
		if !bytes.Equal(expectedHash[:], storedHash) {
			// Hash mismatch means this is a 32-byte actual value, not a hash.
			// Keep the proof as-is since it already contains the actual value.
			continue
		}

		// Replace the hash payload with actual value payload
		proof.Payload = ledger.NewPayload(key, actualValue)
	}

	// Step 3: Re-encode the proof with actual values
	return ledger.EncodeTrieBatchProof(batchProof), nil
}
