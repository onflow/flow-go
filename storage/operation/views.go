package operation

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// UpsertSafetyData inserts safety data into the database.
func UpsertSafetyData(w storage.Writer, chainID flow.ChainID, safetyData *hotstuff.SafetyData) error {
	return UpsertByKey(w, MakePrefix(codeSafetyData, chainID), safetyData)
}

// RetrieveSafetyData retrieves safety data from the database.
func RetrieveSafetyData(r storage.Reader, chainID flow.ChainID, safetyData *hotstuff.SafetyData) error {
	return RetrieveByKey(r, MakePrefix(codeSafetyData, chainID), safetyData)
}

// UpsertLivenessData inserts liveness data into the database.
func UpsertLivenessData(w storage.Writer, chainID flow.ChainID, livenessData *hotstuff.LivenessData) error {
	return UpsertByKey(w, MakePrefix(codeLivenessData, chainID), livenessData)
}

// RetrieveLivenessData retrieves liveness data from the database.
func RetrieveLivenessData(r storage.Reader, chainID flow.ChainID, livenessData *hotstuff.LivenessData) error {
	return RetrieveByKey(r, MakePrefix(codeLivenessData, chainID), livenessData)
}
