package operation

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// UpsertSafetyData inserts or updates the given safety data for this node.
// Intended for consensus participants only (consensus and collector nodes).
// Here, `chainID` specifies which consensus instance specifically the node participates in.
//
// CAUTION:
//   - The caller must acquire the lock [storage.LockInsertSafetyData] and hold it until the database write has been committed.
//   - OVERWRITES existing data (potential for data corruption):
//     The lock proof serves as a reminder that the CALLER is RESPONSIBLE to ensure that the transition from one state to the next
//     is protocol compliant. By holding the lock, the caller has the ability to read the current value and update it atomically.
//
// No error returns are expected during normal operation.
func UpsertSafetyData(lctx lockctx.Proof, rw storage.ReaderBatchWriter, chainID flow.ChainID, safetyData *hotstuff.SafetyData) error {
	if !lctx.HoldsLock(storage.LockInsertSafetyData) {
		return fmt.Errorf("missing required lock: storage.LockInsertSafetyData")
	}

	return UpsertByKey(rw.Writer(), MakePrefix(codeSafetyData, chainID), safetyData)
}

// RetrieveSafetyData retrieves the safety data for this node.
// Intended for consensus participants only (consensus and collector nodes).
// Here, `chainID` specifies which consensus instance specifically the node participates in.
// For consensus and collector nodes, this value should always exist (for the correct chainID).
// No error returns are expected during normal operation.
func RetrieveSafetyData(r storage.Reader, chainID flow.ChainID, safetyData *hotstuff.SafetyData) error {
	return RetrieveByKey(r, MakePrefix(codeSafetyData, chainID), safetyData)
}

// UpsertLivenessData inserts or updates the given liveness data for this node.
// Intended for consensus participants only (consensus and collector nodes).
// Here, `chainID` specifies which consensus instance specifically the node participates in.
//
// CAUTION:
//   - The caller must acquire the lock [storage.LockInsertLivenessData] and hold it until the database write has been committed.
//   - OVERWRITES existing data (potential for data corruption):
//     The lock proof serves as a reminder that the CALLER is RESPONSIBLE to ensure that the transition from one state to the next
//     is protocol compliant. By holding the lock, the caller has the ability to read the current value and update it atomically.
//
// No error returns are expected during normal operation.
func UpsertLivenessData(lctx lockctx.Proof, rw storage.ReaderBatchWriter, chainID flow.ChainID, livenessData *hotstuff.LivenessData) error {
	if !lctx.HoldsLock(storage.LockInsertLivenessData) {
		return fmt.Errorf("missing required lock: storage.LockInsertLivenessData")
	}

	return UpsertByKey(rw.Writer(), MakePrefix(codeLivenessData, chainID), livenessData)
}

// RetrieveLivenessData retrieves the liveness data for this node.
// Intended for consensus participants only (consensus and collector nodes).
// Here, `chainID` specifies which consensus instance specifically the node participates in.
// For consensus and collector nodes, this value should always exist (for the correct chainID).
// No error returns are expected during normal operation.
func RetrieveLivenessData(r storage.Reader, chainID flow.ChainID, livenessData *hotstuff.LivenessData) error {
	return RetrieveByKey(r, MakePrefix(codeLivenessData, chainID), livenessData)
}
