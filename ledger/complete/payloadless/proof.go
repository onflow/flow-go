package payloadless

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/model/flow"
)

// ErrPayloadHashMismatch is returned when the value supplied by valueReader
// does not hash to the leaf hash stored in the payloadless proof.
var ErrPayloadHashMismatch = errors.New("payload hash mismatch: storehouse value inconsistent with trie")

// RegisterValueReader is a function type that reads register values.
// It returns:
//   - (value, nil) if the register is found
//   - (nil, nil) if the register is not found (treated as empty/deleted)
//   - (nil, error) for any other errors
type RegisterValueReader func(registerID flow.RegisterID) (flow.RegisterValue, error)

// ProveAndReconstruct generates a reconstructed full batch proof for the
// given register IDs using a payloadless ledger and a value source. The
// returned bytes encode a *ledger.TrieBatchProof — wire-compatible with the
// full mtrie's proof format — so downstream consumers can stay
// mode-agnostic.
//
// The flow:
//  1. Convert register IDs to ledger keys and derive their paths via
//     pathfinder.KeysToPaths.
//  2. Build a path → registerID map so ReconstructPayloadlessProof can
//     recover the register ID for each leaf in the (path-sorted) proof.
//  3. Call ledger.Prove() to get a *PayloadlessTrieBatchProof (leaf hashes,
//     no values).
//  4. Hand the proof, map, and valueReader to ReconstructPayloadlessProof
//     to verify each leaf hash and re-encode as a full *TrieBatchProof.
//
// Expected errors during normal operation:
//   - [ErrPayloadHashMismatch] if storehouse value doesn't match the leaf
//     hash carried in the proof for some path.
func ProveAndReconstruct(
	l ledger.PayloadlessLedger,
	state ledger.State,
	registerIDs []flow.RegisterID,
	valueReader RegisterValueReader,
	pathFinderVersion uint8,
) ([]byte, error) {
	// Convert register IDs to ledger keys.
	keys := make([]ledger.Key, 0, len(registerIDs))
	for _, id := range registerIDs {
		keys = append(keys, convert.RegisterIDToLedgerKey(id))
	}

	// Build the path → registerID map. We compute paths the same way the
	// ledger does internally, so the resulting paths match the ones
	// carried by the returned proofs.
	paths, err := pathfinder.KeysToPaths(keys, pathFinderVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to derive paths from keys: %w", err)
	}
	pathToRegisterID := make(map[ledger.Path]flow.RegisterID, len(paths))
	for i, p := range paths {
		pathToRegisterID[p] = registerIDs[i]
	}

	query, err := ledger.NewQuery(state, keys)
	if err != nil {
		return nil, fmt.Errorf("failed to create ledger query: %w", err)
	}

	batchProof, err := l.Prove(query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate proof from ledger: %w", err)
	}

	return ReconstructPayloadlessProof(batchProof, pathToRegisterID, valueReader)
}

// ReconstructPayloadlessProof turns a *PayloadlessTrieBatchProof (each leaf
// carrying a leaf hash, not a value) into encoded bytes of a full
// *ledger.TrieBatchProof (each leaf carrying a *Payload). Used when a
// downstream consumer expects the wire format of the full mtrie's proofs.
//
// For each inclusion proof:
//   - The proof's `Path` is used to look up the register ID in
//     `pathToRegisterID`.
//   - The register ID is passed to `valueReader` to fetch the actual value.
//   - The leaf hash is verified against `HashLeaf(path, actualValue)`.
//   - The reconstructed proof's `Payload` is built as
//     `NewPayload(RegisterIDToLedgerKey(registerID), actualValue)`.
//
// Non-inclusion proofs (and inclusion proofs of empty/unallocated leaves,
// signalled by `LeafHash == nil`) carry `EmptyPayload()` on the reconstructed
// side — the full-mtrie convention for "this path has no allocated value."
//
// The caller already has the register IDs (it requested the proof for them),
// so the map takes register IDs directly rather than ledger keys to avoid an
// unnecessary `LedgerKeyToRegisterID` round-trip per proof.
//
// Expected errors during normal operation:
//   - [ErrPayloadHashMismatch] if the supplied value does not hash to the
//     proof's stored leaf hash.
func ReconstructPayloadlessProof(
	batchProof *ledger.PayloadlessTrieBatchProof,
	pathToRegisterID map[ledger.Path]flow.RegisterID,
	valueReader RegisterValueReader,
) ([]byte, error) {
	fullBatch := ledger.NewTrieBatchProofWithEmptyProofs(batchProof.Size())

	for i, proof := range batchProof.Proofs {
		full := fullBatch.Proofs[i]
		// Structural fields are carried over verbatim. They are byte-equivalent
		// between the two proof types by construction (see Spec 001).
		full.Path = proof.Path
		full.Interims = proof.Interims
		full.Inclusion = proof.Inclusion
		full.Flags = proof.Flags
		full.Steps = proof.Steps

		// Non-inclusion proofs and inclusion proofs of empty leaves both map
		// to a full proof carrying an empty payload.
		if !proof.Inclusion || proof.LeafHash == nil {
			full.Payload = ledger.EmptyPayload()
			continue
		}

		// Recover the register ID for this path. The payloadless proof does
		// not carry the key; the caller must have provided pathToRegisterID
		// covering every path the underlying ledger returned a proof for.
		registerID, ok := pathToRegisterID[proof.Path]
		if !ok {
			return nil, fmt.Errorf("no register ID provided for path %x in proof", proof.Path[:])
		}

		// TODO: parallelize value reads if the round-trip latency matters
		actualValue, err := valueReader(registerID)
		if err != nil {
			return nil, fmt.Errorf("failed to read register value for %s: %w", registerID, err)
		}

		// Verify the supplied value hashes to the same leaf hash carried in
		// the proof. If it does not, the storehouse is inconsistent with the
		// trie — either the wrong value, a deleted register, or a malicious
		// reader.
		expectedHash := hash.HashLeaf(hash.Hash(proof.Path), actualValue)
		if expectedHash != *proof.LeafHash {
			return nil, fmt.Errorf(
				"proof reconstruction failed for register %s: storehouse value (len=%d) does not match leaf hash in proof: %w",
				registerID, len(actualValue), ErrPayloadHashMismatch)
		}

		full.Payload = ledger.NewPayload(convert.RegisterIDToLedgerKey(registerID), actualValue)
	}

	return ledger.EncodeTrieBatchProof(fullBatch), nil
}
