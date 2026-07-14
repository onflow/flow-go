package payloadless

import (
	"errors"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/model/flow"
)

// maxConcurrentRegisterReads bounds the fan-out of value reads issued while the
// proof is being generated. The [RegisterValueReader]'s backend (typically a
// storage-backed snapshot) has its own concurrency characteristics, so we cap
// the fan-out rather than launching one goroutine per register.
const maxConcurrentRegisterReads = 16

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
// register ID identifies the leaf for diagnostics; the ledger key is used to
// build the reconstructed payload; the value is the pre-fetched register value
// that must hash to the leaf hash carried in the proof. Callers that have
// already converted register IDs to keys (e.g. to derive trie paths) stash the
// keys here to avoid a second [convert.RegisterIDToLedgerKey] call per leaf.
type registerTarget struct {
	registerID flow.RegisterID
	key        ledger.Key
	value      flow.RegisterValue
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
//  2. Phase A (parallel I/O): fetch the proof via l.Prove(query) while, at the
//     same time, reading every register's value through valueReader. Proof
//     generation and value reads are independent I/O phases keyed off the same
//     inputs, so they overlap. The value reads fan out via an errgroup with a
//     bounded SetLimit ([maxConcurrentRegisterReads]) because the reader's
//     backend has its own concurrency limits — we don't fan out blindly to N.
//  3. Build a path → (registerID, key, value) map so reconstructPayloadlessProof
//     can recover, for each leaf in the (path-sorted) proof, the register ID
//     (for diagnostics), the already-allocated key (for the payload), and the
//     pre-fetched value (to verify against the leaf hash).
//  4. Phase B (pure, no I/O): hand the proof and the map to
//     reconstructPayloadlessProof, which verifies each leaf hash and re-encodes
//     the batch as a full *TrieBatchProof.
//
// Because Phase A reads every queried register up front — before it is known
// which paths are inclusion proofs — valueReader is invoked for every register
// ID, including those that turn out to be non-inclusion or empty leaves. The
// values of such leaves are simply discarded during Phase B.
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

	// Compute paths the same way the ledger does internally, so the resulting
	// paths match the ones carried by the returned proofs.
	paths, err := pathfinder.KeysToPaths(keys, pathFinderVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to derive paths from keys: %w", err)
	}

	query, err := ledger.NewQuery(state, keys)
	if err != nil {
		return nil, fmt.Errorf("failed to create ledger query: %w", err)
	}

	// Phase A: overlap proof generation with the per-register value reads.
	// Both are independent I/O phases; running them concurrently hides the
	// value-read latency behind the (typically slower) proof generation.
	var batchProof *ledger.PayloadlessTrieBatchProof
	values := make([]flow.RegisterValue, len(registerIDs))

	var g errgroup.Group
	g.Go(func() error {
		proof, proveErr := l.Prove(query)
		if proveErr != nil {
			return fmt.Errorf("failed to generate proof from ledger: %w", proveErr)
		}
		batchProof = proof
		return nil
	})
	g.Go(func() error {
		// Bounded fan-out of value reads. Each goroutine writes a distinct
		// index of values, so no synchronization is needed around the slice.
		var reads errgroup.Group
		reads.SetLimit(maxConcurrentRegisterReads)
		for i := range registerIDs {
			reads.Go(func() error {
				v, readErr := valueReader(registerIDs[i])
				if readErr != nil {
					return fmt.Errorf("failed to read register value for %s: %w", registerIDs[i], readErr)
				}
				values[i] = v
				return nil
			})
		}
		return reads.Wait()
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Build the path → (registerID, key, value) map now that all values are
	// available. The already-allocated keys and pre-fetched values are stashed
	// here so the reconstruction step does not have to convert register IDs to
	// keys a second time or perform any further I/O.
	pathToTarget := make(map[ledger.Path]registerTarget, len(paths))
	for i, p := range paths {
		pathToTarget[p] = registerTarget{registerID: registerIDs[i], key: keys[i], value: values[i]}
	}

	// Phase B: pure verify + build, no I/O.
	return reconstructPayloadlessProof(batchProof, pathToTarget)
}

// reconstructPayloadlessProof turns a *PayloadlessTrieBatchProof (each leaf
// carrying a leaf hash, not a value) into encoded bytes of a full
// *ledger.TrieBatchProof (each leaf carrying a *Payload). Used when a
// downstream consumer expects the wire format of the full mtrie's proofs.
//
// This is a pure, CPU-only pass: all register values were pre-fetched by the
// caller and are carried in `pathToTarget`. It performs no I/O.
//
// For each inclusion proof:
//   - The proof's `Path` is used to look up the target in `pathToTarget`.
//   - The leaf hash is verified against `HashLeaf(path, target.value)`.
//   - The reconstructed proof's `Payload` is built from the target's
//     pre-allocated ledger key and pre-fetched value.
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

		// Recover the (registerID, key, value) target for this path. The
		// payloadless proof does not carry the key; the caller must have
		// provided pathToTarget covering every path the underlying ledger
		// returned a proof for.
		target, ok := pathToTarget[proof.Path]
		if !ok {
			return nil, fmt.Errorf("no register target provided for path %x in proof", proof.Path[:])
		}

		// Verify the pre-fetched value hashes to the same leaf hash carried in
		// the proof. If it does not, the storehouse is inconsistent with the
		// trie — either the wrong value, a deleted register, or a malicious
		// reader.
		expectedHash := hash.HashLeaf(hash.Hash(proof.Path), target.value)
		if expectedHash != *proof.LeafHash {
			return nil, fmt.Errorf(
				"proof reconstruction failed for register %s: storehouse value (len=%d) does not match leaf hash in proof: %w",
				target.registerID, len(target.value), ErrPayloadHashMismatch)
		}

		full.Payload = ledger.NewPayload(target.key, target.value)
	}

	return ledger.EncodeTrieBatchProof(fullBatch), nil
}
