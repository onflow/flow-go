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

// registerTarget pairs a register ID with its corresponding ledger key. The
// register ID drives value lookup via [RegisterValueReader]; the ledger key is
// used to build the reconstructed payload. Callers that have already converted
// register IDs to keys (e.g. to derive trie paths) can stash the keys here to
// avoid a second [convert.RegisterIDToLedgerKey] call per leaf.
type registerTarget struct {
	registerID flow.RegisterID
	key        ledger.Key
}

// ProveAndReconstruct generates a reconstructed full batch proof for the
// given register IDs using a payloadless ledger and a value source. The
// returned bytes encode a *ledger.TrieBatchProof — wire-compatible with the
// full mtrie's proof format — so downstream consumers can stay
// mode-agnostic.
//
// The flow:
//  1. Convert register IDs to ledger keys and derive their paths via
//     pathfinder.KeysToPaths.
//  2. Build a path → (registerID, key) map so reconstructPayloadlessProof
//     can recover the register ID for each leaf in the (path-sorted) proof
//     and reuse the already-allocated key when building the payload.
//  3. Call ledger.Prove() to get a *PayloadlessTrieBatchProof (leaf hashes,
//     no values).
//  4. Hand the proof, map, and valueReader to reconstructPayloadlessProof
//     to verify each leaf hash and re-encode as a full *TrieBatchProof.
//
// TODO(perf): overlap step 3 with the value reads from step 4. Today the
// steps run sequentially: Prove finishes, then per-leaf value reads run
// inline inside reconstructPayloadlessProof. The two I/O phases are
// independent and can run in parallel:
//   - Phase A (parallel): l.Prove(query) and one valueReader call per
//     registerID, fanned out via an errgroup with a bounded SetLimit (the
//     reader's backend has its own concurrency limits — don't fan out
//     blindly to N).
//   - Phase B: once both complete, run a pure verify+build pass over the
//     assembled (proof, value) pairs — no I/O.
//
// The per-leaf verify work (HashLeaf + payload build) is microseconds and
// is not worth pipelining at finer grain.
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

	// Build the path → (registerID, key) map. We compute paths the same way
	// the ledger does internally, so the resulting paths match the ones
	// carried by the returned proofs. The already-allocated keys are
	// stashed here so the reconstruction step does not have to convert
	// register IDs to keys a second time.
	paths, err := pathfinder.KeysToPaths(keys, pathFinderVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to derive paths from keys: %w", err)
	}
	pathToTarget := make(map[ledger.Path]registerTarget, len(paths))
	for i, p := range paths {
		pathToTarget[p] = registerTarget{registerID: registerIDs[i], key: keys[i]}
	}

	query, err := ledger.NewQuery(state, keys)
	if err != nil {
		return nil, fmt.Errorf("failed to create ledger query: %w", err)
	}

	batchProof, err := l.Prove(query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate proof from ledger: %w", err)
	}

	return reconstructPayloadlessProof(batchProof, pathToTarget, valueReader)
}

// reconstructPayloadlessProof turns a *PayloadlessTrieBatchProof (each leaf
// carrying a leaf hash, not a value) into encoded bytes of a full
// *ledger.TrieBatchProof (each leaf carrying a *Payload). Used when a
// downstream consumer expects the wire format of the full mtrie's proofs.
//
// For each inclusion proof:
//   - The proof's `Path` is used to look up the target in `pathToTarget`.
//   - The target's register ID is passed to `valueReader` to fetch the
//     actual value.
//   - The leaf hash is verified against `HashLeaf(path, actualValue)`.
//   - The reconstructed proof's `Payload` is built from the target's
//     pre-allocated ledger key and the fetched value.
//
// Non-inclusion proofs (and inclusion proofs of empty/unallocated leaves,
// signalled by `LeafHash == nil`) carry `EmptyPayload()` on the reconstructed
// side — the full-mtrie convention for "this path has no allocated value."
//
// Expected errors during normal operation:
//   - [ErrPayloadHashMismatch] if the supplied value does not hash to the
//     proof's stored leaf hash.
func reconstructPayloadlessProof(
	batchProof *ledger.PayloadlessTrieBatchProof,
	pathToTarget map[ledger.Path]registerTarget,
	valueReader RegisterValueReader,
) ([]byte, error) {
	fullBatch := ledger.NewTrieBatchProofWithEmptyProofs(batchProof.Size())

	for i, proof := range batchProof.Proofs {
		full := fullBatch.Proofs[i]
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

		// Recover the (registerID, key) target for this path. The payloadless
		// proof does not carry the key; the caller must have provided
		// pathToTarget covering every path the underlying ledger returned a
		// proof for.
		target, ok := pathToTarget[proof.Path]
		if !ok {
			return nil, fmt.Errorf("no register target provided for path %x in proof", proof.Path[:])
		}

		// TODO(perf): see ProveAndReconstruct. Once values are pre-fetched in
		// parallel with l.Prove and passed in alongside pathToTarget, this
		// call becomes a map lookup, not a synchronous read.
		actualValue, err := valueReader(target.registerID)
		if err != nil {
			return nil, fmt.Errorf("failed to read register value for %s: %w", target.registerID, err)
		}

		// Verify the supplied value hashes to the same leaf hash carried in
		// the proof. If it does not, the storehouse is inconsistent with the
		// trie — either the wrong value, a deleted register, or a malicious
		// reader.
		expectedHash := hash.HashLeaf(hash.Hash(proof.Path), actualValue)
		if expectedHash != *proof.LeafHash {
			return nil, fmt.Errorf(
				"proof reconstruction failed for register %s: storehouse value (len=%d) does not match leaf hash in proof: %w",
				target.registerID, len(actualValue), ErrPayloadHashMismatch)
		}

		full.Payload = ledger.NewPayload(target.key, actualValue)
	}

	return ledger.EncodeTrieBatchProof(fullBatch), nil
}
