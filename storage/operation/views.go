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
// No errors are expected during normal operation.
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
// No errors are expected during normal operation.
func RetrieveSafetyData(r storage.Reader, chainID flow.ChainID, safetyData *hotstuff.SafetyData) error {
	return RetrieveByKey(r, MakePrefix(codeSafetyData, chainID), safetyData)
}

// UpsertLivenessData inserts or updates the given liveness data for this node.
// Intended for consensus participants only (consensus and collector nodes).
// Here, `chainID` specifies which consensus instance specifically the node participates in.
//
// No errors are expected during normal operation.
func UpsertLivenessData(lctx lockctx.Proof, rw storage.ReaderBatchWriter, chainID flow.ChainID, livenessData *hotstuff.LivenessData) error {
	if !lctx.HoldsLock(storage.LockInsertLivenessData) {
		return fmt.Errorf("missing required lock: storage.LockInsertLivenessData")
	}

	return UpsertByKey(rw.Writer(), MakePrefix(codeLivenessData, chainID), livenessData)
}

// RetrieveSafetyData retrieves the safety data for this node.
// Intended for consensus participants only (consensus and collector nodes).
// Here, `chainID` specifies which consensus instance specifically the node participates in.
// For consensus and collector nodes, this value should always exist (for the correct chainID).
// No errors are expected during normal operation.
func RetrieveLivenessData(r storage.Reader, chainID flow.ChainID, livenessData *hotstuff.LivenessData) error {
	return RetrieveByKey(r, MakePrefix(codeLivenessData, chainID), livenessData)
}
