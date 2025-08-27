package operation

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertSeal inserts a seal into the database.
//
// CAUTION: The caller must ensure sealID is a collision-resistant hash of the provided seal!
// This method silently overrides existing data, which is safe only if for the same key, we
// always write the same value.
//
// No other errors are expected during normal operation.
func InsertSeal(w storage.Writer, sealID flow.Identifier, seal *flow.Seal) error {
	return UpsertByKey(w, MakePrefix(codeSeal, sealID), seal)
}

func RetrieveSeal(r storage.Reader, sealID flow.Identifier, seal *flow.Seal) error {
	return RetrieveByKey(r, MakePrefix(codeSeal, sealID), seal)
}

// IndexPayloadSeals indexes the given Seal IDs by the block ID.
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
// No other errors are expected during normal operation.
func IndexPayloadSeals(lctx lockctx.Proof, w storage.Writer, blockID flow.Identifier, sealIDs []flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertBlock) {
		return fmt.Errorf("cannot index seal for blockID %v without holding lock %s",
			blockID, storage.LockInsertBlock)
	}
	return UpsertByKey(w, MakePrefix(codePayloadSeals, blockID), sealIDs)
}

func LookupPayloadSeals(r storage.Reader, blockID flow.Identifier, sealIDs *[]flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codePayloadSeals, blockID), sealIDs)
}

// IndexPayloadReceipts indexes the given Execution Receipt IDs by the block ID.
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
// No other errors are expected during normal operation.
func IndexPayloadReceipts(lctx lockctx.Proof, w storage.Writer, blockID flow.Identifier, receiptIDs []flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertBlock) {
		return fmt.Errorf("cannot index seal for blockID %v without holding lock %s",
			blockID, storage.LockInsertBlock)
	}
	return UpsertByKey(w, MakePrefix(codePayloadReceipts, blockID), receiptIDs)
}

// IndexPayloadResults indexes the given Execution Result IDs by the block ID.
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
// No other errors are expected during normal operation.
func IndexPayloadResults(lctx lockctx.Proof, w storage.Writer, blockID flow.Identifier, resultIDs []flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertBlock) {
		return fmt.Errorf("cannot index seal for blockID %v without holding lock %s",
			blockID, storage.LockInsertBlock)
	}
	return UpsertByKey(w, MakePrefix(codePayloadResults, blockID), resultIDs)
}

// IndexPayloadProtocolStateID indexes the given Protocol State ID by the block ID.
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
// No other errors are expected during normal operation.
func IndexPayloadProtocolStateID(lctx lockctx.Proof, w storage.Writer, blockID flow.Identifier, stateID flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertBlock) {
		return fmt.Errorf("cannot index seal for blockID %v without holding lock %s",
			blockID, storage.LockInsertBlock)
	}
	return UpsertByKey(w, MakePrefix(codePayloadProtocolStateID, blockID), stateID)
}

func LookupPayloadProtocolStateID(r storage.Reader, blockID flow.Identifier, stateID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codePayloadProtocolStateID, blockID), stateID)
}

func LookupPayloadReceipts(r storage.Reader, blockID flow.Identifier, receiptIDs *[]flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codePayloadReceipts, blockID), receiptIDs)
}

func LookupPayloadResults(r storage.Reader, blockID flow.Identifier, resultIDs *[]flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codePayloadResults, blockID), resultIDs)
}

// IndexLatestSealAtBlock persists the highest seal that was included in the fork up to (and including) blockID.
// In most cases, it is the highest seal included in this block's payload. However, if there are no
// seals in this block, sealID should reference the highest seal in blockID's ancestor.
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
// No other errors are expected during normal operation.
func IndexLatestSealAtBlock(lctx lockctx.Proof, w storage.Writer, blockID flow.Identifier, sealID flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertBlock) {
		return fmt.Errorf("missing required lock: %s", storage.LockInsertBlock)
	}
	return UpsertByKey(w, MakePrefix(codeBlockIDToLatestSealID, blockID), sealID)
}

// LookupLatestSealAtBlock finds the highest seal that was included in the fork up to (and including) blockID.
// In most cases, it is the highest seal included in this block's payload. However, if there are no
// seals in this block, sealID should reference the highest seal in blockID's ancestor.
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
// No other errors are expected during normal operation.
func IndexFinalizedSealByBlockID(w storage.Writer, sealedBlockID flow.Identifier, sealID flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codeBlockIDToFinalizedSeal, sealedBlockID), sealID)
}

// LookupBySealedBlockID finds the seal for the given sealed block ID.
func LookupBySealedBlockID(r storage.Reader, sealedBlockID flow.Identifier, sealID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeBlockIDToFinalizedSeal, sealedBlockID), &sealID)
}
