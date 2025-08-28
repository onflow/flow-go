package operation

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertSeal inserts a [flow.Seal] into the database, keyed by its ID.
//
// CAUTION: The caller must ensure sealID is a collision-resistant hash of the provided seal!
// This method silently overrides existing data, which is safe only if for the same key, we
// always write the same value.
//
// No errors are expected during normal operation.
func InsertSeal(w storage.Writer, sealID flow.Identifier, seal *flow.Seal) error {
	return UpsertByKey(w, MakePrefix(codeSeal, sealID), seal)
}

// RetrieveSeal retrieves the [flow.Seal] by its ID.
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if no seal with the specified `sealID` is known.
func RetrieveSeal(r storage.Reader, sealID flow.Identifier, seal *flow.Seal) error {
	return RetrieveByKey(r, MakePrefix(codeSeal, sealID), seal)
}

// IndexPayloadSeals indexes the list of Seals that were included in the specified block by the block ID.
// It produces a mapping from block ID to the list of seals contained in the block's payload.
// The seals are represented by their respective IDs.
//
// CAUTION:
//   - The caller must acquire the [storage.LockInsertBlock] and hold it until the database write has been committed.
//   - OVERWRITES existing data (potential for data corruption):
//     This method silently overrides existing data without any sanity checks whether data for the same key already exits.
//     Note that the Flow protocol mandates that for a previously persisted key, the data is never changed to a different
//     value. Changing data could cause the node to publish inconsistent data and to be slashed, or the protocol to be
//     compromised as a whole. This method does not contain any safeguards to prevent such data corruption. The lock proof
//     serves as a reminder that the CALLER is responsible to ensure that the DEDUPLICATION CHECK is done elsewhere
//     ATOMICALLY with this write operation.
//
// No errors are expected during normal operation.
func IndexPayloadSeals(lctx lockctx.Proof, w storage.Writer, blockID flow.Identifier, sealIDs []flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertBlock) {
		return fmt.Errorf("cannot index seal for blockID %v without holding lock %s",
			blockID, storage.LockInsertBlock)
	}
	return UpsertByKey(w, MakePrefix(codePayloadSeals, blockID), sealIDs)
}

// LookupPayloadSeals retrieves the list of Seals that were included in the payload
// of the specified block. For every known block (at or above the root block height),
// this index should be populated.
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if `blockID` does not refer to a known block
func LookupPayloadSeals(r storage.Reader, blockID flow.Identifier, sealIDs *[]flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codePayloadSeals, blockID), sealIDs)
}

// IndexPayloadReceipts indexes the list of Execution Receipts that were included in the specified block by the block ID.
// It produces a mapping from block ID to the list of Receipts contained in the block's payload.
// Execution Receipts are represented by their respective IDs.
//
// CAUTION:
//   - The caller must acquire the [storage.LockInsertBlock] and hold it until the database write has been committed.
//   - OVERWRITES existing data (potential for data corruption):
//     This method silently overrides existing data without any sanity checks whether data for the same key already exits.
//     Note that the Flow protocol mandates that for a previously persisted key, the data is never changed to a different
//     value. Changing data could cause the node to publish inconsistent data and to be slashed, or the protocol to be
//     compromised as a whole. This method does not contain any safeguards to prevent such data corruption. The lock proof
//     serves as a reminder that the CALLER is responsible to ensure that the DEDUPLICATION CHECK is done elsewhere
//     ATOMICALLY with this write operation.
//
// No errors are expected during normal operation.
func IndexPayloadReceipts(lctx lockctx.Proof, w storage.Writer, blockID flow.Identifier, receiptIDs []flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertBlock) {
		return fmt.Errorf("cannot index seal for blockID %v without holding lock %s",
			blockID, storage.LockInsertBlock)
	}
	return UpsertByKey(w, MakePrefix(codePayloadReceipts, blockID), receiptIDs)
}

// IndexPayloadResults indexes the list of Execution Results that were included in the specified block by the block ID.
// It produces a mapping from block ID to the list of Results contained in the block's payload.
// Execution Results are represented by their respective IDs.
//
// CAUTION:
//   - The caller must acquire the [storage.LockInsertBlock] and hold it until the database write has been committed.
//   - OVERWRITES existing data (potential for data corruption):
//     This method silently overrides existing data without any sanity checks whether data for the same key already exits.
//     Note that the Flow protocol mandates that for a previously persisted key, the data is never changed to a different
//     value. Changing data could cause the node to publish inconsistent data and to be slashed, or the protocol to be
//     compromised as a whole. This method does not contain any safeguards to prevent such data corruption. The lock proof
//     serves as a reminder that the CALLER is responsible to ensure that the DEDUPLICATION CHECK is done elsewhere
//     ATOMICALLY with this write operation.
//
// No errors are expected during normal operation.
func IndexPayloadResults(lctx lockctx.Proof, w storage.Writer, blockID flow.Identifier, resultIDs []flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertBlock) {
		return fmt.Errorf("cannot index seal for blockID %v without holding lock %s",
			blockID, storage.LockInsertBlock)
	}
	return UpsertByKey(w, MakePrefix(codePayloadResults, blockID), resultIDs)
}

// IndexPayloadProtocolStateID indexes the given Protocol State ID by the block ID.
// The Protocol State ID represents the configuration, which the block proposes to become active *after* the
// block's certification. Every block states the ID of the Protocol State it proposes as part of the payload.
//
// CAUTION:
//   - The caller must acquire the [storage.LockInsertBlock] and hold it until the database write has been committed.
//   - OVERWRITES existing data (potential for data corruption):
//     This method silently overrides existing data without any sanity checks whether data for the same key already exits.
//     Note that the Flow protocol mandates that for a previously persisted key, the data is never changed to a different
//     value. Changing data could cause the node to publish inconsistent data and to be slashed, or the protocol to be
//     compromised as a whole. This method does not contain any safeguards to prevent such data corruption. The lock proof
//     serves as a reminder that the CALLER is responsible to ensure that the DEDUPLICATION CHECK is done elsewhere
//     ATOMICALLY with this write operation.
//
// No errors are expected during normal operation.
func IndexPayloadProtocolStateID(lctx lockctx.Proof, w storage.Writer, blockID flow.Identifier, stateID flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertBlock) {
		return fmt.Errorf("cannot index seal for blockID %v without holding lock %s",
			blockID, storage.LockInsertBlock)
	}
	return UpsertByKey(w, MakePrefix(codePayloadProtocolStateID, blockID), stateID)
}

// LookupPayloadProtocolStateID retrieves the Protocol State ID for the specified block.
// The Protocol State ID represents the configuration, which the block proposes to become active *after*
// the block's certification. For every known block (at or above the root block height), the protocol
// state at the end of the block should be specified in the payload, and hence be indexed.
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if `blockID` does not refer to a known block
func LookupPayloadProtocolStateID(r storage.Reader, blockID flow.Identifier, stateID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codePayloadProtocolStateID, blockID), stateID)
}

// LookupPayloadReceipts retrieves the list of Execution Receipts that were included in the payload of the
// specified block. For every known block (at or above the root block height), this index should be populated.
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if `blockID` does not refer to a known block.
func LookupPayloadReceipts(r storage.Reader, blockID flow.Identifier, receiptIDs *[]flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codePayloadReceipts, blockID), receiptIDs)
}

// LookupPayloadResults retrieves the list of Execution Results that were included in the payload of the
// specified block. For every known block (at or above the root block height), this index should be populated.
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if `blockID` does not refer to a known block
func LookupPayloadResults(r storage.Reader, blockID flow.Identifier, resultIDs *[]flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codePayloadResults, blockID), resultIDs)
}

// IndexLatestSealAtBlock persists the highest seal that was included in the fork with head blockID.
// Frequently, the highest seal included in this block's payload. However, if there are no seals in
// this block, sealID should reference the highest seal in blockID's ancestors.
//
// CAUTION:
//   - The caller must acquire the [storage.LockInsertBlock] and hold it until the database write has been committed.
//   - OVERWRITES existing data (potential for data corruption):
//     This method silently overrides existing data without any sanity checks whether data for the same key already exits.
//     Note that the Flow protocol mandates that for a previously persisted key, the data is never changed to a different
//     value. Changing data could cause the node to publish inconsistent data and to be slashed, or the protocol to be
//     compromised as a whole. This method does not contain any safeguards to prevent such data corruption. The lock proof
//     serves as a reminder that the CALLER is responsible to ensure that the DEDUPLICATION CHECK is done elsewhere
//     ATOMICALLY with this write operation.
//
// No errors are expected during normal operation.
func IndexLatestSealAtBlock(lctx lockctx.Proof, w storage.Writer, blockID flow.Identifier, sealID flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertBlock) {
		return fmt.Errorf("missing required lock: %s", storage.LockInsertBlock)
	}
	return UpsertByKey(w, MakePrefix(codeBlockIDToLatestSealID, blockID), sealID)
}

// LookupLatestSealAtBlock finds the highest seal that was included in the fork up to (and including) blockID.
// Frequently, the highest seal included in this block's payload. However, if there are no seals in
// this block, sealID should reference the highest seal in blockID's ancestors.
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if the specified block is unknown
func LookupLatestSealAtBlock(r storage.Reader, blockID flow.Identifier, sealID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeBlockIDToLatestSealID, blockID), &sealID)
}

// IndexFinalizedSealByBlockID indexes the _finalized_ seal by the sealed block ID.
// Example: A <- B <- C(SealA)
// when block C is finalized, we create the index `A.ID->SealA.ID`
//
// CAUTION:
//   - The caller must acquire the [storage.LockFinalizeBlock] and hold it until the database write has been committed.
//     TODO: add lock proof as input and check for holding the lock in the implementation
//   - OVERWRITES existing data (potential for data corruption):
//     This method silently overrides existing data without any sanity checks whether data for the same key already exits.
//     Note that the Flow protocol mandates that for a previously persisted key, the data is never changed to a different
//     value. Changing data could cause the node to publish inconsistent data and to be slashed, or the protocol to be
//     compromised as a whole. This method does not contain any safeguards to prevent such data corruption. The lock proof
//     serves as a reminder that the CALLER is responsible to ensure that the DEDUPLICATION CHECK is done elsewhere
//     ATOMICALLY with this write operation.
//
// No errors are expected during normal operation.
func IndexFinalizedSealByBlockID(w storage.Writer, sealedBlockID flow.Identifier, sealID flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codeBlockIDToFinalizedSeal, sealedBlockID), sealID)
}

// LookupBySealedBlockID returns the finalized seal for the specified FINALIZED block ID.
// In order for a block to have a seal in a finalized block, it must itself be finalized. Hence,
// this function only works for finalized blocks. However, note that there might be finalized
// for which no seal exits (or the block containing the seal might not yet be finalized).
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if no seal for the specified block is known.
func LookupBySealedBlockID(r storage.Reader, blockID flow.Identifier, sealID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeBlockIDToFinalizedSeal, blockID), &sealID)
}
